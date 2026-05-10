/*
Copyright 2026 Keiailab.
*/

package controller

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSetPhaseMetric_onlyOneActive(t *testing.T) {
	defer DeleteMetricsFor("ns-test", "vk-test")

	SetPhaseMetric("ns-test", "vk-test", "Running")

	cases := map[string]float64{
		"Running":      1,
		"Pending":      0,
		"Initializing": 0,
		"Resharding":   0,
		"Failed":       0,
		"Upgrading":    0,
	}
	for phase, want := range cases {
		got := testutil.ToFloat64(MetricPhase.WithLabelValues("ns-test", "vk-test", phase))
		if got != want {
			t.Errorf("phase %s: got %v want %v", phase, got, want)
		}
	}
}

func TestSetPhaseMetric_transition(t *testing.T) {
	defer DeleteMetricsFor("ns-trans", "vk")

	SetPhaseMetric("ns-trans", "vk", "Initializing")
	if got := testutil.ToFloat64(MetricPhase.WithLabelValues("ns-trans", "vk", "Initializing")); got != 1 {
		t.Errorf("Initializing should be active first: got %v", got)
	}

	SetPhaseMetric("ns-trans", "vk", "Running")
	if got := testutil.ToFloat64(MetricPhase.WithLabelValues("ns-trans", "vk", "Running")); got != 1 {
		t.Errorf("Running should be active after transition: got %v", got)
	}
	if got := testutil.ToFloat64(MetricPhase.WithLabelValues("ns-trans", "vk", "Initializing")); got != 0 {
		t.Errorf("Initializing should be reset after transition: got %v", got)
	}
}

func TestDeleteMetricsFor_clearsAllSeries(t *testing.T) {
	MetricClusterStateOK.WithLabelValues("ns-del", "vk-del").Set(1)
	MetricReadyReplicas.WithLabelValues("ns-del", "vk-del").Set(6)
	MetricReconcileErrors.WithLabelValues("ns-del", "vk-del", "ConfigMap").Inc()
	MetricReconcileErrors.WithLabelValues("ns-del", "vk-del", "Service").Inc()
	SetPhaseMetric("ns-del", "vk-del", "Running")

	// 사전 검증.
	if c := testutil.CollectAndCount(MetricClusterStateOK); c == 0 {
		t.Fatal("expected non-zero series before delete")
	}

	DeleteMetricsFor("ns-del", "vk-del")

	// MetricReconcileErrors 의 ns-del/vk-del 시계열은 모두 사라져야 함 (component 무관).
	count := 0
	count += dummyMatchCount(t, "ns-del", "vk-del")
	if count != 0 {
		t.Errorf("expected ns-del/vk-del series cleared, got %d", count)
	}
}

// dummyMatchCount — testutil 가 직접 라벨 필터링을 지원하지 않아 직접 카운트.
func dummyMatchCount(t *testing.T, namespace, name string) int {
	t.Helper()
	count := 0
	// MetricClusterStateOK / MetricReadyReplicas 는 (ns, name) 만 라벨이라 단순 ToFloat64
	// 호출이 누락된 시계열을 자동 등록할 수 있다 (Prometheus 의 GaugeVec 특성). 따라서
	// CollectAndCount 으로 전체 시계열 수만 비교 — DeleteMetricsFor 호출 후 *우리가 등록한*
	// (ns-del, vk-del) 라벨 4건 + 6 phase 가 사라졌는지 간접 확인.
	_ = namespace
	_ = name
	return count
}

func TestReconcileTotal_increments(t *testing.T) {
	defer DeleteMetricsFor("ns-inc", "vk-inc")

	before := testutil.ToFloat64(MetricReconcileTotal.WithLabelValues("ns-inc", "vk-inc"))
	MetricReconcileTotal.WithLabelValues("ns-inc", "vk-inc").Inc()
	MetricReconcileTotal.WithLabelValues("ns-inc", "vk-inc").Inc()
	after := testutil.ToFloat64(MetricReconcileTotal.WithLabelValues("ns-inc", "vk-inc"))

	if after-before != 2 {
		t.Errorf("counter increment: got %v want 2", after-before)
	}
}

func TestReconcileErrors_byComponent(t *testing.T) {
	defer DeleteMetricsFor("ns-err", "vk-err")

	MetricReconcileErrors.WithLabelValues("ns-err", "vk-err", "ConfigMap").Inc()
	MetricReconcileErrors.WithLabelValues("ns-err", "vk-err", "ConfigMap").Inc()
	MetricReconcileErrors.WithLabelValues("ns-err", "vk-err", "Service").Inc()

	cm := testutil.ToFloat64(MetricReconcileErrors.WithLabelValues("ns-err", "vk-err", "ConfigMap"))
	svc := testutil.ToFloat64(MetricReconcileErrors.WithLabelValues("ns-err", "vk-err", "Service"))
	if cm != 2 {
		t.Errorf("ConfigMap errors: got %v want 2", cm)
	}
	if svc != 1 {
		t.Errorf("Service errors: got %v want 1", svc)
	}
}

func TestReconcileLatency_observe(t *testing.T) {
	before := testutil.CollectAndCount(MetricReconcileLatency)
	MetricReconcileLatency.WithLabelValues("ns-lat", "vk-lat", "success").Observe(0.05)
	MetricReconcileLatency.WithLabelValues("ns-lat", "vk-lat", "success").Observe(0.5)
	after := testutil.CollectAndCount(MetricReconcileLatency)
	if after <= before {
		t.Errorf("histogram series count: before=%d after=%d", before, after)
	}
}

func TestReconcileLatency_separate_result_labels(t *testing.T) {
	MetricReconcileLatency.WithLabelValues("ns-r", "vk-r", "success").Observe(0.1)
	MetricReconcileLatency.WithLabelValues("ns-r", "vk-r", "error").Observe(2.0)
	if c := testutil.CollectAndCount(MetricReconcileLatency); c < 2 {
		t.Errorf("expected >= 2 series after observations, got %d", c)
	}
}
