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
package resources

import (
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestBuildSelfSignedIssuer_minimal(t *testing.T) {
	u := BuildSelfSignedIssuer("vc-prod", "valkey")

	if u.GetName() != "vc-prod-selfsigned" {
		t.Errorf("name: got %q want vc-prod-selfsigned", u.GetName())
	}
	if u.GetNamespace() != "valkey" {
		t.Errorf("namespace: got %q", u.GetNamespace())
	}
	if u.GetKind() != "Issuer" || u.GetAPIVersion() != "cert-manager.io/v1" {
		t.Errorf("GVK: got %s/%s want cert-manager.io/v1/Issuer", u.GetAPIVersion(), u.GetKind())
	}
	spec, ok := u.Object["spec"].(map[string]any)
	if !ok {
		t.Fatalf("spec not a map: %T", u.Object["spec"])
	}
	if _, ok := spec["selfSigned"]; !ok {
		t.Errorf("spec.selfSigned 미설정")
	}
}

func TestBuildCertificateForValkey_AutoSelfSigned_uses_auto_issuer(t *testing.T) {
	v := &cachev1alpha1.Valkey{}
	v.Name = "vk-app"
	v.Namespace = "ns"
	v.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled: true,
		CertManager: &cachev1alpha1.CertManagerSpec{
			AutoSelfSigned: true,
		},
	}

	cert := BuildCertificateForValkey(v)
	if cert == nil {
		t.Fatal("expected Certificate, got nil")
	}
	spec := cert.Object["spec"].(map[string]any)
	issuerRef := spec["issuerRef"].(map[string]any)
	if got := issuerRef["name"]; got != "vk-app-selfsigned" {
		t.Errorf("issuerRef.name: got %v want vk-app-selfsigned", got)
	}
	if got := issuerRef["kind"]; got != "Issuer" {
		t.Errorf("issuerRef.kind: got %v want Issuer", got)
	}
}

func TestBuildCertificateForCluster_AutoSelfSigned_uses_auto_issuer(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vc-prod"
	vc.Namespace = "valkey"
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled: true,
		CertManager: &cachev1alpha1.CertManagerSpec{
			AutoSelfSigned: true,
		},
	}

	cert := BuildCertificateForCluster(vc)
	if cert == nil {
		t.Fatal("expected Certificate, got nil")
	}
	spec := cert.Object["spec"].(map[string]any)
	issuerRef := spec["issuerRef"].(map[string]any)
	if got := issuerRef["name"]; got != "vc-prod-selfsigned" {
		t.Errorf("issuerRef.name: got %v want vc-prod-selfsigned", got)
	}
	if got := issuerRef["kind"]; got != "Issuer" {
		t.Errorf("issuerRef.kind: got %v want Issuer", got)
	}
}

func TestBuildCertificateForCluster_explicit_IssuerRef_unchanged(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vc-prod"
	vc.Namespace = "valkey"
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled: true,
		CertManager: &cachev1alpha1.CertManagerSpec{
			IssuerRef: cachev1alpha1.CertIssuerRef{
				Name: "letsencrypt-prod",
				Kind: "ClusterIssuer",
			},
		},
	}

	cert := BuildCertificateForCluster(vc)
	if cert == nil {
		t.Fatal("expected Certificate")
	}
	issuerRef := cert.Object["spec"].(map[string]any)["issuerRef"].(map[string]any)
	if issuerRef["name"] != "letsencrypt-prod" || issuerRef["kind"] != "ClusterIssuer" {
		t.Errorf("explicit IssuerRef changed: %v", issuerRef)
	}
}

func TestResolveIssuer_priority_AutoSelfSigned_over_IssuerRef(t *testing.T) {
	// AutoSelfSigned=true 면 IssuerRef.Name 도 무시 (webhook 가 사전에 reject 하지만
	// builder 단독 호출 안전성 보장).
	cm := &cachev1alpha1.CertManagerSpec{
		AutoSelfSigned: true,
		IssuerRef:      cachev1alpha1.CertIssuerRef{Name: "external", Kind: "ClusterIssuer"},
	}
	name, kind := resolveIssuer("cr", cm)
	if name != "cr-selfsigned" || kind != "Issuer" {
		t.Errorf("AutoSelfSigned 우선: got %s/%s want cr-selfsigned/Issuer", name, kind)
	}
}

func TestResolveIssuer_default_kind_ClusterIssuer(t *testing.T) {
	cm := &cachev1alpha1.CertManagerSpec{
		IssuerRef: cachev1alpha1.CertIssuerRef{Name: "external"}, // Kind 미명시.
	}
	name, kind := resolveIssuer("cr", cm)
	if name != "external" || kind != "ClusterIssuer" {
		t.Errorf("default Kind: got %s/%s want external/ClusterIssuer", name, kind)
	}
}

func TestResolveIssuer_empty_returns_zero(t *testing.T) {
	cm := &cachev1alpha1.CertManagerSpec{} // 둘 다 미설정.
	name, kind := resolveIssuer("cr", cm)
	if name != "" || kind != "" {
		t.Errorf("empty: got %q/%q want empty", name, kind)
	}
}
