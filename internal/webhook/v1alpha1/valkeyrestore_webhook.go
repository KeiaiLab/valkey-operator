/*
Copyright 2026 Keiailab.
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

var valkeyrestorelog = logf.Log.WithName("valkeyrestore-resource")

// SetupValkeyRestoreWebhookWithManager — admission webhook 등록.
func SetupValkeyRestoreWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1alpha1.ValkeyRestore{}).
		WithValidator(&ValkeyRestoreCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-cache-keiailab-io-v1alpha1-valkeyrestore,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeyrestores,verbs=create;update,versions=v1alpha1,name=vvalkeyrestore-v1alpha1.kb.io,admissionReviewVersions=v1

type ValkeyRestoreCustomValidator struct{}

func (v *ValkeyRestoreCustomValidator) ValidateCreate(_ context.Context, obj *cachev1alpha1.ValkeyRestore) (admission.Warnings, error) {
	valkeyrestorelog.V(1).Info("ValidateCreate", "name", obj.GetName())
	return nil, validateRestore(obj)
}

func (v *ValkeyRestoreCustomValidator) ValidateUpdate(_ context.Context, _, newObj *cachev1alpha1.ValkeyRestore) (admission.Warnings, error) {
	return nil, validateRestore(newObj)
}

func (v *ValkeyRestoreCustomValidator) ValidateDelete(_ context.Context, _ *cachev1alpha1.ValkeyRestore) (admission.Warnings, error) {
	return nil, nil
}

// validateRestore — Source 3-type 상호배제 + PointInTime invariants.
func validateRestore(obj *cachev1alpha1.ValkeyRestore) error {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// 1. Source 3-type 중 정확히 1개 명시.
	sourceCount := 0
	if obj.Spec.Source.PVC != nil && obj.Spec.Source.PVC.Name != "" {
		sourceCount++
	}
	if obj.Spec.Source.TargetRef != nil && obj.Spec.Source.TargetRef.Name != "" {
		sourceCount++
	}
	if obj.Spec.Source.VolumeSnapshot != nil && obj.Spec.Source.VolumeSnapshot.Name != "" {
		sourceCount++
	}
	switch {
	case sourceCount == 0:
		errs = append(errs, field.Required(
			specPath.Child("source"),
			"source must specify exactly one of: pvc.name / targetRef.name / volumeSnapshot.name",
		))
	case sourceCount > 1:
		errs = append(errs, field.Forbidden(
			specPath.Child("source"),
			"source.pvc / source.targetRef / source.volumeSnapshot are mutually exclusive — choose one",
		))
	}

	// 2. PointInTime 은 RestoreType=AOF 일 때만 의미. RDB 는 snapshot fixed-time 이라
	//    replay 불가.
	if obj.Spec.PointInTime != nil {
		rt := obj.Spec.RestoreType
		if rt == "" {
			rt = cachev1alpha1.RestoreTypeRDB // CRD default.
		}
		if rt != cachev1alpha1.RestoreTypeAOF {
			errs = append(errs, field.Forbidden(
				specPath.Child("pointInTime"),
				"pointInTime requires restoreType=AOF (RDB is snapshot at fixed time, replay 불가)",
			))
		}
	}

	if len(errs) > 0 {
		return apiError("ValkeyRestore", obj.GetName(), errs)
	}
	return nil
}
