# ADR-0062: Valkey Modules — v1alpha1 미러 (conversion hub 전환 deferred)

- Date: 2026-06-16
- Status: Accepted
- Authors: @phil
- Refs: ADR-0032 (Custom Modules init container), ADR-0026 (Conversion Webhook deferred), ADR-0036 (PSS Restricted), GitLab keiailab/oss/valkey-operator#1 (PR-C6.2)

## Context

ADR-0032 (PR-C6.1) 은 `ModuleSpec` type + `ValkeySpec.Modules` 를 **v1alpha2 에만**
추가했고, PR-C6.2 (controller wiring) 는 *"PR-A2.2 (v1alpha2 hub 전환) 의존"* 으로
명시했다 (ADR-0032 §Decision 5).

그러나 PR-C6.2 구현 시점 (2026-06-16) 의 실측 결과:

- `api/v1alpha1/valkey_types.go:157` = `+kubebuilder:storageversion` — **v1alpha1 이
  storage version**.
- `internal/controller/valkey_controller.go:85` = `&cachev1alpha1.Valkey{}` — 컨트롤러는
  **v1alpha1 을 reconcile**.
- `internal/webhook/v1alpha1/` — webhook 도 **v1alpha1 검증**.
- conversion webhook 미연결 (`config/crd` 에 `spec.conversion.strategy: Webhook` 부재,
  `cmd/main.go` 미등록). v1alpha2 는 `Hub()` marker 만 보유 (PR-A2.2.1).
- `api/v1alpha1/conversion.go` 의 `convertViaJSON` (JSON byte-copy) 은 v1alpha1 에
  없는 필드를 **silent drop** — 주석이 "Modules: v1alpha1 부재 → JSON unmarshal 시
  nil/zero" 로 자인.

결과: `Modules` 가 v1alpha2 에만 있으면 — 사용자가 v1alpha2 CR 로 제출해도 no-op
conversion (conversion webhook 부재) 이 v1alpha1 storage schema 로 prune → **Modules
가 컨트롤러에 도달 불가**. PR-A2.2 (hub 전환 + conversion webhook serving) 는 별도의
크고 위험한 작업이라 PR-C6.2 와 동시 진행 시 두 `[~]` 를 한 번에 건드리는 부담.

## Decision

`ModuleSpec` + `ValkeySpec.Modules` 를 **v1alpha1 에도 미러링**한다 (v1alpha2 와
*byte-동일* JSON tag/구조). 그 결과:

1. **conversion 무편집 보존**: `convertViaJSON` 이 양버전 동일 JSON tag (`modules` /
   `name` / `image` / `loadModuleArgs`) 를 자동 매핑 → hub↔spoke 왕복에서 Modules
   보존. conversion.go 변경 0.
2. **컨트롤러 직접 소비**: `valkey_controller.go` 가 `v.Spec.Modules` 를 STSParams 로
   직접 전달 (v1alpha1 reconcile path 그대로).
3. **webhook home = v1alpha1**: `validateModules` 가 `validateValkeySpec` 안에서 호출
   (allow-list / 중복 / BYO).
4. **bundle 태그 = major.minor**: valkey-server 는 patch 태그(9.0.4)를 발행하나
   valkey-bundle 은 major.minor(9.0)까지만 안정 발행 (실측: hub.docker.com). module
   `.so` ABI 는 minor 단위 호환 → `BundleTagOrDefault` 가 major.minor 로 resolve.
5. **대상 = Valkey (Standalone / Replication) only**: `ValkeyCluster` 는 Modules 필드
   미보유 → 미지원 (후속 PR).

PR-A2.2 (v1alpha2 storage 승격 + conversion webhook serving) 는 **본 ADR 범위 밖**.
해당 PR 이 landing 하면 v1alpha1 미러는 (a) 호환 보존용으로 유지하거나 (b) v1alpha1
deprecation cycle 에서 제거한다 — 그 시점 결정.

## Consequences

### Positive

- 모듈 프리셋이 **지금** 기능한다 — conversion webhook 전환 (위험 大) 선행 불요.
- 양버전 동일 schema → `convertViaJSON` 무손실, 기존 conversion 테스트 무영향.
- `TestValkey_Modules_RoundTrip_보존` 회귀 가드로 미러 정합 (byte-동일 JSON tag) 봉인.

### Negative

- v1alpha1 / v1alpha2 양쪽에 `ModuleSpec` 중복 정의 — 한쪽만 갱신 시 conversion silent
  drop 위험. 회귀 가드 테스트로 완화하나 *동일 변경 양버전 적용* 규율 의무.
- PR-A2.2 landing 시 v1alpha1 미러 정리 (제거 또는 유지 결정) 라는 후속 부채.

### Trade-offs

- *v1alpha2-only + conversion webhook 선행* (ADR-0032 원안) vs *v1alpha1 미러 (본 ADR)*
  — 전자는 큰 위험(웹훅 cert + storage 승격 + 컨트롤러 import 전환)을 PR-C6.2 에
  결합. 본 ADR 은 *작동하는 v1alpha1 path 재사용* 으로 위험 분리.

## Alternatives Considered

1. **PR-A2.2 (hub 전환) 선행 후 PR-C6.2** — 거부.
   - conversion webhook serving + cert-manager Certificate + storage 승격 + 5 컨트롤러
     import 전환 = 대규모 위험. 두 `[~]` 동시 = scope 폭증.

2. **conversion webhook 의 annotation-preservation 으로 v1alpha2-only 보존** — 거부.
   - conversion webhook 이 *serving* 돼야 동작 → PR-A2.2 의존 (대안 1 과 동일 차단).
   - annotation round-trip 은 컨트롤러가 annotation 을 unmarshal 하는 추가 복잡도.

3. **module 을 별도 CRD (ValkeyModule)** — 거부 (ADR-0032 §Alternatives 3 정합).
   - module 은 Valkey instance 의 일부 — Spec 통합이 정합.

## Refs

- ADR-0032 (Valkey Custom Modules — init container mount + 공식 preset only)
- ADR-0026 (Conversion Webhook deferred until v1alpha1 stable)
- ADR-0036 (PSS Restricted) — init container restricted SecurityContext 정합
- `api/v1alpha1/conversion.go` `convertViaJSON` (JSON byte-copy)
- GitLab keiailab/oss/valkey-operator#1 (PR-C6.2 구현 + e2e)
