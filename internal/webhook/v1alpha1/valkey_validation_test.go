// validateValkeySpec 회귀 보호 (cycle 130) — Valkey CR 의 admission validation
// invariant. ValidateCreate / ValidateUpdate 가 호출 — 각 분기 정확성 보장.

package v1alpha1

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

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
	t.Run("TLS certManager omitempty trap — IssuerRef.Name 비움 → reject", func(t *testing.T) {
		t.Parallel()
		// CertManager pointer 는 non-nil 이지만 IssuerRef.Name 빈 값 — CRD 의
		// required marker 가 통과 허용하므로 webhook 으로 보강 (it46 cross-cut audit).
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		v.Spec.TLS = &cachev1alpha1.TLSSpec{
			Enabled:     true,
			CertManager: &cachev1alpha1.CertManagerSpec{IssuerRef: cachev1alpha1.CertIssuerRef{Name: ""}},
		}
		errs := validateValkeySpec(v)
		// hasCertMgr=false (Name 비어있음) + hasCustom=false → "requires either"
		// 에러 발생 = trap 차단 성공.
		if len(errs) == 0 {
			t.Error("CertManager IssuerRef.Name 비움 → requires either error 발생해야 (omitempty trap)")
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
	// 화이트리스트 검증 — ROADMAP Phase B prerequisite (Valkey 9.x 지원).
	t.Run("supported version 8.1.6 → ok", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		v.Spec.Version.Version = "8.1.6"
		errs := validateValkeySpec(v)
		for _, e := range errs {
			if strings.Contains(e.Error(), "version") {
				t.Errorf("8.1.6 should be supported, got %v", e)
			}
		}
	})
	t.Run("supported version 9.0.4 → ok", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		v.Spec.Version.Version = "9.0.4"
		errs := validateValkeySpec(v)
		for _, e := range errs {
			if strings.Contains(e.Error(), "version") {
				t.Errorf("9.0.4 should be supported, got %v", e)
			}
		}
	})
	t.Run("unsupported version 8.0.0 → error", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		v.Spec.Version.Version = "8.0.0"
		errs := validateValkeySpec(v)
		var hasVersionErr bool
		for _, e := range errs {
			if strings.Contains(e.Error(), "version") {
				hasVersionErr = true
			}
		}
		if !hasVersionErr {
			t.Error("8.0.0 should be rejected by whitelist")
		}
	})
	t.Run("unsupported version 9.99.0 → error", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.Mode = cachev1alpha1.ModeStandalone
		v.Spec.Replicas = 1
		v.Spec.Version.Version = "9.99.0"
		errs := validateValkeySpec(v)
		var hasVersionErr bool
		for _, e := range errs {
			if strings.Contains(e.Error(), "version") {
				hasVersionErr = true
			}
		}
		if !hasVersionErr {
			t.Error("9.99.0 should be rejected by whitelist")
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

func TestValidateStorageSizeMin_LowerBound(t *testing.T) {
	t.Parallel()
	cases := []struct {
		size    string
		wantErr bool
		desc    string
	}{
		{"512Mi", true, "below 1Gi — reject"},
		{"1Gi", false, "exactly 1Gi — boundary OK"},
		{"8Gi", false, "8Gi default — OK"},
		{"50Gi", false, "50Gi production — OK"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			errs := validateStorageSizeMin(nil, resource.MustParse(tc.size))
			hasErr := len(errs) > 0
			if tc.wantErr && !hasErr {
				t.Errorf("size=%s should be rejected", tc.size)
			}
			if !tc.wantErr && hasErr {
				t.Errorf("size=%s should be accepted, got %v", tc.size, errs)
			}
		})
	}
}

func TestValidateStorageSizeMin_ZeroSkip(t *testing.T) {
	t.Parallel()
	// CRD default ('8Gi' for valkey) 가 채워지지 않은 dry-run path —
	// 별도 invariant 위반 아님 (zero 통과).
	errs := validateStorageSizeMin(nil, resource.Quantity{})
	if len(errs) > 0 {
		t.Errorf("zero (unset) should skip, got %v", errs)
	}
}

func TestValidateUsersSecretRefs_OmitEmptyTrap(t *testing.T) {
	t.Parallel()
	t.Run("Users 빈 → ok", func(t *testing.T) {
		t.Parallel()
		errs := validateUsersSecretRefs(nil, nil)
		if len(errs) > 0 {
			t.Errorf("nil users should pass, got %v", errs)
		}
	})
	t.Run("user.passwordSecretRef.name 비움 → reject", func(t *testing.T) {
		t.Parallel()
		users := []cachev1alpha1.ValkeyUser{{
			Name:              "alice",
			PasswordSecretRef: corev1.SecretKeySelector{Key: "password"},
		}}
		errs := validateUsersSecretRefs(nil, users)
		var hasErr bool
		for _, e := range errs {
			if strings.Contains(e.Error(), "passwordSecretRef.name") {
				hasErr = true
			}
		}
		if !hasErr {
			t.Error("empty users[0].passwordSecretRef.name should be rejected")
		}
	})
	t.Run("user.passwordSecretRef.key 비움 → reject", func(t *testing.T) {
		t.Parallel()
		users := []cachev1alpha1.ValkeyUser{{
			Name: "alice",
			PasswordSecretRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "alice-secret"},
			},
		}}
		errs := validateUsersSecretRefs(nil, users)
		var hasErr bool
		for _, e := range errs {
			if strings.Contains(e.Error(), "passwordSecretRef.key") {
				hasErr = true
			}
		}
		if !hasErr {
			t.Error("empty users[0].passwordSecretRef.key should be rejected")
		}
	})
	t.Run("happy path → ok", func(t *testing.T) {
		t.Parallel()
		users := []cachev1alpha1.ValkeyUser{{
			Name: "alice",
			PasswordSecretRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "alice-secret"},
				Key:                  "password",
			},
		}}
		errs := validateUsersSecretRefs(nil, users)
		if len(errs) > 0 {
			t.Errorf("complete user spec should pass, got %v", errs)
		}
	})
}

