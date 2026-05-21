# 容量规划 — valkey-operator (简体中文)

> English: [capacity-planning.md](capacity-planning.md) — canonical / 正本

如何为 `ValkeyCluster` 或 `Valkey` CR 选取 *初始 spec* 的容量规划手册。
正式上线前先用 §3 (工作负载模式矩阵) 完成首轮 sizing,再依据 §4 (运维信号) 按周复盘调整。

本文档不是一份 *完整的 capacity model*,而是 **一份实用的起点**。准确的数值要靠运行中
的指标来收敛。

## 1. sizing 的 4 个维度

任何 sizing 都是以下 4 个维度的乘积:

```
所需资源 = (工作负载模式) × (数据量) × (QPS) × (可用性要求)
```

| 维度 | 衡量单位 | 影响的资源 |
|---|---|---|
| 工作负载模式 | cache / session / queue / pub-sub / leaderboard | memory model、persistence、eviction |
| 数据量 | 活跃 keyspace MB | memory request、PVC size |
| QPS | read / write / batch 分别统计 | CPU request、replica 数 |
| 可用性要求 | RPO / RTO 分钟级 | replicas、shards、备份周期、target 数 |

## 2. 拓扑选型

```
                ┌─ 单实例 + 不需要持久化 → kind: Valkey, mode: Standalone
                │
工作负载 ───────┼─ 1 primary + N read replica,RPO ≈ 0 → kind: Valkey, mode: Replication
                │
                └─ 分布式 (>50 GB 或 >100 K write QPS) → kind: ValkeyCluster (sharded)
```

**切换阈值 (经验法则)**:

- Standalone → Replication: 一旦不能接受 1 秒级别的数据丢失 (需要自动 failover)。
- Replication → Cluster: 单 primary 内存超过 **物理 RAM × 0.5**,或单 primary
  CPU 持续 1 core 80%+。
- Cluster 增加 shard: 单 shard 的 p95 延迟超过 SLO。

## 3. 各工作负载模式的推荐起点

### 3.1 Cache (read-heavy,允许 eviction)

| 字段 | 推荐值 |
|---|---|
| memory | 数据集 × 1.3 (30 % fragmentation 余量) |
| persistence | RDB (或关闭),AOF 关闭 |
| eviction | `allkeys-lru` 或 `allkeys-lfu` |
| replicas | 2 (read scaling) |
| backup | 不需要,或每日 1 次 |
| spec 示例 | `replicas=2, requests.memory=4Gi, persistence.size=8Gi` |

### 3.2 Session store (读写均衡,不容许丢失)

| 字段 | 推荐值 |
|---|---|
| memory | 并发会话数 × 平均 session 大小 × 1.5 |
| persistence | AOF `everysec` |
| eviction | `noeviction` (依赖 TTL 自然过期) |
| replicas | 2 (HA + read scaling) |
| backup | 每 6 小时 1 次 |
| spec 示例 | `replicas=2, requests.memory=8Gi, persistence.size=20Gi` |

### 3.3 Queue (write-heavy,FIFO)

| 字段 | 推荐值 |
|---|---|
| memory | 平均 queue 深度 × message 大小 × 1.5 |
| persistence | AOF `always` (零丢失) 或 `everysec` |
| eviction | `noeviction` (queue 溢出 = consumer 缺失) |
| replicas | 2 (failover) |
| backup | 每小时 1 次 (没有 PITR — 间隔越短越好) |
| spec 示例 | `replicas=2, requests.cpu=1000m, requests.memory=4Gi` |

### 3.4 Pub-Sub (短暂消息,不需要持久化)

| 字段 | 推荐值 |
|---|---|
| memory | 256 MB ~ 1 GB (消息本身不存储) |
| persistence | 关闭 |
| replicas | 0–1 (订阅端能够 reconnect 时单实例即可) |
| spec 示例 | `replicas=1, requests.memory=512Mi, persistence.enabled=false` |

### 3.5 Leaderboard / sorted set (重计算,数据集大)

