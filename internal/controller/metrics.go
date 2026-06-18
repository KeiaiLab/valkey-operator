/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package controller — Prometheus metrics 정의.
//
// controller-runtime 의 글로벌 metrics registry 에 자동 등록.
// 메트릭 노출 endpoint 는 cmd/main.go 의 metricsServer 옵션 (기본 :8443 secure).
package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/keiailab/keiailab-commons/pkg/reconcilemetrics"
)

const (
	metricSubsystem = "valkey_cluster"
)

// reconMetrics — commons reconcile SLO trio (reconcile_total / reconcile_duration_seconds /
// reconcile_errors_total). subsystem 에 기존 상수를 그대로 주입해 fqName
// (valkey_cluster_reconcile_*) byte-동일 보존. 자체 trio 정의는 commons 로 폐기
// (공존 시 duplicate registration panic).
var reconMetrics = reconcilemetrics.New(metricSubsystem)

// trio alias — 기존 콜사이트/테스트 호환 (fqName + 라벨 불변).
var (
	// MetricReconcileTotal — Reconcile 호출 횟수. 라벨: namespace, name.
	MetricReconcileTotal = reconMetrics.Total
	// MetricReconcileLatency — Reconcile wall-clock duration (초) SLO histogram.
	// 라벨: namespace, name, result (success | error).
	MetricReconcileLatency = reconMetrics.Latency
	// MetricReconcileErrors — component 별 reconcile 실패 횟수.
	// 라벨: namespace, name, component.
	MetricReconcileErrors = reconMetrics.Errors
)

// 라벨: namespace, name — Prometheus 시계열 cardinality 제어를 위해 namespace/name 만.
// shard / pod 레벨 라벨은 의도적으로 제외 (대규모 cluster 시 cardinality 폭발 방지).
var labelNamespaceName = []string{"namespace", "name"}

var (
	// MetricClusterStateOK — cluster_state == "ok" 면 1, 아니면 0.
	MetricClusterStateOK = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: metricSubsystem,
			Name:      "state_ok",
			Help:      "1 if cluster state == ok, 0 otherwise",
		},
		labelNamespaceName,
	)

	// MetricClusterAssignedSlots — 할당된 slot 수 (정상 = 16384).
	MetricClusterAssignedSlots = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: metricSubsystem,
			Name:      "assigned_slots",
			Help:      "Number of hash slots assigned (target: 16384)",
		},
		labelNamespaceName,
	)

	// MetricClusterShards — 현재 cluster 의 primary 수.
	MetricClusterShards = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: metricSubsystem,
			Name:      "shards",
			Help:      "Number of primary nodes (cluster size)",
		},
		labelNamespaceName,
	)

	// MetricReadyReplicas — STS 의 readyReplicas.
	MetricReadyReplicas = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: metricSubsystem,
			Name:      "ready_replicas",
			Help:      "Number of pods in StatefulSet that report Ready",
		},
		labelNamespaceName,
	)

	// MetricPhase — phase 라벨 + value 1 (active phase). 다른 phase 는 0.
	// Prometheus 쿼리 예: max by (phase) (valkey_cluster_phase{namespace="..."}) == 1.
	MetricPhase = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: metricSubsystem,
			Name:      "phase",
			Help:      "Current phase (1 for active phase, 0 otherwise)",
		},
		[]string{"namespace", "name", "phase"},
	)

	// MetricBackupTotal — ValkeyBackup terminal phase 도달 카운터.
	// label phase=Completed|Failed. handleBackupTerminal 진입 시 증가.
	MetricBackupTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricSubsystem,
			Name:      "backup_total",
			Help:      "Total ValkeyBackup CRs reaching terminal phase",
		},
		[]string{"namespace", "name", "phase"},
	)

	// MetricRestoreTotal — ValkeyRestore terminal phase 도달 카운터.
	// label phase=Completed|Failed. handleVerifying 의 Completed 진입 또는
	// markFailed 시 증가.
	MetricRestoreTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricSubsystem,
			Name:      "restore_total",
			Help:      "Total ValkeyRestore CRs reaching terminal phase",
		},
		[]string{"namespace", "name", "phase"},
	)

	// MetricFailoverTotal — Replication mode 자동 failover 발생 카운터.
	// reconcileFailover 의 성공 분기 (Status.CurrentPrimary 갱신 후) 증가.
	MetricFailoverTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricSubsystem,
			Name:      "failover_total",
			Help:      "Total automatic failover events (Replication mode, ADR-0017)",
		},
		labelNamespaceName,
	)

	// MetricStuckSlotTakeoverTotal — 결함 ⑤ partial-slot outage 자가복구로 발행한
	// CLUSTER FAILOVER TAKEOVER 카운터. fail master 의 slot 이 healthy replica 로
	// 승계될 때 (성공 분기) 증가.
	MetricStuckSlotTakeoverTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricSubsystem,
			Name:      "stuck_slot_takeover_total",
			Help:      "Total CLUSTER FAILOVER TAKEOVER events to heal partial-slot outages (Cluster mode)",
		},
		labelNamespaceName,
	)

	// MetricCapabilityActive — CR 의 활성 optional capability 추적 (PR #62 Status.Capabilities
	// 의 Prometheus 측 노출). fleet-wide 채택 추적 용:
	//   sum by (capability) (valkey_cluster_capability_active) → namespace 별 채택 CR 수
	//
	// 라벨 cardinality: |capabilities| (≤9) × cluster 수. 운영 1000 cluster + 9 capability
	// = 9000 series — Prometheus 단일 instance 한계 (~10M) 대비 무시.
	MetricCapabilityActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: metricSubsystem,
			Name:      "capability_active",
			Help:      "1 if the optional capability is active for this CR, 0 otherwise",
		},
		[]string{"namespace", "name", "capability"},
	)

	// MetricBuildInfo — kube-state-metrics 표준 패턴. {version, commit, date}
	// 라벨로 운영 중 image 의 정확한 release tag 식별. cycles 53-56 의 ldflags
	// chain 과 정합 — `kubectl exec ... --version` 의 *Prometheus 등가물*.
	MetricBuildInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: metricSubsystem,
			Name:      "build_info",
			Help:      "Build info gauge (always 1) labeled by version/commit/date — for dashboard display.",
		},
		[]string{"version", "commit", "date"},
	)
)

