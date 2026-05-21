# ValkeyCluster Per-Shard PDB 운영 가이드 (한국어)

> English: [pdb-per-shard.md](pdb-per-shard.md) — canonical / 정본

Sharded Valkey 클러스터의 shard 별 `PodDisruptionBudget` 운영 가이드.

## 개요

기본 동작 (`spec.podDisruptionBudget` 미명시 또는 `PerShard=false`) 에서 ValkeyCluster reconciler 는 *cluster-wide PDB 1건* 을 생성한다 — selector = `app.kubernetes.io/instance=<cr>`. 모든 shard 의 pod 가 *동일 disruption budget* 을 공유.

이 모델의 한계:

- 3-shard × 2-replica = 6 pod 중 *aggregate minAvailable=5* (cluster-wide 기준)
- drain 시 *모든 shard 의 primary 가 동시 evict* 가능 — 일부 shard 가 *primary 0 + replica 1* 으로 빠질 수 있다
- shard 단위 *write availability* 보장 안 됨

`PerShard=true` (opt-in) 로 활성화 시 reconciler 는 *shard 별 PDB N건* 을 생성한다:

| PDB Name | Selector | minAvailable (default) |
|---|---|---|
| `<cr>-shard-0` | `instance=<cr>` + `valkey.keiailab.io/shard=0` | `shardReplicas - 1` |
| `<cr>-shard-1` | `instance=<cr>` + `valkey.keiailab.io/shard=1` | `shardReplicas - 1` |
| `<cr>-shard-N` | `instance=<cr>` + `valkey.keiailab.io/shard=N` | `shardReplicas - 1` |

여기서 `shardReplicas = 1 (primary) + ReplicasPerShard`.

## 활성화

`config/samples/cache_v1alpha1_valkeycluster.yaml` 참조. CR spec 에 추가:

```yaml
spec:
  shards: 3
  replicasPerShard: 1
  podDisruptionBudget:
    enabled: true
    perShard: true
```

reconciler 동작:

1. `shouldAutoCreatePDB=true` + `PerShard=true` → shard 별 PDB N건 apply
2. cluster-wide PDB orphan cleanup (`EnsurePDBDeleted(<cr>)`) 자동 실행
3. shard 별 PDB 가 *각 shard 의 primary + replica* 만 보호

## Mode 전환

### `PerShard=false → true` (cluster-wide → per-shard)

reconciler 는 다음 reconcile 시:

1. shard 별 PDB N건 새로 생성
2. 기존 cluster-wide PDB 자동 삭제 (orphan cleanup)
3. drain-safe 상태 유지 (PDB 항상 1건 이상 active)

### `PerShard=true → false` (per-shard → cluster-wide)

reconciler 는 다음 reconcile 시:

1. cluster-wide PDB 1건 새로 생성
2. shard 별 PDB N건 자동 삭제 (orphan cleanup)

## Drain 시나리오

3-shard × `ReplicasPerShard=1` (총 6 pod) ValkeyCluster 에서 `kubectl drain <node>` 실행 시:

**PerShard=false (default)**:

- cluster-wide PDB: `minAvailable=5`
- drain 가능: 6 - 5 = 1 pod evict 허용
- *어느 shard 의 pod 든* evict 가능 → shard 0 primary 가 drain 되면 *shard 0 primary 0* 상태로 빠진다 (replica promote 까지 write 차단)

**PerShard=true**:

- shard 0 PDB: `minAvailable=1` (shardReplicas=2 의 default)
- shard 1 PDB: `minAvailable=1`
- shard 2 PDB: `minAvailable=1`
- drain 가능: 각 shard 당 1 pod evict 허용 (shard 별 *primary 또는 replica 중 하나만*)
- *shard 별 write availability 보장*

## 사용자 명시 minAvailable

`spec.podDisruptionBudget.minAvailable` 또는 `maxUnavailable` 을 명시하면 default 대신 그 값을 사용한다. `PerShard` 영향 없음 (각 shard PDB 가 동일 값 적용).

## 호환성

- v1alpha1 single version (Conversion webhook 불요)
- `PerShard` field 는 `+optional` + default `false` → 기존 ValkeyCluster 무영향
- mode 전환 시 reconciler 가 orphan PDB 자동 cleanup → 데이터 손실 0

## Refs

- 구현: `internal/resources/pdb.go` `BuildShardPDB`
- PerShard field schema: `api/v1alpha1/valkeycluster_types.go`
- PDB delete path helper: `internal/controller/pdb_default.go` `EnsurePDBDeleted`
- 형제 패턴: `mongodb-operator/internal/resources/builder.go`
- 유닛 테스트: `internal/controller/pdb_default_test.go` (BuildShardPDB + EnsurePDBDeleted PASS)