| 字段 | 推荐值 |
|---|---|
| memory | (活跃 leaderboard × top-N × 平均(score+member 大小)) × 2 |
| persistence | RDB 每小时 + AOF `everysec` |
| replicas | 3 (2 个 read replica 分担 `ZRANGE` 压力) |
| topology | 优先 ValkeyCluster (leaderboard 可拆分时) |
| spec 示例 | `shards=3, replicas=2, requests.memory=8Gi/shard` |

## 4. 运维验证 — 上线首周的指标核对

部署后的前 7 天,每天复盘下列指标一次,并按需调整 spec:

| 信号 | PromQL | 阈值 (触发调整) |
|---|---|---|
| 内存占用 | (sidecar exporter) `redis_memory_used_bytes / redis_memory_max_bytes` | 连续 5 天 `> 0.7` → 上调 memory |
| QPS | `rate(redis_commands_total[5m])` | `> capacity × 0.7` → 上调 CPU 或增加 shard |
| reconcile 错误率 | `rate(valkey_cluster_reconcile_errors_total[1h])` | `> 0` → 复查运维稳定性 |
| failover 频率 | `increase(valkey_cluster_failover_total[7d])` | `> 1` → 检查基础设施 (网络、节点) |
| backup 成功率 | `1 - rate(valkey_cluster_backup_total{phase="Failed"}[7d]) / rate(valkey_cluster_backup_total[7d])` | `< 0.99` → 检查备份基础设施 |

## 5. `resources.requests` / `limits` 起步配置

```yaml
# Valkey CR (Replication mode 推荐值)
spec:
  resources:
    requests:
      cpu: 500m       # 5 K QPS 基线
      memory: 2Gi     # 1.6 GB working set + 400 MB overhead
    limits:
      cpu: 2000m      # 4× burst
      memory: 2Gi     # request == limit (规避 OOM,禁用 swap)
  storage:
    size: 10Gi        # data + AOF rewrite 临时空间 = data × 2
    storageClass: gp3 # 优先 IOPS 有保障的 storage class
```

**为什么 `memory request == limit`**: K8s scheduler 仅依据节点的
**可用内存** 来调度。limit 过大而 requests 跟不上会造成节点 over-commit。
对于 Valkey 而言,OOM 等同于关键数据丢失。

**为什么允许 CPU burst**: AOF rewrite、RDB save、BGSAVE 是短时的后台任务,
会瞬时把 CPU 拉到 2–4 倍。CPU limit 设得太低会触发 throttle → 延迟暴涨。

## 6. 可用性矩阵 (replica / shard)

| RPO / RTO | 推荐配置 |
|---|---|
| RPO=0, RTO<10s | Replication 3 replica + AOF `always` + 自动 failover |
| RPO<1s, RTO<30s | Replication 2 replica + AOF `everysec` (默认) |
| RPO<1h, RTO<5min | Replication 1 replica + RDB hourly + S3 备份 |
| RPO<1d, RTO<1h | Standalone + RDB daily + PVC 备份 |

**Cluster 模式的可用性**: 每个 shard 继承 replication 模式的可用性 —
shard 内 replica 数由 `Spec.Shards[].Replicas` 决定。shard 层面的 quorum
是 primaries 的多数 (例如 3 shard 集群在 2 个 shard 正常时仍可部分服务 —
仅缺失 slot 的数据受影响)。

## 7. 范围之外

本指南假设 / 不覆盖以下内容:

- 节点基础设施 (CPU 等级、网卡带宽、磁盘 IOPS) **充足**。EBS gp2 等
  IOPS 受限的卷需要单独 sizing。
- 多 region active-active **不在** 本 operator 的作用域内 (需要额外的
  ADR / 工具)。
- 热 key 场景 (单 key 贡献 cluster 50%+ QPS) 无法通过 sharding 解决 —
  请在应用层拆分,或加一层 caching。

## 8. 参考

- ADR-0017 — Replication failover 策略。
- ADR-0027 — HPA 延后的原因 (推荐人工调整 spec)。
- runbook.md §4 — 扩缩容流程。
- metrics-glossary.md — 本文档使用的 PromQL 指标含义。
