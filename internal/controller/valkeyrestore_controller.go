/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

const (
	finalizerValkeyRestore = "cache.keiailab.io/valkeyrestore-finalizer"
)

// ValkeyRestoreReconciler — Source.PVC 에서 RDB 를 cluster 로 복원 (ADR-0015).
//
// 본 commit 의 범위 (Track A AI-002):
//   - Source: PVC 만 (TargetRef = 외부 저장 은 별개 commit, ADR-0016 통합).
//   - 대상: ClusterRef.Kind="Valkey" + Mode=Standalone (replicas=1) 만.
//     Replication / ValkeyCluster 는 ReadOnlyMany source PVC 가 필요 →
//     별개 commit.
//
// Phase 전이 ("" → Pending → Mounting → Restoring → Verifying → Completed):
//   - "" → Pending: status 초기화 + Conditions["Ready"]=False/Pending.
//   - Pending → Mounting: ClusterRef + Source.PVC 검증 + Standalone 검증.
//   - Mounting → Restoring: paused annotation set + STS 에 init container
//     inject + Update.
//   - Restoring → Verifying: STS 의 모든 pod Ready 확인 (rolling 완료).
//   - Verifying → Completed: STS 원복 (init container 제거) + paused 제거.
//
// Finalizer (cache.keiailab.io/valkeyrestore-finalizer): CR 삭제 시 STS
// 원복 + paused 정리. 정상 Completed 흐름에서는 이미 정리되어 no-op.
type ValkeyRestoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyrestores/finalizers,verbs=update

func (r *ValkeyRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	rest := &cachev1alpha1.ValkeyRestore{}
	if err := r.Get(ctx, req.NamespacedName, rest); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Finalizer / deletion.
	if !rest.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, rest)
	}
	if !controllerutil.ContainsFinalizer(rest, finalizerValkeyRestore) {
		controllerutil.AddFinalizer(rest, finalizerValkeyRestore)
		if err := r.Update(ctx, rest); err != nil {
			return ctrl.Result{}, err
		}
		// finalizer 추가는 객체 ResourceVersion 갱신 → 다음 reconcile 에서 phase 진입.
		return ctrl.Result{Requeue: true}, nil
	}

	if rest.IsTerminal() {
		return ctrl.Result{}, nil
	}

	// Phase 전이.
	switch rest.Status.Phase {
	case "":
		return r.transitionToPending(ctx, rest)
	case cachev1alpha1.RestorePhasePending:
		return r.handlePending(ctx, rest)
	case cachev1alpha1.RestorePhaseMounting:
		return r.handleMounting(ctx, rest)
	case cachev1alpha1.RestorePhaseRestoring:
		return r.handleRestoring(ctx, rest)
	case cachev1alpha1.RestorePhaseVerifying:
		return r.handleVerifying(ctx, rest)
	}

	logger.V(1).Info("unknown phase — no-op", "phase", rest.Status.Phase)
	return ctrl.Result{}, nil
}

