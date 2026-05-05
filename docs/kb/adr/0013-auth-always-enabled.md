# ADR-0013: Auth 는 사실상 항상 enabled — 명시적 false 가 동작하지 않음

- Date: 2026-05-05
- Status: Accepted (옵션 A)
- Authors: @phil

## Context

`AuthSpec.Enabled bool` (CRD default=true) 가 실제 reconcile 흐름에서 *무시*된다.
ValkeyReconciler 의 `ensureAuthSecret` 은 `Enabled` 값을 체크하지 않고 항상
random password 를 생성·secret 으로 저장하며, ConfigMap 의 `requirepass` 도
무조건 설정한다 (`internal/resources/configmap.go::configDataFromValkey` /
`configDataFromCluster`).

또한 ADR-0011 의 패턴 (`omitempty` 부재 → CRD schema default skip) 이
`Enabled bool json:"enabled"` 에도 적용되어, `spec: {}` 으로 만든 CR 의 status
는 `Auth.Enabled=false` 로 보인다 — 그런데 실제 Valkey 는 requirepass 가
설정되어 있어 auth 요구. **사용자 시점에서 status 와 실제 동작이 불일치**.

실측: kind 클러스터 `valkey-sample` (`spec: {}`) 의 status:
```
Auth:
  Enabled: false
```
그러나 `valkey-cli ping` 은 `NOAUTH Authentication required` 응답.

## Decision (Proposed)

다음 중 택1 (구현은 별도 PR + 테스트):

**A. Auth 강제 enable (status quo 정당화)**
- `Enabled` 필드 deprecate, docs 에 "operator 는 항상 auth 를 강제한다" 명시.
- status 출력 도 항상 `Enabled: true` 로 표기 (defaulter 에서 채움).
- 보안 우선 — 특히 multi-tenant K8s 에서 실수로 auth 끄는 위험 차단.

**B. `Enabled bool` → `Enabled *bool` 변경 (proper opt-out)**
- nil = default true (operator 가 auth 강제)
- explicit true = 동일
- explicit false = `requirepass` 안 씀, secret 생성 안 함
- CRD schema breaking change → v1alpha1 → v1beta1 conversion 필요 (현재 cluster 영향 0, 본 operator 가 아직 v1alpha1 이므로 가능).

권장: **A** — security 기본값 + 코드 변경 최소. 추후 사용자 요청 누적 시 B 로
이행.

## Consequences

A 채택 시:
- status report 와 실제 동작 일치.
- 사용자 가 `Enabled: false` 로 설정해도 무시됨 — *명시적으로 docs 에 안내* 필요.
- 보안 기본값 유지.

부정:
- Enabled 필드 가 거짓 약속 (deprecated 표시 가 분명해야).

## Action Items

- [ ] AI-001: 두 옵션 결정 (현재 Proposed)
- [ ] AI-002: A 선택 시 → defaulter 에서 `Auth.Enabled=true` 강제 + docs 갱신
- [ ] AI-003: B 선택 시 → API 변경 + conversion + reconciler 분기 + e2e 테스트
- [ ] AI-004: webhook 단위 테스트 — Auth.Enabled 케이스 매트릭스

Refs: ADR-0011 (omitempty 패턴)
