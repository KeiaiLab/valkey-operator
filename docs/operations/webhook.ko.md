# Admission Webhook — valkey-operator (한국어)

> English: [webhook.md](webhook.md) — canonical / 정본

Validating + mutating admission webhook. *opt-in default* (helm 의
`webhook.enabled=false` 가 기본). 활성화 시 cert-manager 클러스터 사전
설치 필수.

> cross-cut audit (ADR-0016) 으로 invariant + UX 일관.

## Quick Start

### Prerequisites

```bash
kubectl get crd certificates.cert-manager.io
```

미설치 시 [cert-manager docs](https://cert-manager.io/docs/installation/) 참조.

### 활성화

```bash
helm upgrade --reuse-values valkey-operator keiailab/valkey-operator \
  --set webhook.enabled=true
```

자동 생성 리소스: `Issuer`, `Certificate`, `Service`,
`MutatingWebhookConfiguration`, `ValidatingWebhookConfiguration`.

검증:
```bash
kubectl get validatingwebhookconfiguration <release>-valkey-operator-validating
kubectl get mutatingwebhookconfiguration <release>-valkey-operator-mutating
```

## Validation Invariants

### Valkey CR (single-instance / replication)

| Field | Rule |
|---|---|
| `spec.version.version` | 화이트리스트 (8.x / 9.0.x) |
| `spec.mode` + `spec.replicas` | Standalone↔1 / Replication↔>=2 |
| `spec.tls.{certManager,customCert}` | TLS Enabled 시 둘 중 *하나만* (mutual exclusive) |
| `spec.tls.certManager.issuerRef.name` | non-empty (omitempty trap) |
| `spec.tls.customCert.secretName` | non-empty (omitempty trap) |
| `spec.storage.size` | >= 1Gi (RDB+AOF floor) |
| `spec.auth.users[].name` | non-empty |
| `spec.auth.users[].passwordSecretRef.name` | non-empty (no auto-gen for additional users) |
| `spec.auth.users[].passwordSecretRef.key` | non-empty |
| `spec.auth.users` 사용 시 | `spec.auth.enabled=true` 필수 |

### ValkeyCluster CR (sharded cluster)

위 invariants 외 추가:

| Field | Rule |
|---|---|
| `spec.shards * (1 + replicasPerShard)` | <= 100 (operational total nodes) |
| `spec.autoFailover` + `spec.replicasPerShard` | autoFailover=true 시 replicasPerShard >= 1 (조건부 — ADR-0017 Type A') |
| `spec.{storage.{storageClassName,size,dataDirPath},tls.enabled}` | immutable (변경 시 데이터 손실 / cluster 깨짐) |

### Defaulting (mutating)

CRD marker 가 표현 못 하는 *조건부 default*:
- `spec.shards` 0 → 3 (Cluster).
- `spec.replicasPerShard` 0 → 1 (Cluster, ADR-0017 Type A' 의 *omitempty 부재
  보강*).
- `spec.version.version` 빈 → `DefaultValkeyVersion`.
- `spec.slotMigration` 빈 → `Auto`.

## Admission Denial 메시지

K8s `apierrors.NewInvalid` accumulate-errors 형식 — 복수 invariant 위반 시
*모두* 한 번에 보고:

```
Error from server (Invalid): admission webhook "vvalkeycluster-v1alpha1.kb.io"
denied the request: ValkeyCluster.cache.keiailab.io "my-valkey" is invalid:
[spec.tls: TLS.CertManager and TLS.CustomCert are mutually exclusive — choose one,
spec.storage.size: storage.size must be >= 1Gi — RDB snapshot + AOF data dir floor]
```

## failurePolicy=Fail 영향

HA 권장: production 환경 `replicaCount: 2` + PDB.

## Troubleshooting

### `kubectl apply` 가 webhook 도달 못 함

```
Error from server (InternalError): failed calling webhook "..."
```

원인:
1. webhook pod down — `kubectl get pods -l app.kubernetes.io/name=valkey-operator`
2. CABundle 미주입 — `kubectl get validatingwebhookconfiguration ... -o
   jsonpath='{.webhooks[0].clientConfig.caBundle}'`. 비어있으면 cert-manager
   미설치 또는 ca-injector 비활성.

### autoFailover invariant 가 admission 도달 안 함

`webhook.enabled=true` 환경에서 mutating defaulter 가 `replicasPerShard=0→1`
보강 → invariant 도달 0. 이는 *의도된 design* (ADR-0017 Type A' 조건부
unreachable). `webhook.enabled=false` 환경에서는 도달 가능.

## 비활성화

```bash
helm upgrade --reuse-values valkey-operator keiailab/valkey-operator \
  --set webhook.enabled=false
```

cert-manager 리소스 / Webhook Configuration 자동 제거. 기존 valkey CR 영향 0.

## 관련 문서
