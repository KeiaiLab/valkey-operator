# Metrics Glossary — valkey-operator

> 한국어 버전: [metrics-glossary.ko.md](metrics-glossary.ko.md)

Every Prometheus metric the operator exposes — its **meaning,
type, labels, normal range, and diagnostic use**. SSOT for the
code: `internal/controller/metrics.go`. SSOT for the alerts:
`config/prometheus/alert-rules.yaml`.

Use this glossary as the **reference for why an operator looks at
each metric** on a dashboard or alert. New alerts and dashboards
should check this table first.

## 1. Conventions

- **Type**: Gauge (snapshot) / Counter (monotonic, use `rate`) /
  Histogram (none defined yet).
- **Labels**: cardinality is kept low — `namespace` + `name`
  (cluster-level) only. Per-pod and per-shard labels are
  **deliberately excluded** to avoid timeseries explosion on
  100-shard clusters.
- **Normal range**: the **operating envelope** the metric should
  sit in during steady state. Exceeding the threshold maps to an
  alert (§5).

## 2. ValkeyCluster state metrics (Gauge)

| Metric | Meaning | Normal | Abnormal → alert |
|---|---|---|---|
| `valkey_cluster_state_ok{namespace,name}` | `1` when `CLUSTER INFO cluster_state == ok`. | `1` | `0` for 5m → `ValkeyClusterStateNotOK` (critical) |
| `valkey_cluster_assigned_slots{namespace,name}` | Number of allocated hash slots. Normal = 16384 (Redis cluster spec). | `16384` | `< 16384` for 5m → `ValkeyClusterSlotsMismatch` (critical) |
| `valkey_cluster_shards{namespace,name}` | Number of primary nodes (= shard count). | Matches `spec.shards` | Mismatch = scaling in progress or reconcile failing |
| `valkey_cluster_ready_replicas{namespace,name}` | StatefulSet `readyReplicas`. | `>= spec.replicas` | `0` → critical, `0 < x < 2` → warning |
| `valkey_cluster_phase{namespace,name,phase}` | `1` for the active phase, `0` for the rest. `phase` ∈ {Pending, Initializing, Running, Resharding, Failed, Upgrading}. | `Running=1` | `Failed=1` for 5m → `ValkeyClusterPhaseFailed` |

**Diagnostic use**:

- `valkey_cluster_state_ok == 0 unless valkey_cluster_phase{phase="Initializing"} == 1`
  filters out the false positive while a cluster is initializing.
- `valkey_cluster_assigned_slots != 16384 and on(namespace,name) valkey_cluster_phase{phase="Resharding"} == 0`
  shows slot shortfall that is **not** a resharding-in-flight =
  a real incident.

## 3. Reconcile metrics (Counter / Histogram)

| Metric | Type | Meaning | Normal (5m rate) | Abnormal |
|---|---|---|---|---|
| `valkey_cluster_reconcile_total{namespace,name}` | Counter | Cumulative reconcile invocations. | `0.01 ~ 1 /s` (with `RequeueAfter` 30s+) | `> 5 /s` indicates thrashing |
| `valkey_cluster_reconcile_errors_total{namespace,name,component}` | Counter | Failures per reconcile stage. `component` = `secret` / `sts` / `svc` / `tls` / `backup` / … | `0` | `rate > 0.1 /s` for 5m → `ValkeyOperatorReconcileErrorsHigh` |
| `valkey_cluster_reconcile_duration_seconds{namespace,name,result}` | Histogram | Reconcile wall-clock latency. `result` = `success` / `error`. Buckets: 5 ms ~ 30 s. | p95 `< 1s` steady, `< 5s` during init/scale | p95 `> 5s` sustained = SLO breach |

**Histogram PromQL** (SLO tracking):

```promql
# Reconcile p95 (success only) — SLO of the typical operation
histogram_quantile(0.95,
  sum by (le, namespace, name) (
    rate(valkey_cluster_reconcile_duration_seconds_bucket{result="success"}[5m])
  )
)

# Reconcile p99 (all results) — worst-case
histogram_quantile(0.99,
  sum by (le, namespace, name) (
    rate(valkey_cluster_reconcile_duration_seconds_bucket[5m])
  )
)

# Average reconcile latency
rate(valkey_cluster_reconcile_duration_seconds_sum[5m])
  / rate(valkey_cluster_reconcile_duration_seconds_count[5m])
```

**Common patterns**:

