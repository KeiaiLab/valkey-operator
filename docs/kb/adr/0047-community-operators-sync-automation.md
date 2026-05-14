# ADR-0047: community-operators sync 자동화 (RFC 0002 예외 ③ 확장)

- Date: 2026-05-14
- Status: Accepted
- Authors: @eightynine01

## Context

valkey-operator 의 OLM bundle 을 k8s-operatorhub/community-operators upstream 에 sync. 현재 수동 — 2026-05-14 본 라이브 evidence: valkey-operator **community-operators 미등록** (sister mongodb 0.3.0 / postgres 1.4.0 ↔ valkey 부재). 본 turn 의 community-operators#8121 (valkey 1.0.13 신규 등록) 도 *수동* PR.

ADR-0037 'operatorhub-bundle-scaffold' 가 정립한 bundle infrastructure 는 완비. 본 ADR 가 upstream sync 자동화 격차 해소.

## Decision

mongodb ADR-0027 와 동일 패턴 sister parity. `.github/workflows/release.yml` 의 `github-release` job 후속에 `sync-community-operators` job 신설.

조건:
- release tag push trigger
- prerelease tag (alpha/beta/rc) skip
- secret `COMMUNITY_OPERATORS_PAT` 의무
- AI 자동 머지 0 (외부 maintainer 책임)

## Consequences

긍정:
- bundle drift 영구 차단
- sister operator (mongodb ADR-0027, postgres ADR-0014) 일관 패턴

부정:
- RFC 0002 §2 충돌 (§7 예외 ③ 확장 해석으로 정당화)
- COMMUNITY_OPERATORS_PAT secret 회전 의무

## Alternatives Considered

- A (현 수동): community-operators#8121 식 manual PR — 시간 비용 (라이브 evidence)
- B (본 결정): release.yml step 자동화 — RFC 0002 §7 예외 ③ 확장
- C: cluster 내 CI (Tekton) — KeiaiLab cluster 의존 위험

## Refs

- RFC 0002 (no-github-actions)
- ADR-0037 (operatorhub-bundle-scaffold)
- community-operators#8121 (수동 sister 라이브 evidence)
- sister: mongodb ADR-0027 (MERGED PR #169), postgres ADR-0014 (별 PR)
