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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

var valkeybackuptargetlog = logf.Log.WithName("valkeybackuptarget-resource")

// SetupValkeyBackupTargetWebhookWithManager — admission webhook 등록.
func SetupValkeyBackupTargetWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1alpha1.ValkeyBackupTarget{}).
		WithValidator(&ValkeyBackupTargetCustomValidator{}).
		WithDefaulter(&ValkeyBackupTargetCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-cache-keiailab-io-v1alpha1-valkeybackuptarget,mutating=true,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeybackuptargets,verbs=create;update,versions=v1alpha1,name=mvalkeybackuptarget-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyBackupTargetCustomDefaulter — Type 미설정 시 S3 default (CRD default 의 backup).
type ValkeyBackupTargetCustomDefaulter struct{}

func (d *ValkeyBackupTargetCustomDefaulter) Default(_ context.Context, obj *cachev1alpha1.ValkeyBackupTarget) error {
	valkeybackuptargetlog.V(1).Info("Defaulting", "name", obj.GetName())
	if obj.Spec.Type == "" {
		obj.Spec.Type = cachev1alpha1.BackupTargetTypeS3
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-cache-keiailab-io-v1alpha1-valkeybackuptarget,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeybackuptargets,verbs=create;update,versions=v1alpha1,name=vvalkeybackuptarget-v1alpha1.kb.io,admissionReviewVersions=v1

type ValkeyBackupTargetCustomValidator struct{}

func (v *ValkeyBackupTargetCustomValidator) ValidateCreate(_ context.Context, obj *cachev1alpha1.ValkeyBackupTarget) (admission.Warnings, error) {
	return nil, validateBackupTarget(obj)
}

func (v *ValkeyBackupTargetCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *cachev1alpha1.ValkeyBackupTarget) (admission.Warnings, error) {
	if err := validateBackupTarget(newObj); err != nil {
		return nil, err
	}
	// Type 변경은 immutable — provider switch 는 새 CR 생성 권장 (자격증명 / 데이터
	// 마이그레이션 책임 분리).
	if oldObj.Spec.Type != "" && oldObj.Spec.Type != newObj.Spec.Type {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: cachev1alpha1.GroupVersion.Group, Kind: "ValkeyBackupTarget"},
			newObj.GetName(),
			field.ErrorList{field.Forbidden(
				field.NewPath("spec", "type"),
				"spec.type is immutable — create a new ValkeyBackupTarget for provider switch",
			)},
		)
	}
	return nil, nil
}

func (v *ValkeyBackupTargetCustomValidator) ValidateDelete(_ context.Context, _ *cachev1alpha1.ValkeyBackupTarget) (admission.Warnings, error) {
	return nil, nil
}

// validateBackupTarget — Type 과 sub-spec 의 일관성 + 필수 필드 검증.
func validateBackupTarget(obj *cachev1alpha1.ValkeyBackupTarget) error {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	t := obj.Spec.Type
	if t == "" {
		t = cachev1alpha1.BackupTargetTypeS3 // defaulter 가 이미 채웠지만 안전 가드.
	}

	switch t {
	case cachev1alpha1.BackupTargetTypeS3:
		if obj.Spec.S3 == nil {
			errs = append(errs, field.Required(specPath.Child("s3"), "type=S3 requires spec.s3"))
		} else {
			errs = append(errs, validateS3Subspec(specPath.Child("s3"), obj.Spec.S3)...)
		}
		if obj.Spec.GCS != nil {
			errs = append(errs, field.Forbidden(specPath.Child("gcs"), "spec.gcs must be omitted when type=S3"))
		}
		if obj.Spec.Azure != nil {
			errs = append(errs, field.Forbidden(specPath.Child("azure"), "spec.azure must be omitted when type=S3"))
		}
	case cachev1alpha1.BackupTargetTypeGCS:
		if obj.Spec.GCS == nil {
			errs = append(errs, field.Required(specPath.Child("gcs"), "type=GCS requires spec.gcs"))
		} else {
			errs = append(errs, validateGCSSubspec(specPath.Child("gcs"), obj.Spec.GCS)...)
		}
		if obj.Spec.S3 != nil {
			errs = append(errs, field.Forbidden(specPath.Child("s3"), "spec.s3 must be omitted when type=GCS"))
		}
		if obj.Spec.Azure != nil {
			errs = append(errs, field.Forbidden(specPath.Child("azure"), "spec.azure must be omitted when type=GCS"))
		}
	case cachev1alpha1.BackupTargetTypeAzure:
		if obj.Spec.Azure == nil {
			errs = append(errs, field.Required(specPath.Child("azure"), "type=Azure requires spec.azure"))
		} else {
			errs = append(errs, validateAzureSubspec(specPath.Child("azure"), obj.Spec.Azure)...)
		}
		if obj.Spec.S3 != nil {
			errs = append(errs, field.Forbidden(specPath.Child("s3"), "spec.s3 must be omitted when type=Azure"))
		}
		if obj.Spec.GCS != nil {
			errs = append(errs, field.Forbidden(specPath.Child("gcs"), "spec.gcs must be omitted when type=Azure"))
		}
	default:
		errs = append(errs, field.NotSupported(
			specPath.Child("type"), string(t),
			[]string{"S3", "GCS", "Azure"},
		))
	}

	if len(errs) > 0 {
		return apiError("ValkeyBackupTarget", obj.GetName(), errs)
	}
	return nil
}

func validateS3Subspec(p *field.Path, s3 *cachev1alpha1.S3Spec) field.ErrorList {
	var errs field.ErrorList
	if s3.Endpoint == "" {
		errs = append(errs, field.Required(p.Child("endpoint"), "spec.s3.endpoint required"))
	}
	if s3.Bucket == "" {
		errs = append(errs, field.Required(p.Child("bucket"), "spec.s3.bucket required"))
	}
	if s3.CredentialsSecretRef.Name == "" {
		errs = append(errs, field.Required(p.Child("credentialsSecretRef", "name"), "secret name required"))
	}
	return errs
}

func validateGCSSubspec(p *field.Path, gcs *cachev1alpha1.GCSSpec) field.ErrorList {
	var errs field.ErrorList
	if gcs.Bucket == "" {
		errs = append(errs, field.Required(p.Child("bucket"), "spec.gcs.bucket required"))
	}
	if gcs.CredentialsSecretRef.Name == "" {
		errs = append(errs, field.Required(p.Child("credentialsSecretRef", "name"), "secret name required"))
	}
	return errs
}

func validateAzureSubspec(p *field.Path, az *cachev1alpha1.AzureSpec) field.ErrorList {
	var errs field.ErrorList
	if az.AccountName == "" {
		errs = append(errs, field.Required(p.Child("accountName"), "spec.azure.accountName required"))
	}
	if az.Container == "" {
		errs = append(errs, field.Required(p.Child("container"), "spec.azure.container required"))
	}
	if az.CredentialsSecretRef.Name == "" {
		errs = append(errs, field.Required(p.Child("credentialsSecretRef", "name"), "secret name required"))
	}
	return errs
}
