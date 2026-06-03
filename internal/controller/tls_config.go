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

// Package controller — control-plane TLS client config 단일 진실원.
// 기존 tlsConfigForValkey / tlsConfigForCluster / tlsConfigForClusterRef 3중
// 중복을 buildValkeyTLSConfig 하나로 통합. 각 호출부는 thin wrapper.
package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// buildValkeyTLSConfig — Valkey / ValkeyCluster control-plane 공용 TLS client config.
//
// 우선순위: CustomCert > CertManager > InsecureSkipVerify fallback (ADR-0003 / ADR-0010).
// spec=nil 또는 미활성 시 nil 반환(평문 접속). CA bundle 미준비 시 InsecureSkipVerify
// fallback + warning 로그 (다음 reconcile 에서 자동 회복).
func buildValkeyTLSConfig(
	ctx context.Context, c client.Client, namespace, crName string, spec *cachev1alpha1.TLSSpec,
) (*tls.Config, error) {
	if spec == nil || !spec.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// cert-manager SAN 은 FQDN (`<headless>.<ns>.svc`) 만 담으므로 short name 은 검증 실패.
		ServerName: resources.HeadlessServiceName(crName) + "." + namespace + ".svc",
	}
	attach := func(secretName string) (bool, error) {
		pool, err := loadCABundle(ctx, c, namespace, secretName)
		if err != nil || pool == nil {
			return false, err
		}
		cfg.RootCAs = pool
		// mTLS client cert 는 best-effort — 없거나 파싱 실패해도 RootCAs 만으로 진행.
		if cert, certErr := loadClientCert(ctx, c, namespace, secretName); certErr == nil && cert != nil {
			cfg.Certificates = []tls.Certificate{*cert}
		}
		return true, nil
	}
	if spec.CustomCert != nil && spec.CustomCert.SecretName != "" {
		ok, err := attach(spec.CustomCert.SecretName)
		if err != nil {
			return nil, fmt.Errorf("load ca bundle: %w", err)
		}
		if ok {
			return cfg, nil
		}
	}
	if spec.CertManager != nil && spec.CertManager.IssuerRef.Name != "" {
		ok, err := attach(resources.CertificateSecretName(crName))
		if err != nil {
			return nil, fmt.Errorf("load cert-manager ca bundle: %w", err)
		}
		if ok {
			return cfg, nil
		}
	}
	cfg.InsecureSkipVerify = true //nolint:gosec // ADR-0003: CA bundle 미준비 시 fallback
	log.FromContext(ctx).Info("TLS enabled without CA bundle — using InsecureSkipVerify fallback", "cr", crName)
	return cfg, nil
}

// loadCABundle — Secret 의 ca.crt 를 x509 cert pool 로 로드.
// Secret 미존재 / ca.crt 누락 시 (nil, nil), PEM 파싱 실패 시 (nil, err).
func loadCABundle(ctx context.Context, c client.Client, namespace, secretName string) (*x509.CertPool, error) {
	s := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, s); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	caBytes, ok := s.Data["ca.crt"]
	if !ok || len(caBytes) == 0 {
		return nil, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("invalid PEM in %s/%s/ca.crt", namespace, secretName)
	}
	return pool, nil
}

// loadClientCert — mTLS (`tls-auth-clients yes`) 용 client cert (동일 Secret 의 tls.crt+tls.key).
// 둘 중 하나라도 없으면 (nil, nil), 파싱 실패 시 (nil, err).
func loadClientCert(ctx context.Context, c client.Client, namespace, secretName string) (*tls.Certificate, error) {
	s := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, s); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	crt, hasCrt := s.Data["tls.crt"]
	key, hasKey := s.Data["tls.key"]
	if !hasCrt || !hasKey || len(crt) == 0 || len(key) == 0 {
		return nil, nil
	}
	cert, err := tls.X509KeyPair(crt, key)
	if err != nil {
		return nil, fmt.Errorf("invalid keypair in %s/%s: %w", namespace, secretName, err)
	}
	return &cert, nil
}
