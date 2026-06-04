/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 컨트롤러의 순수 helper 회귀 보호.
// primaryOrdinal / exporterImage / hasExternalDestination / keyOrDefault /
// backupTargetTLSSecret / operatorImage (env-driven) — reconcile 분기의 단일 진실.

package controller

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

func TestPrimaryOrdinal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		currentPrimary string
		want           int
	}{
		{"empty → 0", "", 0},
		{"valid pod name", "rs-0", 0},
		{"valid pod name 2", "rs-2", 2},
		{"multi-dash name", "my-valkey-rs-3", 3},
		{"no dash → 0", "primary", 0},
		{"non-numeric suffix → 0", "rs-foo", 0},
		{"large ordinal", "cluster-127", 127},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			v := &cachev1alpha1.Valkey{}
			v.Status.CurrentPrimary = c.currentPrimary
			if got := primaryOrdinal(v); got != c.want {
				t.Errorf("currentPrimary=%q: got %d want %d", c.currentPrimary, got, c.want)
			}
		})
	}
}

func TestExporterImage(t *testing.T) {
	t.Parallel()
	const defaultImg = "oliver006/redis_exporter:latest"

	if got := exporterImage(nil); got != defaultImg {
		t.Errorf("nil monitoring → default, got %q", got)
	}
	m := &cachev1alpha1.MonitoringSpec{}
	if got := exporterImage(m); got != defaultImg {
		t.Errorf("Exporter=nil → default, got %q", got)
	}
	m.Exporter = &cachev1alpha1.ExporterSpec{Image: ""}
	if got := exporterImage(m); got != defaultImg {
		t.Errorf("empty Image → default, got %q", got)
	}
	m.Exporter = &cachev1alpha1.ExporterSpec{Image: "ghcr.io/example/exporter:v1.2.3"}
	if got := exporterImage(m); got != "ghcr.io/example/exporter:v1.2.3" {
		t.Errorf("custom image: got %q", got)
	}
}

func TestHasExternalDestination(t *testing.T) {
	t.Parallel()
	// nil destination → false.
	b := &cachev1alpha1.ValkeyBackup{}
	if hasExternalDestination(b) {
		t.Error("nil Destination → true")
	}
	// PVC type → false.
	b.Spec.Destination = &cachev1alpha1.BackupDestination{Type: cachev1alpha1.BackupDestPVC}
	if hasExternalDestination(b) {
		t.Error("PVC type → true")
	}
	// TargetRef → true.
	b.Spec.Destination.Type = cachev1alpha1.BackupDestTargetRef
	if !hasExternalDestination(b) {
		t.Error("TargetRef → false")
	}
}

func TestKeyOrDefault(t *testing.T) {
	t.Parallel()
	if got := keyOrDefault("", "fallback"); got != "fallback" {
		t.Errorf("empty → default: got %q", got)
	}
	if got := keyOrDefault("explicit", "fallback"); got != "explicit" {
		t.Errorf("non-empty: got %q", got)
	}
	// 둘 다 빈 문자열 → 빈 문자열.
	if got := keyOrDefault("", ""); got != "" {
		t.Errorf("both empty: got %q", got)
	}
}

func TestBackupTargetTLSSecret(t *testing.T) {
	t.Parallel()
	// 둘 다 nil → empty.
	if got := backupTargetTLSSecret(&cachev1alpha1.TLSSpec{}, "rs"); got != "" {
		t.Errorf("empty TLSSpec: got %q", got)
	}
	// CustomCert 우선.
	tls := &cachev1alpha1.TLSSpec{
		CustomCert:  &cachev1alpha1.CustomCertSpec{SecretName: "user-cert"},
		CertManager: &cachev1alpha1.CertManagerSpec{IssuerRef: cachev1alpha1.CertIssuerRef{Name: "issuer-x"}},
	}
	if got := backupTargetTLSSecret(tls, "rs"); got != "user-cert" {
		t.Errorf("CustomCert 우선해야 함, got %q", got)
	}
	// CertManager 만 → "<crName>-tls".
	tls = &cachev1alpha1.TLSSpec{
		CertManager: &cachev1alpha1.CertManagerSpec{IssuerRef: cachev1alpha1.CertIssuerRef{Name: "issuer-x"}},
	}
	if got := backupTargetTLSSecret(tls, "rs"); got != "rs-tls" {
		t.Errorf("CertManager → '<crName>-tls', got %q", got)
	}
	// CustomCert SecretName 빈 문자열 → CertManager fallback.
	tls = &cachev1alpha1.TLSSpec{
		CustomCert:  &cachev1alpha1.CustomCertSpec{SecretName: ""},
		CertManager: &cachev1alpha1.CertManagerSpec{IssuerRef: cachev1alpha1.CertIssuerRef{Name: "issuer-y"}},
	}
	if got := backupTargetTLSSecret(tls, "rs"); got != "rs-tls" {
		t.Errorf("empty CustomCert.SecretName → CertManager, got %q", got)
	}
}

