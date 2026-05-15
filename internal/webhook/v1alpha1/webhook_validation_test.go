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

// 직접 호출 단위테스트 — Ginkgo BDD 의 ValkeyCluster Webhook describe 와 별도.
// 함수 단위 검증 으로 빠른 실행 + 명확한 실패 위치.
package v1alpha1

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestValkeyClusterValidate_AutoFailover_requires_replicas(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.AutoFailover = true
	vc.Spec.ReplicasPerShard = 0 // 모순 — autoFailover 가 replica 없이 동작 불가.
	vc.Spec.Version.Version = "8.1.6"

	if _, err := v.ValidateCreate(context.Background(), vc); err == nil {
		t.Fatal("expected validation error for AutoFailover=true + ReplicasPerShard=0")
	}
}

func TestValkeyClusterValidate_total_node_limit(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 50
	vc.Spec.ReplicasPerShard = 5 // 50 * 6 = 300 > 100.

	if _, err := v.ValidateCreate(context.Background(), vc); err == nil {
		t.Fatal("expected validation error for total > 100")
	}
}

func TestValkeyClusterValidate_TLS_requires_certManager_or_customCert(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true} // 둘 다 미명시.

	if _, err := v.ValidateCreate(context.Background(), vc); err == nil {
		t.Fatal("expected validation error for TLS Enabled without CA")
	}
}

func TestValkeyClusterValidate_TLS_mutually_exclusive(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:     true,
		CertManager: &cachev1alpha1.CertManagerSpec{IssuerRef: cachev1alpha1.CertIssuerRef{Name: "issuer"}},
		CustomCert:  &cachev1alpha1.CustomCertSpec{SecretName: "ca"},
	}

	if _, err := v.ValidateCreate(context.Background(), vc); err == nil {
		t.Fatal("expected validation error for both certManager + customCert")
	}
}

func TestValkeyClusterValidate_Auth_users_requires_enabled(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.Auth = cachev1alpha1.AuthSpec{
		Enabled: false,
		Users: []cachev1alpha1.ValkeyUser{
			{Name: "alice"},
		},
	}

	if _, err := v.ValidateCreate(context.Background(), vc); err == nil {
		t.Fatal("expected validation error for users without Auth.Enabled")
	}
}

func TestValkeyClusterValidate_Update_storageClass_immutable(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	old := &cachev1alpha1.ValkeyCluster{}
	old.Spec.Shards = 3
	old.Spec.ReplicasPerShard = 1
	old.Spec.Storage.StorageClassName = "fast-ssd"
	new := old.DeepCopy()
	new.Spec.Storage.StorageClassName = "slow-hdd"

	if _, err := v.ValidateUpdate(context.Background(), old, new); err == nil {
		t.Fatal("expected validation error for storageClassName change")
	}
}

// TLS.Enabled false → true 는 허용 (Defaulter 가 spec.tls 의도 노출 시 정규화 정합).
// 단방향 immutability: true → false 만 reject (mTLS client 연결 끊김 회피).
func TestValkeyClusterValidate_Update_tls_false_to_true_allowed(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	old := &cachev1alpha1.ValkeyCluster{}
	old.Spec.Shards = 3
	old.Spec.ReplicasPerShard = 1
	old.Spec.Version.Version = "8.1.6"
	old.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: false}
	new := old.DeepCopy()
	new.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:    true,
		CustomCert: &cachev1alpha1.CustomCertSpec{SecretName: "ca"},
	}

	if _, err := v.ValidateUpdate(context.Background(), old, new); err != nil {
		t.Fatalf("false → true 허용 기대, got: %v", err)
	}
}

func TestValkeyClusterValidate_Update_tls_true_to_false_forbidden(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	old := &cachev1alpha1.ValkeyCluster{}
	old.Spec.Shards = 3
	old.Spec.ReplicasPerShard = 1
	old.Spec.Version.Version = "8.1.6"
	old.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:    true,
		CustomCert: &cachev1alpha1.CustomCertSpec{SecretName: "ca"},
	}
	new := old.DeepCopy()
	new.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: false}

	if _, err := v.ValidateUpdate(context.Background(), old, new); err == nil {
		t.Fatal("true → false reject 기대")
	}
}

