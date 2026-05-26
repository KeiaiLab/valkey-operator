# ValkeyCluster 分片级 PDB 运维指南 (简体中文)

> English: [pdb-per-shard.md](pdb-per-shard.md) — canonical / 正本

分片 Valkey 集群的分片级 `PodDisruptionBudget` 运维指南。

## 概述

默认模式 (`spec.podDisruptionBudget` 未指定或 `perShard: false`) 下,
`ValkeyCluster` reconciler 会创建**一个集群级 PDB**,selector 为
`app.kubernetes.io/instance=<cr>`。所有分片下的全部 pod 共用同一个
disruption budget。

集群级模型的局限:

- 3 分片 × 2 副本 = 6 pod → `aggregate minAvailable=5` (集群级)
- drain 过程中,**多个分片的 primary 可能被同时驱逐** —— 导致部分分片
  落入 `primary=0 + replica=1` 状态。
- 无法在分片粒度上保证写可用性。

启用 `perShard: true` 后,reconciler 会创建 **N 个分片级 PDB**:

| PDB 名称        | Selector                                                    | minAvailable (默认值) |
|-----------------|-------------------------------------------------------------|------------------------|
| `<cr>-shard-0`  | `instance=<cr>` + `valkey.keiailab.io/shard=0`              | `shardReplicas - 1`    |
| `<cr>-shard-1` | `instance=<cr>` + `valkey.keiailab.io/shard=1`              | `shardReplicas - 1`    |
| `<cr>-shard-N` | `instance=<cr>` + `valkey.keiailab.io/shard=N`              | `shardReplicas - 1`    |

其中 `shardReplicas = 1 (primary) + ReplicasPerShard`。

## 启用分片级 PDB

参见 `config/samples/cache_v1alpha1_valkeycluster.yaml`。在 CR spec 中
添加:

```yaml
spec:
  shards: 3
  replicasPerShard: 1
  podDisruptionBudget:
    enabled: true
    perShard: true
```

reconciler 行为:

1. `shouldAutoCreatePDB=true` + `perShard=true` → apply N 个分片级 PDB。
2. 集群级 PDB 的孤儿清理 (`EnsurePDBDeleted(<cr>)`) 会自动执行。
3. 每个分片的 PDB 仅保护本分片的 primary + replica。

## 模式切换

### `perShard: false → true` (集群级 → 分片级)

下一次 reconcile 时:

1. 创建 N 个分片级 PDB。
2. 删除原集群级 PDB (孤儿清理)。
3. 全程 drain-safe —— 至少有一个 PDB 始终处于活跃状态。

### `perShard: true → false` (分片级 → 集群级)

下一次 reconcile 时:

1. 创建一个集群级 PDB。
2. 删除 N 个分片级 PDB (孤儿清理)。

## Drain 场景

对于 3 分片 × `ReplicasPerShard=1` (共 6 pod) 的 ValkeyCluster,执行
`kubectl drain <node>` 时:

**`perShard: false` (默认)**:

- 集群级 PDB: `minAvailable=5`
- 可驱逐: `6 - 5 = 1` 个 pod
- **任意分片的 pod** 都可能被驱逐 → 若 shard-0 的 primary 被 drain,
  分片 0 会落入 `primary=0` (在 replica 被晋升前写入被阻塞)。

**`perShard: true`**:

- shard-0 PDB: `minAvailable=1` (`shardReplicas=2` 下的默认值)
- shard-1 PDB: `minAvailable=1`
- shard-2 PDB: `minAvailable=1`
- 可驱逐: 每个分片各 1 个 pod (primary **或** replica,绝不两者同时)
- **分片级写可用性得到保证**。

## 用户自定义 minAvailable

显式设置 `spec.podDisruptionBudget.minAvailable` 或 `maxUnavailable` 时,
用户值会覆盖默认值。`perShard` 与之独立 —— 同一个用户值会应用到每个
分片级 PDB。

## 兼容性

- 仅 `v1alpha1` 单版本 (无需 conversion webhook)。
- `PerShard` 字段为 `+optional`,默认值 `false` → 现有 `ValkeyCluster`
  资源不受影响。
- 模式切换时,reconciler 会清理掉孤儿 PDB → **零数据损失**。

## 参考

- 实现: `internal/resources/pdb.go` (`BuildShardPDB`)
- Schema: `api/v1alpha1/valkeycluster_types.go` (`PerShard` 字段)
- 孤儿清理 helper: `internal/controller/pdb_default.go`
  (`EnsurePDBDeleted`)
- 单元测试: `internal/controller/pdb_default_test.go`
  (`BuildShardPDB` + `EnsurePDBDeleted` —— 全部 PASS)
