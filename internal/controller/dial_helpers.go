/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package controller — ClusterRef 기반 dial helpers (receiver-less).
// ValkeyBackup/ValkeyRestore 양쪽이 같은 패턴으로 사용 — code 중복 해소.
//
// 본 파일의 함수들은 ValkeyBackupReconciler.dialBackupTarget /
// fetchBackupTargetPassword / tlsConfigForBackupTarget 의 본문을 receiver-less
// 형태로 추출한 것. ValkeyBackupReconciler / ValkeyRestoreReconciler 의
// method 들은 본 함수들을 호출하는 thin wrapper.
//
// ADR 별개 — 단순 refactor.
package controller

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

// dialClusterRefTarget — ClusterRef 가 가리키는 인스턴스의 primary (pod-0)
// dial. TLS 활성 시 자동 6380 + cert 로딩.
func dialClusterRefTarget(
	ctx context.Context, c client.Client,
	ref cachev1alpha1.ClusterReference, namespace string,
) (*redis.Client, error) {
	password, err := fetchClusterRefPassword(ctx, c, ref, namespace)
	if err != nil {
		return nil, err
	}
	tlsCfg, err := tlsConfigForClusterRef(ctx, c, ref, namespace)
	if err != nil {
		return nil, err
	}
	port := int32(resources.PortClient)
	if tlsCfg != nil {
		port = resources.PortTLS
	}
	addr := fmt.Sprintf("%s:%d",
		resources.PodFQDN(ref.Name, 0, namespace),
		port)
	opts := vk.DialOptions{Address: addr, Password: password}
	if tlsCfg != nil {
		opts.UseTLS = true
		opts.TLSConf = tlsCfg
	}
	return vk.NewSingleClient(opts), nil
}

// fetchClusterRefPassword — Auth Secret 의 password 추출.
func fetchClusterRefPassword(
	ctx context.Context, c client.Client,
	ref cachev1alpha1.ClusterReference, namespace string,
) (string, error) {
	secretName := resources.DefaultSecretName(ref.Name)
	s := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{
		Name: secretName, Namespace: namespace,
	}, s); err != nil {
		return "", fmt.Errorf("get auth secret %s: %w", secretName, err)
	}
	if len(s.Data[resources.SecretPasswordKey]) == 0 {
		return "", fmt.Errorf("auth secret %s missing password key", secretName)
	}
	return string(s.Data[resources.SecretPasswordKey]), nil
}

// tlsConfigForClusterRef — Spec.TLS 조회 → operator → 노드 control-plane TLS
// config (CustomCert > CertManager > InsecureSkipVerify).
func tlsConfigForClusterRef(
	ctx context.Context, c client.Client,
	ref cachev1alpha1.ClusterReference, namespace string,
) (*tls.Config, error) {
	nsName := types.NamespacedName{Name: ref.Name, Namespace: namespace}
	var tlsSpec *cachev1alpha1.TLSSpec
	switch ref.Kind {
	case cachev1alpha1.KindValkey:
		obj := &cachev1alpha1.Valkey{}
		if err := c.Get(ctx, nsName, obj); err != nil {
			return nil, err
		}
		tlsSpec = obj.Spec.TLS
	case cachev1alpha1.KindValkeyCluster:
		obj := &cachev1alpha1.ValkeyCluster{}
		if err := c.Get(ctx, nsName, obj); err != nil {
			return nil, err
		}
		tlsSpec = obj.Spec.TLS
	}
	return buildValkeyTLSConfig(ctx, c, namespace, ref.Name, tlsSpec)
}
