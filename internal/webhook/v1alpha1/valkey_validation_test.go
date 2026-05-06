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