// transitionToPending — "" → Pending. 단순 status 초기화.
func (r *ValkeyRestoreReconciler) transitionToPending(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	now := metav1.Now()
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	rest.Status.StartedAt = &now
	rest.Status.ObservedGeneration = rest.Generation
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Pending",
		Message:            "validating ClusterRef + Source",
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

// handlePending — ClusterRef + Source 검증 → Mounting.
//
// 본 commit 의 제한: ClusterRef.Kind=="Valkey" + Source.PVC + Mode=Standalone.
// 위 외 케이스는 RestorePhaseFailed.
func (r *ValkeyRestoreReconciler) handlePending(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	if rest.Spec.ClusterRef.Kind != "Valkey" {
		return r.markFailed(ctx, rest, "UnsupportedClusterKind",
			fmt.Sprintf("kind=%s — only Valkey (Standalone) supported in this version",
				rest.Spec.ClusterRef.Kind))
	}
	if rest.Spec.Source.PVC == nil {
		return r.markFailed(ctx, rest, "UnsupportedSource",
			"only Source.PVC supported in this version (TargetRef pending)")
	}
	if rest.Spec.Source.PVC.Name == "" {
		return r.markFailed(ctx, rest, "MissingSourcePVCName",
			"spec.source.pvc.name required")
	}

	// 대상 Valkey 가 Standalone 인지 확인.
	v := &cachev1alpha1.Valkey{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, v); err != nil {
		if errors.IsNotFound(err) {
			return r.markFailed(ctx, rest, "TargetNotFound",
				fmt.Sprintf("Valkey/%s/%s not found", rest.Namespace, rest.Spec.ClusterRef.Name))
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if v.Spec.Mode != "" && v.Spec.Mode != cachev1alpha1.ModeStandalone {
		return r.markFailed(ctx, rest, "UnsupportedMode",
			fmt.Sprintf("Valkey.Spec.Mode=%s — only Standalone supported in this version", v.Spec.Mode))
	}
	if v.Spec.Replicas > 1 {
		return r.markFailed(ctx, rest, "UnsupportedReplicas",
			fmt.Sprintf("Valkey replicas=%d — only single-pod Standalone supported", v.Spec.Replicas))
	}

	// → Mounting.
	rest.Status.Phase = cachev1alpha1.RestorePhaseMounting
	rest.Status.Message = "ClusterRef + Source validated — entering Mounting"
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Mounting",
		Message:            rest.Status.Message,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

// handleMounting — Source PVC 존재 확인 + paused annotation set → Restoring.
func (r *ValkeyRestoreReconciler) handleMounting(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	// PVC 존재 확인.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.Source.PVC.Name,
		Namespace: rest.Namespace,
	}, pvc); err != nil {
		if errors.IsNotFound(err) {
			return r.markFailed(ctx, rest, "SourcePVCNotFound",
				fmt.Sprintf("PVC/%s/%s not found", rest.Namespace, rest.Spec.Source.PVC.Name))
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 대상 Valkey 에 paused annotation set.
	v := &cachev1alpha1.Valkey{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, v); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if v.Annotations == nil {
		v.Annotations = map[string]string{}
	}
	if v.Annotations[PausedAnnotation] != "true" {
		v.Annotations[PausedAnnotation] = "true"
		if err := r.Update(ctx, v); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	// → Restoring.
	rest.Status.Phase = cachev1alpha1.RestorePhaseRestoring
	rest.Status.Message = fmt.Sprintf("Source PVC %s exists, target paused — STS patch pending",
		rest.Spec.Source.PVC.Name)
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Restoring",
		Message:            rest.Status.Message,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

// handleRestoring — STS 에 init container inject + 모든 pod Ready 대기 → Verifying.
func (r *ValkeyRestoreReconciler) handleRestoring(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// STS 이름 — Valkey controller 가 spec.clusterRef.name 그대로 STS 이름 사용.
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, sts); err != nil {
		if errors.IsNotFound(err) {
			return r.markFailed(ctx, rest, "STSNotFound",
				fmt.Sprintf("StatefulSet/%s/%s not found", rest.Namespace, rest.Spec.ClusterRef.Name))
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	srcPath := rest.Spec.Source.PVC.Path
	if srcPath == "" {
		srcPath = "dump.rdb"
	}

	// 멱등 inject.
	hadRestoreContainer := false
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		if c.Name == resources.RestoreInitContainerName {
			hadRestoreContainer = true
			break
		}
	}
	resources.InjectRestoreIntoPodSpec(&sts.Spec.Template.Spec, srcPath, rest.Spec.Source.PVC.Name)
	if !hadRestoreContainer {
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		logger.Info("STS patched with restore init container", "sts", sts.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 모든 pod Ready 인가? — Restoring 중 rolling 진행 중일 수 있음.
	if sts.Status.ReadyReplicas < sts.Status.Replicas || sts.Status.Replicas == 0 {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// → Verifying.
	rest.Status.Phase = cachev1alpha1.RestorePhaseVerifying
	rest.Status.Message = fmt.Sprintf("STS pods Ready (%d/%d) — verifying",
		sts.Status.ReadyReplicas, sts.Status.Replicas)
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Verifying",
		Message:            rest.Status.Message,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

// handleVerifying — STS 원복 + paused annotation 제거 → Completed.
//
// 본 commit 은 데이터 plane 검증 (PING + INFO keyspace) 미구현 — 단순
// "STS pods Ready" 통과 시 Completed. 데이터 plane 검증은 별개 commit.
func (r *ValkeyRestoreReconciler) handleVerifying(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// STS 원복 (init container + source volume 제거).
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, sts); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	hadRestore := false
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		if c.Name == resources.RestoreInitContainerName {
			hadRestore = true
			break
		}
	}
	if hadRestore {
		resources.RemoveRestoreFromPodSpec(&sts.Spec.Template.Spec)
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		logger.Info("STS init container removed — second rolling triggered", "sts", sts.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 두 번째 rolling 도 완료 대기.
	if sts.Status.ReadyReplicas < sts.Status.Replicas || sts.Status.Replicas == 0 {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// paused annotation 제거.
	v := &cachev1alpha1.Valkey{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, v); err == nil {
		if v.Annotations[PausedAnnotation] == "true" {
			delete(v.Annotations, PausedAnnotation)
			if err := r.Update(ctx, v); err != nil {
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
		}
	}

	// → Completed.
	now := metav1.Now()
	rest.Status.Phase = cachev1alpha1.RestorePhaseCompleted
	rest.Status.CompletedAt = &now
	rest.Status.Message = "Restore completed — STS reverted, paused removed"
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Completed",
		Message:            rest.Status.Message,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	logger.Info("Restore Completed", "name", rest.Name)
	return ctrl.Result{}, nil
}

// handleDeletion — finalizer cleanup. STS 원복 + paused annotation 제거.
// 정상 Completed 흐름에서는 이미 정리됨 — no-op.
func (r *ValkeyRestoreReconciler) handleDeletion(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// STS 원복 시도 (best-effort).
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, sts); err == nil {
		hadRestore := false
		for _, c := range sts.Spec.Template.Spec.InitContainers {
			if c.Name == resources.RestoreInitContainerName {
				hadRestore = true
				break
			}
		}
		if hadRestore {
			resources.RemoveRestoreFromPodSpec(&sts.Spec.Template.Spec)
			_ = r.Update(ctx, sts)
		}
	}

	// paused annotation 제거 (best-effort).
	v := &cachev1alpha1.Valkey{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, v); err == nil {
		if v.Annotations[PausedAnnotation] == "true" {
			delete(v.Annotations, PausedAnnotation)
			_ = r.Update(ctx, v)
		}
	}

	controllerutil.RemoveFinalizer(rest, finalizerValkeyRestore)
	if err := r.Update(ctx, rest); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("ValkeyRestore deleted — STS reverted + paused removed (best-effort)", "name", rest.Name)
	return ctrl.Result{}, nil
}

// markFailed — Phase=Failed + Reason + Message 기록 후 종료.
func (r *ValkeyRestoreReconciler) markFailed(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
	reason, msg string,
) (ctrl.Result, error) {
	now := metav1.Now()
	rest.Status.Phase = cachev1alpha1.RestorePhaseFailed
	rest.Status.CompletedAt = &now
	rest.Status.Message = msg
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager — manager 에 등록.
func (r *ValkeyRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.ValkeyRestore{}).
		Owns(&appsv1.StatefulSet{}).
		Named("valkeyrestore").
		Complete(r)
}
