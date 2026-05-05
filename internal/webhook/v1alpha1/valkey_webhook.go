/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

var valkeylog = logf.Log.WithName("valkey-resource")

// SetupValkeyWebhookWithManager registers the webhook for Valkey in the manager.
func SetupValkeyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1alpha1.Valkey{}).
		WithValidator(&ValkeyCustomValidator{}).
		WithDefaulter(&ValkeyCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-cache-keiailab-io-v1alpha1-valkey,mutating=true,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeys,verbs=create;update,versions=v1alpha1,name=mvalkey-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyCustomDefaulter — derived defaults.
type ValkeyCustomDefaulter struct{}

// Default — Mode=Standalone 일 때 Replicas 강제 1, Mode=Replication 일 때 Replicas
// 미명시 시 2 (primary + 1 replica).
//
// Why: CRD 의 +kubebuilder:default 는 부모 객체가 nil 이면 자식 default 를
// 적용하지 않는다. spec.version 이 통째로 누락된 CR (샘플 포함) 의 경우
// version.version 이 빈 문자열이 되어 CRD 정규식 검증을 실패시켜 reconcile
// 무한 루프를 유발한다. 따라서 defaulting webhook 에서 빈 값을 채워야 한다.
func (d *ValkeyCustomDefaulter) Default(_ context.Context, obj *cachev1alpha1.Valkey) error {
	valkeylog.V(1).Info("Defaulting", "name", obj.GetName())
	if obj.Spec.Mode == cachev1alpha1.ModeStandalone {
		obj.Spec.Replicas = 1
	}
	if obj.Spec.Mode == cachev1alpha1.ModeReplication && obj.Spec.Replicas < 2 {
		obj.Spec.Replicas = 2
	}
	if obj.Spec.Version.Version == "" {
		obj.Spec.Version.Version = cachev1alpha1.DefaultValkeyVersion
	}
	if obj.Spec.Version.Image == "" {
		obj.Spec.Version.Image = cachev1alpha1.DefaultValkeyImage
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-cache-keiailab-io-v1alpha1-valkey,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeys,verbs=create;update,versions=v1alpha1,name=vvalkey-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyCustomValidator — 조합 / immutable 검증.
type ValkeyCustomValidator struct{}

// ValidateCreate — 신규 CR 검증.
func (v *ValkeyCustomValidator) ValidateCreate(_ context.Context, obj *cachev1alpha1.Valkey) (admission.Warnings, error) {
	if errs := validateValkeySpec(obj); len(errs) > 0 {
		return nil, apiError("Valkey", obj.GetName(), errs)
	}
	return nil, nil
}

// ValidateUpdate — Mode immutable + spec 검증.
func (v *ValkeyCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *cachev1alpha1.Valkey) (admission.Warnings, error) {
	errs := validateValkeySpec(newObj)
	errs = append(errs, validateValkeyImmutable(oldObj, newObj)...)
	if len(errs) > 0 {
		return nil, apiError("Valkey", newObj.GetName(), errs)
	}
	return nil, nil
}

// ValidateDelete — 항상 허용.
func (v *ValkeyCustomValidator) ValidateDelete(_ context.Context, _ *cachev1alpha1.Valkey) (admission.Warnings, error) {
	return nil, nil
}

func validateValkeySpec(v *cachev1alpha1.Valkey) field.ErrorList {
	var errs field.ErrorList
	p := field.NewPath("spec")

	if v.Spec.Mode == cachev1alpha1.ModeStandalone && v.Spec.Replicas > 1 {
		errs = append(errs, field.Invalid(
			p.Child("replicas"), v.Spec.Replicas,
			"replicas must be 1 when mode=Standalone",
		))
	}
	if v.Spec.Mode == cachev1alpha1.ModeReplication && v.Spec.Replicas < 2 {
		errs = append(errs, field.Invalid(
			p.Child("replicas"), v.Spec.Replicas,
			"replicas must be >= 2 when mode=Replication",
		))
	}
	if v.Spec.TLS != nil && v.Spec.TLS.Enabled {
		hasCertMgr := v.Spec.TLS.CertManager != nil
		hasCustom := v.Spec.TLS.CustomCert != nil && v.Spec.TLS.CustomCert.SecretName != ""
		if !hasCertMgr && !hasCustom {
			errs = append(errs, field.Required(
				p.Child("tls"),
				"TLS.Enabled=true requires either tls.certManager or tls.customCert.secretName",
			))
		}
		if hasCertMgr && hasCustom {
			errs = append(errs, field.Forbidden(
				p.Child("tls"),
				"TLS.CertManager and TLS.CustomCert are mutually exclusive — choose one",
			))
		}
	}
	if len(v.Spec.Auth.Users) > 0 && !v.Spec.Auth.Enabled {
		errs = append(errs, field.Forbidden(
			p.Child("auth"),
			"auth.users requires auth.enabled=true",
		))
	}
	return errs
}

func validateValkeyImmutable(oldObj, newObj *cachev1alpha1.Valkey) field.ErrorList {
	var errs field.ErrorList
	p := field.NewPath("spec")
	if oldObj.Spec.Mode != "" && oldObj.Spec.Mode != newObj.Spec.Mode {
		errs = append(errs, field.Forbidden(
			p.Child("mode"),
			"spec.mode is immutable (Standalone↔Replication transitions require manual migration)",
		))
	}
	if oldObj.Spec.Storage.StorageClassName != "" &&
		oldObj.Spec.Storage.StorageClassName != newObj.Spec.Storage.StorageClassName {
		errs = append(errs, field.Forbidden(
			p.Child("storage", "storageClassName"),
			"storage.storageClassName is immutable",
		))
	}
	oldTLS := oldObj.Spec.TLS != nil && oldObj.Spec.TLS.Enabled
	newTLS := newObj.Spec.TLS != nil && newObj.Spec.TLS.Enabled
	if oldTLS != newTLS {
		errs = append(errs, field.Forbidden(
			p.Child("tls", "enabled"),
			"tls.enabled is immutable",
		))
	}
	return errs
}
