# Metrics Glossary — valkey-operator (日本語)

> English: [metrics-glossary.md](metrics-glossary.md) — canonical / 正本

operator が公開する全 Prometheus metric の **意味・タイプ・ラベル・正常範囲・
診断用途**。コードの SSOT は `internal/controller/metrics.go`、アラートの SSOT は
`config/prometheus/alert-rules.yaml`。

本 glossary は、運用者が dashboard やアラート上で **各 metric を *なぜ見るのか***
を理解するための reference。新規のアラート追加や dashboard 作成時はまず本表を
参照する。

## 1. 表記規約

- **タイプ**: Gauge (snapshot) / Counter (monotonic、`rate` で扱う) /
  Histogram (現状は未定義)。
- **ラベル**: cardinality を低く保ち、`namespace` + `name` のみ (cluster 単位)。
  pod / shard 単位のラベルは **意図的に除外** している — 100 shard cluster で
  timeseries が爆発するため。
- **正常範囲**: 定常運用中に metric が収まるべき **operating envelope**。閾値を
  超えるとアラートに紐づく (§5)。

## 2. ValkeyCluster の状態 metric (Gauge)

| Metric | 意味 | 正常 | 異常 → alert |
|---|---|---|---|
| `valkey_cluster_state_ok{namespace,name}` | `CLUSTER INFO cluster_state == ok` のとき `1`。 | `1` | `0` が 5m 継続 → `ValkeyClusterStateNotOK` (critical) |
| `valkey_cluster_assigned_slots{namespace,name}` | 割り当て済みハッシュスロット数。正常 = 16384 (Redis cluster spec)。 | `16384` | `< 16384` が 5m 継続 → `ValkeyClusterSlotsMismatch` (critical) |
| `valkey_cluster_shards{namespace,name}` | プライマリノード数 (= shard 数)。 | `spec.shards` と一致 | 不一致 = スケール進行中もしくは reconcile 失敗 |
| `valkey_cluster_ready_replicas{namespace,name}` | StatefulSet の `readyReplicas`。 | `>= spec.replicas` | `0` → critical、`0 < x < 2` → warning |
| `valkey_cluster_phase{namespace,name,phase}` | 現 phase のみ `1`、それ以外は `0`。`phase` ∈ {Pending, Initializing, Running, Resharding, Failed, Upgrading}。 | `Running=1` | `Failed=1` が 5m 継続 → `ValkeyClusterPhaseFailed` |

**診断用途**:

- `valkey_cluster_state_ok == 0 unless valkey_cluster_phase{phase="Initializing"} == 1`
  で、初期化中の cluster に対する false positive を除外できる。
- `valkey_cluster_assigned_slots != 16384 and on(namespace,name) valkey_cluster_phase{phase="Resharding"} == 0`
  は、resharding 進行中でないにもかかわらず slot が不足している = 実障害の検出。

## 3. Reconcile metric (Counter / Histogram)

| Metric | タイプ | 意味 | 正常 (5m rate) | 異常 |
|---|---|---|---|---|
| `valkey_cluster_reconcile_total{namespace,name}` | Counter | reconcile 呼び出し累計。 | `0.01 ~ 1 /s` (`RequeueAfter` 30s+ 前提) | `> 5 /s` は thrashing の兆候 |
| `valkey_cluster_reconcile_errors_total{namespace,name,component}` | Counter | reconcile 段階別の失敗数。`component` = `secret` / `sts` / `svc` / `tls` / `backup` / … | `0` | `rate > 0.1 /s` が 5m 継続 → `ValkeyOperatorReconcileErrorsHigh` |
| `valkey_cluster_reconcile_duration_seconds{namespace,name,result}` | Histogram | reconcile の wall-clock latency。`result` = `success` / `error`。Buckets: 5ms ~ 30s。 | p95 `< 1s` (steady)、`< 5s` (init/scale 時) | p95 `> 5s` が継続 = SLO 違反 |

**Histogram 活用 PromQL** (SLO 追跡):

```promql
# Reconcile p95 (success のみ) — 通常運用の SLO
histogram_quantile(0.95,
  sum by (le, namespace, name) (
    rate(valkey_cluster_reconcile_duration_seconds_bucket{result="success"}[5m])
  )
)

# Reconcile p99 (全結果) — worst-case
histogram_quantile(0.99,
  sum by (le, namespace, name) (
    rate(valkey_cluster_reconcile_duration_seconds_bucket[5m])
  )
)

# Reconcile の平均 latency
rate(valkey_cluster_reconcile_duration_seconds_sum[5m])
  / rate(valkey_cluster_reconcile_duration_seconds_count[5m])
```

**典型パターン**:

