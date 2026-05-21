# Secondary-Promote 切换

> English: [secondary-promote.md](secondary-promote.md) — canonical / 正本

> 将 ValkeyCluster 中的 replica 提升为 primary,随后下线旧 primary。
> 数据一致性优先模式 — 相比 RDB snapshot,优先保留 *当前 in-flight write*。

## 流程

```
初始:  primary(A)  replica(B,C)
1 步: primary(A)  replica(B,C)  + 强制 sync
2 步: primary(A)  replica(B,C)  + 阻止写入 (read-only)
3 步: primary(A)  replica(B,C)  + B 提升 → primary
4 步: replica(A)  primary(B)  replica(C)  (A 降级)
5 步: 下线 A
```

## 执行

### 1. 强制 replica 对齐

```bash
kubectl -n <ns> exec <cluster>-shard-0-0 -- valkey-cli WAIT 1 5000
# 5 秒内须有 1 个 replica ack — 不达标则重试
```

### 2. 阻止写入

```bash
kubectl -n <ns> patch valkeycluster <cluster> --type=merge \
    -p '{"spec":{"writePolicy":"ReadOnly"}}'
# operator 更新客户端 ConfigMap + 向 Pod ENV 传播
```

### 3. Replica 提升

```bash
kubectl -n <ns> exec <cluster>-shard-0-1 -- valkey-cli REPLICAOF NO ONE
kubectl -n <ns> annotate pod <cluster>-shard-0-1 \
    valkey.keiailab.com/primary-override=true
# operator reconcile → 更新 Service selector
```

### 4. 旧 primary 降级

```bash
kubectl -n <ns> exec <cluster>-shard-0-0 -- \
    valkey-cli REPLICAOF <cluster>-shard-0-1.<cluster>-headless 6379
```

### 5. 恢复写入

```bash
kubectl -n <ns> patch valkeycluster <cluster> --type=merge \
    -p '{"spec":{"writePolicy":"ReadWrite"}}'
```

## 验证

- 第 1 步: `WAIT` 输出 ≥1
- 第 3 步: 新 primary 的 `INFO replication` 显示 role:master
- 第 5 步: 连续写入 100 条 PASS

## Refs

- ROADMAP.md (P-C.3.2)
- `zero-downtime.zh.md` (父级流程)
