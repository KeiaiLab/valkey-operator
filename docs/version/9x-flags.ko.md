# Valkey 9.x Feature Flag 후속 작업 (한국어)

> English: [9x-flags.md](9x-flags.md) — canonical / 정본

> Valkey 9.0+ 의 신규 cluster-mode flag + cluster-mode 기능 도입 가이드.

## 배경 (Background)

Valkey 9.0 (2026-04 release) 은 RDB v80 + cluster-mode 개선 + 다음 flag 를 도입:

| Flag | Valkey 9.x 의미 | Default | Operator 적용 |
|---|---|---|---|
| `cluster-allow-replica-migration` | replica 자동 migration | `yes` | StatefulSet env 또는 ConfigMap |
| `cluster-allow-pubsubshard-when-down` | partial cluster 시 PubSub | `yes` | ConfigMap |
| `cluster-link-sendbuf-limit` | inter-node link buffer 한도 (bytes) | 0 (unlimited) | ConfigMap |
| `cluster-port` | cluster bus port (gossip) | 16379 | Service + headless port 추가 |
| `enable-debug-command` | debug command 허용 | `no` (prod 권장) | ConfigMap (NEVER enable in prod) |

## Operator 도입 경로

### 1단계: CRD ValkeyClusterSpec 신규 field

`api/v1alpha2/valkeycluster_types.go`:

```go
type ClusterSpec struct {
    // ValkeyConfig — additional valkey.conf overrides.
    ValkeyConfig map[string]string `json:"valkeyConfig,omitempty"`
    // ...
}
```

(이미 v1alpha2 에 존재할 가능성 — verify 후 신규 spec 분기.)

### 2단계: ConfigMap rendering

`internal/controller/cluster/configmap.go` 의 `renderValkeyConf` 가
`spec.valkeyConfig` 의 모든 키-값을 `<key> <value>` 로 emit.

### 3단계: Validation webhook

`pkg/webhook` (commons) 의 ValidateWithPredicate 로 known-flag 화이트리스트 검사:

```go
allowedFlags := map[string]bool{
    "cluster-allow-replica-migration":      true,
    "cluster-allow-pubsubshard-when-down":  true,
    "cluster-link-sendbuf-limit":           true,
    // ...
}
```

### 4단계: e2e

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

- Flag rollout: rolling restart 1 shard at a time
- Validation: webhook rejects unknown flags

## 참조 (Refs)

- ROADMAP.md L172 (P-C.8.1 Valkey 9.x feature follow-up)
- Valkey 9.0 release notes: https://github.com/valkey-io/valkey/releases/tag/9.0.0
