/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// hashTLSSecret — TLS Secret data (tls.crt + tls.key + ca.crt) 의 SHA256 (hex)
// hash. STS PodTemplate annotation `cache.keiailab.io/tls-cert-hash` 으로 주입되어
// *cert rotation 추적*. cert-manager 가 leaf/CA 를 재발급(예: CA rotation)하면:
//
//   - 다음 reconcile 에서 Secret data 가 변경됨
//   - 새 hash 가 STS Template 에 set
//   - STS controller 가 PodTemplate 변경 감지 → rolling update 시작
//   - 모든 pod 가 *새 cert/CA* 를 마운트한 채 재시작 (프로세스가 디스크 cert 를
//     시작 시점에만 read 하는 결함을 우회 — old cert 로 서빙하는 mTLS 단절 방지)
//
// secretName 이 빈 문자열이거나 Secret 미존재 시 빈 문자열 반환 → annotation
// 미설정 (TLS rotation 추적 비활성). Secret 은 곧 cert-manager 가 채우므로
// 다음 reconcile 에서 hash 가 채워진다 (fail-soft).
//
// 보안: SHA256 의 *hash* 만 노출. 원본 cert/key 는 K8s API(annotation)에 노출 안 됨.
func hashTLSSecret(ctx context.Context, c client.Client, namespace, secretName string) (string, error) {
	if secretName == "" {
		return "", nil
	}
	s := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, s); err != nil {
		if apierrors.IsNotFound(err) {
			// cert-manager 가 아직 Secret 을 만들지 않음 — fail-soft.
			return "", nil
		}
		return "", err
	}
	h := sha256.New()
	// 결정적 순서로 누적 (map 순회 비결정성 회피).
	for _, key := range []string{"tls.crt", "tls.key", "ca.crt"} {
		// key 자체도 누적해 빈 값/누락을 구분 (예: ca.crt 만 변경되어도 hash 변경).
		h.Write([]byte(key))
		h.Write(s.Data[key])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
