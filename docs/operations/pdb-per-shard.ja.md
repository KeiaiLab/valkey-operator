# ValkeyCluster Per-Shard PDB 運用ガイド (日本語)

> English: [pdb-per-shard.md](pdb-per-shard.md) — canonical / 正本

シャーディングされた Valkey クラスタに対する shard 単位の `PodDisruptionBudget` 運用ガイド。

## 概要

既定動作 (`spec.podDisruptionBudget` 未設定、または `perShard: false`) では、`ValkeyCluster` reconciler は selector が `app.kubernetes.io/instance=<cr>` の **クラスタ全体に対する PDB 1 件** を作成する。全 shard の全 pod が単一の disruption budget を共有することになる。

クラスタ全体モデルの限界:

- 3-shard × 2-replica = 6 pod → `aggregate minAvailable=5` (cluster-wide)
- drain 時に **複数 shard のプライマリが同時に evict される可能性** がある — 一部の shard が `primary=0 + replica=1` の状態に陥りうる。
- shard 単位の書き込み可用性は保証されない。

`perShard: true` を有効化すると、reconciler は **shard 単位の PDB を N 件** 作成する:

| PDB Name        | Selector                                                    | minAvailable (default) |
|-----------------|-------------------------------------------------------------|------------------------|
| `<cr>-shard-0`  | `instance=<cr>` + `valkey.keiailab.io/shard=0`              | `shardReplicas - 1`    |
| `<cr>-shard-1` | `instance=<cr>` + `valkey.keiailab.io/shard=1`              | `shardReplicas - 1`    |
| `<cr>-shard-N` | `instance=<cr>` + `valkey.keiailab.io/shard=N`              | `shardReplicas - 1`    |

ここで `shardReplicas = 1 (primary) + ReplicasPerShard` である。

## Per-shard PDB の有効化

`config/samples/cache_v1alpha1_valkeycluster.yaml` を参照。CR spec に以下を追加する:

```yaml
spec:
  shards: 3
  replicasPerShard: 1
  podDisruptionBudget:
    enabled: true
    perShard: true
```

reconciler の挙動:

1. `shouldAutoCreatePDB=true` + `perShard=true` → shard 単位の PDB N 件を apply する。
2. cluster-wide PDB の orphan cleanup (`EnsurePDBDeleted(<cr>)`) が自動実行される。
3. 各 shard の PDB は当該 shard のプライマリとレプリカのみを保護する。

## モード遷移

### `perShard: false → true` (cluster-wide → per-shard)

次回 reconcile 時:

1. shard 単位の PDB を N 件作成する。
2. 既存の cluster-wide PDB を削除する (orphan cleanup)。
3. 常に drain-safe — PDB は最低 1 件が active 状態を保つ。

### `perShard: true → false` (per-shard → cluster-wide)

次回 reconcile 時:

1. cluster-wide PDB を 1 件作成する。
2. shard 単位の PDB N 件を削除する (orphan cleanup)。

## Drain シナリオ

3-shard × `ReplicasPerShard=1` (合計 6 pod) の ValkeyCluster に対して `kubectl drain <node>` を実行した場合:

**`perShard: false` (既定)**:

- Cluster-wide PDB: `minAvailable=5`
- Drain 可能数: `6 - 5 = 1` pod の evict が許可される
- **任意の shard の pod** が evict 対象になりうる → shard-0 のプライマリが drain された場合、shard 0 は `primary=0` 状態 (レプリカが昇格するまで書き込み停止) に陥る。

**`perShard: true`**:

- shard-0 PDB: `minAvailable=1` (`shardReplicas=2` の既定値)
- shard-1 PDB: `minAvailable=1`
- shard-2 PDB: `minAvailable=1`
- Drain 可能数: shard ごとに 1 pod (プライマリ **または** レプリカのいずれか一方のみ、両方は不可)
- **shard 単位の書き込み可用性が保証される**。

## ユーザー指定の minAvailable

`spec.podDisruptionBudget.minAvailable` または `maxUnavailable` を明示的に設定した場合、既定値はユーザー指定値で置換される。`perShard` の値とは独立しており、同一のユーザー指定値が全ての shard 単位 PDB に適用される。

## 互換性

- `v1alpha1` 単一バージョン (conversion webhook 不要)。
- `PerShard` フィールドは `+optional` で既定 `false` → 既存の `ValkeyCluster` リソースには影響なし。
- モード遷移時は reconciler が orphan PDB を片付けるため **データ損失ゼロ**。

## 参照

- 実装: `internal/resources/pdb.go` (`BuildShardPDB`)
- スキーマ: `api/v1alpha1/valkeycluster_types.go` (`PerShard` フィールド)
- Orphan cleanup ヘルパー: `internal/controller/pdb_default.go` (`EnsurePDBDeleted`)
- 兄弟パターン: `mongodb-operator/internal/resources/builder.go`
- ユニットテスト: `internal/controller/pdb_default_test.go` (`BuildShardPDB` + `EnsurePDBDeleted` — 全て PASS)
