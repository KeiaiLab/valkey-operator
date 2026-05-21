# Admission Webhook — valkey-operator

> 한국어 버전: [webhook.ko.md](webhook.ko.md)

Validating + mutating admission webhook. **Opt-in by default**
(`webhook.enabled=false` in the Helm chart). Requires cert-manager
on the cluster before you flip it on.

> Same pattern as
> [mongodb-operator's webhook doc](https://github.com/keiailab/mongodb-operator/blob/main/docs/advanced/webhook.md)
> — 3-operator cross-cut audit (ADR-0016) keeps the invariants and
> UX consistent.

## Quick start

### Prerequisites

```bash
kubectl get crd certificates.cert-manager.io
```

If absent, install
[cert-manager](https://cert-manager.io/docs/installation/) first.

### Enable

```bash
helm upgrade --reuse-values valkey-operator keiailab/valkey-operator \
  --set webhook.enabled=true
```

Resources auto-created: `Issuer`, `Certificate`, `Service`,
`MutatingWebhookConfiguration`, `ValidatingWebhookConfiguration`.

Verify:

```bash
kubectl get validatingwebhookconfiguration <release>-valkey-operator-validating
kubectl get mutatingwebhookconfiguration <release>-valkey-operator-mutating
```

## Validation invariants

### `Valkey` CR (single-instance / replication)

| Field | Rule |
|---|---|
| `spec.version.version` | Whitelist (8.x / 9.0.x) |
| `spec.mode` + `spec.replicas` | Standalone ↔ 1 / Replication ↔ ≥ 2 |
| `spec.tls.{certManager,customCert}` | When TLS is enabled, exactly one is set (mutually exclusive) |
| `spec.tls.certManager.issuerRef.name` | Non-empty (`omitempty` trap) |
| `spec.tls.customCert.secretName` | Non-empty (`omitempty` trap) |
| `spec.storage.size` | ≥ 1 Gi (RDB + AOF floor) |
| `spec.auth.users[].name` | Non-empty |
| `spec.auth.users[].passwordSecretRef.name` | Non-empty (no auto-gen for extra users) |
| `spec.auth.users[].passwordSecretRef.key` | Non-empty |
| `spec.auth.users` set | `spec.auth.enabled=true` required |

### `ValkeyCluster` CR (sharded cluster)

In addition to the above:

| Field | Rule |
|---|---|
| `spec.shards * (1 + replicasPerShard)` | ≤ 100 (operational total node cap) |
| `spec.autoFailover` + `spec.replicasPerShard` | When `autoFailover=true`, `replicasPerShard ≥ 1` (conditional — ADR-0017 Type A') |
| `spec.{storage.{storageClassName,size,dataDirPath},tls.enabled}` | Immutable (changes would corrupt data or break the cluster) |

### Defaulting (mutating)

Conditional defaults that CRD markers cannot express:

- `spec.shards` 0 → 3 (Cluster).
- `spec.replicasPerShard` 0 → 1 (Cluster, ADR-0017 Type A' —
  reinforcing the missing `omitempty`).
- `spec.version.version` empty → `DefaultValkeyVersion`.
- `spec.slotMigration` empty → `Auto`.

## Admission denial message

Built with K8s `apierrors.NewInvalid` accumulate-errors form —
multiple invariant violations are surfaced **together** in one
response:

```
Error from server (Invalid): admission webhook "vvalkeycluster-v1alpha1.kb.io"
denied the request: ValkeyCluster.cache.keiailab.io "my-valkey" is invalid:
[spec.tls: TLS.CertManager and TLS.CustomCert are mutually exclusive — choose one,
spec.storage.size: storage.size must be >= 1Gi — RDB snapshot + AOF data dir floor]
```

## `failurePolicy=Fail` impact

When the webhook server pod is down, every `valkey` CR CRUD is
blocked. See mongodb-operator
[ADR-0015](https://github.com/keiailab/mongodb-operator/blob/main/docs/kb/adr/0015-webhook-failure-policy-fail.md)
(same policy in all 3 operators).

HA recommendation: production runs `replicaCount: 2` + PDB.

## Troubleshooting

### `kubectl apply` never reaches the webhook

```
Error from server (InternalError): failed calling webhook "..."
```

Root causes:

1. Webhook pod down —
   `kubectl get pods -l app.kubernetes.io/name=valkey-operator`.
2. `CABundle` not injected —
   `kubectl get validatingwebhookconfiguration ... -o jsonpath='{.webhooks[0].clientConfig.caBundle}'`.
   Empty means cert-manager is missing or its `ca-injector` is off.

### `autoFailover` invariant never reaches admission

With `webhook.enabled=true`, the mutating defaulter fills
`replicasPerShard=0→1` before the invariant is checked — the
violation can never be observed. This is **intentional design**
(ADR-0017 Type A' "conditional unreachable"). With
`webhook.enabled=false`, it becomes reachable again.

## Disable

```bash
helm upgrade --reuse-values valkey-operator keiailab/valkey-operator \
  --set webhook.enabled=false
```

Removes the cert-manager resources and the Webhook Configurations
automatically. No impact on existing `valkey` CRs.

## Related

- mongodb-operator
  [ADR-0015](https://github.com/keiailab/mongodb-operator/blob/main/docs/kb/adr/0015-webhook-failure-policy-fail.md)
  — `failurePolicy=Fail`.
- mongodb-operator
  [ADR-0016](https://github.com/keiailab/mongodb-operator/blob/main/docs/kb/adr/0016-cross-cut-audit-pattern.md)
  — cross-cut audit pattern.
- mongodb-operator
  [ADR-0017](https://github.com/keiailab/mongodb-operator/blob/main/docs/kb/adr/0017-crd-default-vs-webhook-invariant.md)
  — CRD default vs webhook invariant (Type A' errata).
