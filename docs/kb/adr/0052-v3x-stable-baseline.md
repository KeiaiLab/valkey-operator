# ADR-0052: v3.x-stable baseline 인정 (audit ❌ 0 충족)

| Meta | Value |
|---|---|
| Status | Accepted |
| Date | 2026-05-21 |
| Author | keiailab |
| Supersedes | (none) |
| Related | commons-ADR/0013 (audit SSOT), CLAUDE.md §7 (v3.x-stable 정의), valkey-ADR/0049 (Sprint 1 commons pvc+topology), valkey-ADR/0050 (audit augmentation), valkey-ADR/0048 (GHA retention) |

## Context

CLAUDE.md §7: "본 규약은 **상용 제품 수준**의 다중 프로젝트 일관성을 목표로 한다 — `standards/enforcement.md`의 P0+P1+P2 자동화 모두 충족 시 *v3.x-stable* 선언."

본 repo (valkey-operator) 는 다음 두 축으로 *v3.x-stable* 진입 조건을 충족했다.

### 1. audit ❌ 0 측정 — 2026-05-21 15:30

`commons/scripts/audit-production-grade.sh` (commons-ADR/0013 SSOT) 가 5 repo (postgres / mongodb / valkey / commons / forgewise) 의 P0 (기본 안전) + P1 (품질 게이트) + P2 (거버넌스) + OP (운영) + C (커뮤니티) 50+ 항목을 자동 측정. 본 repo 의 결과 (ralph-loop 다수 sub-cycle 후 audit ❌ 0 도달):

- P0 (기본 안전): ✅ pre-commit / pre-push / secrets / 한국어 검사 모두 통과
- P1 (품질 게이트): ✅ lint (`golangci-lint`) / test / typecheck / build / audit / import-graph 통과 (valkey-ADR/0050 의 P1-11/12/13 보강 — kube-linter + go-licenses + markdown-link-check)
- P2 (거버넌스): ✅ ADR coverage (0001~0051) / RFC-0002 GHA dual-track / 표준 모듈 정합
- OP (운영): ✅ release.sh 자동화 / helm-publish.sh 신규 (valkey-ADR/0050 OP-2) / chart .tgz publish / OCI image / UPGRADING.md (OP-10)
- C (커뮤니티): ✅ ADOPTERS.md / CONTRIBUTING / CODE_OF_CONDUCT / SECURITY / GOVERNANCE / 외부 chart 호환성 매트릭스 (valkey-ADR/0043) / SLSA Level 3 + cosign supply chain (valkey-ADR/0046) / ArtifactHub 서명 배지 (valkey-ADR/0044)

### 2. 거버넌스 baseline

- **RFC-0002 정합** (GitHub Actions 영구 금지) — 본 repo 는 valkey-ADR/0048 dual-track + valkey-ADR/0045 (oss CI restore) 로 운영. 예외 3종 (Pages 정적 배포 + Dependabot/Renovate + release tag → Release body) 명시.
- **i18n** (en/ko) README — supercycle 2026-05-21 Wave 4 부분 완료 (ja/zh 는 후속 RFC).
- **공급망 보안**: valkey-ADR/0046 (SLSA Level 3 + cosign signature) + valkey-ADR/0044 (ArtifactHub trust badge) — 외부 사용자 검증 신호.
- **상업 호환**: valkey-ADR/0042 (commercial parity series closure) + valkey-ADR/0043 (외부 chart 호환 매트릭스).

## Decision

본 repo (`keiailab/valkey-operator`) 를 **v3.x-stable** 로 인정한다.

- *외부 사용자 대상 운영 등급* 으로 공개 가능.
- 후속 release tag `vX.Y.Z` 권장 — 구체 버전 (현 v0.x → v1.0.0 GA) 은 별 사용자 결정 + 별 ADR 로 추적 (CHANGELOG 정합 + semver 판단).
- 본 ADR 자체는 *baseline 인정* 만 — 실 tag 행위는 사용자가 별도 명시.

## Consequences

### Positive

- **외부 신뢰** — audit 자동 측정 (commons-ADR/0013) + 본 baseline ADR + 거버넌스 4종 + 공급망 보안 (SLSA L3 + cosign) + 외부 chart 호환 매트릭스의 5 축이 *상용 등급* 신뢰 신호로 작용.
- **거버넌스 정합** — RFC-0002 위반 / standards/* 일탈은 ADR 부재 시 §5 실패로 자동 차단. 회귀 시 본 baseline 무효화.
- **공급망 차별화** — SLSA Level 3 + cosign signature 는 5 repo 중 valkey 가 선도. 후속 sister repo 의 reference 패턴.
- **상업 대안** — 외부 chart / 외부 chart 비교 매트릭스로 외부 사용자 평가 자료 자체 제공.

### Negative / 회귀 차단 조건

- **audit ❌ ≥ 1 회귀 시** — v3.x-stable 인정 *유지 불가*. 본 ADR 갱신 + commons audit-history 시계열 기록 필수.
- **standards/* 일탈 시** — ADR 부재면 §5 실패. 일탈 자체는 ADR 동반 시 허용 (§7 우선순위 사용자 명시 > Tier-3 > Tier-2 > Tier-1).
- **i18n 부분 정합** — 현 en/ko 만. ja/zh README 누락은 v3.1+ 의 P5 커뮤니티 KPI 에서 보강 후보.

### Trade-offs

- *v3.x-stable 본 선언* (본 ADR) vs *RFC-0005 글로벌 선언 대기* — 본 repo 는 baseline 만 인정하고 글로벌 RFC-0005 는 별 사용자 결정 영역으로 분리. 글로벌 선언 부재 시에도 본 repo 의 audit ❌ 0 자체가 *측정 가능한 운영 등급 신호*.
- *현 v0.x 인정* vs *v1.0 GA 격상* — 본 ADR 은 격상 강제 안 함. CHANGELOG 정합 + 사용자 결정.

## 후속 (v3.1+)

본 baseline 후 v3.1+ 진화 후보:
- P3 성능 게이트 (valkey benchmark + budget) — 별 RFC
- P4 DR 게이트 (backup + restore + chaos) — 별 RFC
- P5 커뮤니티 KPI (이슈 응답 SLA + adopter 성장) — 별 RFC
- i18n ja/zh README 보강 — supercycle 2026-05-21 Wave 4 잔여 → 별 sub-cycle
- audit 자동 측정 cron (월 1회) + audit-history 자동 갱신 — commons 측 별 ADR

## 참조

- commons-ADR/0013: `audit-production-grade.sh` 5 repo SSOT 측정 자동화
- CLAUDE.md §7 (v3.x-stable 정의): https://github.com/keiailab/.codex (글로벌 standards, private)
- valkey-ADR/0042: Commercial parity series closure
- valkey-ADR/0043: 외부 chart valkey 호환 매트릭스
- valkey-ADR/0044: ArtifactHub signed official trust badges
- valkey-ADR/0046: SLSA Level 3 + cosign supply chain
- valkey-ADR/0048: GHA retention (dual-track)
- valkey-ADR/0049: Sprint 1 commons pkg/pvc + pkg/topology
- valkey-ADR/0050: Audit augmentation (P1-11/12/13 + OP-2 + OP-10)
- valkey-ADR/0051: Multi-arch build enablement
