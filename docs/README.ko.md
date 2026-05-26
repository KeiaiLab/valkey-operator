# 문서 — valkey-operator (한국어)

> English: [README.md](README.md) — canonical / 정본

> 모든 문서의 정본은 **영어**다. 한국어 자매 문서가 있는 경우 `<name>.ko.md`
> 형태로 보관한다 (`.ja.md`, `.zh.md` 도 동일 규칙). 본 페이지는 `docs/`
> 하위의 모든 문서로 향하는 **단일 진입점**이다.

## 운영 가이드

| 문서 | 용도 |
|---|---|
| [Operations index](operations/README.md) | 일상 runbook + 트러블슈팅 + metric + webhook + PDB + sizing |
| [Runbook](operations/runbook.md) | 장애 대응과 일상 운영 — 모든 Prometheus alert 의 `runbook_url` SSOT |
| [Troubleshooting](operations/troubleshooting.md) | 증상 → 원인 → 진단 → 조치 |
| [Metrics glossary](operations/metrics-glossary.md) | 모든 `valkey_cluster_*` metric, cardinality, alert 소비자 |
| [Capacity planning](operations/capacity-planning.md) | sizing — 메모리, replication factor, shard 수, p95/p99 latency 목표 |
| [Webhook](operations/webhook.md) | admission webhook 아키텍처, cert-manager 경로, denial 디버깅 |
| [PDB per shard](operations/pdb-per-shard.md) | shard 단위 `PodDisruptionBudget` 운영, drain 시나리오 |
| [PITR guide](operations/pitr-guide.md) | Point-in-time recovery — API + webhook + 수동 우회 |
| [Chaos testing](operations/chaos-testing.md) | chaos-mesh 4 시나리오 e2e 절차 |

## 릴리즈 & 공급망

| 문서 | 용도 |
|---|---|
| [Release checklist](operations/release-checklist.md) | release 직전 게이트 목록: build, SSOT gate, supply chain, 문서 |
| [Post-merge cleanup](operations/post-merge-cleanup.md) | squash-merge 이후 로컬 branch 정리 |
| [Artifact Hub trust](operations/artifacthub-trust.md) | Artifact Hub 의 `Signed` / `Official` badge 운영 절차 |
| [Upgrading](UPGRADING.md) | minor / major 업그레이드 마이그레이션, 정적 manifest 사용자용 RBAC patch |

## 마이그레이션

| 문서 | 용도 |
|---|---|
| [Migration index](migration/README.md) | StatefulSet → ValkeyCluster CR 마이그레이션 runbook 카탈로그 |
| [Sentinel migration](operations/sentinel-migration.md) | 기존 Sentinel HA → Replication 모드 + AutoFailover (ADR-0017 backstop) |

## 아키텍처 & 의사 결정 기록

| 문서 | 용도 |
|---|---|
| [ADR index](kb/adr/INDEX.md) | 52건 이상의 Architecture Decision Record 전체 |
| [Observability — OpenTelemetry](observability/otel.md) | OTel trace propagation, OTLP exporter, controller-reconcile span |
| [Valkey 9.x feature flag](version/9x-flags.md) | 신규 cluster-mode flag 와 operator 통합 경로 |

## 지식 베이스

| 문서 | 용도 |
|---|---|
| [Incident KB](kb/incident/INDEX.md) | postmortem-lite 기록 (blameless) |
| [Dependency change log](kb/deps/2026-05.md) | go.mod / go.sum diff 이력 |

## 커뮤니티 건전성

GitHub 가 자동 인식하는 커뮤니티 건전성 파일은 ADR-0053 (root `.md` 정책 +
도구 의존성 예외) 에 따라 `.github/` 에 위치한다:

| 문서 | 경로 |
|---|---|
| Contributing 가이드 | [.github/CONTRIBUTING.md](../.github/CONTRIBUTING.md) |
| Code of Conduct | [.github/CODE_OF_CONDUCT.md](../.github/CODE_OF_CONDUCT.md) |
| 보안 정책 + artifact 검증 | [.github/SECURITY.md](../.github/SECURITY.md) |
| 지원 채널 | [.github/SUPPORT.md](../.github/SUPPORT.md) |
| 프로젝트 거버넌스 | [.github/GOVERNANCE.md](../.github/GOVERNANCE.md) |

## i18n 상태

한국어 자매 문서는 각 영어 정본 옆에 `<name>.ko.md` 로 둔다. 日本語
(`.ja.md`) 와 中文 (`.zh.md`) 의 커버리지는 **최상위 파일에 집중**되어
있다 (README, ROADMAP, CONTRIBUTING). 운영 runbook 은 채택 수요에
따라 점진적으로 현지화한다.
