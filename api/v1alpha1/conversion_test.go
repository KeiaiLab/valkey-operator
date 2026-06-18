/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package v1alpha1_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/api/v1alpha2"
)

// TestValkey_ConvertTo_v1alpha2 — v1alpha1.Valkey → v1alpha2.Valkey
// JSON byte-copy 매핑 round-trip 검증.
//
// AAA 형식:
//
//	Arrange — v1alpha1.Valkey 인스턴스 (Mode + Replicas + Auth.Enabled).
//	Act — ConvertTo(v1alpha2.Valkey 빈 인스턴스).
//	Assert — Spec 동일 필드 매핑 + ObjectMeta 보존.
func TestValkey_ConvertTo_v1alpha2(t *testing.T) {
	src := &v1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-valkey",
			Namespace: "default",
		},
		Spec: v1alpha1.ValkeySpec{
			Mode:     v1alpha1.ModeReplication,
			Replicas: 3,
			Auth: v1alpha1.AuthSpec{
				Enabled: true,
			},
		},
	}
	dst := &v1alpha2.Valkey{}

	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("ConvertTo failed: %v", err)
	}

	if dst.Name != "test-valkey" || dst.Namespace != "default" {
		t.Errorf("ObjectMeta 보존 실패: name=%q ns=%q", dst.Name, dst.Namespace)
	}
	if dst.Spec.Replicas != 3 {
		t.Errorf("Replicas = %d, want 3", dst.Spec.Replicas)
	}
	if dst.Spec.Mode != v1alpha2.ModeReplication {
		t.Errorf("Mode = %v, want Replication", dst.Spec.Mode)
	}
	if !dst.Spec.Auth.Enabled {
		t.Error("Auth.Enabled = false, want true")
	}
	// v1alpha2 신규 필드는 nil — controller default 적용 (kubebuilder).
	if dst.Spec.Auth.Required != nil {
		t.Errorf("Auth.Required = %v, want nil (controller default)", *dst.Spec.Auth.Required)
	}
}

// TestValkey_ConvertFrom_v1alpha2 — 역방향 round-trip.
func TestValkey_ConvertFrom_v1alpha2(t *testing.T) {
	required := true
	src := &v1alpha2.Valkey{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-valkey-v2",
			Namespace: "default",
		},
		Spec: v1alpha2.ValkeySpec{
			Mode:     v1alpha2.ModeReplication,
			Replicas: 5,
			Auth: v1alpha2.AuthSpec{
				Enabled:        true,
				Required:       &required,
				RotationPolicy: "OnSecretChange",
			},
		},
	}
	dst := &v1alpha1.Valkey{}

	if err := dst.ConvertFrom(src); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}

	if dst.Name != "test-valkey-v2" {
		t.Errorf("ObjectMeta 보존 실패: name=%q", dst.Name)
	}
	if dst.Spec.Replicas != 5 {
		t.Errorf("Replicas = %d, want 5", dst.Spec.Replicas)
	}
	if !dst.Spec.Auth.Enabled {
		t.Error("Auth.Enabled = false, want true")
	}
	// v1alpha2 신규 필드 (Required, RotationPolicy) 는 v1alpha1 에 부재 —
	// 정보 손실 허용. v1alpha1 의 표면은 변경 안 됨.
}

// TestValkeyCluster_ConvertTo_v1alpha2 — sharded cluster type round-trip.
func TestValkeyCluster_ConvertTo_v1alpha2(t *testing.T) {
	src := &v1alpha1.ValkeyCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-test"},
		Spec: v1alpha1.ValkeyClusterSpec{
			Shards:           3,
			ReplicasPerShard: 0, // defect ④: masters-only 명시 0 round-trip.
			// defect ⑥: cluster modules round-trip (v1alpha1 ↔ v1alpha2).
			Modules: []v1alpha1.ModuleSpec{{Name: "valkey-search"}},
		},
	}
	dst := &v1alpha2.ValkeyCluster{}

	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("ConvertTo failed: %v", err)
	}
	if dst.Spec.Shards != 3 {
		t.Errorf("Shards = %d, want 3", dst.Spec.Shards)
	}
	if dst.Spec.ReplicasPerShard != 0 {
		t.Errorf("ReplicasPerShard = %d, want 0 (masters-only must round-trip)", dst.Spec.ReplicasPerShard)
	}
	if len(dst.Spec.Modules) != 1 || dst.Spec.Modules[0].Name != "valkey-search" {
		t.Errorf("Modules = %+v, want [valkey-search]", dst.Spec.Modules)
	}

	// 역방향 round-trip — v1alpha2 → v1alpha1 도 보존.
	back := &v1alpha1.ValkeyCluster{}
	if err := back.ConvertFrom(dst); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}
	if back.Spec.ReplicasPerShard != 0 {
		t.Errorf("round-trip ReplicasPerShard = %d, want 0", back.Spec.ReplicasPerShard)
	}
	if len(back.Spec.Modules) != 1 || back.Spec.Modules[0].Name != "valkey-search" {
		t.Errorf("round-trip Modules = %+v, want [valkey-search]", back.Spec.Modules)
	}
}