func TestOperatorImageEnvOverride(t *testing.T) {
	// t.Parallel 사용 불가 — t.Setenv 와 충돌.
	r := &ValkeyBackupReconciler{}

	t.Setenv("OPERATOR_IMAGE", "")
	if got := r.operatorImage(); got != "controller:latest" {
		t.Errorf("env empty → default 'controller:latest', got %q", got)
	}

	t.Setenv("OPERATOR_IMAGE", "ghcr.io/keiailab/valkey-operator:v0.1.0")
	if got := r.operatorImage(); got != "ghcr.io/keiailab/valkey-operator:v0.1.0" {
		t.Errorf("env override: got %q", got)
	}
}

// SetBuildInfo (cycle 57) 의 회귀 보호 — Prometheus build_info gauge 의 label
// 값 들 정확히 set. Grafana dashboard 의 *현재 운영 version 식별* contract.
// cycle 119: prometheus testutil 으로 *실제 gauge value 검증* (cycle 118 의
// panic-only 검증 강화).
func TestSetBuildInfo(t *testing.T) {
	cases := []struct {
		name, version, commit, date string
	}{
		{"defaults", "dev", "none", "unknown"},
		{"release", "v0.1.0-alpha.1", "abc1234", "2026-05-06"},
		{"empty values", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// t.Parallel 비활성 — Gauge 시계열이 process-global, parallel 시 race.
			defer MetricBuildInfo.DeleteLabelValues(c.version, c.commit, c.date)
			SetBuildInfo(c.version, c.commit, c.date)
			got := testutil.ToFloat64(MetricBuildInfo.WithLabelValues(c.version, c.commit, c.date))
			if got != 1.0 {
				t.Errorf("MetricBuildInfo[%q,%q,%q] = %v, want 1.0", c.version, c.commit, c.date, got)
			}
		})
	}
}

// sortPrimariesBySlotStart + primaryFirstSlot 회귀 보호 (cycle 133) —
// ValkeyCluster reconcile 의 *shard ordering 안정성* invariant. slot 시작점
// 기준 정렬 — Index 부여 의 deterministic.
func TestSortPrimariesBySlotStart(t *testing.T) {
	t.Parallel()
	t.Run("sorted by first slot start", func(t *testing.T) {
		t.Parallel()
		nodes := []vk.NodeView{
			{ID: "n3", Slots: []vk.SlotRange{{Start: 10923, End: 16383}}},
			{ID: "n1", Slots: []vk.SlotRange{{Start: 0, End: 5460}}},
			{ID: "n2", Slots: []vk.SlotRange{{Start: 5461, End: 10922}}},
		}
		sortPrimariesBySlotStart(nodes)
		if nodes[0].ID != "n1" || nodes[1].ID != "n2" || nodes[2].ID != "n3" {
			t.Errorf("정렬 후 ID 순서: %s, %s, %s (want n1, n2, n3)",
				nodes[0].ID, nodes[1].ID, nodes[2].ID)
		}
	})
	t.Run("unassigned (no slots) → sorted last", func(t *testing.T) {
		t.Parallel()
		nodes := []vk.NodeView{
			{ID: "no-slots"},
			{ID: "first", Slots: []vk.SlotRange{{Start: 0, End: 100}}},
		}
		sortPrimariesBySlotStart(nodes)
		if nodes[0].ID != "first" || nodes[1].ID != "no-slots" {
			t.Errorf("미할당 노드 가 마지막 으로 정렬되어야: %v", nodes)
		}
	})
	t.Run("primaryFirstSlot empty → max int", func(t *testing.T) {
		t.Parallel()
		got := primaryFirstSlot(vk.NodeView{})
		if got != 1<<30 {
			t.Errorf("empty slots → 1<<30, got %d", got)
		}
	})
	t.Run("primaryFirstSlot returns first slot", func(t *testing.T) {
		t.Parallel()
		n := vk.NodeView{Slots: []vk.SlotRange{{Start: 5461, End: 10922}, {Start: 11000, End: 12000}}}
		if got := primaryFirstSlot(n); got != 5461 {
			t.Errorf("first slot start = 5461, got %d", got)
		}
	})
}

// filterConditionsByType — applyErrorCondition 의 helper. 특정 type 의 condition
// 만 제거 (ReconcileError 갱신 전 cleanup 패턴).
func TestFilterConditionsByType(t *testing.T) {
	t.Parallel()
	makeConds := func() []metav1.Condition {
		return []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue},
			{Type: "ReconcileError", Status: metav1.ConditionTrue},
			{Type: "Available", Status: metav1.ConditionFalse},
			{Type: "ReconcileError", Status: metav1.ConditionFalse}, // 중복 — filter 로 모두 제거.
		}
	}
	t.Run("removes matching type", func(t *testing.T) {
		t.Parallel()
		got := filterConditionsByType(makeConds(), "ReconcileError")
		if len(got) != 2 {
			t.Errorf("expected 2 (Ready + Available), got %d", len(got))
		}
		for _, c := range got {
			if c.Type == "ReconcileError" {
				t.Errorf("ReconcileError 잔존: %v", c)
			}
		}
	})
	t.Run("non-matching type leaves intact", func(t *testing.T) {
		t.Parallel()
		got := filterConditionsByType(makeConds(), "Unknown")
		if len(got) != 4 {
			t.Errorf("expected 4 (no removal), got %d", len(got))
		}
	})
	t.Run("empty input", func(t *testing.T) {
		t.Parallel()
		got := filterConditionsByType(nil, "ReconcileError")
		if len(got) != 0 {
			t.Errorf("expected 0, got %d", len(got))
		}
	})
}