- A reconcile-error rate concentrated on one component points to
  that component's dependency:
  - `secret` → missing AuthSecret / insufficient RBAC
  - `sts` → admission webhook reject / quota exhaustion
  - `tls` → cert-manager `Certificate` missing or `NotReady`
- A spike in total with zero errors = phase oscillation (every
  phase change triggers a reconcile).

## 4. Lifecycle event metrics (Counter)

| Metric | Meaning | Normal (1h rate) | Abnormal |
|---|---|---|---|
| `valkey_cluster_backup_total{namespace,name,phase}` | ValkeyBackup termination counter. `phase` = `Completed` / `Failed`. | `> 0` (`Completed` only), `0` (`Failed`) | `rate(...phase="Failed"[1h]) > 0.0017` (~1/week) → `ValkeyBackupFailureRateHigh` |
| `valkey_cluster_restore_total{namespace,name,phase}` | ValkeyRestore termination counter. | `0` (only during disaster recovery) | `rate(...phase="Failed"[1h]) > 0.0017` → `ValkeyRestoreFailureRateHigh` |
| `valkey_cluster_failover_total{namespace,name}` | Replication-mode automatic failover events. ADR-0017. | `0` | `increase(...[1h]) >= 2` → `ValkeyFailoverHigh` (≥ 2 in 1 h = infrastructure instability) |

**SLO formulas**:

- Backup success rate SLO:
  `1 - rate(backup_total{phase="Failed"}[7d]) / rate(backup_total[7d])`,
  target ≥ 99 %.
- Failover MTTR: `histogram_quantile(0.95, …)` — the Histogram is
  not yet defined; use the proxy in §6.

## 5. Operator-state metrics (Gauge / from kube-state)

| Metric | Source | Normal | Abnormal → alert |
|---|---|---|---|
| `up{job=~"valkey-operator.*"}` | Prometheus scrape | `1` | `0` for 2m → `ValkeyOperatorDown` (critical) |
| `valkey_cluster_build_info{version,commit,date}` | `SetBuildInfo` at boot, once | always `1` (identifies current release tag) | Label mismatch = `ldflags` injection failure (cycles 53–56) |
| `valkey_cluster_capability_active{namespace,name,capability}` | Updated on every reconcile (#64) | `active=1`, `inactive=0`. Capability tokens: TLS / TLS-AutoCA / Auth / Autoscaling / SlowLog / EncryptionAudit / EncryptionEnforce / NetworkPolicy / Monitoring | Use `sum by (capability) (valkey_cluster_capability_active)` for fleet-wide adoption tracking |

## 6. Derived / recommended PromQL (operator dashboards)

```promql
# Cluster availability (SLI)
avg_over_time(valkey_cluster_state_ok[5m])

# Reconcile latency proxy (Histogram not defined yet → rate ratio)
rate(valkey_cluster_reconcile_total[5m]) / scalar(count(up{job=~"valkey-operator.*"} == 1))

# Backup success rate (SLO)
1 - (
  rate(valkey_cluster_backup_total{phase="Failed"}[7d])
  / rate(valkey_cluster_backup_total[7d])
)

# Image identifier of the live operator (release sanity)
valkey_cluster_build_info * 0 + group_left(version, commit) (1)
```

## 7. Cardinality estimate (capacity planning)

| Operating scale | Estimated timeseries |
|---|---|
| 10 clusters (small) | ~110 (= 11 metrics × 10 clusters) |
| 100 clusters | ~1 100 |
| 1 000 clusters | ~11 000 (well below the single-Prometheus recommended limit of ~5–10 M) |

**On scale-out**: `reconcile_errors_total`'s `component`
dimension contributes `N × cluster`. When adding new component
labels, update this table.

## 8. Deliberately not collected

- Pod-level latency (per-instance `INFO latest_fork_usec` etc.) is
  outside the operator's scope. Collect with a dedicated
  `redis_exporter` sidecar.
- Key count and memory usage — same (sidecar exporter).
- Network bytes — covered by `node_exporter` / cAdvisor.

The operator's job is to surface **Kubernetes-level controller
state**. Data-plane telemetry lives on a separate layer.

## 9. References

- Code: `internal/controller/metrics.go`
- Alerts: `config/prometheus/alert-rules.yaml`
- Alert MTTR: `docs/operations/runbook.md` §9.x
- ADRs: ADR-0017 (failover), ADR-0027 (HPA deferred), ADR-0030
  (`build_info` ldflags).
