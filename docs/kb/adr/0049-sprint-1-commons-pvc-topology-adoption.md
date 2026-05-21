# ADR-0049: Sprint 1 — operator-commons pkg/pvc + pkg/topology 채택 (-322 LOC)

- Date: 2026-05-21
- Status: Accepted
- Authors: @eightynine01 (Codex Major #7 — Sprint 1 Phase 2)
- Refs: operator-commons ADR-0012 (commons-side decisions)

## Context

valkey-operator 의 `internal/controller/pvc_resize.go` (~136 LOC) + test
(166 LOC) + `internal/resources/statefulset.go` 의 inline
`defaultTopologySpread` (~22 LOC) 가 postgres / mongodb 와 거의 동일
cross-repo 중복. commons Sprint 1 (ADR-0012) 에서 `pkg/pvc` +
`pkg/topology` 신규 추출.

## Decision

1. **pkg/pvc 어댑션** — `internal/controller/pvc_resize.go` (136 LOC) +
   test (166 LOC) 삭제. 2개 callsite 교체:
   - `valkey_controller.go:235` (Valkey CR / standalone).
   - `valkeycluster_controller.go:239` (ValkeyCluster CR / cluster mode).
   - 시그니처 변경: `expandDataPVCs(ctx, c, ns, crName, size)` →
     `commonspvc.ExpandDataPVCs(ctx, c, ns, []string{crName}, size)`.

2. **pkg/topology 어댑션** — `internal/resources/statefulset.go` 의 inline
   `defaultTopologySpread` (22 LOC) + 호출부 조건분기 (5 LOC) 를 commons
   호출 1줄로 압축:
   ```go
   // 이전: 분기 + inline 함수.
   if len(podSpec.TopologySpreadConstraints) == 0 && p.Replicas >= 2 {
       podSpec.TopologySpreadConstraints = defaultTopologySpread(selector)
   }
   // 이후: commons single call.
   podSpec.TopologySpreadConstraints = commonstopology.Defaulted(
       podSpec.TopologySpreadConstraints, p.Replicas, selector,
   )
   ```
   - valkey 는 `Replicas >= 2` 의미 → commons 기본 `WithMinReplicas(2)` 와 동일.

3. **go.mod**: `operator-commons v0.7.0 → v0.8.1-0.20260521045707-85a46ba80952`
   (commons PR #52 pre-merge — 그리고 valkey 는 v0.7.0 에서 v0.8.0 skip 후
   바로 신규 commit hash 로 이동). v0.9.0 tag 후 본 ADR 갱신.

## Consequences

### Positive

- LOC 감축: -322 LOC.
- 단일 SSOT.
- mongodb 와 동일 — default semantics → 옵션 0건.

### Negative

- valkey 는 commons v0.7.0 → v0.8.1+ 로 jump (postgres/mongodb 보다 큰 bump).
  v0.7.0 → v0.8.0 의 변화도 함께 적용됨 — 변경 범위 확장.
- Beta tier 채택.

### Trade-offs

- *v0.8.0 단계적 채택 (별 PR)* vs *직접 v0.8.1+ 이동* — 본 PR 은 후자.
  v0.8.0 cycle 의 변경은 commons CHANGELOG 와 family.md 가 이미 검증 +
  postgres/mongodb 가 운영 중이라 risk 낮음. 단계적 채택은 e2e 충돌 시
  bisect 비용 증가.

## Refs

- operator-commons PR #52, ADR-0012.
- 삭제된 원본:
  - `internal/controller/pvc_resize.go` (-136 LOC).
  - `internal/controller/pvc_resize_test.go` (-166 LOC).
- 수정된 파일:
  - `internal/controller/valkey_controller.go` (callsite + import).
  - `internal/controller/valkeycluster_controller.go` (callsite + import).
  - `internal/resources/statefulset.go` (inline `defaultTopologySpread` 삭제 + commons 호출).
