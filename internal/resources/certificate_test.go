/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"reflect"
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestBuildCertificate_disabled_returnsNil(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	if got := BuildCertificateForCluster(vc); got != nil {
		t.Errorf("nil TLS → expected nil, got %+v", got)
	}

	vc.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
	if got := BuildCertificateForCluster(vc); got != nil {
		t.Errorf("no CertManager → expected nil")
	}

	vc.Spec.TLS.CertManager = &cachev1alpha1.CertManagerSpec{} // IssuerRef.Name 비어있음
	if got := BuildCertificateForCluster(vc); got != nil {
		t.Errorf("empty IssuerRef.Name → expected nil")
	}
}

func TestBuildCertificate_enabled_clusterIssuer_default(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled: true,
		CertManager: &cachev1alpha1.CertManagerSpec{
			IssuerRef: cachev1alpha1.CertIssuerRef{Name: "letsencrypt"},
		},
	}

	got := BuildCertificateForCluster(vc)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.GetName() != "vk-tls" {
		t.Errorf("name: %q want vk-tls", got.GetName())
	}
	if got.GroupVersionKind().Kind != "Certificate" {
		t.Errorf("kind: %v", got.GroupVersionKind())
	}

	spec, _ := got.Object["spec"].(map[string]any)
	if spec["secretName"] != "vk-tls" {
		t.Errorf("secretName: %v", spec["secretName"])
	}
	if spec["commonName"] != "vk.ns.svc" {
		t.Errorf("commonName: %v", spec["commonName"])
	}
	issuerRef, _ := spec["issuerRef"].(map[string]any)
	if issuerRef["kind"] != "ClusterIssuer" {
		t.Errorf("default kind: %v want ClusterIssuer", issuerRef["kind"])
	}
	if issuerRef["name"] != "letsencrypt" {
		t.Errorf("issuer name: %v", issuerRef["name"])
	}

	dnsNames, _ := spec["dnsNames"].([]any)
	wantDNS := []any{
		"vk.ns.svc",
		"vk.ns.svc.cluster.local",
		"vk-headless.ns.svc",
		"*.vk-headless.ns.svc",
	}
	if !reflect.DeepEqual(dnsNames, wantDNS) {
		t.Errorf("dnsNames: got %v want %v", dnsNames, wantDNS)
	}
}

func TestBuildCertificate_namespace_issuer(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled: true,
		CertManager: &cachev1alpha1.CertManagerSpec{
			IssuerRef:   cachev1alpha1.CertIssuerRef{Name: "ns-issuer", Kind: "Issuer"},
			Duration:    "8760h",
			RenewBefore: "720h",
		},
	}
	got := BuildCertificateForCluster(vc)
	spec, _ := got.Object["spec"].(map[string]any)
	issuerRef, _ := spec["issuerRef"].(map[string]any)
	if issuerRef["kind"] != "Issuer" {
		t.Errorf("kind: %v", issuerRef["kind"])
	}
	if spec["duration"] != "8760h" {
		t.Errorf("duration: %v", spec["duration"])
	}
	if spec["renewBefore"] != "720h" {
		t.Errorf("renewBefore: %v", spec["renewBefore"])
	}
}

func TestCertificateNames(t *testing.T) {
	if CertificateName("vk") != "vk-tls" {
		t.Errorf("certificate name: %s", CertificateName("vk"))
	}
	if CertificateSecretName("vk") != "vk-tls" {
		t.Errorf("certificate secret name: %s", CertificateSecretName("vk"))
	}
}
