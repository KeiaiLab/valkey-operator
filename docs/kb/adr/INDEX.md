# ADR Index — valkey-operator

| ID | Title | Status | Date |
|----|-------|--------|------|
| [0001](0001-operator-side-defaulting.md) | Operator-side defaulting (vs admission webhook) | Superseded by 0009 | 2026-05-05 |
| [0002](0002-deferred-events-api-migration.md) | Deferred migration to client-go events API | Accepted | 2026-05-05 |
| [0003](0003-tls-insecure-skip-verify-temporary.md) | Temporary InsecureSkipVerify until cert-manager CA wiring | Accepted | 2026-05-05 |
| [0004](0004-shardstatus-spec-derived.md) | ShardStatus derived from Spec (not CLUSTER NODES) | Superseded by 0007 | 2026-05-05 |
| [0005](0005-graceful-cluster-teardown.md) | Graceful cluster teardown via best-effort CLUSTER FORGET | Accepted | 2026-05-05 |
| [0006](0006-scale-policy-deliberate.md) | ScalePolicy.Deliberate=false default | Accepted | 2026-05-05 |
| [0007](0007-shardstatus-from-nodes.md) | ShardStatus from CLUSTER NODES (supersedes 0004) | Accepted | 2026-05-05 |
| [0008](0008-tls-ca-bundle-loading.md) | TLS RootCAs from Spec.TLS.CustomCert.SecretName | Accepted | 2026-05-05 |
| [0009](0009-webhook-validation-defaulting.md) | Validating + Mutating Webhook (supersedes 0001) | Accepted | 2026-05-05 |
| [0010](0010-cert-manager-auto-discovery.md) | cert-manager Certificate auto-discovery | Accepted | 2026-05-05 |
| [0011](0011-required-fields-webhook-defaulting.md) | Required 필드는 mutating webhook 에서 직접 default 채움 | Accepted | 2026-05-05 |
| [0012](0012-cluster-meet-requires-ip.md) | CLUSTER MEET 는 hostname 미지원 → DNS 해석 후 IP 사용 | Accepted | 2026-05-05 |
