# Sentinel → valkey-operator Replication 模式迁移 runbook (简体中文)

> English: [sentinel-migration.md](sentinel-migration.md) — canonical / 正本

> ADR-0017 (Replication Mode Failover) 决定不交付 Sentinel 模式;本文是给
> **外部用户的迁移路径**。

## 背景

valkey-operator 在 ADR-0017 中明确不实现 Sentinel,改用
**Replication 模式 + AutoFailover** (operator 主导的 leader election +
STS rollout + 选取 `master_repl_offset` 最大的 replica) 来提供等价的可用性。

本 runbook 面向 **已有 Sentinel 架构的 Redis/Valkey 部署**,指导运维人员将
其迁移到 valkey-operator。

## 可用性对照

| 维度 | Sentinel HA (现状) | valkey-operator Replication + AutoFailover |
|---|---|---|
| failover 决策 | Sentinel quorum 投票 | operator leader election + ADR-0017 取 `master_repl_offset` 最大者 |
| 无数据丢失保证 | Sentinel `min-replicas-to-write` 守卫 | Replication 模式下通过 `additionalConfig` 提供等价配置 |
| 恢复时间 | Sentinel tilt 阈值 (~5–30 s) | operator reconcile interval (~10–30 s,`RequeueAfter` `requeueSteady`) |
| split-brain 防护 | Sentinel quorum (≥ 3) | operator leader election (single leader,K8s `Lease`) |
| 客户端发现 | Sentinel-aware client (Sentinel 地址池) | Service ClusterIP / DNS (`<name>.<ns>.svc.cluster.local`) |

**核心差异**: 客户端发现方式。Sentinel-aware 客户端 (jedis / redisson /
go-redis sentinel 模式) 必须改造为 **Service-aware** 客户端。

## 4 步迁移流程

### 第 1 步 — 盘点现有 Sentinel 架构

```bash
# 识别 Sentinel 实例
kubectl -n <ns> get pods -l app.kubernetes.io/component=sentinel
kubectl -n <ns> get svc <release>-sentinel

# 当前 master / replica 映射
kubectl -n <ns> exec -it <sentinel-pod> -- redis-cli -p 26379 sentinel masters
kubectl -n <ns> exec -it <sentinel-pod> -- redis-cli -p 26379 sentinel slaves <master-name>

# 当前的持久化设置
kubectl -n <ns> exec -it <master-pod> -- redis-cli config get save
kubectl -n <ns> exec -it <master-pod> -- redis-cli config get appendonly
```

### 第 2 步 — 安装 valkey-operator 并创建 Valkey CR

```bash
# Helm 安装
helm repo add keiailab https://keiailab.github.io/valkey-operator
helm install valkey-operator keiailab/valkey-operator -n valkey-operator-system --create-namespace

# 或通过 manifest:
kubectl apply -f https://github.com/keiailab/valkey-operator/releases/latest/download/install.yaml
```

Valkey CR (Replication 模式):

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: my-cache
  namespace: data
spec:
  mode: Replication
  replicas: 3                       # 1 primary + 2 replica (与 Sentinel quorum 等价)
  version: 9.0.4
  storage:
    size: 8Gi
    storageClassName: <fast-ssd>
  auth:
    enabled: true                   # ADR-0013 — v1alpha1 强制启用,v1alpha2 中为 *required 开关
  monitoring:
    enabled: true
    serviceMonitor:
      enabled: true
  scalePolicy:
    deliberate: false               # 自动 failover 启用 (ADR-0006)
  additionalConfig: |
    # Sentinel min-slaves-to-write 等价配置 — 写操作至少要 1 个 replica ack
    min-replicas-to-write 1
    min-replicas-max-lag 10
```

### 第 3 步 — 数据迁移

#### 方案 A: RDB 导入 (可接受停机)

```bash
# 1. 从 Sentinel 侧 master 导出 RDB
kubectl -n <ns> exec -it <sentinel-master-pod> -- redis-cli BGSAVE
kubectl -n <ns> cp <sentinel-master-pod>:/data/dump.rdb /tmp/migration.rdb

# 2. 通过 ValkeyRestore 还原 (ADR-0015 init-container 模式)
kubectl -n data create configmap migration-rdb --from-file=dump.rdb=/tmp/migration.rdb
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata:
  name: migrate-from-sentinel
  namespace: data
spec:
  sourceBackup: ...
  targetRef:
    name: my-cache
    kind: Valkey
EOF
```

> ⚠️ 实测 Redis 8.2.x → Valkey 9.0.4 之间的 RDB *不兼容*
> (RDB format version 12)。**建议改用方案 B。**

#### 方案 B: 在线 key 复制 (停机最小化)

```bash
# 使用 valkey-cli MIGRATE,或者 redis-shake / redis-port 等
# 用户侧工具。
# 示例: redis-shake (支持在线 sync,可双写)

# 1. valkey-shake config (source = 现有 Sentinel master,target = valkey-operator primary)
cat > shake.toml <<EOF
[source]
type = "standalone"
address = "<sentinel-master-svc>:6379"
password = "<old-password>"

[target]
type = "standalone"
address = "my-cache.data.svc.cluster.local:6379"
password = "<new-password>"

type = "sync"
EOF

# 2. 启动同步 (长时间运行)
redis-shake -c shake.toml
```

### 第 4 步 — 客户端切换 + Sentinel 下线

#### 客户端改造 (示例: go-redis)

**Sentinel-aware (改造前)**:

```go
client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{"sentinel-0:26379", "sentinel-1:26379", "sentinel-2:26379"},
    Password:      "<old-password>",
})
```

**Service-aware (改造后)**:

```go
client := redis.NewClient(&redis.Options{
    Addr:     "my-cache.data.svc.cluster.local:6379",  // operator 管理的 Service
    Password: "<new-password>",
})
```

发生 failover 时,valkey-operator 会自动将 Service endpoint 切到新的 primary
(由 Service selector + readiness probe 联动)。客户端只需要 **在出错时
重连** — 多数客户端库会透明地处理这一点。

#### Decommission

```bash
# 1. 把 100 % 客户端流量切到 valkey-operator,然后验证
kubectl -n data port-forward svc/my-cache 6379:6379
redis-cli ping  # PONG

# 2. 删除原有的 Sentinel master/replica/sentinel 实例
helm uninstall <existing-release> -n <ns>

# 3. 清理 PVC (确认已干净关闭后)
kubectl -n <ns> delete pvc -l app.kubernetes.io/instance=<existing-release>
```

## 运维验证清单

- [ ] Valkey CR `status.phase=Running` 且
      `status.readyReplicas == replicas`。
- [ ] `valkey-cli INFO replication` 显示 `role=master`、
      `connected_slaves=N-1`。
- [ ] failover 演练: 删除 primary pod → 30 s 内选出新的 primary,Service
      endpoint 跟着更新。
- [ ] 数据完整性: 在 10 K key 抽样上,迁移前后 GET 结果一致。
- [ ] 客户端重连: primary 切换后,客户端在 5 s 内从错误状态恢复。

## 参考

- ADR-0017 — Replication Mode Failover (选取 `master_repl_offset` 最大的
  replica);Sentinel 拒绝的依据。
- ADR-0006 — `ScalePolicy.Deliberate=false` 默认值 (自动 failover 启用)。
- ADR-0015 — `ValkeyRestore` 基于 Init Container 的 RDB 加载。
