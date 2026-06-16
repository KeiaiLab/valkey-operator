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
			ReplicasPerShard: 1,
		},
	}
	dst := &v1alpha2.ValkeyCluster{}

	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("ConvertTo failed: %v", err)
	}
	if dst.Spec.Shards != 3 {
		t.Errorf("Shards = %d, want 3", dst.Spec.Shards)
	}
	if dst.Spec.ReplicasPerShard != 1 {
		t.Errorf("ReplicasPerShard = %d, want 1", dst.Spec.ReplicasPerShard)
	}
}

// TestValkey_Modules_RoundTrip_보존 — v1alpha2 Modules 가 v1alpha1 storage
// 왕복(hub→spoke→hub)에서 보존되는지 검증.
//
// 배경: 컨트롤러/webhook/storage 모두 v1alpha1 이고, conversion webhook
// 미연결 상태 (PR-C6.2). v1alpha1.ValkeySpec 에 Modules 필드가 부재하면
// convertViaJSON(JSON byte-copy)이 hub→spoke 변환 시 Modules 를 silent drop
// → 왕복 후 손실. ADR-0062: Modules 를 v1alpha1 에도 미러링하여 byte-copy 가
// 자동 보존하도록 한다.
//
// AAA:
//
//	Arrange — v1alpha2.Valkey 에 official preset + BYO module 2개.
//	Act — ConvertFrom(hub→spoke) → ConvertTo(spoke→hub) 왕복.
//	Assert — Modules 2개 + 각 필드(Name/Image/LoadModuleArgs) 보존.
func TestValkey_Modules_RoundTrip_보존(t *testing.T) {
	src := &v1alpha2.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: "vk-mod", Namespace: "default"},
		Spec: v1alpha2.ValkeySpec{
			Mode:     v1alpha2.ModeStandalone,
			Replicas: 1,
			Modules: []v1alpha2.ModuleSpec{
				{Name: "valkey-search"},
				{Name: "my-mod", Image: "example.com/mod:1", LoadModuleArgs: []string{"--foo", "bar"}},
			},
		},
	}

	// hub(v1alpha2) → spoke(v1alpha1): storage 변환.
	spoke := &v1alpha1.Valkey{}
	if err := spoke.ConvertFrom(src); err != nil {
		t.Fatalf("ConvertFrom failed: %v", err)
	}
	// spoke(v1alpha1) → hub(v1alpha2): serving 변환.
	back := &v1alpha2.Valkey{}
	if err := spoke.ConvertTo(back); err != nil {
		t.Fatalf("ConvertTo failed: %v", err)
	}

	if len(back.Spec.Modules) != 2 {
		t.Fatalf("Modules 손실: got %d, want 2", len(back.Spec.Modules))
	}
	if back.Spec.Modules[0].Name != "valkey-search" {
		t.Errorf("Modules[0].Name = %q, want valkey-search", back.Spec.Modules[0].Name)
	}
	if back.Spec.Modules[1].Image != "example.com/mod:1" {
		t.Errorf("Modules[1].Image = %q, want example.com/mod:1", back.Spec.Modules[1].Image)
	}
	if len(back.Spec.Modules[1].LoadModuleArgs) != 2 || back.Spec.Modules[1].LoadModuleArgs[0] != "--foo" {
		t.Errorf("Modules[1].LoadModuleArgs = %v, want [--foo bar]", back.Spec.Modules[1].LoadModuleArgs)
	}
}
