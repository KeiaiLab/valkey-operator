# TASKS — valkey-operator

> standards/workflow.md §3. ID 형식: F=기능 / I=개선 / B=버그 / T=기타.
> 한 번 부여한 ID 는 재사용 금지. 완성도 단계: 10/60/90/100%.

## 작업 표

| ID  | 기능명/요약                                                  | 단계   | 완성도 | 의존  | 영향        | 비고                        |
|-----|--------------------------------------------------------------|--------|--------|-------|-------------|-----------------------------|
| F01 | Helm chart scaffold + ArtifactHub publish 파이프라인         | 완료   | 100%   | -     | 모든 release | commit 8a54d3d (ADR-0024) |
| T01 | ArtifactHub UI 신규 등록 + repositoryID 교체                  | 차단됨 | 10%    | F01   | F02         | 사용자 수동 (외부 UI)      |
| T02 | 첫 release 트리거 (`make release VERSION=v0.1.0-alpha.1`)    | 차단됨 | 10%    | T01   | -           | 사용자 결정 (GHCR push)    |
| T03 | GitHub Pages 활성화 (gh-pages 첫 publish 후)                  | 차단됨 | 10%    | T02   | -           | gh API 또는 사용자 수동    |
| I01 | Chart.yaml `artifacthub.io/changes` ↔ CHANGELOG 자동 sync     | 설계   | 10%    | T02   | 모든 release | release-time hook 도입     |
| I02 | values.schema.json 정밀 schema (features.cluster/backup/...) | 설계   | 10%    | F01   | helm install | 현재 `additionalProperties: true` minimal |

## 차단됨

- [!] T01: ArtifactHub UI 신규 등록 (https://artifacthub.io/control-panel/repositories) — 사용자 수동 작업 필요. 해소 조건: 사용자가 등록 후 부여받은 UUID 를 `charts/artifacthub-repo.yml` 에 교체.
- [!] T02: 첫 release 트리거 — 사용자 명시 결정 필요 (GHCR push + GitHub Release 등 외부 영향). 해소 조건: 사용자 "release 진행" 명령.
- [!] T03: GitHub Pages 활성화 — gh-pages 브랜치 첫 publish 후 자동 또는 수동 활성화. 해소 조건: T02 완료 후.
