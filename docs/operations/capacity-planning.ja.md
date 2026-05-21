# Capacity Planning — valkey-operator (日本語)

> English: [capacity-planning.md](capacity-planning.md) — canonical / 正本


ValkeyCluster / Valkey CR を *どの spec で起動するか* の見積もりガイド。
production 投入前に本書 §3 のワークロードパターンマトリクスで 1 次 sizing →
§4 の運用指標で *週次の再見積もり* を行う。

本書は *完全な capacity model* ではなく *実用的な出発点* である。正確な数値は
運用中の metric から導く。

## 1. 見積もりの 4 軸

すべての sizing は次の 4 軸の積として表される:

```
必要リソース = (ワークロードパターン) × (データ量) × (QPS) × (可用性要件)
```

| 軸 | 単位 | 影響を受けるリソース |
|---|---|---|
| ワークロードパターン | cache / session / queue / pub-sub / leaderboard | memory model, persistence, eviction |
| データ量 | 有効 keyspace の MB | memory request, PVC size |
| QPS | read / write / batch を分けて計測 | CPU request, replica 数 |
| 可用性要件 | RPO / RTO (分単位) | replicas, shards, backup 周期, target 数 |

## 2. トポロジー選択

```
                ┌─ 単一インスタンス + 永続化不要 → kind: Valkey, mode: Standalone
                │
ワークロード ───┼─ primary 1 + read replica N + RPO≈0 → kind: Valkey, mode: Replication
                │
                └─ 分散 (>50GB または write QPS >100K) → kind: ValkeyCluster (sharded)
```

**移行のしきい値 (経験則)**:
- Standalone → Replication: 1 秒のデータ損失すら許容できない時点 (自動 failover が必須)。
- Replication → Cluster: 単一 primary の memory が *physical RAM × 0.5* を超えるか、
  単一 primary の CPU が 1 core 80%+ で常時張り付く場合。
- Cluster shard の追加: 単一 shard の p95 latency が SLO を超えた時点。

## 3. ワークロードパターン別の推奨開始値

### 3.1 Cache (read-heavy, eviction 許容)

| 項目 | 推奨 |
|---|---|
| memory | データセット × 1.3 (fragmentation 30% の余裕) |
| persistence | RDB (無しでも可), AOF off |
| eviction | `allkeys-lru` または `allkeys-lfu` |
| replicas | 2 (read scaling) |
| backup | 不要または 1 日 1 回 |
| spec 例 | `replicas=2, requests.memory=4Gi, persistence.size=8Gi` |

### 3.2 Session store (read/write 均等, 損失不可)

| 項目 | 推奨 |
|---|---|
| memory | 同時セッション数 × 平均 session サイズ × 1.5 |
| persistence | AOF `everysec` |
| eviction | `noeviction` (TTL による自然失効) |
| replicas | 2 (HA + read scaling) |
| backup | 6 時間に 1 回 |
| spec 例 | `replicas=2, requests.memory=8Gi, persistence.size=20Gi` |

### 3.3 Queue (write-heavy, FIFO)

| 項目 | 推奨 |
|---|---|
| memory | 平均 queue depth × message サイズ × 1.5 |
| persistence | AOF `always` (損失 0 が要件の場合) または `everysec` |
| eviction | `noeviction` (queue overflow = consumer 不在のサイン) |
| replicas | 2 (failover) |
| backup | 1 時間に 1 回 (point-in-time なし — interval を短く保つ) |
| spec 例 | `replicas=2, requests.cpu=1000m, requests.memory=4Gi` |

### 3.4 Pub-Sub (transient, 永続化なし)

| 項目 | 推奨 |
|---|---|
| memory | 256MB ~ 1GB (メッセージ自体は保存されない) |
| persistence | off |
| replicas | 0〜1 (subscriber が reconnect 可能なら単一でも可) |
| spec 例 | `replicas=1, requests.memory=512Mi, persistence.enabled=false` |

### 3.5 Leaderboard / sorted set (重い計算 + 大規模データセット)

