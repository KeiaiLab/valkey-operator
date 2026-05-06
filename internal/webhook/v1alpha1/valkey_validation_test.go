// validateValkeySpec 회귀 보호 (cycle 130) — Valkey CR 의 admission validation
// invariant. ValidateCreate / ValidateUpdate 가 호출 — 각 분기 정확성 보장.

package v1alpha1

import (
	"strings"
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestValidateValkeySpec(t *testing.T) {
	t.Parallel()
	t.Run("standalone replicas>1 → error", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 3
		errs := validateValkeySpec(v)
		if len(errs) == 0 {
			t.Error("expected error for Standalone+replicas=3")
		}
		if !strings.Contains(errs[0].Error(), "replicas must be 1") {
			t.Errorf("error message: %v", errs[0])
		}
	})
	t.Run("replication replicas<2 → error", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeReplication
		v.Spec.Replicas = 1
		errs := validateValkeySpec(v)
		if len(errs) == 0 {
			t.Error("expected error for Replication+replicas=1")
		}
	})
	t.Run("standalone replicas=1 → ok", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		errs := validateValkeySpec(v)
		if len(errs) > 0 {
			t.Errorf("Standalone+replicas=1 → expected no error, got %v", errs)
		}
	})
	t.Run("TLS enabled requires certManager or customCert", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		v.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
		errs := validateValkeySpec(v)
		if len(errs) == 0 {
			t.Error("TLS.Enabled=true without cert source → expected error")
		}
	})
	t.Run("TLS certManager + customCert → mutually exclusive", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		v.Spec.TLS = &cachev1alpha1.TLSSpec{
			Enabled:     true,
			CertManager: &cachev1alpha1.CertManagerSpec{IssuerRef: cachev1alpha1.CertIssuerRef{Name: "ca"}},
			CustomCert:  &cachev1alpha1.CustomCertSpec{SecretName: "user-cert"},
		}
		errs := validateValkeySpec(v)
		var hasMutex bool
		for _, e := range errs {
			if strings.Contains(e.Error(), "mutually exclusive") {
				hasMutex = true
			}
		}
		if !hasMutex {
			t.Error("certManager + customCert → mutually exclusive error 누락")
		}
	})
	t.Run("auth.users without auth.enabled → error", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		v.Spec.Auth.Enabled = false
		v.Spec.Auth.Users = []cachev1alpha1.ValkeyUser{{Name: "alice"}}
		errs := validateValkeySpec(v)
		if len(errs) == 0 {
			t.Error("auth.users with auth.enabled=false → expected error")
		}
	})
}

// validateClusterSpec 회귀 보호 (cycle 131).
func TestValidateClusterSpec(t *testing.T) {
	t.Parallel()
	t.Run("autoFailover=true + replicasPerShard=0 → error", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Spec.Shards = 3
		vc.Spec.ReplicasPerShard = 0
		vc.Spec.AutoFailover = true
		errs := validateClusterSpec(vc)
		if len(errs) == 0 {
			t.Error("autoFailover=true + replicasPerShard=0 → expected error")
		}
	})
	t.Run("totalNodes > 100 → error", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Spec.Shards = 50
		vc.Spec.ReplicasPerShard = 1 // total = 50 * 2 = 100, OK.
		errs := validateClusterSpec(vc)
		if len(errs) > 0 {
			t.Errorf("100 nodes → expected ok, got %v", errs)
		}
		vc.Spec.ReplicasPerShard = 2 // total = 50 * 3 = 150, error.
		errs = validateClusterSpec(vc)
		var hasOver100 bool
		for _, e := range errs {
			if strings.Contains(e.Error(), "must not exceed 100") {
				hasOver100 = true
			}
		}
		if !hasOver100 {
			t.Errorf("150 nodes → expected 'must not exceed 100' error, got %v", errs)
		}
	})
	t.Run("TLS dual-source mutually exclusive", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Spec.Shards = 3
		vc.Spec.ReplicasPerShard = 1
		vc.Spec.TLS = &cachev1alpha1.TLSSpec{
			Enabled:     true,
			CertManager: &cachev1alpha1.CertManagerSpec{IssuerRef: cachev1alpha1.CertIssuerRef{Name: "ca"}},
			CustomCert:  &cachev1alpha1.CustomCertSpec{SecretName: "user-cert"},
		}
		errs := validateClusterSpec(vc)
		var hasMutex bool
		for _, e := range errs {
			if strings.Contains(e.Error(), "mutually exclusive") {
				hasMutex = true
			}
		}
		if !hasMutex {
			t.Error("certManager + customCert → mutually exclusive error 누락")
		}
	})
	t.Run("auth.users without auth.enabled → error", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Spec.Shards = 3
		vc.Spec.ReplicasPerShard = 1
		vc.Spec.Auth.Enabled = false
		vc.Spec.Auth.Users = []cachev1alpha1.ValkeyUser{{Name: "alice"}}
		errs := validateClusterSpec(vc)
		if len(errs) == 0 {
			t.Error("auth.users with auth.enabled=false → expected error")
		}
	})
	t.Run("valid spec → no error", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Spec.Shards = 3
		vc.Spec.ReplicasPerShard = 1
		vc.Spec.AutoFailover = true
		errs := validateClusterSpec(vc)
		if len(errs) > 0 {
			t.Errorf("valid 3-shard cluster → expected no error, got %v", errs)
		}
	})
}

