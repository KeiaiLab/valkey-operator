/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package v1alpha1

import (
	"context"
	"fmt"
	"time"

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
		// 시간 의미 가드 (PR #71):
		// - future PointInTime: 의미 모순 (없는 미래 데이터)
		// - 30일 이상 과거: backup retention 일반 범위 초과 — 사용자 실수 가능성
		errs = append(errs, validatePointInTimeBounds(specPath.Child("pointInTime"), obj.Spec.PointInTime.Time)...)
	}

	if len(errs) > 0 {
		return apiError("ValkeyRestore", obj.GetName(), errs)
	}
	return nil
}

// validatePointInTimeBounds — PointInTime 의 시간 의미 가드.
//
// 검사:
//
//	(a) future (now() 보다 미래) → reject — 의미 모순 (없는 데이터 요청)
//	(b) 너무 오래된 과거 (now() - 30일 초과) → reject — backup retention 범위 외,
//	    일반 운영에서 무의미. 사용자 실수 (timezone / format 등) 사전 차단.
//
// 30일은 ADR-0042 commercial parity 시리즈 의 *typical retention* 가정. 사용자
// 가 더 긴 retention 운영 시 *별도 epic* 으로 max 조정 가능.
//
// 본 검사는 *시간 의미적* 범위만 — 실제 backup CompletedAt 와의 대소비교는 별도
// (backup CR lookup 필요, webhook 의 read 부담).
var pointInTimeMaxPastDays = 30 // var: 테스트에서 override 가능.

func validatePointInTimeBounds(p *field.Path, pit time.Time) field.ErrorList {
	var errs field.ErrorList
	now := timeNow()
	if pit.After(now) {
		errs = append(errs, field.Invalid(
			p, pit.Format(time.RFC3339),
			fmt.Sprintf("pointInTime is in the future (now=%s) — 의미 모순", now.Format(time.RFC3339)),
		))
	}
	maxPast := now.AddDate(0, 0, -pointInTimeMaxPastDays)
	if pit.Before(maxPast) {
		errs = append(errs, field.Invalid(
			p, pit.Format(time.RFC3339),
			fmt.Sprintf("pointInTime is more than %d days in the past (max=%s) — typical backup retention 초과",
				pointInTimeMaxPastDays, maxPast.Format(time.RFC3339)),
		))
	}
	return errs
}

// timeNow — test override 가능 (현재는 time.Now 직접 호출).
var timeNow = time.Now
