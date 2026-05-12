# Capacity planning — valkey-operator

> 한국어 버전: [capacity-planning.ko.md](capacity-planning.ko.md)

How to choose the initial spec for a `ValkeyCluster` or `Valkey`
CR. Use §3 (workload-pattern matrix) for the first sizing, then
§4 (operational signals) to re-tune weekly.

This is a **practical starting point**, not a complete capacity
model. Precise numbers come from the running metrics.

## 1. The four sizing dimensions

Every size is the product of:

```
Required resources = (workload pattern) × (data volume) × (QPS) × (availability requirement)
```

| Dimension | Unit | Resources it drives |
|---|---|---|
| Workload pattern | cache / session / queue / pub-sub / leaderboard | memory model, persistence, eviction |
| Data volume | active keyspace MB | memory request, PVC size |
| QPS | read / write / batch separately | CPU request, replica count |
| Availability requirement | RPO / RTO (minutes) | replicas, shards, backup cadence, target count |

## 2. Topology choice

```
                ┌─ Single instance, no persistence required → kind: Valkey, mode: Standalone
                │
Workload ───────┼─ 1 primary + N read replicas, RPO ≈ 0    → kind: Valkey, mode: Replication
                │
                └─ Distributed (>50 GB or >100 K write QPS) → kind: ValkeyCluster (sharded)
```

**Transition thresholds (rules of thumb)**:

- Standalone → Replication: any moment a 1-second data loss is
  unacceptable (automatic failover required).
- Replication → Cluster: when a single primary's memory exceeds
  **physical RAM × 0.5** or a single primary's CPU sits at 1 core
  80 %+ continuously.
- Add a Cluster shard: when a single shard's p95 latency exceeds
  the SLO.

## 3. Recommended starting points per workload pattern

### 3.1 Cache (read-heavy, eviction allowed)

| Field | Recommendation |
|---|---|
| memory | data set × 1.3 (30 % fragmentation headroom) |
| persistence | RDB (or off), AOF off |
| eviction | `allkeys-lru` or `allkeys-lfu` |
| replicas | 2 (read scaling) |
| backup | none or daily |
| Example spec | `replicas=2, requests.memory=4Gi, persistence.size=8Gi` |

### 3.2 Session store (balanced read/write, no loss)

| Field | Recommendation |
|---|---|
| memory | concurrent sessions × avg session size × 1.5 |
| persistence | AOF `everysec` |
| eviction | `noeviction` (TTL-driven natural expiry) |
| replicas | 2 (HA + read scaling) |
| backup | every 6 hours |
| Example spec | `replicas=2, requests.memory=8Gi, persistence.size=20Gi` |

### 3.3 Queue (write-heavy, FIFO)

| Field | Recommendation |
|---|---|
| memory | avg queue depth × message size × 1.5 |
| persistence | AOF `always` (zero-loss) or `everysec` |
| eviction | `noeviction` (queue overflow = absent consumer) |
| replicas | 2 (failover) |
| backup | hourly (no PITR — keep intervals short) |
| Example spec | `replicas=2, requests.cpu=1000m, requests.memory=4Gi` |

### 3.4 Pub-Sub (transient, no persistence)

| Field | Recommendation |
|---|---|
| memory | 256 MB ~ 1 GB (messages are not stored) |
| persistence | off |
| replicas | 0–1 (a single instance is fine if subscribers can reconnect) |
| Example spec | `replicas=1, requests.memory=512Mi, persistence.enabled=false` |

### 3.5 Leaderboard / sorted set (heavy compute, large dataset)

| Field | Recommendation |
|---|---|
| memory | (active leaderboards × top-N × avg(score+member size)) × 2 |
| persistence | RDB hourly + AOF `everysec` |
| replicas | 3 (2 read replicas distribute `ZRANGE` load) |
| topology | ValkeyCluster (when the leaderboard can be partitioned) |
| Example spec | `shards=3, replicas=2, requests.memory=8Gi/shard` |

## 4. Operational validation — first week metric checks

For the first 7 days after deployment, review these once per day
and re-tune the spec:

| Signal | PromQL | Threshold (re-tune trigger) |
|---|---|---|
| Memory usage | (sidecar exporter) `redis_memory_used_bytes / redis_memory_max_bytes` | `> 0.7` for 5 consecutive days → raise memory |
| QPS | `rate(redis_commands_total[5m])` | `> capacity × 0.7` → raise CPU or add a shard |
| Reconcile error rate | `rate(valkey_cluster_reconcile_errors_total[1h])` | `> 0` → investigate operational stability |
| Failover frequency | `increase(valkey_cluster_failover_total[7d])` | `> 1` → check infrastructure (network, nodes) |
| Backup success rate | `1 - rate(valkey_cluster_backup_total{phase="Failed"}[7d]) / rate(valkey_cluster_backup_total[7d])` | `< 0.99` → check backup infrastructure |

## 5. Starting `resources.requests` / `limits`

```yaml
# Valkey CR (recommended for Replication mode)
spec:
  resources:
    requests:
      cpu: 500m       # ~5 K QPS baseline
      memory: 2Gi     # 1.6 GB working set + 400 MB overhead
    limits:
      cpu: 2000m      # 4× burst
      memory: 2Gi     # request == limit (avoid OOM, no swap)
  storage:
    size: 10Gi        # data + AOF rewrite scratch = data × 2
    storageClass: gp3 # IOPS-guaranteed storage class
```

**Why `memory request == limit`**: the K8s scheduler only looks at
**available memory** per node when scheduling. Large limits without
matching requests risk node over-commit. Valkey treats OOM as
critical data loss.

**Why we allow CPU burst**: AOF rewrite, RDB save, and BGSAVE are
short background jobs that temporarily burst 2–4× CPU. Setting a
low CPU limit causes throttling → latency blow-up.

## 6. Availability matrix (replica / shard)

| RPO / RTO | Recommendation |
|---|---|
| RPO=0, RTO<10s | Replication 3 replicas + AOF `always` + auto failover |
| RPO<1s, RTO<30s | Replication 2 replicas + AOF `everysec` (default) |
| RPO<1h, RTO<5min | Replication 1 replica + RDB hourly + S3 backup |
| RPO<1d, RTO<1h | Standalone + RDB daily + PVC backup |

**Cluster-mode availability**: each shard inherits replication-mode
availability — shard replica count is
`Spec.Shards[].Replicas`. Shard-level quorum is a majority of
primaries (e.g. a 3-shard cluster keeps partial operation when 2
shards run — only the missing slot's data is impacted).

## 7. Out of scope

This guide assumes / excludes:

- Node infrastructure (CPU class, NIC bandwidth, disk IOPS) is
  **sufficient**. EBS gp2 and similar IOPS-capped volumes require
  separate sizing.
- Multi-region active-active is **outside** this operator's scope
  (needs a separate ADR / tooling).
- Hot-key scenarios (a single key carries 50 %+ of cluster QPS)
  are not solved by sharding — split at the application layer or
  add a caching layer.

## 8. References

- ADR-0017 — Replication failover policy.
- ADR-0027 — Why HPA is deferred (recommend manual spec change).
- runbook.md §4 — Scale up/down procedure.
- metrics-glossary.md — Semantics of the PromQL metrics used here.
