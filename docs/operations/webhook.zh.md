# Admission Webhook — valkey-operator (简体中文)

> English: [webhook.md](webhook.md) — canonical / 正本

Validating + mutating admission webhook。**默认 opt-in**
(Helm chart 中的 `webhook.enabled=false`)。启用前必须先在集群中安装
cert-manager。

> 与
> 同一套模式 —— 通过 cross-cut audit (ADR-0016) 保证不变式
> 与 UX 一致。

## 快速开始

### 前置条件

```bash
kubectl get crd certificates.cert-manager.io
```

如未安装,请先安装
[cert-manager](https://cert-manager.io/docs/installation/)。

### 启用

```bash
helm upgrade --reuse-values valkey-operator keiailab/valkey-operator \
  --set webhook.enabled=true
```

会自动创建以下资源: `Issuer`、`Certificate`、`Service`、
`MutatingWebhookConfiguration`、`ValidatingWebhookConfiguration`。

校验:

```bash
kubectl get validatingwebhookconfiguration <release>-valkey-operator-validating
kubectl get mutatingwebhookconfiguration <release>-valkey-operator-mutating
```

## 校验不变式

### `Valkey` CR (单实例 / 复制集)

| 字段 | 规则 |
|---|---|
| `spec.version.version` | 白名单 (8.x / 9.0.x) |
| `spec.mode` + `spec.replicas` | Standalone ↔ 1 / Replication ↔ ≥ 2 |
| `spec.tls.{certManager,customCert}` | 启用 TLS 时,有且仅有一个被设置 (互斥) |
| `spec.tls.certManager.issuerRef.name` | 非空 (`omitempty` 陷阱) |
| `spec.tls.customCert.secretName` | 非空 (`omitempty` 陷阱) |
| `spec.storage.size` | ≥ 1 Gi (RDB + AOF 下限) |
| `spec.auth.users[].name` | 非空 |
| `spec.auth.users[].passwordSecretRef.name` | 非空 (额外用户不会自动生成凭据) |
| `spec.auth.users[].passwordSecretRef.key` | 非空 |
| 设置了 `spec.auth.users` | 必须同时设置 `spec.auth.enabled=true` |

### `ValkeyCluster` CR (分片集群)

在上述规则之上追加:

| 字段 | 规则 |
|---|---|
| `spec.shards * (1 + replicasPerShard)` | ≤ 100 (运维侧总节点上限) |
| `spec.autoFailover` + `spec.replicasPerShard` | 当 `autoFailover=true` 时,要求 `replicasPerShard ≥ 1` (条件式 —— ADR-0017 Type A') |
| `spec.{storage.{storageClassName,size,dataDirPath},tls.enabled}` | 不可变 (变更会破坏数据或集群) |

### Defaulting (mutating)

CRD marker 无法表达的条件式默认值:

- `spec.shards` 0 → 3 (Cluster)。
- `spec.replicasPerShard` 0 → 1 (Cluster, ADR-0017 Type A' ——
  对缺失的 `omitempty` 进行加固)。
- `spec.version.version` 为空 → `DefaultValkeyVersion`。
- `spec.slotMigration` 为空 → `Auto`。

## Admission 拒绝消息

基于 K8s `apierrors.NewInvalid` 的 accumulate-errors 形式构造 ——
多条不变式同时违反时会在**一次响应中合并报告**:

```
Error from server (Invalid): admission webhook "vvalkeycluster-v1alpha1.kb.io"
denied the request: ValkeyCluster.cache.keiailab.io "my-valkey" is invalid:
[spec.tls: TLS.CertManager and TLS.CustomCert are mutually exclusive — choose one,
spec.storage.size: storage.size must be >= 1Gi — RDB snapshot + AOF data dir floor]
```

## `failurePolicy=Fail` 的影响

当 webhook server pod 宕机时,所有 `valkey` CR 的 CRUD 都会被阻塞。
(3 个 operator 采用同一策略)。

HA 建议: 生产环境运行 `replicaCount: 2` + PDB。

## 排障

### `kubectl apply` 始终无法到达 webhook

```
Error from server (InternalError): failed calling webhook "..."
```

根因:

1. webhook pod 宕机 ——
   `kubectl get pods -l app.kubernetes.io/name=valkey-operator`。
2. `CABundle` 未注入 ——
   `kubectl get validatingwebhookconfiguration ... -o jsonpath='{.webhooks[0].clientConfig.caBundle}'`。
   为空表示 cert-manager 未安装,或其 `ca-injector` 未启用。

### `autoFailover` 不变式始终无法触达 admission

当 `webhook.enabled=true` 时,mutating defaulter 会在不变式校验之前
将 `replicasPerShard=0→1` 补齐 —— 因此该违反场景永远观察不到。这是
**有意为之的设计** (ADR-0017 Type A' "条件式 unreachable")。在
`webhook.enabled=false` 模式下,该不变式重新变得可达。

## 关闭

```bash
helm upgrade --reuse-values valkey-operator keiailab/valkey-operator \
  --set webhook.enabled=false
```

cert-manager 相关资源与 Webhook Configurations 会被自动移除。对现有的
`valkey` CR 无影响。

## 相关文档

  —— `failurePolicy=Fail`。
  —— cross-cut audit 模式。
  —— CRD default vs webhook invariant (Type A' errata)。
