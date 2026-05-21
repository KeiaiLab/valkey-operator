# 指标术语表 — valkey-operator (简体中文)

> English: [metrics-glossary.md](metrics-glossary.md) — canonical / 正本

operator 对外暴露的全部 Prometheus 指标 — 包括其 **含义、类型、标签、正常
范围与诊断用途**。代码 SSOT: `internal/controller/metrics.go`;告警 SSOT:
`config/prometheus/alert-rules.yaml`。

本术语表是 **运维人员在 dashboard 或告警中查看每个指标时,理解“为何要看
这个指标”的参考手册**。新增告警或 dashboard 时,请先查阅本表。

## 1. 表示约定

- **类型**: Gauge (快照) / Counter (单调递增,使用 `rate`) /
  Histogram (目前尚未定义)。
- **标签**: 为压低 cardinality,仅保留 `namespace` + `name`
  (集群粒度)。pod 级、shard 级标签被 **有意省略**,以避免 100
  分片集群下的 timeseries 爆炸。
- **正常范围**: 稳态运行时指标应处于的 **运行包络**。一旦突破阈值,
  即对应到某条告警 (§5)。

## 2. ValkeyCluster 状态指标 (Gauge)

| 指标 | 含义 | 正常值 | 异常 → 告警 |
|---|---|---|---|
| `valkey_cluster_state_ok{namespace,name}` | `CLUSTER INFO cluster_state == ok` 时为 `1`。 | `1` | `0` 持续 5m → `ValkeyClusterStateNotOK` (critical) |
| `valkey_cluster_assigned_slots{namespace,name}` | 已分配的 hash slot 数量。正常值 = 16384 (Redis cluster 规范)。 | `16384` | `< 16384` 持续 5m → `ValkeyClusterSlotsMismatch` (critical) |
| `valkey_cluster_shards{namespace,name}` | primary 节点数量 (= 分片数)。 | 与 `spec.shards` 一致 | 不一致 = 扩缩容进行中,或 reconcile 失败 |
| `valkey_cluster_ready_replicas{namespace,name}` | StatefulSet 的 `readyReplicas`。 | `>= spec.replicas` | `0` → critical;`0 < x < 2` → warning |
| `valkey_cluster_phase{namespace,name,phase}` | 当前 phase 为 `1`,其余 phase 为 `0`。`phase` ∈ {Pending, Initializing, Running, Resharding, Failed, Upgrading}。 | `Running=1` | `Failed=1` 持续 5m → `ValkeyClusterPhaseFailed` |

**诊断用途**:

- `valkey_cluster_state_ok == 0 unless valkey_cluster_phase{phase="Initializing"} == 1`
  可过滤掉集群正在初始化时的假阳性。
- `valkey_cluster_assigned_slots != 16384 and on(namespace,name) valkey_cluster_phase{phase="Resharding"} == 0`
  可凸显 **不是** resharding 进行中却出现的 slot 缺失 =
  真正的故障。

## 3. Reconcile 指标 (Counter / Histogram)

| 指标 | 类型 | 含义 | 正常值 (5m rate) | 异常 |
|---|---|---|---|---|
| `valkey_cluster_reconcile_total{namespace,name}` | Counter | reconcile 累计调用次数。 | `0.01 ~ 1 /s` (基于 `RequeueAfter` 30s+) | `> 5 /s` 即为抖动 |
| `valkey_cluster_reconcile_errors_total{namespace,name,component}` | Counter | 各 reconcile 阶段的失败计数器。`component` = `secret` / `sts` / `svc` / `tls` / `backup` / … | `0` | `rate > 0.1 /s` 持续 5m → `ValkeyOperatorReconcileErrorsHigh` |
| `valkey_cluster_reconcile_duration_seconds{namespace,name,result}` | Histogram | reconcile 的 wall-clock 延迟。`result` = `success` / `error`。Buckets: 5 ms ~ 30 s。 | 稳态 p95 `< 1s`;init/scale 期间 `< 5s` | p95 `> 5s` 持续 = SLO 违约 |

**Histogram PromQL** (SLO 追踪):

```promql
# reconcile p95 (仅 success) — 典型操作的 SLO
histogram_quantile(0.95,
  sum by (le, namespace, name) (
    rate(valkey_cluster_reconcile_duration_seconds_bucket{result="success"}[5m])
  )
)

# reconcile p99 (全部 result) — 最差情况
histogram_quantile(0.99,
  sum by (le, namespace, name) (
    rate(valkey_cluster_reconcile_duration_seconds_bucket[5m])
  )
)

# reconcile 平均延迟
rate(valkey_cluster_reconcile_duration_seconds_sum[5m])
  / rate(valkey_cluster_reconcile_duration_seconds_count[5m])
```

**常见模式**:

