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
| [0013](0013-auth-always-enabled.md) | Auth.Enabled 필드 가 무시되고 사실상 항상 enabled (옵션 A) | Accepted | 2026-05-05 |
| [0014](0014-tls-volume-mount-and-port-routing.md) | TLS Secret STS 마운트 + operator 가 6380 (TLS port) 로 control-plane | Accepted | 2026-05-05 |
| [0015](0015-valkeyrestore-init-container-pattern.md) | ValkeyRestore — Init Container 기반 RDB 로드 + STS 재시작 | Accepted | 2026-05-06 |
| [0016](0016-valkeybackuptarget-crd-external-storage.md) | ValkeyBackupTarget CRD — S3-compatible 외부 저장 추상화 | Accepted | 2026-05-06 |
| [0022](0022-s3-client-library-minio-go.md) | S3 Client Library — minio-go v7 채택 (sonatype + context7 검증) | Accepted | 2026-05-06 |
| [0017](0017-replication-failover-replica-with-largest-offset.md) | Replication Mode Failover — Replica with Largest master_repl_offset | Accepted | 2026-05-06 |
| [0025](0025-otel-tracer-provider-optional.md) | OTEL Tracer Provider — Optional, OTLP gRPC Exporter | Accepted | 2026-05-06 |
| [0026](0026-conversion-webhook-deferred-until-v1alpha1-stable.md) | Conversion Webhook — v1alpha1 Stable 도달 후 v1beta1 도입 (deferred) | Accepted | 2026-05-06 |
| [0027](0027-hpa-replication-mode-only-deferred.md) | HPA — Replication Mode 만 + Operator-managed (deferred) | Accepted | 2026-05-06 |
| [0021](0021-helm-chart-kubebuilder-helm-plugin.md) | Helm Chart — kubebuilder helm/v2-alpha plugin 채택 | Superseded by 0024 | 2026-05-06 |
| [0023](0023-operator-binary-subcommand-upload-download.md) | Operator binary 의 upload/download sub-command — 이미지 통합 | Accepted | 2026-05-06 |
| [0024](0024-helm-chart-manual-pattern-artifacthub.md) | Helm Chart — 수기 작성 + ArtifactHub publish 패턴 (3-repo 통일, supersedes 0021) | Accepted | 2026-05-06 |