// validateValkeyImmutable 회귀 보호 (cycle 132).
func TestValidateValkeyImmutable(t *testing.T) {
	t.Parallel()
	t.Run("mode change → forbidden", func(t *testing.T) {
		t.Parallel()
		old := &cachev1alpha1.Valkey{}
		old.Spec.Mode = cachev1alpha1.ModeStandalone
		newV := &cachev1alpha1.Valkey{}
		newV.Spec.Mode = cachev1alpha1.ModeReplication
		errs := validateValkeyImmutable(old, newV)
		var hasModeErr bool
		for _, e := range errs {
			if strings.Contains(e.Error(), "spec.mode is immutable") {
				hasModeErr = true
			}
		}
		if !hasModeErr {
			t.Error("mode change → expected immutable error")
		}
	})
	t.Run("storageClassName change → forbidden", func(t *testing.T) {
		t.Parallel()
		old := &cachev1alpha1.Valkey{}
		old.Spec.Storage.StorageClassName = "fast-ssd"
		newV := &cachev1alpha1.Valkey{}
		newV.Spec.Storage.StorageClassName = "slow-hdd"
		errs := validateValkeyImmutable(old, newV)
		if len(errs) == 0 {
			t.Error("storageClassName change → expected error")
		}
	})
	t.Run("tls.enabled toggle → forbidden", func(t *testing.T) {
		t.Parallel()
		old := &cachev1alpha1.Valkey{}
		old.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: false}
		newV := &cachev1alpha1.Valkey{}
		newV.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
		errs := validateValkeyImmutable(old, newV)
		if len(errs) == 0 {
			t.Error("tls.enabled toggle → expected error")
		}
	})
	t.Run("no change → no error", func(t *testing.T) {
		t.Parallel()
		old := &cachev1alpha1.Valkey{}
		old.Spec.Mode = cachev1alpha1.ModeStandalone
		old.Spec.Storage.StorageClassName = "fast-ssd"
		newV := &cachev1alpha1.Valkey{}
		newV.Spec.Mode = cachev1alpha1.ModeStandalone
		newV.Spec.Storage.StorageClassName = "fast-ssd"
		errs := validateValkeyImmutable(old, newV)
		if len(errs) > 0 {
			t.Errorf("no change → expected no error, got %v", errs)
		}
	})
}

// validateClusterImmutable 회귀 보호 (cycle 132).
func TestValidateClusterImmutable(t *testing.T) {
	t.Parallel()
	t.Run("storageClassName change → forbidden", func(t *testing.T) {
		t.Parallel()
		old := &cachev1alpha1.ValkeyCluster{}
		old.Spec.Storage.StorageClassName = "fast-ssd"
		newV := &cachev1alpha1.ValkeyCluster{}
		newV.Spec.Storage.StorageClassName = "slow-hdd"
		errs := validateClusterImmutable(old, newV)
		if len(errs) == 0 {
			t.Error("storageClassName change → expected error")
		}
	})
	t.Run("dataDirPath change → forbidden", func(t *testing.T) {
		t.Parallel()
		old := &cachev1alpha1.ValkeyCluster{}
		old.Spec.Storage.DataDirPath = "/data"
		newV := &cachev1alpha1.ValkeyCluster{}
		newV.Spec.Storage.DataDirPath = "/var/data"
		errs := validateClusterImmutable(old, newV)
		if len(errs) == 0 {
			t.Error("dataDirPath change → expected error")
		}
	})
	t.Run("tls.enabled toggle → forbidden", func(t *testing.T) {
		t.Parallel()
		old := &cachev1alpha1.ValkeyCluster{}
		old.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
		newV := &cachev1alpha1.ValkeyCluster{}
		newV.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: false}
		errs := validateClusterImmutable(old, newV)
		if len(errs) == 0 {
			t.Error("tls.enabled toggle → expected error")
		}
	})
}
