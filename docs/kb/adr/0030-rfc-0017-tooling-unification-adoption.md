# ADR-0030: RFC-0017 operator tooling unification 채택

- Date: 2026-05-09
- Status: Proposed
- Authors: @eightynine01
- Tags: tooling, ci, hook, lint, makefile

## Context

ai-dev RFC-0017 가 4 operator repo 도구 통합을 제안한다. 본 ADR 은 valkey-operator 측 채택 결정.

본 repo 의 현 상태 (2026-05-09 audit):
- Hook: `.lefthook.yml` ★ **RFC-0017 §3.1 표준 원본**
- `.golangci.yml`: **부재** — RFC-0017 §3.2 위반
- `.custom-gcl.yml`: 부재
- Makefile: lint/test/audit 보유, **validate 부재** — RFC-0017 §3.3 위반
- Dockerfile: distroless static nonroot — RFC-0017 §3.5 (HEALTHCHECK) 철회 후 N/A. helm chart probe 정합 확인 필요.
- EventRecorder: ✓
- Observability ★ **PrometheusRule + ServiceMonitor 보유** — 4-repo 모범

## Decision

RFC-0017 을 **Accepted** 로 채택하고 본 repo 에서:

1. `.lefthook.yml` 변경 없음 (이미 표준 원본)
2. `.golangci.yml` 신규 — postgres 패턴 채택, valkey 고유 depguard 규칙 (있다면) 추가
3. `.custom-gcl.yml` 신규 — logcheck plugin
4. Makefile `validate` 타겟 추가 — `kustomize build config/default | kubectl apply --dry-run=server -f -` + `helm lint charts/valkey-operator`
5. ~~Dockerfile HEALTHCHECK 추가~~ — 철회. 본 repo 의 helm chart probe 검증.

## Consequences

### 긍정
- 본 repo 의 lefthook 패턴이 4-repo 표준 원본으로 승격
- 본 repo 의 PrometheusRule + ServiceMonitor 패턴이 RFC-0017-followup observability RFC 의 표준 원본 후보 (별도 RFC 필요)
- 18 linter 활성화로 잠재 issue 노출

### 부정 / 트레이드오프
- 기존 코드 18 linter 통과 검증 시 신규 issue 발견 가능 — 단계 fix 필요
- depguard 규칙 (있는 경우) 도입 시 기존 internal import 경고 가능

### 후속 작업
- [ ] AI-VK30-1: golangci v2 18-linter 활성화 후 issue 분류 (Owner: @eightynine01, Due: 2026-05-19)
- [ ] AI-VK30-2: Makefile validate 타겟 동작 검증 (`make validate` PASS) (Owner: @eightynine01, Due: 2026-05-12)
- [ ] AI-VK30-3: helm chart probe 정합성 검증 (livenessProbe + readinessProbe httpGet:/healthz:8081) (Owner: @eightynine01, Due: 2026-05-12)
- [ ] AI-VK30-4: PrometheusRule + ServiceMonitor 패턴 commons 추출 RFC 후속 검토 (Owner: @eightynine01, Due: 2026-05-26)

## Alternatives Considered

| 대안 | 거절 사유 |
|------|----------|
| `.golangci.yml` 부재 유지 | RFC-0017 §3.2 직접 위반, 본 repo 코드 품질 보증 부재 |
| validate 타겟 미추가 | manifest 회귀 검증 부재 — kustomize/helm drift 발생 시 운영 단계까지 누설 |

## References

- 글로벌 RFC: `~/Documents/ai-dev/rfcs/0017-operator-tooling-unification.md`
- 관련 audit: `~/.claude/plans/mongodb-operator-operator-commons-postgr-tranquil-horizon.md`
