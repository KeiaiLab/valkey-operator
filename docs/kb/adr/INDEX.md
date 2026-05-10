# ADR Index — valkey-operator

본 디렉터리는 valkey-operator 의 비역행 결정(architecture decisions)을 Nygard
5섹션 형식으로 보존한다. 결정의 *이유*가 코드보다 오래 살아남도록 한다.

경로 표준: `<repo>/docs/kb/adr/` (글로벌 `standards/adr.md §1`).

## 활성 ADR (ID 오름차순)

| ID | 제목 | 상태 | 날짜 |
|----|------|------|------|
| [0001](0001-operator-side-defaulting.md) | Operator-side defaulting (vs admission webhook) | Superseded by 0009 | 2026-05-05 |
| [0002](0002-deferred-events-api-migration.md) | Deferred migration to client-go events API | Accepted | 2026-05-05 |
| [0003](0003-tls-insecure-skip-verify-temporary.md) | Temporary InsecureSkipVerify until cert-manager CA wiring | Accepted | 2026-05-05 |
| [0004](0004-shardstatus-spec-derived.md) | ShardStatus derived from Spec (not CLUSTER NODES) | Superseded by 0007 | 2026-05-05 |
| [0005](0005-graceful-cluster-teardown.md) | Graceful cluster teardown via best-effort CLUSTER FORGET | Accepted | 2026-05-05 |
| [0006](0006-scale-policy-deliberate.md) | ScalePolicy.Deliberate=false 기본값 | Accepted | 2026-05-05 |
| [0007](0007-shardstatus-from-nodes.md) | ShardStatus from CLUSTER NODES (supersedes 0004) | Accepted | 2026-05-05 |
| [0008](0008-tls-ca-bundle-loading.md) | TLS RootCAs from Spec.TLS.CustomCert.SecretName | Accepted | 2026-05-05 |
| [0009](0009-webhook-validation-defaulting.md) | Validating + Mutating Webhook (supersedes 0001) | Accepted | 2026-05-05 |
| [0010](0010-cert-manager-auto-discovery.md) | cert-manager Certificate auto-discovery | Accepted | 2026-05-05 |
| [0011](0011-required-fields-webhook-defaulting.md) | Required 필드는 mutating webhook 에서 직접 default 채움 | Accepted | 2026-05-05 |
| [0012](0012-cluster-meet-requires-ip.md) | CLUSTER MEET 는 hostname 미지원 → DNS 해석 후 IP 사용 | Accepted | 2026-05-05 |
| [0013](0013-auth-always-enabled.md) | Auth.Enabled 는 사실상 항상 enabled (옵션 A) | Accepted | 2026-05-05 |
| [0014](0014-tls-volume-mount-and-port-routing.md) | TLS Secret STS 마운트 + operator 가 6380 (TLS port) 로 control-plane | Accepted | 2026-05-05 |
| [0015](0015-valkeyrestore-init-container-pattern.md) | ValkeyRestore — Init Container 기반 RDB 로드 + STS 재시작 | Accepted | 2026-05-06 |
| [0016](0016-valkeybackuptarget-crd-external-storage.md) | ValkeyBackupTarget CRD — S3-compatible 외부 저장 추상화 | Accepted | 2026-05-06 |
| [0017](0017-replication-failover-replica-with-largest-offset.md) | Replication Mode Failover — Replica with Largest master_repl_offset | Accepted | 2026-05-06 |
| [0018](0018-cluster-auto-resharding.md) | Cluster Auto-Resharding (SlotMigrationPolicy Auto 활성, PR-B8.1 ADR 정식 작성 — controller 구현 PR-B8.2 후속) | Accepted | 2026-05-09 |
| 0019 | *Reserved (사용 미정)*. | Reserved | — |
| 0020 | *Reserved (사용 미정)*. | Reserved | — |
| [0021](0021-helm-chart-kubebuilder-helm-plugin.md) | Helm Chart — kubebuilder helm/v2-alpha plugin 채택 | Superseded by 0024 | 2026-05-06 |
| [0022](0022-s3-client-library-minio-go.md) | S3 Client Library — minio-go v7 채택 (sonatype + context7 검증) | Accepted | 2026-05-06 |
| [0023](0023-operator-binary-subcommand-upload-download.md) | Operator binary 의 upload/download sub-command — 이미지 통합 | Accepted | 2026-05-06 |
| [0024](0024-helm-chart-manual-pattern-artifacthub.md) | Helm Chart — 수기 작성 + ArtifactHub publish 패턴 (3-repo 통일, supersedes 0021) | Accepted | 2026-05-06 |
| [0025](0025-otel-tracer-provider-optional.md) | OTEL Tracer Provider — Optional, OTLP gRPC Exporter | Accepted | 2026-05-06 |
| [0026](0026-conversion-webhook-deferred-until-v1alpha1-stable.md) | Conversion Webhook — v1alpha1 Stable 도달 후 v1beta1 도입 (deferred) | Accepted | 2026-05-06 |
| [0027](0027-hpa-replication-mode-only-deferred.md) | HPA — Replication Mode 만 + Operator-managed (deferred) | Accepted | 2026-05-06 |
| [0028](0028-helm-kustomize-parity-invariant.md) | Helm vs Kustomize Parity Invariant — 5 sibling silent failure family 차단 | Accepted | 2026-05-06 |
| [0029](0029-gitops-deploy-overlay.md) | GitOps deploy 오버레이 도입 (3-repo 정합) | Accepted | 2026-05-06 |
| [0030](0030-rfc-0017-tooling-unification-adoption.md) | RFC-0017 operator tooling unification 채택 (.golangci.yml 신규 + Makefile validate + HEALTHCHECK) | Proposed | 2026-05-09 |
| [0031](0031-auth-rotation-policy.md) | Password Rotation reflect path (AuthSpec.RotationPolicy enum, v1alpha2 PR-B7.1 type module — controller 분기 PR-B7.2 후속) | Accepted | 2026-05-09 |
| [0032](0032-custom-modules-init-container.md) | Valkey Custom Modules — init container mount + 공식 preset only (v1alpha2 PR-C6.1 type module — controller 분기 PR-C6.2 후속) | Accepted | 2026-05-09 |
| [0033](0033-supply-chain-cosign-slsa.md) | Supply Chain — cosign sign + SLSA L2 in-toto attestation (Plan §2 D5, PR-A4) | Accepted | 2026-05-09 |
| [0034](0034-auth-optional-v1alpha2.md) | Auth Optional + v1alpha2 신규 (supersedes ADR-0013, PR-A2.1 type module) | Accepted | 2026-05-09 |
| [0035](0035-networkpolicy-autocreate-optional.md) | NetworkPolicy.AutoCreate Optional Toggle (v1alpha2, PR-A3.1 type module — controller 분기 PR-A3.1.2 후속) | Accepted | 2026-05-09 |
| [0036](0036-pod-security-restricted-optional.md) | PodSecurity Restricted Optional Toggle (v1alpha2 PodSpec.PodSecurityRestricted, PR-A3.2 type module — controller 분기 PR-A3.2.2 후속) | Accepted | 2026-05-09 |
| [0037](0037-operatorhub-bundle-scaffold.md) | OperatorHub.io bundle scaffold — operator-sdk v1.42 + kustomize, 5 CRD owned, Makefile bundle/bundle-build 타겟 (PR-B9 first cut, alm-examples + community-operators PR 후속) | Accepted | 2026-05-10 |
| [0038](0038-rfc-0018-pkg-finalizer-migration.md) | RFC-0018 채택 — pkg/finalizer migration (controllerutil → commons, 5 controller, PR-A6 first cut, status 별도) | Accepted | 2026-05-09 |
| [0039](0039-cluster-self-heal-post-init.md) | ValkeyCluster post-init self-heal — INC-0001 영구 fix, ClusterInitialized=true && state!=ok 시 ensureClusterMeet 재호출 | Accepted | 2026-05-10 |
| [0040](0040-helm-chart-vs-operator-adoption.md) | Helm chart vs Operator 채택 정책 (Bitnami / CloudPirates / valkey-operator 의사결정 매트릭스 + 5 gap) | Accepted | 2026-05-10 |

## 작성 가이드

- 형식: Nygard 5섹션 (Context / Decision / Consequences / Alternatives Considered / Status).
- 위치: `docs/kb/adr/NNNN-<영어 kebab-case slug>.md` (글로벌 표준).
- 번호 부여: 4자리 0-padded, 한 번 부여한 번호는 *재사용 금지*. Reserved 슬롯은 INDEX 에 명시.
- 본 INDEX.md 는 신규 ADR 추가 시 *수동 갱신 의무* — `standards/enforcement.md §2.1`.
- 정렬: ID 오름차순 — Reserved 항목 포함 (gap 가시화).

## Reserved 슬롯 정책

ADR 번호 0018-0020 은 plans 단계에서 예약되었으나 *작성 전* 상태로 보존. 재사용 금지 원칙에 따라 새 ADR 은 다음 가용 번호 (0030+) 부터 부여한다. Reserved 슬롯이 작성되면 INDEX 행을 정식 항목으로 교체한다.

## 글로벌 참조

- 글로벌 ADR 표준: `~/Documents/ai-dev/standards/adr.md`
- ADR 커버리지 게이트: `scripts/check-adr-coverage.sh` (글로벌)
- 강제 표준: `~/Documents/ai-dev/standards/enforcement.md §2.1`
