/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestComputeValkeyCapabilities_minimal_returns_empty(t *testing.T) {
	v := &cachev1alpha1.Valkey{}
	got := computeValkeyCapabilities(v)
	if len(got) != 0 {
		t.Errorf("minimal CR should have 0 capabilities, got %v", got)
	}
}

func TestComputeValkeyCapabilities_TLS_enabled(t *testing.T) {
	v := &cachev1alpha1.Valkey{}
	v.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
	got := computeValkeyCapabilities(v)
	if !reflect.DeepEqual(got, []string{CapabilityTLS}) {
		t.Errorf("expected [TLS], got %v", got)
	}
}

func TestComputeValkeyCapabilities_TLS_AutoCA(t *testing.T) {
	v := &cachev1alpha1.Valkey{}
	v.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:     true,
		CertManager: &cachev1alpha1.CertManagerSpec{AutoSelfSigned: true},
	}
	got := computeValkeyCapabilities(v)
	want := []string{CapabilityTLS, CapabilityTLSAutoCA}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestComputeValkeyCapabilities_full_features(t *testing.T) {
	v := &cachev1alpha1.Valkey{}
	v.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:     true,
		CertManager: &cachev1alpha1.CertManagerSpec{AutoSelfSigned: true},
	}
	v.Spec.Auth = cachev1alpha1.AuthSpec{Enabled: true}
	v.Spec.Autoscaling = &cachev1alpha1.AutoscalingSpec{Enabled: true}
	v.Spec.SlowLog = &cachev1alpha1.SlowLogSpec{ThresholdMicros: 5000}
	v.Spec.Storage = cachev1alpha1.StorageSpec{
		Size:               resource.MustParse("8Gi"),
		EncryptionRequired: true,
		EncryptionEnforce:  true,
	}
	v.Spec.NetworkPolicy = &cachev1alpha1.NetworkPolicySpec{Enabled: true}
	v.Spec.Monitoring = &cachev1alpha1.MonitoringSpec{Enabled: true}
	v.Spec.Modules = []cachev1alpha1.ModuleSpec{{Name: "valkey-search"}}

	got := computeValkeyCapabilities(v)
	want := []string{
		CapabilityTLS, CapabilityTLSAutoCA,
		CapabilityAuth, CapabilityAutoscaling,
		CapabilitySlowLog,
		CapabilityEncryptionAudit, CapabilityEncryptionEnforce,
		CapabilityNetworkPolicy, CapabilityMonitoring,
		CapabilityModules,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("full features:\n got: %v\nwant: %v", got, want)
	}
}

func TestComputeValkeyCapabilities_disabled_TLS_no_capability(t *testing.T) {
	v := &cachev1alpha1.Valkey{}
	v.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: false}
	got := computeValkeyCapabilities(v)
	if len(got) != 0 {
		t.Errorf("TLS disabled should not produce capability, got %v", got)
	}
}

func TestComputeValkeyCapabilities_SlowLog_zero_values_skipped(t *testing.T) {
	// Spec.SlowLog 명시했지만 모든 값이 0 = 사실상 default.
	v := &cachev1alpha1.Valkey{}
	v.Spec.SlowLog = &cachev1alpha1.SlowLogSpec{}
	got := computeValkeyCapabilities(v)
	if len(got) != 0 {
		t.Errorf("zero-value SlowLog should not produce capability, got %v", got)
	}
}

func TestComputeValkeyCapabilities_EncryptionEnforce_requires_Required(t *testing.T) {
	// EncryptionEnforce=true 만 있고 EncryptionRequired=false → audit 진입 안 함 → enforce 도 미발생.
	// 본 helper 의 의미적 일관성: enforce 는 audit 의 sub-mode.
	v := &cachev1alpha1.Valkey{}
	v.Spec.Storage.EncryptionEnforce = true
	got := computeValkeyCapabilities(v)
	if len(got) != 0 {
		t.Errorf("EncryptionEnforce without Required should not produce any capability, got %v", got)
	}
}

func TestComputeClusterCapabilities_full_features(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
	vc.Spec.Auth = cachev1alpha1.AuthSpec{Enabled: true}
	vc.Spec.SlowLog = &cachev1alpha1.SlowLogSpec{MaxEntries: 256}
	vc.Spec.Storage = cachev1alpha1.StorageSpec{EncryptionRequired: true}
	vc.Spec.NetworkPolicy = &cachev1alpha1.NetworkPolicySpec{Enabled: true}
	vc.Spec.Monitoring = &cachev1alpha1.MonitoringSpec{Enabled: true}
	vc.Spec.Modules = []cachev1alpha1.ModuleSpec{{Name: "valkey-search"}}

	got := computeClusterCapabilities(vc)
	want := []string{
		CapabilityTLS, CapabilityAuth,
		CapabilitySlowLog,
		CapabilityEncryptionAudit,
		CapabilityNetworkPolicy, CapabilityMonitoring,
		CapabilityModules,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
