/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package controller — Valkey + ValkeyCluster reconciler 가 공유하는 헬퍼.

package controller

import (
	"context"
	"fmt"

	commonsfinalizer "github.com/keiailab/operator-commons/pkg/finalizer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileSecretIfNotExists — Secret 멱등 생성 (password Secret 처럼 immutable).
func reconcileSecretIfNotExists(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	owner client.Object,
	secretName string,
	build func() *corev1.Secret,
) error {
	existing := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{Name: secretName, Namespace: owner.GetNamespace()}, existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	secret := build()
	if err := controllerutil.SetControllerReference(owner, secret, scheme); err != nil {
		return fmt.Errorf("set owner ref: %w", err)
	}
	return c.Create(ctx, secret)
}

// handleFinalizerCleanup — deletionTimestamp 설정된 객체 정리 패턴.
//
//nolint:unparam // controller-runtime 표준 (ctrl.Result, error) 시그니처 보존 — 호출자 일관성.
func handleFinalizerCleanup(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	finalizer string,
	cleanup func(context.Context) error,
) (ctrl.Result, error) {
	if !commonsfinalizer.Has(obj, finalizer) {
		return ctrl.Result{}, nil
	}
	if cleanup != nil {
		if err := cleanup(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("finalizer cleanup: %w", err)
		}
	}
	commonsfinalizer.Remove(obj, finalizer)
	if err := c.Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// Statusable — Valkey / ValkeyCluster 가 모두 구현하는 status 추상화.
type Statusable interface {
	client.Object
	GetConditions() *[]metav1.Condition
	SetPhase(phase string)
}

// applyErrorCondition — reconcile 에러 표준 처리.
func applyErrorCondition(
	ctx context.Context,
	c client.Client,
	obj Statusable,
	component string,
	reconcileErr error,
	rec events.EventRecorder,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Error(reconcileErr, "Failed to reconcile component", "component", component)
	MetricReconcileErrors.WithLabelValues(obj.GetNamespace(), obj.GetName(), component).Inc()
	if rec != nil {
		rec.Eventf(obj, nil, corev1.EventTypeWarning, "ReconcileError", "ReconcileError",
			"Failed to reconcile %s: %v", component, reconcileErr)
	}
	obj.SetPhase("Failed")
	conds := obj.GetConditions()
	*conds = filterConditionsByType(*conds, "ReconcileError")
	*conds = append(*conds, metav1.Condition{
		Type:               "ReconcileError",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "ReconcileFailed",
		Message:            fmt.Sprintf("Failed to reconcile %s: %v", component, reconcileErr),
	})
	if statusErr := updateStatusWithRetry(ctx, c, obj); statusErr != nil {
		logger.Error(statusErr, "Failed to update status")
	}
	return ctrl.Result{RequeueAfter: requeueSteady}, reconcileErr
}

func filterConditionsByType(conds []metav1.Condition, t string) []metav1.Condition {
	out := make([]metav1.Condition, 0, len(conds))
	for _, c := range conds {
		if c.Type != t {
			out = append(out, c)
		}
	}
	return out
}

// 표준 condition Type 상수 — Reconcile / 운영자 가시성용.
const (
	CondTypeReady             = "Ready"             // 종합 — Phase 미러 (legacy).
	CondTypeClusterReady      = "ClusterReady"      // CLUSTER state=ok && slots=16384.
	CondTypeCertReady         = "CertReady"         // TLS RootCAs 로드 성공 (또는 TLS 미활성).
	CondTypeScalePending      = "ScalePending"      // Spec ↔ STS replicas 차이 + Deliberate=false.
	CondTypeUpgradeInProgress = "UpgradeInProgress" // Spec.Version != Status.Version + rolling.
)

// setCondition — 동일 Type 의 기존 condition 을 *교체*. status 가 변경된 경우에만
// LastTransitionTime 갱신, 그 외에는 보존.
//
// iteration 32 (2026-05-07): k8s.io/apimachinery/pkg/api/meta.SetStatusCondition
// 위임 — upstream 이 *동일 logic* 제공 (LastTransitionTime 보존/갱신, append).
// 자체 reimplementation 제거. upstream 이 *(changed bool)* 반환하지만 호출자
// 가 무시하므로 시그너처 호환.
func setCondition(conds *[]metav1.Condition, c metav1.Condition) {
	meta.SetStatusCondition(conds, c)
}

// boolToConditionStatus — true → ConditionTrue, false → ConditionFalse.
func boolToConditionStatus(b bool) metav1.ConditionStatus {
	if b {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

// PausedAnnotation — set 시 ValkeyController/ValkeyClusterController 의
// 정상 reconcile 가 no-op. ValkeyRestore (ADR-0015) 가 STS 를 직접 patch 하는
// 동안 controller 가 init container 를 제거하는 충돌 방지.
//
// Deletion 은 paused 와 무관하게 진행 — finalizer cleanup 차단 위험 회피.
const PausedAnnotation = "cache.keiailab.io/paused"

// pausedAnnotationTrue — PausedAnnotation 의 표준 active 값. goconst 회피용 const.
const pausedAnnotationTrue = "true"

// isPaused — 객체에 PausedAnnotation="true" 가 있으면 true.
func isPaused(obj client.Object) bool {
	if obj == nil {
		return false
	}
	return obj.GetAnnotations()[PausedAnnotation] == pausedAnnotationTrue
}