- reconcile error rate が特定 component に集中する場合は、その component の
  依存先を疑う。
  - `secret` → AuthSecret 欠落 / RBAC 不足
  - `sts` → admission webhook reject / quota 枯渇
  - `tls` → cert-manager `Certificate` が未生成または `NotReady`
- Total が急増しているのに error がゼロ = phase の振動 (phase が変わるたびに
  reconcile が発火している)。

## 4. Lifecycle event metric (Counter)

| Metric | 意味 | 正常 (1h rate) | 異常 |
|---|---|---|---|
| `valkey_cluster_backup_total{namespace,name,phase}` | ValkeyBackup の終了カウンタ。`phase` = `Completed` / `Failed`。 | `> 0` (`Completed` のみ)、`0` (`Failed`) | `rate(...phase="Failed"[1h]) > 0.0017` (≈ 1 週 1 回) → `ValkeyBackupFailureRateHigh` |
| `valkey_cluster_restore_total{namespace,name,phase}` | ValkeyRestore の終了カウンタ。 | `0` (災害復旧時以外は発生しない) | `rate(...phase="Failed"[1h]) > 0.0017` → `ValkeyRestoreFailureRateHigh` |
| `valkey_cluster_failover_total{namespace,name}` | replication mode の自動 failover 発生数。ADR-0017。 | `0` | `increase(...[1h]) >= 2` → `ValkeyFailoverHigh` (1 時間に 2 回以上 = インフラ不安定) |

**SLO の組み立て**:

- backup 成功率の SLO:
  `1 - rate(backup_total{phase="Failed"}[7d]) / rate(backup_total[7d])`、
  目標 ≥ 99%。
- failover MTTR: `histogram_quantile(0.95, …)` — Histogram は未定義なので
  §6 の proxy を用いる。

## 5. operator 自身の状態 metric (Gauge / from kube-state)

| Metric | source | 正常 | 異常 → alert |
|---|---|---|---|
| `up{job=~"valkey-operator.*"}` | Prometheus scrape | `1` | `0` が 2m 継続 → `ValkeyOperatorDown` (critical) |
| `valkey_cluster_build_info{version,commit,date}` | 起動時に `SetBuildInfo` で 1 回設定 | 常に `1` (現リリースタグの識別用) | ラベル不一致 = `ldflags` 注入失敗 (cycles 53–56) |
| `valkey_cluster_capability_active{namespace,name,capability}` | reconcile 毎に更新 (PR #64) | `active=1`、`inactive=0`。capability トークン: TLS / TLS-AutoCA / Auth / Autoscaling / SlowLog / EncryptionAudit / EncryptionEnforce / NetworkPolicy / Monitoring | fleet 全体の採用状況追跡には `sum by (capability) (valkey_cluster_capability_active)` を使う |

## 6. 派生 / 推奨 PromQL (運用 dashboard)

```promql
# Cluster 可用性 (SLI)
avg_over_time(valkey_cluster_state_ok[5m])

# Reconcile latency proxy (Histogram 未定義 → rate 比で代用)
rate(valkey_cluster_reconcile_total[5m]) / scalar(count(up{job=~"valkey-operator.*"} == 1))

# Backup 成功率 (SLO)
1 - (
  rate(valkey_cluster_backup_total{phase="Failed"}[7d])
  / rate(valkey_cluster_backup_total[7d])
)

# 稼働中 operator の image 識別 (release sanity)
valkey_cluster_build_info * 0 + group_left(version, commit) (1)
```

## 7. cardinality 推定 (capacity planning)

| 運用規模 | timeseries 推定 |
|---|---|
| 10 cluster (小規模) | ~110 (= 11 metric × 10 cluster) |
| 100 cluster | ~1,100 |
| 1,000 cluster | ~11,000 (Prometheus 単一インスタンスの推奨上限 ~5–10M を十分に下回る) |

**スケールアウト時の注意**: `reconcile_errors_total` の `component` 次元は
`N × cluster` で増える。新規 component ラベル追加時は本表を更新する。

## 8. 意図的に取得しないもの

- pod 単位の latency (個別 instance の `INFO latest_fork_usec` など) は operator
  のスコープ外。専用 `redis_exporter` sidecar で取得する。
- key 件数とメモリ使用量 — 同上 (sidecar exporter)。
- ネットワークバイト数 — `node_exporter` / cAdvisor で取得する。

operator の責務は **Kubernetes レベルのコントローラ状態** を可視化することに
ある。データプレーン側のテレメトリは別レイヤで扱う。

## 9. 参照

- コード: `internal/controller/metrics.go`
- アラート: `config/prometheus/alert-rules.yaml`
- アラート MTTR: `docs/operations/runbook.md` §9.x
- ADR: ADR-0017 (failover)、ADR-0027 (HPA deferred)、ADR-0030
  (`build_info` ldflags)。
