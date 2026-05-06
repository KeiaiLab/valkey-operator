/*
Copyright 2026 Keiailab.
*/

// Package controller — Prometheus metrics 정의.
//
// controller-runtime 의 글로벌 metrics registry 에 자동 등록.
// 메트릭 노출 endpoint 는 cmd/main.go 의 metricsServer 옵션 (기본 :8443 secure).
package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricSubsystem = "valkey_cluster"
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

	// MetricReconcileTotal — Reconcile 호출 횟수.
	MetricReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricSubsystem,
			Name:      "reconcile_total",
			Help:      "Total Reconcile invocations",
		},
		labelNamespaceName,
	)

	// MetricReconcileErrors — component 별 reconcile 실패 횟수.
	MetricReconcileErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricSubsystem,
			Name:      "reconcile_errors_total",
			Help:      "Total Reconcile component failures",
		},
		[]string{"namespace", "name", "component"},
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
	metrics.Registry.MustRegister(
		MetricClusterStateOK,
		MetricClusterAssignedSlots,
		MetricClusterShards,
		MetricReadyReplicas,
		MetricReconcileTotal,
		MetricReconcileErrors,
		MetricPhase,
		MetricBackupTotal,
		MetricRestoreTotal,
		MetricFailoverTotal,
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
func DeleteMetricsFor(namespace, name string) {
	MetricClusterStateOK.DeleteLabelValues(namespace, name)
	MetricClusterAssignedSlots.DeleteLabelValues(namespace, name)
	MetricClusterShards.DeleteLabelValues(namespace, name)
	MetricReadyReplicas.DeleteLabelValues(namespace, name)
	MetricReconcileTotal.DeleteLabelValues(namespace, name)
	allPhases := []string{"Pending", "Initializing", "Running", "Resharding", "Failed", "Upgrading"}
	for _, p := range allPhases {
		MetricPhase.DeleteLabelValues(namespace, name, p)
	}
	// MetricReconcileErrors 는 component 차원이 추가되어 별도 cleanup 어려움.
	// CR 삭제 시점에는 component 종류를 모두 알지 못하므로 label-match-delete 사용.
	MetricReconcileErrors.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace, "name": name,
	})
}
