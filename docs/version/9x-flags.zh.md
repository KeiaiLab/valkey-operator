# Valkey 9.x 特性 Flag 后续跟进

> English: [9x-flags.md](9x-flags.md) — canonical / 正本

> Valkey 9.0+ 新增的 cluster-mode flag,以及 cluster-mode 功能落地指南。

## Background

Valkey 9.0 (2026-04 release) 引入了 RDB v80 + cluster-mode 改进,以及以下 flag:

| Flag | Valkey 9.x 含义 | Default | Operator 落地方式 |
|---|---|---|---|
| `cluster-allow-replica-migration` | replica 自动迁移 | `yes` | StatefulSet env 或 ConfigMap |
| `cluster-allow-pubsubshard-when-down` | 集群部分不可用时仍支持 PubSub | `yes` | ConfigMap |
| `cluster-link-sendbuf-limit` | inter-node link buffer 上限 (bytes) | 0 (unlimited) | ConfigMap |
| `cluster-port` | cluster bus port (gossip) | 16379 | Service + headless port 补齐 |
| `enable-debug-command` | 是否允许 debug command | `no` (生产推荐) | ConfigMap (NEVER enable in prod) |

## Operator 落地路径

### Step 1: CRD ValkeyClusterSpec 新增字段

`api/v1alpha2/valkeycluster_types.go`:

```go
type ClusterSpec struct {
    // ValkeyConfig — additional valkey.conf overrides.
    ValkeyConfig map[string]string `json:"valkeyConfig,omitempty"`
    // ...
}
```

(v1alpha2 中可能已经存在 — 先 verify,再分支新增 spec。)

### Step 2: ConfigMap rendering

`internal/controller/cluster/configmap.go` 中的 `renderValkeyConf` 把
`spec.valkeyConfig` 中所有键值对以 `<key> <value>` 的格式 emit。

### Step 3: Validation webhook

`pkg/webhook` (commons) 的 ValidateWithPredicate 用 known-flag 白名单做校验:

```go
allowedFlags := map[string]bool{
    "cluster-allow-replica-migration":      true,
    "cluster-allow-pubsubshard-when-down":  true,
    "cluster-link-sendbuf-limit":           true,
    // ...
}
```

### Step 4: e2e

`test/e2e/valkey_9x_flags_test.go`:

```go
func Test9xFlags(t *testing.T) {
    cluster := makeCluster()
    cluster.Spec.ValkeyConfig = map[string]string{
        "cluster-link-sendbuf-limit": "1048576",
    }
    require.NoError(t, k8sClient.Create(ctx, cluster))
    // verify ConfigMap contains the flag
    cm := &corev1.ConfigMap{}
    require.Eventually(t, func() bool {
        return k8sClient.Get(ctx, key, cm) == nil &&
            strings.Contains(cm.Data["valkey.conf"], "cluster-link-sendbuf-limit 1048576")
    }, 30*time.Second, 1*time.Second)
}
```

## SLO

- Flag 灰度: 一次滚动重启一个 shard
- 校验: webhook 拒绝未知 flag

## Refs

- ROADMAP.md L172 (P-C.8.1 Valkey 9.x feature follow-up)
- Valkey 9.0 release notes: https://github.com/valkey-io/valkey/releases/tag/9.0.0