// TestValidateStorageClassName — ROADMAP "RBD storageClass 기본 검증".
// DNS-1123 subdomain 위반 이름 reject + 정상 이름 / unset 통과.
func TestValidateStorageClassName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty → ok (cluster default)", "", false},
		{"ceph-rbd → ok", "ceph-rbd", false},
		{"fast-ssd → ok", "fast-ssd", false},
		{"rook-ceph-block → ok", "rook-ceph-block", false},
		{"uppercase → reject", "Ceph-RBD", true},
		{"underscore → reject", "ceph_rbd", true},
		{"leading dash → reject", "-ceph", true},
		{"empty segment → reject", "ceph..rbd", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := validateStorageClassName(nil, tc.input)
			if tc.wantErr && len(errs) == 0 {
				t.Errorf("input %q → expected error, got nil", tc.input)
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Errorf("input %q → expected no error, got %v", tc.input, errs)
			}
		})
	}
}

// TestValidateTopologySpread — ROADMAP "topology spread 일관성 검증".
// MaxSkew / TopologyKey / WhenUnsatisfiable / 중복 key reject 패턴.
func TestValidateTopologySpread(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		tscs    []corev1.TopologySpreadConstraint
		wantErr bool
		errSub  string
	}{
		{"empty list → ok", nil, false, ""},
		{
			"valid single TSC → ok",
			[]corev1.TopologySpreadConstraint{{
				MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone",
				WhenUnsatisfiable: corev1.ScheduleAnyway,
			}},
			false, "",
		},
		{
			"MaxSkew=0 → reject",
			[]corev1.TopologySpreadConstraint{{
				MaxSkew: 0, TopologyKey: "kubernetes.io/hostname",
				WhenUnsatisfiable: corev1.DoNotSchedule,
			}},
			true, "maxSkew",
		},
		{
			"empty TopologyKey → reject",
			[]corev1.TopologySpreadConstraint{{
				MaxSkew: 1, TopologyKey: "",
				WhenUnsatisfiable: corev1.ScheduleAnyway,
			}},
			true, "topologyKey",
		},
		{
			"empty WhenUnsatisfiable → reject",
			[]corev1.TopologySpreadConstraint{{
				MaxSkew: 1, TopologyKey: "kubernetes.io/hostname",
			}},
			true, "whenUnsatisfiable",
		},
		{
			"invalid WhenUnsatisfiable → reject",
			[]corev1.TopologySpreadConstraint{{
				MaxSkew: 1, TopologyKey: "kubernetes.io/hostname",
				WhenUnsatisfiable: corev1.UnsatisfiableConstraintAction("Maybe"),
			}},
			true, "whenUnsatisfiable",
		},
		{
			"duplicate TopologyKey → reject",
			[]corev1.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: corev1.ScheduleAnyway},
				{MaxSkew: 2, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: corev1.DoNotSchedule},
			},
			true, "already specified",
		},
		{
			"zone + hostname → ok (default-equivalent)",
			[]corev1.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.ScheduleAnyway},
				{MaxSkew: 1, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: corev1.ScheduleAnyway},
			},
			false, "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := validateTopologySpread(nil, tc.tscs)
			if tc.wantErr && len(errs) == 0 {
				t.Fatalf("expected error containing %q, got nil", tc.errSub)
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Fatalf("expected no error, got %v", errs)
			}
			if tc.wantErr {
				var found bool
				for _, e := range errs {
					if strings.Contains(e.Error(), tc.errSub) {
						found = true
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got %v", tc.errSub, errs)
				}
			}
		})
	}
}

// TestValidateValkeySpec_TopologySpread — Valkey CR 단위 통합. invalid TSC 가
// 전파되는지 검증.
func TestValidateValkeySpec_TopologySpread(t *testing.T) {
	t.Parallel()
	v := &cachev1alpha1.Valkey{}
	v.Spec.Mode = cachev1alpha1.ModeReplication
	v.Spec.Replicas = 2
	v.Spec.Version.Version = cachev1alpha1.DefaultValkeyVersion
	v.Spec.Storage.Size = resource.MustParse("2Gi")
	v.Spec.Pod = &cachev1alpha1.PodSpec{
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
			MaxSkew: 0, TopologyKey: "kubernetes.io/hostname",
			WhenUnsatisfiable: corev1.ScheduleAnyway,
		}},
	}
	errs := validateValkeySpec(v)
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Error(), "maxSkew") {
			found = true
		}
	}
	if !found {
		t.Errorf("invalid TSC MaxSkew=0 → expected maxSkew error, got %v", errs)
	}
}

// validateClusterSpec 통합 — StorageClassName invalid 이름이 CR 단위 reject 까지 전파되는지.
func TestValidateClusterSpec_StorageClassName(t *testing.T) {
	t.Parallel()
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Version.Version = cachev1alpha1.DefaultValkeyVersion
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.Storage.Size = resource.MustParse("2Gi")
	vc.Spec.Storage.StorageClassName = "Bad_Name"
	errs := validateClusterSpec(vc)
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Error(), "storageClassName") &&
			strings.Contains(e.Error(), "DNS-1123") {
			found = true
		}
	}
	if !found {
		t.Errorf("invalid storageClassName → expected DNS-1123 error, got %v", errs)
	}
}
