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

## 차단됨

- [!] T01: valkey ArtifactHub UI 신규 등록 — name 충돌 회피 위해 `keiailab-valkey-operator` 권장. `scripts/artifacthub-register.sh <uuid>` 로 placeholder 교체 + commit + push + `make helm-publish` 재실행.

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
