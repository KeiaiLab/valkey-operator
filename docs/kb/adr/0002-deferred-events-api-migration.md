# ADR-0002: Deferred migration to client-go events API

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil

## Context

controller-runtime v0.23.3 는 두 가지 EventRecorder 경로를 노출한다:

| 경로 | 패키지 | 시그니처 |
|------|--------|---------|
| Legacy | `k8s.io/client-go/tools/record` | `Eventf(obj, type, reason, format string, args ...any)` |
| New | `k8s.io/client-go/tools/events` | `Eventf(regarding, related runtime.Object, type, reason, action, note string, args ...any)` |

manager API 는 `GetEventRecorderFor(name) record.EventRecorder` (deprecated, SA1019) 와
`GetEventRecorder(name) events.EventRecorder` (preferred) 둘 다 제공.

`internal/controller/helpers.go:applyErrorCondition` 는 `record.EventRecorder` 시그니처에
의존 — 마이그레이션은 helpers.go + valkey_controller.go + valkeycluster_controller.go
+ 모든 Eventf 호출 사이트 동시 변경 필요.

## Decision

본 PR 에서는 **마이그레이션 보류**. `//nolint:staticcheck` + 본 ADR 참조 주석으로
deprecation 경고 무음 처리. M2 단계 별도 PR 로 분리.

근거:
- 단일 deprecation 처리를 위해 4 파일 동시 변경은 §3 Surgical 위반.
- controller-runtime v0.23.3 는 양쪽 API 모두 동작 — *기능 결함 아님*.
- `events.EventRecorder.Eventf(regarding, related, type, reason, action, note)` 의
  `action` / `note` 분리는 호출 사이트 디자인 검토가 필요 (단순 기계적 변환 아님).

## Consequences

**긍정:**
- 본 PR 작업 범위 명확.
- 후속 마이그레이션 시 단일 PR 으로 깨끗하게 진행 가능.

**부정:**
- controller-runtime v1.0.0 (legacy API 제거) 출시 전까지 기술 부채 잔존.
- `make lint` 통과를 위해 `nolint` 주석 2건 의존.

## 후속 작업

- Trigger: controller-runtime v0.24+ 또는 client-go v0.36+ 에서 legacy API deprecation
  scheduled-removal 표시 시.
- 작업 항목 (현재 상태, cycle 47 갱신):
  1. `helpers.go:applyErrorCondition` 시그니처 → `events.EventRecorder`.
  2. **5 reconciler** 의 `Recorder` 필드 타입 변경:
     valkey_controller, valkeycluster_controller, valkeybackup_controller (cycle 19),
     valkeybackuptarget_controller (cycle 19), valkeyrestore_controller (cycle 19).
  3. `Eventf` 호출 사이트 1곳 (`helpers.go:applyErrorCondition`) action/note 분리.
     본 함수가 reconciler 별 분기 없이 모든 reconciler 의 events 를 발행 — 단일
     변경점.
  4. **nolint 5건 제거** (cycles 19/46): valkey_controller:519,
     valkeycluster_controller:1024, valkeybackup_controller:704,
     valkeybackuptarget_controller:282, valkeyrestore_controller:781.
- 검증: `make lint` → SA1019 0건 + `kubectl get events --field-selector
  involvedObject.kind=Valkey` 정상 발행.
