# Admission Webhook — valkey-operator (日本語)

> English: [webhook.md](webhook.md) — canonical / 正本

Validating + mutating admission webhook。**既定では opt-in** (Helm chart の `webhook.enabled=false`)。有効化する前に、クラスタへの cert-manager 導入が必須である。

> [mongodb-operator の webhook ドキュメント](https://github.com/keiailab/mongodb-operator/blob/main/docs/advanced/webhook.md) と同一パターン — 3 operator 横断監査 (ADR-0016) によって invariant と UX が揃えられている。

## クイックスタート

### 前提条件

```bash
kubectl get crd certificates.cert-manager.io
```

未導入の場合は、まず [cert-manager](https://cert-manager.io/docs/installation/) をインストールすること。

### 有効化

```bash
helm upgrade --reuse-values valkey-operator keiailab/valkey-operator \
  --set webhook.enabled=true
```

自動生成されるリソース: `Issuer`、`Certificate`、`Service`、`MutatingWebhookConfiguration`、`ValidatingWebhookConfiguration`。

検証:

```bash
kubectl get validatingwebhookconfiguration <release>-valkey-operator-validating
kubectl get mutatingwebhookconfiguration <release>-valkey-operator-mutating
```

## バリデーション invariant

### `Valkey` CR (single-instance / replication)

| Field | Rule |
|---|---|
| `spec.version.version` | ホワイトリスト (8.x / 9.0.x) |
| `spec.mode` + `spec.replicas` | Standalone ↔ 1 / Replication ↔ ≥ 2 |
| `spec.tls.{certManager,customCert}` | TLS 有効時は片方のみ設定可 (排他) |
| `spec.tls.certManager.issuerRef.name` | 非空 (`omitempty` 落とし穴) |
| `spec.tls.customCert.secretName` | 非空 (`omitempty` 落とし穴) |
| `spec.storage.size` | ≥ 1 Gi (RDB + AOF の下限) |
| `spec.auth.users[].name` | 非空 |
| `spec.auth.users[].passwordSecretRef.name` | 非空 (追加ユーザーには自動生成なし) |
| `spec.auth.users[].passwordSecretRef.key` | 非空 |
| `spec.auth.users` 設定時 | `spec.auth.enabled=true` 必須 |

### `ValkeyCluster` CR (sharded cluster)

上記に加えて:

| Field | Rule |
|---|---|
| `spec.shards * (1 + replicasPerShard)` | ≤ 100 (運用上の総ノード上限) |
| `spec.autoFailover` + `spec.replicasPerShard` | `autoFailover=true` の場合 `replicasPerShard ≥ 1` (条件付き — ADR-0017 Type A') |
| `spec.{storage.{storageClassName,size,dataDirPath},tls.enabled}` | immutable (変更するとデータ破損やクラスタ破壊につながる) |

### Defaulting (mutating)

CRD マーカーでは表現できない条件付きデフォルト:

- `spec.shards` 0 → 3 (Cluster)。
- `spec.replicasPerShard` 0 → 1 (Cluster、ADR-0017 Type A' — 欠落している `omitempty` の補強)。
- `spec.version.version` 空 → `DefaultValkeyVersion`。
- `spec.slotMigration` 空 → `Auto`。

## Admission 拒否メッセージ

K8s `apierrors.NewInvalid` の accumulate-errors 形式で構築されており、複数 invariant 違反は **まとめて** 1 つのレスポンスに乗せて返される:

```
Error from server (Invalid): admission webhook "vvalkeycluster-v1alpha1.kb.io"
denied the request: ValkeyCluster.cache.keiailab.io "my-valkey" is invalid:
[spec.tls: TLS.CertManager and TLS.CustomCert are mutually exclusive — choose one,
spec.storage.size: storage.size must be >= 1Gi — RDB snapshot + AOF data dir floor]
```

## `failurePolicy=Fail` の影響

webhook サーバ pod が落ちると、全ての `valkey` CR の CRUD がブロックされる。詳細は mongodb-operator [ADR-0015](https://github.com/keiailab/mongodb-operator/blob/main/docs/kb/adr/0015-webhook-failure-policy-fail.md) を参照 (3 operator 全てで同一ポリシー)。

HA 推奨: production では `replicaCount: 2` + PDB を必ず設定する。

## トラブルシューティング

### `kubectl apply` が webhook に到達しない

```
Error from server (InternalError): failed calling webhook "..."
```

根本原因:

1. Webhook pod down — `kubectl get pods -l app.kubernetes.io/name=valkey-operator`。
2. `CABundle` が未注入 — `kubectl get validatingwebhookconfiguration ... -o jsonpath='{.webhooks[0].clientConfig.caBundle}'`。空ならば cert-manager 未導入か、その `ca-injector` が無効。

### `autoFailover` invariant に admission が到達しない

`webhook.enabled=true` の環境では、mutating defaulter が invariant チェックの前に `replicasPerShard=0→1` を補完するため、違反は観測できない。これは **意図された設計** である (ADR-0017 Type A' の「条件付き到達不能」)。`webhook.enabled=false` の環境では再び到達可能となる。

## 無効化

```bash
helm upgrade --reuse-values valkey-operator keiailab/valkey-operator \
  --set webhook.enabled=false
```

cert-manager 関連リソースと Webhook Configuration が自動的に削除される。既存の `valkey` CR への影響はない。

## 関連

- mongodb-operator [ADR-0015](https://github.com/keiailab/mongodb-operator/blob/main/docs/kb/adr/0015-webhook-failure-policy-fail.md) — `failurePolicy=Fail`。
- mongodb-operator [ADR-0016](https://github.com/keiailab/mongodb-operator/blob/main/docs/kb/adr/0016-cross-cut-audit-pattern.md) — cross-cut audit pattern。
- mongodb-operator [ADR-0017](https://github.com/keiailab/mongodb-operator/blob/main/docs/kb/adr/0017-crd-default-vs-webhook-invariant.md) — CRD default vs webhook invariant (Type A' errata)。