func TestValkeyClusterValidate_valid_passes(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.AutoFailover = true
	vc.Spec.Version.Version = "8.1.6"

	if _, err := v.ValidateCreate(context.Background(), vc); err != nil {
		t.Fatalf("valid spec should pass: %v", err)
	}
}

// Defaulter — SlotMigration 빈 → Auto.
func TestValkeyClusterDefaulter_slotMigration(t *testing.T) {
	d := &ValkeyClusterCustomDefaulter{}
	vc := &cachev1alpha1.ValkeyCluster{}
	if err := d.Default(context.Background(), vc); err != nil {
		t.Fatalf("default: %v", err)
	}
	if vc.Spec.SlotMigration != cachev1alpha1.SlotMigrationAuto {
		t.Errorf("SlotMigration: got %q want Auto", vc.Spec.SlotMigration)
	}
}

func TestValkeyClusterDefaulter_preserve_explicit_slotMigration(t *testing.T) {
	d := &ValkeyClusterCustomDefaulter{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.SlotMigration = cachev1alpha1.SlotMigrationManual
	_ = d.Default(context.Background(), vc)
	if vc.Spec.SlotMigration != cachev1alpha1.SlotMigrationManual {
		t.Errorf("explicit Manual should be preserved: got %q", vc.Spec.SlotMigration)
	}
}

// Valkey webhook — Mode immutable.
func TestValkey_Validate_Mode_immutable(t *testing.T) {
	v := &ValkeyCustomValidator{}
	old := &cachev1alpha1.Valkey{}
	old.Spec.Mode = cachev1alpha1.ModeStandalone
	old.Spec.Replicas = 1
	old.Spec.Version.Version = "8.1.6"
	new := old.DeepCopy()
	new.Spec.Mode = cachev1alpha1.ModeReplication
	new.Spec.Replicas = 3

	if _, err := v.ValidateUpdate(context.Background(), old, new); err == nil {
		t.Fatal("expected validation error for Mode change")
	}
}

func TestValkey_Validate_Standalone_replicas_eq_1(t *testing.T) {
	v := &ValkeyCustomValidator{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeStandalone
	vc.Spec.Replicas = 3 // 모순.
	vc.Spec.Version.Version = "8.1.6"

	if _, err := v.ValidateCreate(context.Background(), vc); err == nil {
		t.Fatal("expected validation error for Standalone + Replicas > 1")
	}
}

func TestValkey_Validate_Replication_replicas_min_2(t *testing.T) {
	v := &ValkeyCustomValidator{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeReplication
	vc.Spec.Replicas = 1 // 모순.

	if _, err := v.ValidateCreate(context.Background(), vc); err == nil {
		t.Fatal("expected validation error for Replication + Replicas < 2")
	}
}

// Defaulter — Standalone 강제 Replicas=1.
func TestValkeyDefaulter_standalone_replicas_forced(t *testing.T) {
	d := &ValkeyCustomDefaulter{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeStandalone
	vc.Spec.Replicas = 5
	_ = d.Default(context.Background(), vc)
	if vc.Spec.Replicas != 1 {
		t.Errorf("Standalone should force Replicas=1, got %d", vc.Spec.Replicas)
	}
}

// Defaulter — Replication + Replicas<2 → 2.
func TestValkeyDefaulter_replication_min_2(t *testing.T) {
	d := &ValkeyCustomDefaulter{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeReplication
	vc.Spec.Replicas = 1
	_ = d.Default(context.Background(), vc)
	if vc.Spec.Replicas != 2 {
		t.Errorf("Replication should default Replicas=2, got %d", vc.Spec.Replicas)
	}
}

func TestValkeyClusterValidate_Update_storageSize_shrink_rejected(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	old := &cachev1alpha1.ValkeyCluster{}
	old.Spec.Shards = 3
	old.Spec.ReplicasPerShard = 1
	old.Spec.Version.Version = "8.1.6"
	old.Spec.Storage.Size = resource.MustParse("16Gi")
	new := old.DeepCopy()
	new.Spec.Storage.Size = resource.MustParse("8Gi")

	_, err := v.ValidateUpdate(context.Background(), old, new)
	if err == nil {
		t.Fatal("expected validation error for storage.size decrease")
	}
	if !strings.Contains(err.Error(), "storage.size cannot be decreased") {
		t.Errorf("error message should mention shrink, got: %v", err)
	}
}

func TestValkeyClusterValidate_Update_storageSize_grow_accepted(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	old := &cachev1alpha1.ValkeyCluster{}
	old.Spec.Shards = 3
	old.Spec.ReplicasPerShard = 1
	old.Spec.Version.Version = "8.1.6"
	old.Spec.Storage.Size = resource.MustParse("8Gi")
	new := old.DeepCopy()
	new.Spec.Storage.Size = resource.MustParse("16Gi")

	if _, err := v.ValidateUpdate(context.Background(), old, new); err != nil {
		t.Fatalf("expected no error for storage.size grow, got %v", err)
	}
}

func TestValkeyValidate_Update_storageSize_shrink_rejected(t *testing.T) {
	v := &ValkeyCustomValidator{}
	old := &cachev1alpha1.Valkey{}
	old.Spec.Mode = cachev1alpha1.ModeStandalone
	old.Spec.Replicas = 1
	old.Spec.Version.Version = "8.1.6"
	old.Spec.Storage.Size = resource.MustParse("16Gi")
	new := old.DeepCopy()
	new.Spec.Storage.Size = resource.MustParse("8Gi")

	_, err := v.ValidateUpdate(context.Background(), old, new)
	if err == nil {
		t.Fatal("expected validation error for storage.size decrease")
	}
	if !strings.Contains(err.Error(), "storage.size cannot be decreased") {
		t.Errorf("error message should mention shrink, got: %v", err)
	}
}

func TestValkeyValidate_Update_storageSize_grow_accepted(t *testing.T) {
	v := &ValkeyCustomValidator{}
	old := &cachev1alpha1.Valkey{}
	old.Spec.Mode = cachev1alpha1.ModeStandalone
	old.Spec.Replicas = 1
	old.Spec.Version.Version = "8.1.6"
	old.Spec.Storage.Size = resource.MustParse("8Gi")
	new := old.DeepCopy()
	new.Spec.Storage.Size = resource.MustParse("16Gi")

	if _, err := v.ValidateUpdate(context.Background(), old, new); err != nil {
		t.Fatalf("expected no error for storage.size grow, got %v", err)
	}
}

func TestValkeyCluster_TLS_AutoSelfSigned_alone_passes(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.Version.Version = "8.1.6"
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:     true,
		CertManager: &cachev1alpha1.CertManagerSpec{AutoSelfSigned: true},
	}

	if _, err := v.ValidateCreate(context.Background(), vc); err != nil {
		t.Fatalf("AutoSelfSigned alone should pass: %v", err)
	}
}

func TestValkeyCluster_TLS_AutoSelfSigned_with_IssuerRef_rejected(t *testing.T) {
	v := &ValkeyClusterCustomValidator{}
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.Version.Version = "8.1.6"
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled: true,
		CertManager: &cachev1alpha1.CertManagerSpec{
			AutoSelfSigned: true,
			IssuerRef:      cachev1alpha1.CertIssuerRef{Name: "external"},
		},
	}

	_, err := v.ValidateCreate(context.Background(), vc)
	if err == nil {
		t.Fatal("expected validation error for AutoSelfSigned + IssuerRef.Name simultaneous")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error message should mention mutual exclusion, got: %v", err)
	}
}

func TestValkey_TLS_AutoSelfSigned_alone_passes(t *testing.T) {
	v := &ValkeyCustomValidator{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeStandalone
	vc.Spec.Replicas = 1
	vc.Spec.Version.Version = "8.1.6"
	vc.Spec.Storage.Size = resource.MustParse("8Gi")
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:     true,
		CertManager: &cachev1alpha1.CertManagerSpec{AutoSelfSigned: true},
	}

	if _, err := v.ValidateCreate(context.Background(), vc); err != nil {
		t.Fatalf("AutoSelfSigned alone should pass: %v", err)
	}
}

func TestValkey_TLS_AutoSelfSigned_with_IssuerRef_rejected(t *testing.T) {
	v := &ValkeyCustomValidator{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeStandalone
	vc.Spec.Replicas = 1
	vc.Spec.Version.Version = "8.1.6"
	vc.Spec.Storage.Size = resource.MustParse("8Gi")
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled: true,
		CertManager: &cachev1alpha1.CertManagerSpec{
			AutoSelfSigned: true,
			IssuerRef:      cachev1alpha1.CertIssuerRef{Name: "external"},
		},
	}

	_, err := v.ValidateCreate(context.Background(), vc)
	if err == nil {
		t.Fatal("expected validation error for AutoSelfSigned + IssuerRef.Name simultaneous")
	}
}

func TestValkey_Autoscaling_replication_passes(t *testing.T) {
	v := &ValkeyCustomValidator{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeReplication
	vc.Spec.Replicas = 2
	vc.Spec.Version.Version = "8.1.6"
	vc.Spec.Storage.Size = resource.MustParse("8Gi")
	vc.Spec.Autoscaling = &cachev1alpha1.AutoscalingSpec{
		Enabled:     true,
		MinReplicas: 2,
		MaxReplicas: 5,
	}
	if _, err := v.ValidateCreate(context.Background(), vc); err != nil {
		t.Fatalf("autoscaling Replication: %v", err)
	}
}

func TestValkey_Autoscaling_standalone_rejected(t *testing.T) {
	v := &ValkeyCustomValidator{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeStandalone
	vc.Spec.Replicas = 1
	vc.Spec.Version.Version = "8.1.6"
	vc.Spec.Storage.Size = resource.MustParse("8Gi")
	vc.Spec.Autoscaling = &cachev1alpha1.AutoscalingSpec{
		Enabled:     true,
		MinReplicas: 2,
		MaxReplicas: 5,
	}
	_, err := v.ValidateCreate(context.Background(), vc)
	if err == nil || !strings.Contains(err.Error(), "Replication") {
		t.Errorf("expected Replication-only reject, got %v", err)
	}
}

func TestValkey_Autoscaling_min_below_2_rejected(t *testing.T) {
	v := &ValkeyCustomValidator{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeReplication
	vc.Spec.Replicas = 2
	vc.Spec.Version.Version = "8.1.6"
	vc.Spec.Storage.Size = resource.MustParse("8Gi")
	vc.Spec.Autoscaling = &cachev1alpha1.AutoscalingSpec{
		Enabled:     true,
		MinReplicas: 1,
		MaxReplicas: 5,
	}
	_, err := v.ValidateCreate(context.Background(), vc)
	if err == nil || !strings.Contains(err.Error(), "minReplicas") {
		t.Errorf("expected min-replicas reject, got %v", err)
	}
}

func TestValkey_Autoscaling_max_below_min_rejected(t *testing.T) {
	v := &ValkeyCustomValidator{}
	vc := &cachev1alpha1.Valkey{}
	vc.Spec.Mode = cachev1alpha1.ModeReplication
	vc.Spec.Replicas = 2
	vc.Spec.Version.Version = "8.1.6"
	vc.Spec.Storage.Size = resource.MustParse("8Gi")
	vc.Spec.Autoscaling = &cachev1alpha1.AutoscalingSpec{
		Enabled:     true,
		MinReplicas: 5,
		MaxReplicas: 3,
	}
	_, err := v.ValidateCreate(context.Background(), vc)
	if err == nil || !strings.Contains(err.Error(), "maxReplicas") {
		t.Errorf("expected max-replicas reject, got %v", err)
	}
}
