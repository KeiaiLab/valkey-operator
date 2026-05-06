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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// ValkeyBackupTargetReconciler — 외부 저장 target 의 reachability 검증.
//
// 본 commit (Track A AI-002 첫 단계) 책임:
//  1. Spec 기반 자격증명 Secret 존재 + key 검증.
//  2. Phase 전이: Pending → Reachable | Unreachable.
//  3. LastVerifiedAt 갱신.
//  4. 5분 requeue — Secret 회전 / endpoint 변경 감지.
//
// 별개 commit (AWS SDK 통합 후):
//   - 실제 S3 reachability ping (HEAD bucket / ListObjects v2).
//
// ADR-0016.
type ValkeyBackupTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackuptargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackuptargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackuptargets/finalizers,verbs=update

func (r *ValkeyBackupTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	t := &cachev1alpha1.ValkeyBackupTarget{}
	if err := r.Get(ctx, req.NamespacedName, t); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 신규 — Pending 으로 표시 후 검증 진입.
	if t.Status.Phase == "" {
		t.Status.Phase = cachev1alpha1.BackupTargetPhasePending
		t.Status.ObservedGeneration = t.Generation
		setCondition(t.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Pending",
			Message:            "validating credentials",
			ObservedGeneration: t.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, t); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	// 자격증명 검증.
	reason, msg, ok := r.verifyCredentials(ctx, t)
	now := metav1.Now()

	if ok {
		t.Status.Phase = cachev1alpha1.BackupTargetPhaseReachable
		t.Status.LastVerifiedAt = &now
		t.Status.Message = msg
		setCondition(t.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			Message:            msg,
			ObservedGeneration: t.Generation,
		})
	} else {
		t.Status.Phase = cachev1alpha1.BackupTargetPhaseUnreachable
		t.Status.Message = msg
		setCondition(t.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            msg,
			ObservedGeneration: t.Generation,
		})
	}

	t.Status.ObservedGeneration = t.Generation
	if err := updateStatusWithRetry(ctx, r.Client, t); err != nil {
		logger.V(1).Info("status update conflict — requeue", "name", t.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 5분마다 재검증 (Secret 회전 / endpoint 변경 감지).
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// verifyCredentials — 자격증명 Secret 존재 + key 가 비어있지 않은지 검증.
//
// 본 함수는 "schema-level" 검증만 수행. 실제 S3 endpoint reachability 는
// 별개 commit (AWS SDK 통합 후). 그때 본 함수가 verifySchema + verifyEndpoint
// 두 단계로 분해.
func (r *ValkeyBackupTargetReconciler) verifyCredentials(
	ctx context.Context,
	t *cachev1alpha1.ValkeyBackupTarget,
) (reason, msg string, ok bool) {
	if t.Spec.Type != cachev1alpha1.BackupTargetTypeS3 {
		return "UnsupportedType",
			fmt.Sprintf("type %s not yet implemented", t.Spec.Type), false
	}
	if t.Spec.S3 == nil {
		return "MissingS3Spec", "spec.s3 required when type=S3", false
	}
	s3 := t.Spec.S3
	if s3.Endpoint == "" || s3.Region == "" || s3.Bucket == "" {
		return "MissingFields",
			"spec.s3 endpoint/region/bucket all required", false
	}

	secretName := s3.CredentialsSecretRef.Name
	if secretName == "" {
		return "MissingSecretRef",
			"spec.s3.credentialsSecretRef.name required", false
	}

	accessKey := s3.CredentialsSecretRef.AccessKeyIDKey
	if accessKey == "" {
		accessKey = "AWS_ACCESS_KEY_ID"
	}
	secretKey := s3.CredentialsSecretRef.SecretAccessKeyKey
	if secretKey == "" {
		secretKey = "AWS_SECRET_ACCESS_KEY"
	}

	sec := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: t.Namespace,
	}, sec); err != nil {
		if errors.IsNotFound(err) {
			return "SecretNotFound",
				fmt.Sprintf("Secret %s/%s not found", t.Namespace, secretName),
				false
		}
		return "SecretGetFailed", err.Error(), false
	}
	if len(sec.Data[accessKey]) == 0 {
		return "MissingAccessKey",
			fmt.Sprintf("Secret %s key %q empty", secretName, accessKey), false
	}
	if len(sec.Data[secretKey]) == 0 {
		return "MissingSecretKey",
			fmt.Sprintf("Secret %s key %q empty", secretName, secretKey), false
	}
	return "CredentialsValid",
		fmt.Sprintf("S3 endpoint=%s bucket=%s region=%s — credentials present (S3 ping pending SDK)",
			s3.Endpoint, s3.Bucket, s3.Region),
		true
}

// SetupWithManager — manager 에 등록.
func (r *ValkeyBackupTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.ValkeyBackupTarget{}).
		Named("valkeybackuptarget").
		Complete(r)
}
