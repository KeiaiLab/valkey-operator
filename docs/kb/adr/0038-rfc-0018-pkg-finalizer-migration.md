# ADR-0038: RFC-0018 채택 — pkg/finalizer migration (controllerutil → commons)

- Date: 2026-05-09
- Status: Accepted (PR-A6 first cut — finalizer only, status migration 별 PR)
- Authors: @eightynine01

## Context

RFC-0018 §3.2 의 valkey-operator 측 채택. 본 ADR 작성 시점:

| 호출 위치 | 변경 전 | 변경 후 |
|---|---|---|
| `internal/controller/helpers.go:67,75` | `controllerutil.ContainsFinalizer / RemoveFinalizer` | `commonsfinalizer.Has / Remove` |
| `internal/controller/valkey_controller.go:87,88` | `ContainsFinalizer / AddFinalizer` | `Has / Add` |
| `internal/controller/valkeycluster_controller.go:93,94` | 동일 | 동일 |
| `internal/controller/valkeybackup_controller.go:93,94,634` | `Contains/Add/Remove` | `Has/Add/Remove` |
| `internal/controller/valkeyrestore_controller.go:110,111,835` | `Contains/Add/Remove` | `Has/Add/Remove` |

핵심 보존:
- **wire contract** (`api/v1alpha1/finalizers.go` 의 `FinalizerValkey` 등 상수) 변경 없음 — 외부 사용자 (kubectl jsonpath, ArgoCD finalizer cleanup, Argo Events) 의존성 보호.
- 호출 시그니처 동등 (`metav1.Object` 가 `client.Object` 의 superset, 호환).
- 동작 동등 (commons API 가 controllerutil 과 동일한 idempotent semantics).

## Decision

2. **API 매핑**:
   - `controllerutil.ContainsFinalizer(o, name)` → `commonsfinalizer.Has(o, name)`
   - `controllerutil.AddFinalizer(o, name)` → `commonsfinalizer.Add(o, name)`
   - `controllerutil.RemoveFinalizer(o, name)` → `commonsfinalizer.Remove(o, name)`

3. **Wire contract 불변**: `api/v1alpha1/finalizers.go` 의 4 finalizer string 상수 (`FinalizerValkey` / `FinalizerValkeyCluster` / `FinalizerValkeyBackup` / `FinalizerValkeyRestore`) 그대로. 외부 의존자 영향 0.

4. **status migration 분리**: 본 PR 은 *finalizer 만*. `setCondition` / `meta.SetStatusCondition` → `commonsstatus.SetReady/SetReadyFalse/SetAvailable` 위임은 *별 PR* (PR-A6.2).

5. **controllerutil import 정리**: 2 파일 (`valkey_controller.go`, `valkeycluster_controller.go`) 가 finalizer 외 controllerutil 사용 없음 → unused import 제거. 3 파일 (`helpers.go`, `valkeybackup_controller.go`, `valkeyrestore_controller.go`) 는 SetControllerReference 등 다른 controllerutil 함수 사용 → import 보존.

## Consequences

### Positive

- commons `pkg/finalizer` 채택률 0% → 25% (4 repo 중 valkey 가 첫 도입). mongodb / postgres 후속 시 75% → postgres 비대칭 보존하면 67%.
- 호출 표면 변화 없음 — 기존 호출 패턴 그대로, 단순 import path 변경.

### Negative

- 외부 contributor 가 *commons API* 학습 의무. 단 controllerutil 와 sig 동등이라 학습 비용 미미.

### Trade-offs

- *5 controller 일괄 migration* (본 PR) vs *별 PR 으로 분할* (controller 별) — 5 파일 동일 패턴 일괄이 review 부담 < 5 PR 분할.
- *finalizer + status 통합* (단일 PR-A6) vs *분리* (PR-A6 finalizer + PR-A6.2 status) — 후자 채택. status migration 은 *도메인 ConditionType 영향 분석* 필요로 분리 review 가치.

## Alternatives Considered

1. **`commonsfinalizer.Add` 의 return value 활용 단순화** — 보류.
   - 기존 `if !ContainsFinalizer { AddFinalizer; Update; }` 패턴 이 `commonsfinalizer.Add` 의 return 활용 시 `if commonsfinalizer.Add(o, n) { Update }` 로 단순화 가능.
   - 본 PR 은 *최소 변경* 우선 — 호출 line 일대일 매핑 보존. 후속 refactor PR.

2. **commons `pkg/finalizer/runtime.EnsureRemoval` 신설** — 거부 (ADR-0003).
   - controller-runtime client 의존 도입 → commons zero-dep 원칙 위반.

3. **valkey 자체 wrapper 신설** — 거부.
   - mongodb/postgres 와 정합 깨짐. RFC-0018 §3.2 표준 채택이 cross-repo 일관성에 우위.

## Refs

- ADR-0003 (commons): pkg/status 슈가 + pkg/finalizer 변경 없음 결정.
- Plan §2 D10/D11 (4-repo migration matrix).
- PR-A1 머지 commit: 9891bf3 (main).
- valkey-operator commons v0.6.0 bump: chore/bump-commons-v0.6.0 머지.
- 후속 PR-A6.2 (별도): `setCondition` → `commonsstatus.SetReady` 위임.
