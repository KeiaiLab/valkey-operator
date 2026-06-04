/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
	"github.com/keiailab/valkey-operator/internal/secretrotation"
)

// decideRotation — 자체 시크릿 로테이션 결정(순수). 반환 "none"|"baseline"|"rotate".
//
//	interval 빈/형식위반/<=0 → "none" (비활성/안전)
//	last 미설정(nil/zero)     → "baseline" (첫 reconcile, 회전 X, 시각만 기록)
//	경과 >= interval          → "rotate"
//	그 외                     → "none"
func decideRotation(intervalStr string, last *metav1.Time, now time.Time) string {
	interval, err := time.ParseDuration(intervalStr)
	if err != nil || interval <= 0 {
		return "none"
	}
	if last == nil || last.Time.IsZero() {
		return "baseline"
	}
	if secretrotation.ShouldRotate(last.Time, interval, now) {
		return "rotate"
	}
	return "none"
}

// rotatePasswordIfDue — AuthSpec.RotationInterval 정책에 따라 operator-managed
// 비밀번호를 회전한다. user-provided secret(PasswordSecretRef)은 호출 측에서 제외.
// 반환: (회전 후/기존 password, rotated, error). rotated=true 면 호출 측이 password 를
// 재사용해 ConfigMap + auth-secret-hash 에 반영(→ STS 롤링).
func (r *ValkeyReconciler) rotatePasswordIfDue(
	ctx context.Context,
	v *cachev1alpha1.Valkey,
	currentPw string,
	secretRef *corev1.SecretKeySelector,
	now time.Time,
) (string, bool, error) {
	switch decideRotation(v.Spec.Auth.RotationInterval, v.Status.LastPasswordRotation, now) {
	case "baseline":
		v.Status.LastPasswordRotation = &metav1.Time{Time: now}
		return currentPw, false, r.Status().Update(ctx, v)
	case "rotate":
		newPw, err := secretrotation.GeneratePassword()
		if err != nil {
			return currentPw, false, err
		}
		sec := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: secretRef.Name, Namespace: v.Namespace}, sec); err != nil {
			return currentPw, false, err
		}
		if sec.Data == nil {
			sec.Data = map[string][]byte{}
		}
		sec.Data[resources.SecretPasswordKey] = []byte(newPw)
		if err := r.Update(ctx, sec); err != nil {
			return currentPw, false, err
		}
		v.Status.LastPasswordRotation = &metav1.Time{Time: now}
		if err := r.Status().Update(ctx, v); err != nil {
			return newPw, true, err
		}
		return newPw, true, nil
	default:
		return currentPw, false, nil
	}
}
