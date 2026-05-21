# TASKS — valkey-operator

> standards/workflow.md §3. ID 형식: F=기능 / I=개선 / B=버그 / T=기타.
> 한 번 부여한 ID 는 재사용 금지. 완성도 단계: 10/60/90/100%.

## 작업 표

| ID  | 기능명/요약                                                   | 단계   | 완성도 | 의존 | 영향          | 비고                         |
|-----|---------------------------------------------------------------|--------|--------|------|---------------|------------------------------|
| F01 | Helm chart scaffold + ArtifactHub publish 파이프라인           | 완료   | 100%   | -    | 모든 release  | commit 8a54d3d (ADR-0024)   |
| T01 | valkey ArtifactHub UI 자동 등록 (claude-in-chrome MCP)        | 완료   | 100%    | F01  | F02           | 사용자 수동, name 권장: keiailab-valkey-operator |
| T02 | valkey 첫 release v0.1.0-alpha.1 트리거                         | 완료   | 100%   | T01  | -             | GHCR + GH Release + gh-pages |
| T03 | valkey GitHub Pages 활성화                                      | 완료   | 100%   | T02  | -             | 자동 (gh-pages push 트리거)  |
| T04 | postgres 첫 release v0.3.0-alpha.1 트리거 (3-repo 통일)         | 완료   | 100%   | -    | -             | sha256:7658a42e, gh-pages 817399a |
| T05 | postgres GitHub Pages + ArtifactHub (이미 등록 e7f6b661)        | 완료   | 100%   | T04  | -             | ~30분 polling 자동 인덱싱   |
| I01 | grpc v1.72.2→v1.81.0 (CVE-2026-33186 CRITICAL)                  | 완료   | 100%   | -    | gate audit    | commit a353b44              |
| I02 | otel SDK v1.36.0→v1.43.0 (GO-2026-4394 PATH hijacking)          | 완료   | 100%   | -    | gate audit    | commit c05b251              |
| I03 | Makefile audit trivy fail-handling 보강 (silent-fail 제거)      | 완료   | 100%   | -    | release gate  | valkey a353b44, mongodb 2b7c44a |
| I04 | ADR-0021 supersede + ADR-0024 작성 + INDEX 갱신                 | 완료   | 100%   | F01  | 추적성        | commit 8a54d3d              |
| I05 | scripts/artifacthub-register.sh helper                          | 완료   | 100%   | F01  | T01           | UUID 검증 + sed 교체 + 검증 + name 충돌 안내 |
| I06 | Renovate 3-repo 추가 (RFC 0002 §7 예외)                         | 완료   | 100%   | -    | 3-repo        | valkey 2869b93, mongodb bf772ce, postgres 0ab83ef |
| I07 | postgres docker buildx --platform linux/amd64 강제 (글로벌 §2)  | 완료   | 100%   | -    | postgres release | commit 314af15           |
| I08 | Chart.yaml `artifacthub.io/changes` ↔ CHANGELOG 자동 sync       | 설계   | 10%    | T02  | 모든 release  | release-time hook 도입       |
| I09 | values.schema.json 정밀 schema (features.cluster/backup/...)    | 설계   | 10%    | F01  | helm install  | 현재 `additionalProperties: true` minimal |
| T06 | GitOps deploy 오버레이 도입 (3-repo 정합)                       | 완료   | 100%   | -    | 운영 배포     | 2026-05-06. ADR-0029. `deploy/overlays/prod/{kustomization,delete-namespace}.yaml` + `deploy/valkey-cluster.yaml` (db ns, sharded 3×1) + `deploy/README.md`. `kustomize build` PASS, Namespace 0. patch target raw `system`. |
| B01 | `spec.version.version` 8.1.6→9.0.4 rolling upgrade 회귀 가드     | 완료   | 100%   | -    | Phase B        | 2026-05-07. Valkey/ValkeyCluster envtest 로 STS image 갱신 검증 + Kind E2E 로 pod UID 변경/Ready 복귀/status.version=9.0.4 확인. |
| I10 | Kind E2E 기본 배포 전제 보강 — Prometheus Operator CRD bootstrap | 완료   | 100%   | B01  | test-e2e       | `config/default` 가 ServiceMonitor/PrometheusRule 을 기본 렌더하므로 E2E BeforeSuite 에 monitoring.coreos.com CRD 설치/정리 추가. focused spec 반복 실행 위해 manager namespace 생성 idempotent 처리. |
| B02 | Redis 8.2.1 RDB → Valkey 9.0.4 restore 무한대기 fail-fast        | 완료   | 100%   | B01  | Phase B        | 2026-05-07. Redis 8.2.1 RDB 는 Valkey 9.0.4 에서 `Can't handle RDB format version 12` 로 직접 restore 불가. Restoring 중 pod CrashLoopBackOff 를 `ValkeyRestore.status.phase=Failed` 로 표시하는 단위/E2E 회귀 가드 추가. |
| T11 | ValkeyCluster 9.0.4 sharded 3×1 Kind smoke                     | 완료   | 100%   | B01  | Phase B        | 2026-05-07. 3 shards × 1 replica, 6 pod Ready, `cluster_state=ok`, `assignedSlots=16384`, `status.version=9.0.4`, `valkey-cli -c SET/GET` 검증. |
| T12 | latest 기본값 정렬 — Valkey 9.0.4 + 8.0/8.1 milestone whitelist | 완료   | 100%   | T11  | chart/API/deploy | 2026-05-07. API default, CRD default, Helm values, ArtifactHub examples/images, samples, GitOps CR 기본값을 9.0.4 로 정렬. `SupportedValkeyVersions` 는 8.0.9/8.1.6/8.1.7/9.0.4 허용. `make test`, `make lint`, `make helm-template`, deploy overlay render PASS. |
| T13 | CloudPirates valkey 0.20.2 운영 knob 교차검증 + CRD 호환 확장 | 구현   | 90%    | T12  | API/controller/chart/docs | 2026-05-12. imageRef, storage ephemeral/existing/accessModes/metadata, service type/IP family/metadata, pod metadata/probe/env/hostAliases, externalReplica, revisionHistoryLimit, chart extraObjects 구현. 로컬 게이트와 현 클러스터 적용 검증 진행 중. |

