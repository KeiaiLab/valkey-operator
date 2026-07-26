/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// applyAuthConfSecret — 인증 조각(auth.conf) Secret 을 idempotent 하게 적용한다.
//
// keiailab-commons 는 SecretIfNotExists 만 제공하는데, 이 Secret 은 password
// 회전 시 **갱신되어야** 하므로 CreateOrUpdate 로 적용한다 (commonsapply.ConfigMap
// 과 동일한 소유권/라벨 계약).
//
// desired 가 nil 이면 (auth 미사용) no-op.
func applyAuthConfSecret(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	owner client.Object,
	desired *corev1.Secret,
) error {
	if desired == nil {
		return nil
	}
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, c, target, func() error {
		target.Labels = desired.Labels
		target.Annotations = desired.Annotations
		target.Type = desired.Type
		// StringData 는 API server 가 Data 로 병합하므로, 기존 Data 를 비워
		// 삭제된 키가 잔존하지 않게 한다 (단일 진실 = desired).
		target.Data = nil
		target.StringData = desired.StringData
		return controllerutil.SetControllerReference(owner, target, scheme)
	})
	return err
}
