# TASKS — valkey-operator

> standards/workflow.md §3. ID 형식: F=기능 / I=개선 / B=버그 / T=기타.
> 한 번 부여한 ID 는 재사용 금지. 완성도 단계: 10/60/90/100%.

## 작업 표

| ID  | 기능명/요약                                                   | 단계   | 완성도 | 의존 | 영향          | 비고                         |
|-----|---------------------------------------------------------------|--------|--------|------|---------------|------------------------------|
| F01 | Helm chart scaffold + ArtifactHub publish 파이프라인           | 완료   | 100%   | -    | 모든 release  | commit 8a54d3d (ADR-0024)   |
| T01 | ArtifactHub UI 신규 등록 + repositoryID 교체                    | 차단됨 | 10%    | F01  | F02           | 사용자 수동 (UI 만 지원)     |
| T02 | 첫 release v0.1.0-alpha.1 트리거                                | 완료   | 100%   | T01  | -             | GHCR + GH Release + gh-pages |
| T03 | GitHub Pages 활성화                                             | 완료   | 100%   | T02  | -             | 자동 (gh-pages push 트리거)  |
| I01 | grpc v1.72.2→v1.81.0 (CVE-2026-33186 CRITICAL)                  | 완료   | 100%   | -    | gate audit    | commit a353b44              |
| I02 | otel SDK v1.36.0→v1.43.0 (GO-2026-4394 PATH hijacking)          | 완료   | 100%   | -    | gate audit    | commit c05b251              |
| I03 | Makefile audit trivy fail-handling 보강 (silent-fail 제거)      | 완료   | 100%   | -    | release gate  | commit a353b44              |
| I04 | ADR-0021 supersede + ADR-0024 작성 + INDEX 갱신                 | 완료   | 100%   | F01  | 추적성        | commit 8a54d3d              |
| I05 | scripts/artifacthub-register.sh helper                          | 완료   | 100%   | F01  | T01           | UUID 검증 + sed 교체 + 검증  |
| I06 | Chart.yaml `artifacthub.io/changes` ↔ CHANGELOG 자동 sync       | 설계   | 10%    | T02  | 모든 release  | release-time hook 도입       |
| I07 | values.schema.json 정밀 schema (features.cluster/backup/...)    | 설계   | 10%    | F01  | helm install  | 현재 `additionalProperties: true` minimal |
| I08 | mongodb-operator audit 에 trivy 추가 보강 (RFC 0002 L3 정합)    | 설계   | 10%    | -    | mongodb       | 별 repo 작업                |

## 차단됨

- [!] T01: ArtifactHub UI 신규 등록 (https://artifacthub.io/control-panel/repositories) — 사용자 수동 작업 필요. 해소 조건: 사용자가 등록 후 부여받은 UUID 를 인자로 `scripts/artifacthub-register.sh <uuid>` 실행 → repositoryID 교체 + commit + push + `make helm-publish` 재실행.

## 완료된 publish 산출물 (2026-05-06)

- **GHCR image**: `ghcr.io/keiailab/valkey-operator:v0.1.0-alpha.1` (sha256:2d1463bf...) + `:0.1.0-alpha.1` tag.
- **GitHub Release**: https://github.com/keiailab/valkey-operator/releases/tag/v0.1.0-alpha.1 (prerelease=true, asset valkey-operator-0.1.0-alpha.1.tgz).
- **Helm repo**: https://keiailab.github.io/valkey-operator (gh-pages branch, commit 37716ff).
  - `index.yaml` (5063 bytes) — apiVersion: v1, entries.valkey-operator
  - `valkey-operator-0.1.0-alpha.1.tgz` (44557 bytes)
  - `artifacthub-repo.yml` (5014 bytes, repositoryID placeholder)
- **GitHub Pages**: status=built (활성화 자동, gh-pages push 트리거).