| 項目 | 推奨 |
|---|---|
| memory | (active leaderboards × top-N × 平均 score+member サイズ) × 2 |
| persistence | RDB 1 時間 + AOF `everysec` |
| replicas | 3 (read replica 2 — `ZRANGE` の負荷を分散) |
| topology | ValkeyCluster 推奨 (leaderboard を分割できる場合) |
| spec 例 | `shards=3, replicas=2, requests.memory=8Gi/shard` |

## 4. 見積もり検証 — 運用初週の metric チェック

デプロイ後 7 日間、次を 1 日 1 回確認して spec を補正する:

| 指標 | PromQL | しきい値 (補正トリガー) |
|---|---|---|
| memory 使用率 | (sidecar exporter) `redis_memory_used_bytes / redis_memory_max_bytes` | `> 0.7` が 5 日連続 → memory 増設 |
| QPS | `rate(redis_commands_total[5m])` | `> capacity × 0.7` → CPU 増設または shard 追加 |
| reconcile error rate | `rate(valkey_cluster_reconcile_errors_total[1h])` | `> 0` → 運用安定性を点検 |
| failover 頻度 | `increase(valkey_cluster_failover_total[7d])` | `> 1` → インフラ点検 (ネットワーク / ノード) |
| backup 成功率 | `1 - rate(valkey_cluster_backup_total{phase="Failed"}[7d]) / rate(valkey_cluster_backup_total[7d])` | `< 0.99` → backup インフラ点検 |

## 5. resources.requests / limits の開始値

```yaml
# Valkey CR (Replication mode の推奨値)
spec:
  resources:
    requests:
      cpu: 500m       # 平均 5K QPS を想定
      memory: 2Gi     # 1.6GB working set + 400MB overhead
    limits:
      cpu: 2000m      # burst 4 倍
      memory: 2Gi     # request == limit (OOM 予防, swap 回避)
  storage:
    size: 10Gi        # data + AOF rewrite の作業領域 = data × 2
    storageClass: gp3 # IOPS 保証つきの storage class
```

**memory request == limit を推奨する理由**: K8s scheduler はノード単位の
*available memory* だけを見て schedule する。limit だけ大きいとノードの
over-commit を招く。Valkey は OOM が即データ損失に直結する。

**CPU の burst を許容する理由**: AOF rewrite、RDB save、BGSAVE は CPU を
一時的に 2〜4 倍まで使う短時間の background ジョブである。CPU limit を
低くすると throttle が発生して latency が爆発する。

## 6. 可用性要件別の replica / shard マトリクス

| RPO / RTO | 推奨 |
|---|---|
| RPO=0, RTO<10s | Replication 3 replica + AOF `always` + 自動 failover |
| RPO<1s, RTO<30s | Replication 2 replica + AOF `everysec` (default) |
| RPO<1h, RTO<5min | Replication 1 replica + RDB hourly + S3 backup |
| RPO<1d, RTO<1h | Standalone + RDB daily + PVC backup |

**Cluster mode の可用性**: 各 shard は *replication mode と同等の可用性* を
持つ — shard ごとの replica 数は `Spec.Shards[].Replicas`。shard 自体の
quorum は primary の過半数 (例: 3 shard cluster は 2 shard が動作していれば
partial cluster として運用継続可能 — 欠落した slot のデータのみが影響を受ける)。

## 7. スコープ外

本ガイドは次を *前提 / 除外* する:

- ノードインフラ (CPU クラス、NIC bandwidth、disk IOPS) は *十分* であると
  仮定する。EBS gp2 のような IOPS 制限つき volume は別途見積もりが必要。
- マルチリージョン active-active は本 operator のスコープ外 (別 ADR / ツールが必要)。
- Hot-key シナリオ (単一 key が cluster 全体 QPS の 50%+ を占める) は
  sharding では解消しない — アプリケーション層で分割するか、キャッシュ層を
  追加する。

## 8. 参照

- ADR-0017 — Replication failover ポリシー
- ADR-0027 — HPA 見送りの理由 (手動 spec 変更を推奨)
- runbook.md §4 — scale up/down 手順
- metrics-glossary.md — 本書で利用する PromQL metric の意味