- reconcile 错误率集中在某个 component,即指向该 component 的依赖问题:
  - `secret` → AuthSecret 缺失 / RBAC 不足
  - `sts` → admission webhook 拒绝 / quota 耗尽
  - `tls` → cert-manager `Certificate` 缺失或 `NotReady`
- total 飙升但 errors 为零 = phase 振荡 (每次 phase 变化都会触发
  reconcile)。

## 4. 生命周期事件指标 (Counter)

| 指标 | 含义 | 正常值 (1h rate) | 异常 |
|---|---|---|---|
| `valkey_cluster_backup_total{namespace,name,phase}` | ValkeyBackup 终止计数器。`phase` = `Completed` / `Failed`。 | `> 0` (仅 `Completed`),`0` (`Failed`) | `rate(...phase="Failed"[1h]) > 0.0017` (约每周 1 次) → `ValkeyBackupFailureRateHigh` |
| `valkey_cluster_restore_total{namespace,name,phase}` | ValkeyRestore 终止计数器。 | `0` (仅在灾难恢复时非零) | `rate(...phase="Failed"[1h]) > 0.0017` → `ValkeyRestoreFailureRateHigh` |
| `valkey_cluster_failover_total{namespace,name}` | 复制集模式下的自动 failover 事件。ADR-0017。 | `0` | `increase(...[1h]) >= 2` → `ValkeyFailoverHigh` (1h 内 ≥ 2 次 = 基础设施不稳定) |

**SLO 公式**:

- 备份成功率 SLO:
  `1 - rate(backup_total{phase="Failed"}[7d]) / rate(backup_total[7d])`,
  目标 ≥ 99 %。
- failover MTTR: `histogram_quantile(0.95, …)` — Histogram 尚未定义,
  请使用 §6 中的代理指标。

## 5. operator 自身状态指标 (Gauge / from kube-state)

| 指标 | 来源 | 正常值 | 异常 → 告警 |
|---|---|---|---|
| `up{job=~"valkey-operator.*"}` | Prometheus scrape | `1` | `0` 持续 2m → `ValkeyOperatorDown` (critical) |
| `valkey_cluster_build_info{version,commit,date}` | 启动时调用一次 `SetBuildInfo` | 恒为 `1` (用于识别当前的 release tag) | label 不匹配 = `ldflags` 注入失败 (cycles 53–56) |
| `valkey_cluster_capability_active{namespace,name,capability}` | 每次 reconcile 都会刷新 (PR #64) | `active=1`,`inactive=0`。capability 取值: TLS / TLS-AutoCA / Auth / Autoscaling / SlowLog / EncryptionAudit / EncryptionEnforce / NetworkPolicy / Monitoring | 用 `sum by (capability) (valkey_cluster_capability_active)` 跟踪 fleet 级的采用情况 |

## 6. 派生 / 推荐 PromQL (运维 dashboard)

```promql
# 集群可用性 (SLI)
avg_over_time(valkey_cluster_state_ok[5m])

# reconcile 延迟代理 (Histogram 尚未定义 → 用 rate 比值)
rate(valkey_cluster_reconcile_total[5m]) / scalar(count(up{job=~"valkey-operator.*"} == 1))

# 备份成功率 (SLO)
1 - (
  rate(valkey_cluster_backup_total{phase="Failed"}[7d])
  / rate(valkey_cluster_backup_total[7d])
)

# 运行中 operator 的镜像标识 (release 自检)
valkey_cluster_build_info * 0 + group_left(version, commit) (1)
```

## 7. cardinality 估算 (容量规划)

| 运营规模 | timeseries 估算 |
|---|---|
| 10 个集群 (小规模) | ~110 (= 11 个指标 × 10 个集群) |
| 100 个集群 | ~1 100 |
| 1 000 个集群 | ~11 000 (远低于 Prometheus 单实例约 5–10 M 的推荐上限) |

**横向扩容时**: `reconcile_errors_total` 的 `component` 维度会贡献
`N × cluster` 的乘积。新增 component label 时,请同步更新本表。

## 8. 有意未采集的指标

- Pod 级延迟 (单实例的 `INFO latest_fork_usec` 等) 超出 operator 作用域。
  请通过专用的 `redis_exporter` sidecar 采集。
- key 数量与内存使用 — 同上 (sidecar exporter)。
- 网络字节数 — 由 `node_exporter` / cAdvisor 覆盖。

operator 的职责是暴露 **Kubernetes 层面的控制器状态**;数据面遥测属于
另一层。

## 9. 参考

- 代码: `internal/controller/metrics.go`
- 告警: `config/prometheus/alert-rules.yaml`
- 告警 MTTR: `docs/operations/runbook.md` §9.x
- ADR: ADR-0017 (failover)、ADR-0027 (HPA deferred)、ADR-0030
  (`build_info` ldflags)。
