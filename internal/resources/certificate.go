/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/keiailab/keiailab-commons/pkg/certmanager"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// CertificateGVK — cert-manager.io/v1 Certificate. commons 선언 alias —
// cert-manager 의존성 없이 unstructured 로 다룬다 (CRD 미설치 시 NoMatchError
// 가 자연스럽게 fail-soft).
var CertificateGVK = certmanager.CertificateGVK

// IssuerGVK — cert-manager.io/v1 Issuer (namespace-scope). commons 선언 alias.
// AutoSelfSigned 옵션에서 operator 가 자동 생성.
var IssuerGVK = certmanager.IssuerGVK

// IssuerKind 표준 식별자 — cert-manager 의 IssuerRef.Kind 기본값.
const IssuerKindClusterIssuer = "ClusterIssuer"

// IssuerKindIssuer — namespace-scope Issuer (AutoSelfSigned 시 사용).
const IssuerKindIssuer = "Issuer"

// CertificateSecretName — cert-manager 가 만들 Secret 이름.
func CertificateSecretName(crName string) string { return crName + "-tls" }

// CertificateName — Certificate CR 이름.
func CertificateName(crName string) string { return crName + "-tls" }

// SelfSignedIssuerName — AutoSelfSigned 시 자동 생성하는 Issuer 이름.
func SelfSignedIssuerName(crName string) string { return crName + "-selfsigned" }

// BuildSelfSignedIssuer — Spec.TLS.CertManager.AutoSelfSigned=true 일 때 namespace
// 에 자동 생성하는 cert-manager Issuer (kind=Issuer, spec.selfSigned: {}).
//
// 골격은 commons certmanager.BuildSelfSignedIssuer — name 관례
// ("<crName>-selfsigned") + label 셋만 본 함수가 결정.
func BuildSelfSignedIssuer(crName, namespace string) *unstructured.Unstructured {
	return certmanager.BuildSelfSignedIssuer(
		SelfSignedIssuerName(crName), namespace, CommonLabels(crName, "valkey"))
}

// valkeyCertDNSNames — Valkey/ValkeyCluster 공통의 DNS SAN 표면:
//   - <client-svc>.<ns>.svc / <client-svc>.<ns>.svc.cluster.local (client Service)
//   - <headless-svc>.<ns>.svc (headless Service)
//   - *.<headless-svc>.<ns>.svc (StatefulSet pod)
//
// SAN 형태가 commons certmanager.ServiceSANs 의 4단 FQDN 확장과 다르므로
// (svc 단독/2단 미포함 + headless 와일드카드) 조립은 로컬 도메인 책임 유지.
func valkeyCertDNSNames(crName, namespace string) []string {
	return []string{
		ClientServiceName(crName) + "." + namespace + ".svc",
		ClientServiceName(crName) + "." + namespace + ".svc.cluster.local",
		HeadlessServiceName(crName) + "." + namespace + ".svc",
		"*." + HeadlessServiceName(crName) + "." + namespace + ".svc",
	}
}

// buildValkeyCertificate — Valkey/ValkeyCluster 공용 Certificate 빌드 골격.
// commons certmanager.BuildCertificate 위임 (기존 Valkey/Cluster 두 함수의
// 동일 본문 중복을 commons 화로 동시 해소).
func buildValkeyCertificate(crName, namespace, component string, cm *cachev1alpha1.CertManagerSpec) *unstructured.Unstructured {
	issuerName, issuerKind := resolveIssuer(crName, cm)
	if issuerName == "" {
		return nil
	}
	return certmanager.BuildCertificate(certmanager.CertParams{
		Name:        CertificateName(crName),
		Namespace:   namespace,
		Labels:      CommonLabels(crName, component),
		SecretName:  CertificateSecretName(crName),
		CommonName:  ClientServiceName(crName) + "." + namespace + ".svc",
		DNSNames:    valkeyCertDNSNames(crName, namespace),
		IssuerName:  issuerName,
		IssuerKind:  issuerKind, // resolveIssuer 가 항상 비-빈 값 반환 — commons fallback 미발동.
		Duration:    cm.Duration,
		RenewBefore: cm.RenewBefore,
		// ECDSAPrivateKey=false — privateKey 블록 미발행 (cert-manager default 위임,
		// 기존 valkey 출력 보존).
	})
}

// BuildCertificateForValkey — Valkey 단일 인스턴스 / replication 토폴로지용 Certificate.
// 미활성 (TLS nil / Enabled=false / CertManager.IssuerRef 누락) 시 nil 반환.
func BuildCertificateForValkey(v *cachev1alpha1.Valkey) *unstructured.Unstructured {
	if v.Spec.TLS == nil || !v.Spec.TLS.Enabled || v.Spec.TLS.CertManager == nil {
		return nil
	}
	return buildValkeyCertificate(v.Name, v.Namespace, "valkey", v.Spec.TLS.CertManager)
}

// resolveIssuer — Spec.TLS.CertManager 에서 사용할 issuer name + kind 결정.
// AutoSelfSigned=true 시 operator 가 자동 생성한 namespace-scope Issuer 를 사용.
// 그 외엔 사용자 명시 IssuerRef 사용. 둘 다 비어있으면 ("", "") 반환 → 호출자가 nil.
func resolveIssuer(crName string, cm *cachev1alpha1.CertManagerSpec) (name, kind string) {
	if cm.AutoSelfSigned {
		return SelfSignedIssuerName(crName), IssuerKindIssuer
	}
	if cm.IssuerRef.Name == "" {
		return "", ""
	}
	if cm.IssuerRef.Kind != "" {
		return cm.IssuerRef.Name, cm.IssuerRef.Kind
	}
	return cm.IssuerRef.Name, IssuerKindClusterIssuer
}

// BuildCertificateForCluster — cert-manager Certificate CR. 미활성 시 nil.
//
// commonName / dnsNames 는 valkey 노드의 DNS 표면을 모두 커버 (valkeyCertDNSNames).
// duration / renewBefore 는 Spec.TLS.CertManager 의 값을 사용하며, 미명시 시 cert-manager
// 기본값 (90d / 30d) 위임.
func BuildCertificateForCluster(vc *cachev1alpha1.ValkeyCluster) *unstructured.Unstructured {
	if vc.Spec.TLS == nil || !vc.Spec.TLS.Enabled || vc.Spec.TLS.CertManager == nil {
		return nil
	}
	return buildValkeyCertificate(vc.Name, vc.Namespace, "valkey-cluster", vc.Spec.TLS.CertManager)
}