## 차단됨

| ID  | 차단 내용 | 증거 | 다음 결정 |
|-----|-----------|------|-----------|
| C01 | 현재 Bitnami redis-cluster appVersion 계열 Redis 8.2.x RDB 를 Valkey 9.0.4 로 직접 restore 하는 경로는 호환되지 않음 | `redis:8.2.1` 로 생성한 RDB 를 `valkey:9.0.4` 가 `Can't handle RDB format version 12` 로 거부. Kind E2E 도 동일 원인 log 확인. | Bitnami 대체 마이그레이션은 RDB 직접 restore 가 아니라 온라인 key copy/dual-write/cutover 또는 Valkey 호환 source dump 경로로 별도 설계 필요. |

## 완료된 publish 산출물 (3-repo, 2026-05-06)

| repo | version | GHCR sha | gh-pages | Pages | ArtifactHub |
|---|---|---|---|---|---|
| **mongodb-operator** | v1.4.5 | (사전 publish) | live | built | ✓ 인덱싱 (1.4.5) |
| **postgres-operator** | v0.3.0-alpha.1 | sha256:7658a42e | live (817399a) | built | repositoryID e7f6b661, ~30분 polling |
| **valkey-operator** | v0.1.0-alpha.1 | sha256:2d1463bf | live (37716ff) | built | placeholder, T01 등록 후 |

## 검증 PASS 인용

```
$ helm pull valkey-test/valkey-operator --version 0.1.0-alpha.1
/tmp/valkey-operator-0.1.0-alpha.1.tgz   (44557 bytes)

$ helm pull postgres-test/postgresql-operator --version 0.3.0-alpha.1
/tmp/postgresql-operator-0.3.0-alpha.1.tgz   (19807 bytes)

$ curl https://artifacthub.io/api/v1/packages/helm/mongodb-operator/mongodb-operator
name: mongodb-operator   version: 1.4.5   prerelease: False
```
