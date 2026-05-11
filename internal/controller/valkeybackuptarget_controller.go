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
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/observability"
	"github.com/keiailab/valkey-operator/internal/storage"
)

// Reason 상수 — verifyEndpoint 가 반환하는 reason 식별자 (goconst).
const (
	reasonEndpointPingFailed = "EndpointPingFailed"
	reasonEndpointReachable  = "EndpointReachable"
)

// s3ClientBuilder — 테스트에서 mock client 주입을 위한 indirection.
//
// nil 시 기본 storage.BuildS3Client 사용.
type s3ClientBuilder func(s3 *cachev1alpha1.S3Spec, ak, sk string) (s3Reachable, error)

// s3Reachable — Reconciler 가 S3 client 에서 사용할 단일 메서드.
type s3Reachable interface {
	Reachable(ctx context.Context) (bool, error)
	EndpointHost() string
}

func defaultS3ClientBuilder(s3 *cachev1alpha1.S3Spec, ak, sk string) (s3Reachable, error) {
	return storage.BuildS3Client(s3, ak, sk)
}

// ValkeyBackupTargetReconciler — 외부 저장 target 의 reachability 검증.
//
// 책임:
//  1. Spec schema 검증 (verifyCredentials).
//  2. 실제 S3 endpoint reachability — BucketExists 호출 (verifyEndpoint, ADR-0022).
//  3. Phase 전이: Pending → Reachable | Unreachable.
//  4. LastVerifiedAt 갱신.
//  5. 5분 requeue — Secret 회전 / endpoint 변경 / bucket 권한 변경 감지.
//
// ADR-0016 + ADR-0022.
type ValkeyBackupTargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder

	// S3ClientBuilder — 테스트 주입. nil 시 기본 minio-go wrapper.
	S3ClientBuilder s3ClientBuilder
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackuptargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackuptargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackuptargets/finalizers,verbs=update

func (r *ValkeyBackupTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := observability.StartReconcileSpan(ctx, "ValkeyBackupTarget", req.Namespace, req.Name)
	defer span.End()

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
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
	}

	// Phase 전환 시점에만 Event 발행 — 5분 polling noise 회피.
	previousPhase := t.Status.Phase

	// 1. Schema 검증.
	schemaReason, schemaMsg, ak, sk, schemaOK := r.verifyCredentials(ctx, t)
	now := metav1.Now()

	if !schemaOK {
		// Schema 단계 실패 → Unreachable, endpoint ping 생략.
		t.Status.Phase = cachev1alpha1.BackupTargetPhaseUnreachable
		t.Status.Message = schemaMsg
		setCondition(t.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             schemaReason,
			Message:            schemaMsg,
			ObservedGeneration: t.Generation,
		})
	} else {
		// 2. Endpoint ping (BucketExists, ADR-0022).
		endpointReason, endpointMsg, endpointOK := r.verifyEndpoint(ctx, t, ak, sk)
		if !endpointOK {
			t.Status.Phase = cachev1alpha1.BackupTargetPhaseUnreachable
			t.Status.Message = endpointMsg
			setCondition(t.GetConditions(), metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             endpointReason,
				Message:            endpointMsg,
				ObservedGeneration: t.Generation,
			})
		} else {
			// 양 단계 모두 통과 → Reachable.
			t.Status.Phase = cachev1alpha1.BackupTargetPhaseReachable
			t.Status.LastVerifiedAt = &now
			t.Status.Message = endpointMsg
			setCondition(t.GetConditions(), metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             endpointReason,
				Message:            endpointMsg,
				ObservedGeneration: t.Generation,
			})
		}
	}

	t.Status.ObservedGeneration = t.Generation
	if err := updateStatusWithRetry(ctx, r.Client, t); err != nil {
		logger.V(1).Info("status update conflict — requeue", "name", t.Name)
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	// Phase 전환 시 운영자 시점 Event 발행 (kubectl describe 가시).
	if r.Recorder != nil && previousPhase != t.Status.Phase {
		eventType := "Warning"
		if t.Status.Phase == cachev1alpha1.BackupTargetPhaseReachable {
			eventType = "Normal"
		}
		reason := string(t.Status.Phase)
		msg := t.Status.Message
		for _, c := range *t.GetConditions() {
			if c.Type == CondTypeReady {
				reason = c.Reason
				break
			}
		}
		r.Recorder.Eventf(t, nil, eventType, reason, reason, "%s", msg)
	}

	// 5분마다 재검증 (Secret 회전 / endpoint 변경 감지).
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// verifyCredentials — Spec schema 검증 + Secret 의 access/secret key 추출.
//
// ok=true 시 ak/sk 가 Secret data 의 raw bytes (string 변환). caller 가
// verifyEndpoint 에 그대로 전달.
func (r *ValkeyBackupTargetReconciler) verifyCredentials(
	ctx context.Context,
	t *cachev1alpha1.ValkeyBackupTarget,
) (reason, msg, ak, sk string, ok bool) {
	if t.Spec.Type != cachev1alpha1.BackupTargetTypeS3 {
		return "UnsupportedType",
			fmt.Sprintf("type %s not yet implemented", t.Spec.Type), "", "", false
	}
	if t.Spec.S3 == nil {
		return "MissingS3Spec", "spec.s3 required when type=S3", "", "", false
	}
	s3 := t.Spec.S3
	if s3.Endpoint == "" || s3.Region == "" || s3.Bucket == "" {
		return "MissingFields",
			"spec.s3 endpoint/region/bucket all required", "", "", false
	}

	secretName := s3.CredentialsSecretRef.Name
	if secretName == "" {
		return "MissingSecretRef",
			"spec.s3.credentialsSecretRef.name required", "", "", false
	}

	accessKey := s3.CredentialsSecretRef.AccessKeyIDKey
	if accessKey == "" {
		accessKey = "AWS_ACCESS_KEY_ID" // #nosec G101 -- env var key 이름 (자격증명 값 아님).
	}
	secretKey := s3.CredentialsSecretRef.SecretAccessKeyKey
	if secretKey == "" {
		secretKey = "AWS_SECRET_ACCESS_KEY" // #nosec G101 -- env var key 이름 (자격증명 값 아님).
	}

	sec := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: t.Namespace,
	}, sec); err != nil {
		if errors.IsNotFound(err) {
			return "SecretNotFound",
				fmt.Sprintf("Secret %s/%s not found", t.Namespace, secretName),
				"", "", false
		}
		return "SecretGetFailed", err.Error(), "", "", false
	}
	if len(sec.Data[accessKey]) == 0 {
		return "MissingAccessKey",
			fmt.Sprintf("Secret %s key %q empty", secretName, accessKey),
			"", "", false
	}
	if len(sec.Data[secretKey]) == 0 {
		return "MissingSecretKey",
			fmt.Sprintf("Secret %s key %q empty", secretName, secretKey),
			"", "", false
	}
	return "CredentialsValid",
		fmt.Sprintf("S3 endpoint=%s bucket=%s region=%s — credentials present",
			s3.Endpoint, s3.Bucket, s3.Region),
		string(sec.Data[accessKey]), string(sec.Data[secretKey]),
		true
}

