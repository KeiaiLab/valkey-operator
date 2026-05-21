# ADR-0051: 멀티아키 빌드 opt-in 활성화 (ARM 노드 도입 + 외부 GA 대비)

- Date: 2026-05-19
- Status: Proposed
- Authors: @phil
- Refs: GOVERNANCE.md §2.3 (amd64-only 정책)

## Context

valkey-operator 의 `Makefile` `docker-buildx` target 은 *이미* `--platform=$(PLATFORMS)` 인자 기반 멀티아키 빌드 capability 를 보유. Dockerfile 도 `TARGETOS`/`TARGETARCH` ARG 기반 cross-platform 호환. 차단점은 **글로벌 정책 GOVERNANCE.md §2.3** 의 *"linux/amd64. 커스텀 빌더·멀티아키텍처 금지"* 조항.

본 작업 trigger:
- ARM 노드 (Graviton / Ampere / Apple Silicon CI runner) 호환 필수
- OperatorHub.io 채택 path 진입 시 멀티아키 manifest 가 정합 baseline
- 사용자 명시 GO (2026-05-19): "멀티아키텍처 + OLM 배포까지 진행"

## Decision

본 ADR 은 **로컬 enablement 만** 결정. 글로벌 정책 변경은 RFC-0048 (별도) 가 처리.

1. **Makefile `PLATFORMS` default = `linux/amd64` 유지** — 정책 회귀 위험 0.
2. **opt-in override**: `make docker-buildx PLATFORMS=linux/amd64,linux/arm64 IMG=...` 사용 가능.
3. **release pipeline 변경 없음** — `scripts/release` (cycle 54) 의 amd64-only path 유지. ARM 활성은 별도 release-multiarch target 또는 RFC-0048 Accepted 후 default 전환.
4. **bundle.Dockerfile 도 동일 패턴** — bundle image (`ghcr.io/keiailab/valkey-operator-bundle`) 도 amd64-only default 유지 + opt-in 멀티아키.

## Consequences

### 긍정
- ARM 노드 도입 시 *즉시 적용* 가능 (env override 1줄).
- 외부 OperatorHub.io 채택 path 의 *기술적 차단점 0*.
- GOVERNANCE.md §2.3 *직접 위반 없음* (default 유지 + opt-in only).

### 부정
- 정책 변경 RFC-0048 Accepted 전까지는 *공식 release artifact* = amd64-only.
- ARM 사용자가 *직접 빌드* 가능하나 *공식 GHCR pre-built image* 미제공.

### 후속 작업
- RFC-0048 Accepted 시점에 PLATFORMS default → `linux/amd64,linux/arm64` 전환 PR
- `scripts/release` cycle 55 phase 에서 release-multiarch path 추가
- governance-report `multi_arch_release_enabled` rule (별도 instance #)

## Alternatives Considered

### A1. 글로벌 RFC-0048 머지 전까지 변경 0 (status quo)
- 거절: 사용자 명시 GO + 외부 도입 시점 차단점 영구 잔존. OLM 채택 path 와
  *진본 정합 부재*.

### A2. PLATFORMS default 즉시 `linux/amd64,linux/arm64`
- 거절: GOVERNANCE.md §2.3 *직접 위반*. RFC-0048 Accepted 후 진행.

### A3. 별도 멀티아키 target (`docker-buildx-multiarch`) 신설
- 거절: 기존 `docker-buildx` 가 이미 `PLATFORMS` 매개변수화. duplication = §principles.md §2 Simplicity First 위반.
