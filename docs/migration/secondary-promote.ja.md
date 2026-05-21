# Secondary-Promote Cutover

> English: [secondary-promote.md](secondary-promote.md) — canonical / 正本

> ValkeyCluster の replica を primary に昇格させた後、既存 primary を廃止する。
> データ整合性を最優先するパターン — RDB snapshot よりも *現在 in-flight な write* の保護を優先する。

## 流れ

```
初期:  primary(A)  replica(B,C)
1step: primary(A)  replica(B,C)  + sync 強制
2step: primary(A)  replica(B,C)  + write 遮断 (read-only)
3step: primary(A)  replica(B,C)  + B promote → primary
4step: replica(A)  primary(B)  replica(C)  (A demote)
5step: A teardown
```

## 実行

### 1. Replica 整合の強制

```bash
kubectl -n <ns> exec <cluster>-shard-0-0 -- valkey-cli WAIT 1 5000
# 5 秒以内に 1 replica が ack — 未達なら retry
```

### 2. Write 遮断

```bash
kubectl -n <ns> patch valkeycluster <cluster> --type=merge \
    -p '{"spec":{"writePolicy":"ReadOnly"}}'
# operator が client config map を更新し Pod の ENV に伝播させる
```

### 3. Replica の promote

```bash
kubectl -n <ns> exec <cluster>-shard-0-1 -- valkey-cli REPLICAOF NO ONE
kubectl -n <ns> annotate pod <cluster>-shard-0-1 \
    valkey.keiailab.com/primary-override=true
# operator が reconcile し Service の selector を更新する
```

### 4. Old primary の demote

```bash
kubectl -n <ns> exec <cluster>-shard-0-0 -- \
    valkey-cli REPLICAOF <cluster>-shard-0-1.<cluster>-headless 6379
```

### 5. Write の復帰

```bash
kubectl -n <ns> patch valkeycluster <cluster> --type=merge \
    -p '{"spec":{"writePolicy":"ReadWrite"}}'
```

## Verify

- Step 1: `WAIT` の出力が ≥ 1
- Step 3: 新 primary の `INFO replication` が role:master を返す
- Step 5: 連続 write 100 件 PASS

## Refs

- ROADMAP.md (P-C.3.2)
- `zero-downtime.md` (parent の流れ)