// verifyEndpoint — 실제 S3 endpoint 에 BucketExists 호출하여 reachability +
// 자격증명 + 버킷 존재를 동시 검증 (ADR-0022).
//
// 10초 타임아웃 — invalid endpoint / 자격증명 시 reconcile 무한 대기 방지.
//
// 첫 commit 의 의도적 한계 (별개 commit 보강):
//   - 실패 사유 분류 (AccessDenied / InvalidAccessKeyId / NoSuchBucket /
//     network) 가 단일 "EndpointPingFailed" reason 으로 통합. 추후 minio-go
//     ErrorResponse 파싱.
func (r *ValkeyBackupTargetReconciler) verifyEndpoint(
	ctx context.Context,
	t *cachev1alpha1.ValkeyBackupTarget,
	ak, sk string,
) (reason, msg string, ok bool) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyBackupTarget/BucketExists")
	defer span.End()
	build := r.S3ClientBuilder
	if build == nil {
		build = defaultS3ClientBuilder
	}
	s3c, err := build(t.Spec.S3, ak, sk)
	if err != nil {
		return "ClientBuildFailed", err.Error(), false
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	exists, err := s3c.Reachable(pingCtx)
	if err != nil {
		return reasonEndpointPingFailed,
			fmt.Sprintf("BucketExists failed: %s", err.Error()), false
	}
	if !exists {
		return "BucketNotFound",
			fmt.Sprintf("bucket %s not found at %s", t.Spec.S3.Bucket, s3c.EndpointHost()),
			false
	}
	return reasonEndpointReachable,
		fmt.Sprintf("S3 bucket %s @ %s reachable", t.Spec.S3.Bucket, s3c.EndpointHost()),
		true
}

// SetupWithManager — manager 에 등록.
func (r *ValkeyBackupTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// events API 마이그레이션 완료 (RFC-0023 Phase 2, 2026-05-11).
	r.Recorder = mgr.GetEventRecorder("valkeybackuptarget-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.ValkeyBackupTarget{}).
		Named("valkeybackuptarget").
		Complete(r)
}
