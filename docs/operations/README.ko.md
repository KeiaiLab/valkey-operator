# 운영 문서 색인 (한국어)

> English: [README.md](README.md) — canonical / 정본

> 한국어 사용자: 모든 문서가 `<name>.ko.md` 한국어 사본을 함께 보관합니다.

이 디렉토리는 `valkey-operator` 의 운영 절차를 모아 둡니다.
**모든 문서는 영문이 정본** 이며, 각 문서에 대해 `<name>.ko.md` 한국어 형제 파일을 둡니다 (`artifacthub-trust.md` 만 처음부터 영문으로 작성되어 한국어 사본이 없음).

## 일상 운영

| 문서 | 용도 |
|---|---|
| [runbook.md](runbook.md) | 장애 대응 + 일상 운영. 모든 Prometheus alert 의 `runbook_url` annotation 의 SSOT. |
| [troubleshooting.md](troubleshooting.md) | 알람이 울리지 않거나 (또는 알람이 생기기 전) 의 문제에 대한 증상 → 원인 → 진단 → 조치 흐름도. |
| [metrics-glossary.md](metrics-glossary.md) | 모든 `valkey_cluster_*` 메트릭 — 라벨 cardinality, 발생 위치, 어떤 alert 가 소비하는지. |
| [capacity-planning.md](capacity-planning.md) | Sizing 가이드 — 토폴로지별 메모리, 복제 계수, 샤드 수, p95/p99 지연 목표. |
| [webhook.md](webhook.md) | Admission webhook 아키텍처, cert-manager 인증서 경로, webhook 거부 디버깅. |
| [pdb-per-shard.md](pdb-per-shard.md) | 샤드형 `ValkeyCluster` 의 샤드별 `PodDisruptionBudget` 운영 — drain 시나리오, 모드 전환, 쓰기 가용성 보장. |

## 백업, 복원, 재해 복구

| 문서 | 용도 |
|---|---|
| [pitr-guide.md](pitr-guide.md) | Point-in-time recovery: phase-1 API + webhook, phase-2 reconciler dispatch, 수동 우회, 롤백. |
| [chaos-testing.md](chaos-testing.md) | chaos-mesh 4 시나리오 e2e 절차: pod-kill, network partition, IO ENOSPC, IO latency. |

## 릴리즈 & 공급망

| 문서 | 용도 |
|---|---|
| [release-checklist.md](release-checklist.md) | 사전 릴리즈 게이트 목록: build, 47 SSOT gate, 공급망 (SLSA + cosign), docs, operations. |
| [post-merge-cleanup.md](post-merge-cleanup.md) | Squash-merge 이후 로컬 브랜치 정리. |
| [artifacthub-trust.md](artifacthub-trust.md) | Artifact Hub `Signed` / `Official` 신뢰 배지 운영 절차. |

## 마이그레이션

| 문서 | 용도 |
|---|---|
| [sentinel-migration.md](sentinel-migration.md) | Sentinel → valkey-operator Replication 모드 마이그레이션 runbook (ADR-0017 backstop). |

## 횡단 참조

- 아키텍처 결정: [`docs/kb/adr/INDEX.md`](../kb/adr/INDEX.md)
- 인시던트 이력: [`docs/kb/incident/`](../kb/incident/)
- 릴리즈 검증 커맨드: [`SECURITY.md → "Verifying release artifacts"`](../../.github/SECURITY.md#verifying-release-artifacts-signed-releases--v1013)
- 로드맵 (프로젝트 방향): [`ROADMAP.md`](../ROADMAP.md)

## i18n 상태

`docs/operations/` 의 모든 운영 문서는 현재 **영문 정본** 상태입니다. i18n 작업은 PR #93 / #97 / #98 / #103 / #104 / #105 / #106 / #107 / #108 / #109 / #110 / #111 에 걸쳐 진행되었습니다. 한국어 원본은 `<name>.ko.md` 형제 파일로 그대로 보존됩니다 (`artifacthub-trust.md` 는 처음부터 영문으로 작성).
