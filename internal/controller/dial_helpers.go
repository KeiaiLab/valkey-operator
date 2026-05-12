/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Copyright 2026 Keiailab.

Package controller — ClusterRef 기반 dial helpers (receiver-less).
ValkeyBackup/ValkeyRestore 양쪽이 같은 패턴으로 사용 — code 중복 해소.

본 파일의 함수들은 ValkeyBackupReconciler.dialBackupTarget /
fetchBackupTargetPassword / tlsConfigForBackupTarget 의 본문을 receiver-less
형태로 추출한 것. ValkeyBackupReconciler / ValkeyRestoreReconciler 의
method 들은 본 함수들을 호출하는 thin wrapper.

ADR 별개 — 단순 refactor.
*/

package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	var (
		tlsSpec  *cachev1alpha1.TLSSpec
		certName = resources.CertificateSecretName(ref.Name)
		nsName   = types.NamespacedName{Name: ref.Name, Namespace: namespace}
		headless = resources.HeadlessServiceName(ref.Name) + "." + namespace + ".svc"
	)
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
	if tlsSpec == nil || !tlsSpec.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: headless}

	loadAttach := func(secretName string) (bool, error) {
		s := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{
			Name: secretName, Namespace: namespace,
		}, s); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		caBytes, ok := s.Data["ca.crt"]
		if !ok || len(caBytes) == 0 {
			return false, nil
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return false, fmt.Errorf("invalid PEM in %s/ca.crt", secretName)
		}
		cfg.RootCAs = pool
		if crt, hasCrt := s.Data["tls.crt"]; hasCrt {
			if key, hasKey := s.Data["tls.key"]; hasKey && len(crt) > 0 && len(key) > 0 {
				if pair, err := tls.X509KeyPair(crt, key); err == nil {
					cfg.Certificates = []tls.Certificate{pair}
				}
			}
		}
		return true, nil
	}
	if tlsSpec.CustomCert != nil && tlsSpec.CustomCert.SecretName != "" {
		if ok, err := loadAttach(tlsSpec.CustomCert.SecretName); err != nil {
			return nil, err
		} else if ok {
			return cfg, nil
		}
	}
	if tlsSpec.CertManager != nil && tlsSpec.CertManager.IssuerRef.Name != "" {
		if ok, err := loadAttach(certName); err != nil {
			return nil, err
		} else if ok {
			return cfg, nil
		}
	}
	cfg.InsecureSkipVerify = true
	return cfg, nil
}
