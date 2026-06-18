/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTLSSecret(name string, crt, key, ca string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Data: map[string][]byte{
			"tls.crt": []byte(crt),
			"tls.key": []byte(key),
			"ca.crt":  []byte(ca),
		},
	}
}

// Defect ①: empty secretName → empty hash (annotation 미설정).
func TestHashTLSSecret_emptyName(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	h, err := hashTLSSecret(testCtx(), c, "ns", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if h != "" {
		t.Errorf("empty secretName should produce empty hash, got %q", h)
	}
}

// Secret 미존재 → fail-soft (빈 hash, no error). cert-manager 가 아직 미발급한 경우.
func TestHashTLSSecret_notFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	h, err := hashTLSSecret(testCtx(), c, "ns", "missing")
	if err != nil {
		t.Fatalf("missing secret should be fail-soft, got err: %v", err)
	}
	if h != "" {
		t.Errorf("missing secret → empty hash, got %q", h)
	}
}

// 존재하는 Secret → 비어 있지 않은 deterministic hash.
func TestHashTLSSecret_stableAndDeterministic(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	sec := newTLSSecret("vk-tls", "CERT", "KEY", "CA")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()
	h1, err := hashTLSSecret(testCtx(), c, "ns", "vk-tls")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if h1 == "" {
		t.Fatal("expected non-empty hash for existing secret")
	}
	h2, _ := hashTLSSecret(testCtx(), c, "ns", "vk-tls")
	if h1 != h2 {
		t.Errorf("hash must be deterministic: %q != %q", h1, h2)
	}
}

// Defect ① 핵심: Secret 내용 변경(예: CA rotation) 시 hash 가 변경되어야 함.
func TestHashTLSSecret_changesWhenContentChanges(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	build := func(crt, key, ca string) string {
		sec := newTLSSecret("vk-tls", crt, key, ca)
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()
		h, err := hashTLSSecret(testCtx(), c, "ns", "vk-tls")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		return h
	}

	base := build("CERT", "KEY", "CA")
	cases := map[string]string{
		"cert rotated": build("CERT2", "KEY", "CA"),
		"key rotated":  build("CERT", "KEY2", "CA"),
		"CA rotated":   build("CERT", "KEY", "CA2"),
	}
	for name, h := range cases {
		if h == base {
			t.Errorf("%s: hash must change when secret content changes (got same as base)", name)
		}
	}
}
