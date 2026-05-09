/*
Copyright 2026 Keiailab.
*/

package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// CertificateGVK — cert-manager.io/v1 Certificate.
//
// 본 패키지는 cert-manager 의존성을 추가하지 않고 unstructured 로 다룬다 — CRD 미설치
// 시 NoMatchError 가 자연스럽게 fail-soft.
var CertificateGVK = schema.GroupVersionKind{
	Group:   "cert-manager.io",
	Version: "v1",
	Kind:    "Certificate",
}

// IssuerKind 표준 식별자 — cert-manager 의 IssuerRef.Kind 기본값.
const IssuerKindClusterIssuer = "ClusterIssuer"

// CertificateSecretName — cert-manager 가 만들 Secret 이름.
func CertificateSecretName(crName string) string { return crName + "-tls" }

// CertificateName — Certificate CR 이름.
func CertificateName(crName string) string { return crName + "-tls" }

// BuildCertificateForValkey — Valkey 단일 인스턴스 / replication 토폴로지용 Certificate.
// 미활성 (TLS nil / Enabled=false / CertManager.IssuerRef 누락) 시 nil 반환.
func BuildCertificateForValkey(v *cachev1alpha1.Valkey) *unstructured.Unstructured {
	if v.Spec.TLS == nil || !v.Spec.TLS.Enabled || v.Spec.TLS.CertManager == nil {
		return nil
	}
	cm := v.Spec.TLS.CertManager
	if cm.IssuerRef.Name == "" {
		return nil
	}

	dnsNames := []any{
		ClientServiceName(v.Name) + "." + v.Namespace + ".svc",
		ClientServiceName(v.Name) + "." + v.Namespace + ".svc.cluster.local",
		HeadlessServiceName(v.Name) + "." + v.Namespace + ".svc",
		"*." + HeadlessServiceName(v.Name) + "." + v.Namespace + ".svc",
	}

	issuerKind := IssuerKindClusterIssuer
	if cm.IssuerRef.Kind != "" {
		issuerKind = cm.IssuerRef.Kind
	}

	spec := map[string]any{
		"secretName": CertificateSecretName(v.Name),
		"commonName": ClientServiceName(v.Name) + "." + v.Namespace + ".svc",
		"dnsNames":   dnsNames,
		"issuerRef": map[string]any{
			"name": cm.IssuerRef.Name,
			"kind": issuerKind,
		},
		"usages": []any{"server auth", "client auth"},
	}
	if cm.Duration != "" {
		spec["duration"] = cm.Duration
	}
	if cm.RenewBefore != "" {
		spec["renewBefore"] = cm.RenewBefore
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(CertificateGVK)
	u.SetName(CertificateName(v.Name))
	u.SetNamespace(v.Namespace)
	u.SetLabels(CommonLabels(v.Name, "valkey"))
	u.Object["spec"] = spec
	return u
}

// BuildCertificateForCluster — cert-manager Certificate CR. 미활성 시 nil.
//
// commonName / dnsNames 는 valkey 노드의 DNS 표면을 모두 커버:
//   - <crName>.<namespace>.svc / <crName>.<namespace>.svc.cluster.local (client Service)
//   - <crName>-headless.<namespace>.svc (headless Service)
//   - *.<crName>-headless.<namespace>.svc (StatefulSet pod)
//
// duration / renewBefore 는 Spec.TLS.CertManager 의 값을 사용하며, 미명시 시 cert-manager
// 기본값 (90d / 30d) 위임.
func BuildCertificateForCluster(vc *cachev1alpha1.ValkeyCluster) *unstructured.Unstructured {
	if vc.Spec.TLS == nil || !vc.Spec.TLS.Enabled || vc.Spec.TLS.CertManager == nil {
		return nil
	}
	cm := vc.Spec.TLS.CertManager
	if cm.IssuerRef.Name == "" {
		return nil
	}

	dnsNames := []any{
		ClientServiceName(vc.Name) + "." + vc.Namespace + ".svc",
		ClientServiceName(vc.Name) + "." + vc.Namespace + ".svc.cluster.local",
		HeadlessServiceName(vc.Name) + "." + vc.Namespace + ".svc",
		"*." + HeadlessServiceName(vc.Name) + "." + vc.Namespace + ".svc",
	}

	issuerKind := IssuerKindClusterIssuer
	if cm.IssuerRef.Kind != "" {
		issuerKind = cm.IssuerRef.Kind
	}

	spec := map[string]any{
		"secretName": CertificateSecretName(vc.Name),
		"commonName": ClientServiceName(vc.Name) + "." + vc.Namespace + ".svc",
		"dnsNames":   dnsNames,
		"issuerRef": map[string]any{
			"name": cm.IssuerRef.Name,
			"kind": issuerKind,
		},
		"usages": []any{"server auth", "client auth"},
	}
	if cm.Duration != "" {
		spec["duration"] = cm.Duration
	}
	if cm.RenewBefore != "" {
		spec["renewBefore"] = cm.RenewBefore
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(CertificateGVK)
	u.SetName(CertificateName(vc.Name))
	u.SetNamespace(vc.Namespace)
	u.SetLabels(CommonLabels(vc.Name, "valkey-cluster"))
	u.Object["spec"] = spec
	return u
}
