# ValkeyCluster Per-Shard PDB Operations Guide

> 한국어: [pdb-per-shard.ko.md](pdb-per-shard.ko.md)

Per-shard `PodDisruptionBudget` operations guide for sharded Valkey
clusters.

## Overview

In the default mode (`spec.podDisruptionBudget` unset or
`perShard: false`), the `ValkeyCluster` reconciler creates **one
cluster-wide PDB** with selector `app.kubernetes.io/instance=<cr>`.
Every pod across every shard shares a single disruption budget.

Limitations of the cluster-wide model:

- 3-shard × 2-replica = 6 pods → `aggregate minAvailable=5` (cluster-wide)
- During drain, **primaries from multiple shards may evict
  simultaneously** — leaving some shards in a `primary=0 + replica=1`
  state.
- No per-shard write-availability guarantee.

When `perShard: true` is enabled, the reconciler creates **N per-shard
PDBs**:

| PDB Name        | Selector                                                    | minAvailable (default) |
|-----------------|-------------------------------------------------------------|------------------------|
| `<cr>-shard-0`  | `instance=<cr>` + `valkey.keiailab.io/shard=0`              | `shardReplicas - 1`    |
| `<cr>-shard-1` | `instance=<cr>` + `valkey.keiailab.io/shard=1`              | `shardReplicas - 1`    |
| `<cr>-shard-N` | `instance=<cr>` + `valkey.keiailab.io/shard=N`              | `shardReplicas - 1`    |

Where `shardReplicas = 1 (primary) + ReplicasPerShard`.

## Enabling per-shard PDBs

See `config/samples/cache_v1alpha1_valkeycluster.yaml`. Add to the CR
spec:

```yaml
spec:
  shards: 3
  replicasPerShard: 1
  podDisruptionBudget:
    enabled: true
    perShard: true
```

Reconciler behaviour:

1. `shouldAutoCreatePDB=true` + `perShard=true` → apply N per-shard PDBs.
2. Cluster-wide PDB orphan cleanup (`EnsurePDBDeleted(<cr>)`) runs
   automatically.
3. Each shard's PDB protects only that shard's primary + replicas.

## Mode transitions

### `perShard: false → true` (cluster-wide → per-shard)

On the next reconcile:

1. Create N per-shard PDBs.
2. Delete the existing cluster-wide PDB (orphan cleanup).
3. Drain-safe at all times — at least one PDB remains active.

### `perShard: true → false` (per-shard → cluster-wide)

On the next reconcile:

1. Create a single cluster-wide PDB.
2. Delete N per-shard PDBs (orphan cleanup).

## Drain scenarios

For a 3-shard × `ReplicasPerShard=1` (6 pods total) ValkeyCluster, when
`kubectl drain <node>` runs:

**`perShard: false` (default)**:

- Cluster-wide PDB: `minAvailable=5`
- Drainable: `6 - 5 = 1` pod eviction allowed
- **Any shard's pod** may be evicted → if shard-0's primary drains,
  shard 0 falls to `primary=0` (writes blocked until replica promotion).

**`perShard: true`**:

- shard-0 PDB: `minAvailable=1` (default for `shardReplicas=2`)
- shard-1 PDB: `minAvailable=1`
- shard-2 PDB: `minAvailable=1`
- Drainable: one pod per shard (either the primary **or** a replica,
  never both)
- **Per-shard write availability guaranteed**.

## User-specified minAvailable

When `spec.podDisruptionBudget.minAvailable` or `maxUnavailable` is set
explicitly, the user value replaces the default. `perShard` is
independent — the same user value applies to every per-shard PDB.

## Compatibility

- Single-version `v1alpha1` (no conversion webhook needed).
- `PerShard` field is `+optional` with default `false` → existing
  `ValkeyCluster` resources are unaffected.
- On mode transition, the reconciler cleans up the orphan PDB →
  **zero data loss**.

## References

- Implementation: `internal/resources/pdb.go` (`BuildShardPDB`)
- Schema: `api/v1alpha1/valkeycluster_types.go` (`PerShard` field)
- Orphan cleanup helper: `internal/controller/pdb_default.go`
  (`EnsurePDBDeleted`)
- Sibling pattern: `mongodb-operator/internal/resources/builder.go`
- Unit tests: `internal/controller/pdb_default_test.go`
  (`BuildShardPDB` + `EnsurePDBDeleted` — all PASS)
