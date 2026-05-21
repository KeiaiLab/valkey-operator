# Valkey 9.x Feature Flag Follow-up

> English: [9x-flags.md](9x-flags.md) — canonical / 正本

> Valkey 9.0+ で導入された新規 cluster-mode flag および cluster-mode 機能の導入ガイド。

## Background

Valkey 9.0 (2026-04 release) は RDB v80 + cluster-mode の改善に加え、以下の flag を導入する:

| Flag | Valkey 9.x での意味 | Default | Operator への適用 |
|---|---|---|---|
| `cluster-allow-replica-migration` | replica の自動 migration | `yes` | StatefulSet env または ConfigMap |
| `cluster-allow-pubsubshard-when-down` | partial cluster 状態での PubSub | `yes` | ConfigMap |
| `cluster-link-sendbuf-limit` | inter-node link buffer の上限 (bytes) | 0 (unlimited) | ConfigMap |
| `cluster-port` | cluster bus port (gossip) | 16379 | Service + headless port を追加 |
| `enable-debug-command` | debug command の許可 | `no` (prod 推奨) | ConfigMap (NEVER enable in prod) |

## Operator への導入 path

### Step 1: CRD ValkeyClusterSpec の新規 field

`api/v1alpha2/valkeycluster_types.go`:

```go
type ClusterSpec struct {
    // ValkeyConfig — additional valkey.conf overrides.
    ValkeyConfig map[string]string `json:"valkeyConfig,omitempty"`
    // ...
}
```

(既に v1alpha2 へ存在している可能性あり — verify したうえで新規 spec を分岐させる。)

### Step 2: ConfigMap rendering

`internal/controller/cluster/configmap.go` の `renderValkeyConf` が、
`spec.valkeyConfig` の全 key-value を `<key> <value>` 形式で emit する。

### Step 3: Validation webhook

`pkg/webhook` (commons) の ValidateWithPredicate により、known-flag のホワイトリスト検査を行う:

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

- Flag rollout: 1 shard ずつの rolling restart
- Validation: webhook が unknown flag を reject する

## Refs

- ROADMAP.md L172 (P-C.8.1 Valkey 9.x feature follow-up)
- Valkey 9.0 release notes: https://github.com/valkey-io/valkey/releases/tag/9.0.0
