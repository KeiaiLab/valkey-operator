/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package controller — Valkey + ValkeyCluster reconciler 가 공유하는 헬퍼.
// mongodb-operator/internal/controller/helpers.go 패턴 차용.
package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
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
func handleFinalizerCleanup(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	finalizer string,
	cleanup func(context.Context) error,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		return ctrl.Result{}, nil
	}
	if cleanup != nil {
		if err := cleanup(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("finalizer cleanup: %w", err)
		}
	}
	controllerutil.RemoveFinalizer(obj, finalizer)
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
	rec record.EventRecorder,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Error(reconcileErr, "Failed to reconcile component", "component", component)
	MetricReconcileErrors.WithLabelValues(obj.GetNamespace(), obj.GetName(), component).Inc()
	if rec != nil {
		rec.Eventf(obj, corev1.EventTypeWarning, "ReconcileError",
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
	return ctrl.Result{RequeueAfter: 30 * time.Second}, reconcileErr
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
func setCondition(conds *[]metav1.Condition, c metav1.Condition) {
	for i := range *conds {
		if (*conds)[i].Type == c.Type {
			if (*conds)[i].Status != c.Status {
				c.LastTransitionTime = metav1.Now()
			} else {
				c.LastTransitionTime = (*conds)[i].LastTransitionTime
			}
			(*conds)[i] = c
			return
		}
	}
	if c.LastTransitionTime.IsZero() {
		c.LastTransitionTime = metav1.Now()
	}
	*conds = append(*conds, c)
}

// boolToConditionStatus — true → ConditionTrue, false → ConditionFalse.
func boolToConditionStatus(b bool) metav1.ConditionStatus {
	if b {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}