func init() {
	// reconcile trio 는 commons 가 생성 — commons 경유 1회만 등록 (중복 등록 panic 차단).
	reconMetrics.MustRegister(metrics.Registry)
	metrics.Registry.MustRegister(
		MetricClusterStateOK,
		MetricClusterAssignedSlots,
		MetricClusterShards,
		MetricReadyReplicas,
		MetricPhase,
		MetricBackupTotal,
		MetricRestoreTotal,
		MetricFailoverTotal,
		MetricStuckSlotTakeoverTotal,
		MetricCapabilityActive,
		MetricBuildInfo,
	)
}

// SetBuildInfo — operator 부팅 시 cmd/main.go 가 1회 호출. version/commit/date
// 모두 ldflags 주입 값 (cycle 53). 미주입 시 default ("dev"/"none"/"unknown").
func SetBuildInfo(version, commit, date string) {
	MetricBuildInfo.WithLabelValues(version, commit, date).Set(1)
}

// SetPhaseMetric — 활성 phase 만 1, 나머지 phase 라벨은 0 으로 설정.
func SetPhaseMetric(namespace, name, activePhase string) {
	allPhases := []string{"Pending", "Initializing", "Running", "Resharding", "Failed", "Upgrading"}
	for _, p := range allPhases {
		v := 0.0
		if p == activePhase {
			v = 1.0
		}
		MetricPhase.WithLabelValues(namespace, name, p).Set(v)
	}
}

// DeleteMetricsFor — CR 삭제 시 cardinality 누적 방지를 위해 모든 시계열 제거.
//
// reconcile trio 는 commons DeleteFor 위임 — 기존 자체 구현이 누락하던
// MetricReconcileLatency (reconcile_duration_seconds) 시계열도 함께 제거된다
// (삭제된 CR 의 latency 시계열 영구 잔존 누수 fix).
func DeleteMetricsFor(namespace, name string) {
	reconMetrics.DeleteFor(namespace, name)
	MetricClusterStateOK.DeleteLabelValues(namespace, name)
	MetricClusterAssignedSlots.DeleteLabelValues(namespace, name)
	MetricClusterShards.DeleteLabelValues(namespace, name)
	MetricReadyReplicas.DeleteLabelValues(namespace, name)
	allPhases := []string{"Pending", "Initializing", "Running", "Resharding", "Failed", "Upgrading"}
	for _, p := range allPhases {
		MetricPhase.DeleteLabelValues(namespace, name, p)
	}
	// MetricCapabilityActive 는 capability 차원이 추가되어 label-match-delete 사용.
	MetricCapabilityActive.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "name": name,
	})
}

// SetCapabilityMetrics — Status.Capabilities 슬라이스를 Prometheus Gauge 로 반영.
// active 한 capability 만 1 로 set, 본 함수가 inactive 를 명시 0 으로 set 하지
// 는 않음 (cleanup 은 DeleteMetricsFor 또는 다음 reconcile 에서 자동 갱신).
//
// 단, *비활성 전환* 케이스 (사용자가 spec field 제거) 처리 위해 caller 가 가능
// capability 전체 리스트 를 함께 전달 — inactive 는 0 으로 명시 set.
func SetCapabilityMetrics(namespace, name string, allCapabilities []string, active []string) {
	activeSet := make(map[string]bool, len(active))
	for _, cap := range active {
		activeSet[cap] = true
	}
	for _, cap := range allCapabilities {
		v := 0.0
		if activeSet[cap] {
			v = 1.0
		}
		MetricCapabilityActive.WithLabelValues(namespace, name, cap).Set(v)
	}
}
