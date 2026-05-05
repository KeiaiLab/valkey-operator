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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// ValkeyBackupReconciler — RDB / AOF backup 트리거 + 상태 추적.
//
// 본 iter (M2) 의 책임:
//  1. Spec.ClusterRef 가 가리키는 Valkey / ValkeyCluster 존재 검증.
//  2. Phase 전이 (Pending → InProgress → Completed | Failed).
//  3. Status.StartedAt / CompletedAt / Conditions 기록.
//
// 미구현 (M3 후속):
//   - 실제 BGSAVE / BGREWRITEAOF 명령 발행 + LASTSAVE 폴링.
//   - 결과 PVC 동적 프로비저닝 + RDB 파일 복사 (Job 기반).
//   - TTL 기반 자동 삭제.
type ValkeyBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups/finalizers,verbs=update

func (r *ValkeyBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	b := &cachev1alpha1.ValkeyBackup{}
	if err := r.Get(ctx, req.NamespacedName, b); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Terminal phase 면 추가 작업 없음.
	if b.IsTerminal() {
		return ctrl.Result{}, nil
	}

	// 1. ClusterRef 검증 — 대상 CR 존재 확인.
	if err := r.validateClusterRef(ctx, b); err != nil {
		return r.markFailed(ctx, b, "TargetNotFound", err.Error())
	}

	// 2. Phase 전이.
	switch b.Status.Phase {
	case "":
		// 신규 — Pending 으로 시작.
		b.Status.Phase = cachev1alpha1.BackupPhasePending
		b.Status.ObservedGeneration = b.Generation
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Pending",
			Message:            "Backup queued",
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil

	case cachev1alpha1.BackupPhasePending:
		// InProgress 로 전이 + StartedAt 기록.
		now := metav1.Now()
		b.Status.Phase = cachev1alpha1.BackupPhaseInProgress
		b.Status.StartedAt = &now
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "InProgress",
			Message:            fmt.Sprintf("Backup %s started for %s/%s", b.Spec.Type, b.Spec.ClusterRef.Kind, b.Spec.ClusterRef.Name),
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		logger.Info("Backup transitioned to InProgress",
			"name", b.Name, "type", b.Spec.Type, "target", b.Spec.ClusterRef.Name)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil

	case cachev1alpha1.BackupPhaseInProgress:
		// 실제 BGSAVE / LASTSAVE 폴링은 M3. 본 iter 는 *전이만* — 즉시 Completed 처리.
		// (사용자가 backup 트리거는 했지만 실제 데이터 복사는 미구현 임을 status message 로 명시.)
		now := metav1.Now()
		b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
		b.Status.CompletedAt = &now
		b.Status.Message = "Backup phase transition complete (data copy pending — M3 implementation)"
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Completed",
			Message:            b.Status.Message,
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// validateClusterRef — Spec.ClusterRef 가 가리키는 Valkey / ValkeyCluster 존재 확인.
func (r *ValkeyBackupReconciler) validateClusterRef(ctx context.Context, b *cachev1alpha1.ValkeyBackup) error {
	key := types.NamespacedName{Name: b.Spec.ClusterRef.Name, Namespace: b.Namespace}
	switch b.Spec.ClusterRef.Kind {
	case "ValkeyCluster":
		obj := &cachev1alpha1.ValkeyCluster{}
		if err := r.Get(ctx, key, obj); err != nil {
			return fmt.Errorf("get ValkeyCluster %s: %w", key, err)
		}
	case "Valkey":
		obj := &cachev1alpha1.Valkey{}
		if err := r.Get(ctx, key, obj); err != nil {
			return fmt.Errorf("get Valkey %s: %w", key, err)
		}
	default:
		return fmt.Errorf("unsupported ClusterRef.Kind: %q", b.Spec.ClusterRef.Kind)
	}
	return nil
}

// markFailed — 백업을 Failed phase 로 전이 + 에러 condition.
func (r *ValkeyBackupReconciler) markFailed(ctx context.Context, b *cachev1alpha1.ValkeyBackup, reason, msg string) (ctrl.Result, error) {
	b.Status.Phase = cachev1alpha1.BackupPhaseFailed
	b.Status.Message = msg
	now := metav1.Now()
	b.Status.CompletedAt = &now
	setCondition(b.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: b.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ValkeyBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.ValkeyBackup{}).
		Named("valkeybackup").
		Complete(r)
}
