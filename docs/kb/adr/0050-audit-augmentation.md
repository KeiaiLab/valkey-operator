# ADR-0050: Audit Augmentation — postgres 패턴 cp (P1-11/12/13 + OP-2 + OP-10)

- Date: 2026-05-21
- Status: Accepted
- Authors: @eightynine01 (S-valkey audit 보강, subagent dispatch)

## Context

`scripts/audit-production-grade.sh` 의 5 repo (commons / postgres
/ mongodb / valkey / forgewise) 측정에서 valkey-operator 잔여 ❌ 5건:

| ID | 항목 | 현 상태 |
|---|---|---|
| P1-11 | kube-linter hook | `.lefthook.yml` 에 없음 |
| P1-12 | go-licenses hook | 없음 |
| P1-13 | markdown-link-check hook | 없음 |
| OP-2 | scripts/helm-publish.sh | 없음 |
| OP-10 | docs/UPGRADING.md | 없음 |

keiailab 의 표준 패턴을 valkey-operator 에 적용.
일관성 회복이 필요.

## Decision

검증된 패턴을 *최소 정합 조정* (chart name, repo name)
후 cp:

1. **`.lefthook.yml` 의 pre-push 에 3 hook 추가** (PR #171):
   - `kube-linter` — `helm template valkey-operator charts/valkey-operator --include-crds | kube-linter lint -`
   - `go-licenses` — `go-licenses check ./... --disallowed_types=forbidden,restricted`
   - `markdown-link-check` — README + CHANGELOG + docs/*.md
   - 모두 graceful skip (도구 미설치 시 warn + exit 0)
   - 우회: `KUBE_LINTER_SKIP=1` / `GO_LICENSES_SKIP=1` / `MD_LINK_CHECK_SKIP=1`

2. **`scripts/helm-publish.sh` 신규** (PR #172):
   - postgres 의 helm-publish.sh 패턴
   - chart 이름: `charts/valkey-operator`
   - HELM_REPO_URL 기본값: `https://keiailab.github.io/valkey-operator`
   - HELM_SIGN=1 시 PGP 서명 지원 (keiailab 공통 key)

3. **`docs/UPGRADING.md` 신규** (PR #173):
   - postgres UPGRADING 패턴
   - valkey-specific 본문: v1.0.x, ValkeyCluster CRD, v1alpha1↔v1alpha2 conversion
   - Sprint 1 채택 (ADR-0049) 인용
   - GHA dual-track 정책 (ADR-0048) 인용

## Consequences

### Positive

- valkey audit P1-11 + P1-12 + P1-13 + OP-2 + OP-10 ✅ 적용 (5 항목)
- 5 repo (postgres / mongodb / valkey) 일관성 회복
- 외부 contributor 에게 명확한 upgrade path 제공 (`docs/UPGRADING.md`)
- helm-publish 의 manual workflow 가 SSOT 화 (Makefile target → scripts/ 분리)

### Negative

- lefthook pre-push 의 hook 수 증가 → graceful skip 으로 완화하나 *전체* push
  시간 증가 가능 (도구 설치 시).
- UPGRADING.md 의 ValkeyCluster-specific 본문은 별도 maintenance 필요 (CRD 변경
  시 갱신 의무).

### Trade-offs

- *cp + 정합 조정* vs *valkey 만의 신규 설계* — 본 ADR 은 전자. postgres 패턴은
  이미 5 repo 중 2개에서 검증 + 사용자 결정 (subagent dispatch + 영역 분리)
  으로 빠른 적용 우선.
- valkey 만의 *features.cluster.enabled* 같은 chart feature 가 kube-linter helm
  template 인자에 반영되지 않음 (default values 만 lint) — 향후 별 ADR 로
  feature matrix lint 검토 가능.

## Refs

- PR #171 — `feat(lefthook): kube-linter + go-licenses + markdown-link-check`
- PR #172 — `feat(scripts): helm-publish.sh`
- PR #173 — `docs(upgrading): UPGRADING.md`
- postgres 참조:
