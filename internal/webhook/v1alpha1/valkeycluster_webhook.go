/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package v1alpha1 — Valkey / ValkeyCluster admission webhook.
//
// 본 webhook 의 역할:
//  1. Defaulting (mutating): CRD marker 가 처리 못 하는 *조합 derivable* 기본값.
//     단순 zero → 상수 기본값은 CRD marker 가 처리 — 본 webhook 은 *조건부 derived*
//     필드만 다룬다.
//  2. Validation: *조합 검증* (CRD marker 의 정적 단일 필드 검증으로 표현 불가).
//     - immutable 필드 가드 (예: Storage.Size 변경 금지).
//     - 모순 조합 reject (예: AutoFailover=true + ReplicasPerShard=0).
//     - TLS Enabled 시 CertManager / CustomCert 둘 중 하나 필수.
package v1alpha1

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	commonswebhook "github.com/keiailab/operator-commons/pkg/webhook"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

var valkeyclusterlog = logf.Log.WithName("valkeycluster-resource")

// SetupValkeyClusterWebhookWithManager registers the webhook for ValkeyCluster in the manager.
func SetupValkeyClusterWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1alpha1.ValkeyCluster{}).
		WithValidator(&ValkeyClusterCustomValidator{}).
		WithDefaulter(&ValkeyClusterCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-cache-keiailab-io-v1alpha1-valkeycluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeyclusters,verbs=create;update,versions=v1alpha1,name=mvalkeycluster-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyClusterCustomDefaulter — derived defaults (CRD marker 미커버 영역).
type ValkeyClusterCustomDefaulter struct{}

// Default — admission 시점 derived 기본값 적용.
//
// CRD marker 는 *상수 기본값* 만 처리. 본 함수는 *조건부* 보정만 다룬다 — 그래야
// 한 곳에서 logic 추적 가능.
func (d *ValkeyClusterCustomDefaulter) Default(_ context.Context, obj *cachev1alpha1.ValkeyCluster) error {
	valkeyclusterlog.V(1).Info("Defaulting", "name", obj.GetName())

	// AutoFailover 의 zero-value 와 명시 false 를 webhook 에서 구별 불가 — CRD
	// default=true 가 강한 신호이므로 여기서 손대지 않음 (mutating webhook 의 한계).

	// SlotMigration 빈 문자열 → "Auto" (CRD default 와 같지만 명시).
	if obj.Spec.SlotMigration == "" {
		obj.Spec.SlotMigration = cachev1alpha1.SlotMigrationAuto
	}
	// `omitempty` 없는 required 필드는 mutating webhook 이 zero value 로 직렬화하면
	// CRD schema default 가 skip 되어 결국 0 으로 남는다. 따라서 여기서 직접 채움.
	if obj.Spec.Shards == 0 {
		obj.Spec.Shards = 3
	}
	// ReplicasPerShard: AutoFailover=true 가 default 인데 0 이면 webhook validator
	// 가 reject 한다. CRD default=1 이지만 omitempty 부재로 schema default 가 skip
	// 되므로 여기서 채움.
	if obj.Spec.ReplicasPerShard == 0 {
		obj.Spec.ReplicasPerShard = 1
	}
	if obj.Spec.Version.Version == "" {
		obj.Spec.Version.Version = cachev1alpha1.DefaultValkeyVersion
	}
	if obj.Spec.Version.Image == "" {
		obj.Spec.Version.Image = cachev1alpha1.DefaultValkeyImage
	}
	// Auth.Enabled — ADR-0013 옵션 A: 항상 강제 (보안 기본값).
	obj.Spec.Auth.Enabled = true
	return nil
}

// +kubebuilder:webhook:path=/validate-cache-keiailab-io-v1alpha1-valkeycluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeyclusters,verbs=create;update,versions=v1alpha1,name=vvalkeycluster-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyClusterCustomValidator — 조합 / immutable 검증.
type ValkeyClusterCustomValidator struct{}

// ValidateCreate — 신규 CR 검증.
func (v *ValkeyClusterCustomValidator) ValidateCreate(_ context.Context, obj *cachev1alpha1.ValkeyCluster) (admission.Warnings, error) {
	valkeyclusterlog.V(1).Info("ValidateCreate", "name", obj.GetName())
	if errs := validateClusterSpec(obj); len(errs) > 0 {
		return nil, apiError("ValkeyCluster", obj.GetName(), errs)
	}
	return nil, nil
}

// ValidateUpdate — 기존 CR 변경 검증. spec 검증 + immutable 가드.
func (v *ValkeyClusterCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *cachev1alpha1.ValkeyCluster) (admission.Warnings, error) {
	valkeyclusterlog.V(1).Info("ValidateUpdate", "name", newObj.GetName())
	errs := validateClusterSpec(newObj)
	errs = append(errs, validateClusterImmutable(oldObj, newObj)...)
	if len(errs) > 0 {
		return nil, apiError("ValkeyCluster", newObj.GetName(), errs)
	}
	return nil, nil
}

// ValidateDelete — 삭제 검증. 현재는 항상 허용 (finalizer 가 graceful teardown 처리).
func (v *ValkeyClusterCustomValidator) ValidateDelete(_ context.Context, _ *cachev1alpha1.ValkeyCluster) (admission.Warnings, error) {
	return nil, nil
}

// validateClusterSpec — 조합 검증. 정적 단일 필드 검증은 CRD marker 가 담당.
func validateClusterSpec(vc *cachev1alpha1.ValkeyCluster) field.ErrorList {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// iteration 31 (2026-05-07): operator-commons/pkg/webhook v0.4.0 위임.
	if err := commonswebhook.ValidateWithPredicate(
		specPath.Child("version", "version"), vc.Spec.Version.Version,
		cachev1alpha1.IsSupportedValkeyVersion,
		cachev1alpha1.SupportedValkeyVersions,
	); err != nil {
		errs = append(errs, err)
	}

	// AutoFailover=true + ReplicasPerShard=0 → failover 불가 (replica 부재).
	if vc.Spec.AutoFailover && vc.Spec.ReplicasPerShard == 0 {
		errs = append(errs, field.Forbidden(
			specPath.Child("autoFailover"),
			"autoFailover=true requires replicasPerShard >= 1 (no replicas means no failover possible)",
		))
	}

	// 총 노드 수 상한 — Valkey cluster 권장 (>100 노드는 운영 부담 + gossip 오버헤드).
	total := vc.Spec.TotalNodes()
	if total > 100 {
		errs = append(errs, field.Invalid(
			specPath,
			total,
			"total nodes (shards * (1 + replicasPerShard)) must not exceed 100",
		))
	}

	// TLS.Enabled=true 면 CertManager 또는 CustomCert 중 하나는 명시.
	if vc.Spec.TLS != nil && vc.Spec.TLS.Enabled {
		// hasCertMgr: omitempty trap 가드 — CertManager pointer non-nil + IssuerRef.Name
		// 비어있지 않을 때만 *유효* 로 간주 (mongodb-operator it46 와 동일 패턴).
		hasCertMgr := vc.Spec.TLS.CertManager != nil && vc.Spec.TLS.CertManager.IssuerRef.Name != ""
		hasCustom := vc.Spec.TLS.CustomCert != nil && vc.Spec.TLS.CustomCert.SecretName != ""
		if !hasCertMgr && !hasCustom {
			errs = append(errs, field.Required(
				specPath.Child("tls"),
				"TLS.Enabled=true requires either tls.certManager or tls.customCert.secretName",
			))
		}
		if hasCertMgr && hasCustom {
			errs = append(errs, field.Forbidden(
				specPath.Child("tls"),
				"TLS.CertManager and TLS.CustomCert are mutually exclusive — choose one",
			))
		}
	}

	// SlotMigration=Manual 일 때 AutoFailover=true 는 허용 — slot 이동 정책과 failover
	// 는 독립적 (모순 아님). 검증 항목 아님.

	// Auth.Users 사용 시 Auth.Enabled=true 필수.
	if len(vc.Spec.Auth.Users) > 0 && !vc.Spec.Auth.Enabled {
		errs = append(errs, field.Forbidden(
			specPath.Child("auth"),
			"auth.users requires auth.enabled=true",
		))
	}

	// storage.size 하한 1Gi (cross-cut audit, mongodb-operator it46 step 7
	// commit 8b2414f 와 동일 invariant). RDB snapshot + AOF 합산 floor 보장.
	errs = append(errs, validateStorageSizeMin(specPath.Child("storage", "size"), vc.Spec.Storage.Size)...)

	return errs
}

// validateStorageSizeMin — storage.size 하한 1Gi 검증. mongodb-operator it46
// step 7 와 동일 패턴 (cross-cut audit per ADR-0016). zero (unset) 은 CRD
// default 가 채우므로 본 함수 도달 시점엔 양수.
func validateStorageSizeMin(path *field.Path, size resource.Quantity) field.ErrorList {
	if size.IsZero() {
		return nil
	}
	min := resource.MustParse("1Gi")
	if size.Cmp(min) < 0 {
		return field.ErrorList{field.Invalid(
			path, size.String(),
			"storage.size must be >= 1Gi — RDB snapshot + AOF data dir floor",
		)}
	}
	return nil
}

// validateClusterImmutable — 변경 금지 필드 가드.
//
// 변경하면 *데이터 손실 또는 cluster topology 깨짐* 위험인 필드:
//   - Storage.StorageClassName / DataDirPath: PVC 재생성 = 데이터 손실.
//   - TLS.Enabled true → false 또는 false → true: 진행 중인 client 연결 단절.
func validateClusterImmutable(oldObj, newObj *cachev1alpha1.ValkeyCluster) field.ErrorList {
	var errs field.ErrorList
	p := field.NewPath("spec")

	if oldObj.Spec.Storage.StorageClassName != "" &&
		oldObj.Spec.Storage.StorageClassName != newObj.Spec.Storage.StorageClassName {
		errs = append(errs, field.Forbidden(
			p.Child("storage", "storageClassName"),
			"storage.storageClassName is immutable",
		))
	}
	if oldObj.Spec.Storage.DataDirPath != "" &&
		oldObj.Spec.Storage.DataDirPath != newObj.Spec.Storage.DataDirPath {
		errs = append(errs, field.Forbidden(
			p.Child("storage", "dataDirPath"),
			"storage.dataDirPath is immutable",
		))
	}

	oldTLS := oldObj.Spec.TLS != nil && oldObj.Spec.TLS.Enabled
	newTLS := newObj.Spec.TLS != nil && newObj.Spec.TLS.Enabled
	if oldTLS != newTLS {
		errs = append(errs, field.Forbidden(
			p.Child("tls", "enabled"),
			"tls.enabled is immutable (toggling breaks active client connections)",
		))
	}

	return errs
}

// apiError — field.ErrorList 를 status=Invalid 의 K8s API error 로 변환.
func apiError(kind, name string, errs field.ErrorList) error {
	return apierrors.NewInvalid(
		schema.GroupKind{Group: cachev1alpha1.GroupVersion.Group, Kind: kind},
		name, errs,
	)
}

// _ runtime.Object 를 위한 명시적 import (admission webhook 등록 시 사용).
var _ runtime.Object = &cachev1alpha1.ValkeyCluster{}
