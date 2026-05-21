# ADR-0034: Auth Optional + v1alpha2 신규 (supersedes ADR-0013)

- Date: 2026-05-09
- Status: Accepted (PR-A2.1 — type definition module)
- Authors: @eightynine01
- Refs: Plan §2 D1 (`~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-synthetic-gem.md`),
  ADR-0013 (supersede 대상), ADR-0026 (deferred 회복)

## Context

ArtifactHub 비교 분석 결과 외부 두 redis Helm 차트 (외부 chart,
외부 chart) 모두 `auth.enabled` 토글을 기본값 true 로 노출.
사용자가 외부 인증 (Istio mTLS, sidecar proxy auth, network-level isolated
namespace) 사용 시 명시적으로 비활성화 가능.

valkey-operator 현재 ADR-0013 (Auth Always Enabled) 으로 controller 가
`spec.Auth.Enabled` 를 *무시하고* 항상 random 32B password 강제 생성.
Phase 1 분석에서 이는 valkey-operator 의 *secure-by-default* 철학과
정합하나 *유연성 부족* — 외부 인증 시나리오 미지원.

사용자 결정 (Plan AskUserQuestion §1): Auth + NetworkPolicy + PSS *3종
모두 토글*, **default=true 유지** (secure-by-default 보존), CRD 호환성은
**v1alpha2 신규 + conversion webhook** (사용자 결정 §4).

## Decision

1. **v1alpha2 신규 패키지** (`api/v1alpha2/`) 가 v1alpha1 의 *type
   definition* 을 cp 하여 신규 작성. SchemeGroupVersion.Version =
   "v1alpha2", Group 동일 (`cache.keiailab.io`).

2. **AuthSpec.Required `*bool`** 신규 (`omitempty` + `kubebuilder:default:=true`):
   - `nil` (legacy v1alpha1 호환): `Enabled` 필드 fallback.
   - `true` (default): operator 가 random 32B password Secret 자동 생성
     + requirepass 강제 (v1alpha1 ADR-0013 동등 동작).
   - `false`: requirepass 미설정 + AUTH 명령 거부 + NOAUTH 응답.
     외부 인증 시나리오 (Istio mTLS, sidecar) 지원.

3. **AuthSpec.Enabled 는 deprecated 예정** — PR-A2.2 controller migration
   완료 후 v1alpha2 doc 에 `Deprecated:` 명시. v1alpha3 또는 v1beta1
   에서 제거 가능.

4. **PR 분할**:
   - **PR-A2.1** (본 ADR): v1alpha2 *type definition module* 만 추가.
     controller / webhook / cmd/main.go 미수정. v1alpha1 가 여전히 hub.
     conversion webhook 미설정.
   - **PR-A2.2** (별도, 후속): v1alpha2 가 hub 가 되도록 conversion
     webhook 활성 + ConvertTo/ConvertFrom 작성 + controller import
     v1alpha1 → v1alpha2 + cmd/main.go SchemeBuilder 등록 +
     `ensureAuthSecret` conditional 분기 (Required=false 시 skip).

5. **ADR-0026 deferred 회복**: ADR-0026 ("Conversion Webhook deferred
   until v1alpha1 stable") 의 결정을 *부분 회복*. 본 ADR 은 v1alpha2
   *추가* 만 결정 — conversion webhook 자체는 PR-A2.2 에서 별도 ADR.

## Consequences

### Positive

- 외부 인증 시나리오 (Istio mTLS, sidecar) 사용자가 명시적으로 `auth.required=false`
  로 활성화 가능 — 외부 chart 와 동등 유연성.
- default=true 유지로 v1alpha1 ADR-0013 의 *secure-by-default* 철학 보존.
- v1alpha2 신규 (vs v1alpha1 동작 변경) 로 기존 사용자 무중단 — 명시적
  마이그레이션 경로 제공.
- PR 분할 (A2.1 type / A2.2 controller) 로 각 PR 이 *독립적으로 reviewable*.

### Negative

- 두 필드 (Enabled + Required) 공존 → 혼동 가능. 완화: doc 에서
  Required 우선 + Enabled deprecated 명시. PR-A2.2 controller 가 nil-safe
  fallback (Required != nil ? *Required : Enabled).
- Conversion webhook 신규 — TLS cert + service endpoint 운영 부담 (PR-A2.2).
  완화: cert-manager 자동 발급 (ADR-0010 패턴 재사용).
- v1alpha2 = type module 만 (PR-A2.1) → controller migration (PR-A2.2)
  까지 *kubectl apply cache.keiailab.io/v1alpha2 가 reconcile 되지 않음*.
  완화: 본 ADR 의 §Decision 4 PR 분할 명시 + doc.go 에 anatomy 안내.

### Trade-offs

- *v1alpha2 신규 + conversion webhook* (사용자 결정 §4) vs *v1alpha1
  동작 변경 + default false* — 후자는 기존 사용자 무중단 X (CHANGELOG
  breaking 표기 필요). 본 ADR 은 전자 채택.

## Alternatives Considered

1. **v1alpha1 의 AuthSpec.Enabled 의미 변경** (controller 가 false 존중) — 거부.
   - ADR-0013 의 *항상 강제* 결정과 직접 충돌.
   - CRD 변경 없이 동작만 변경 → 사용자가 *기존 manifest* 의미 변경
     알아채기 어려움. v1alpha2 신규로 *명시적 마이그레이션 신호*.

2. **AuthSpec.Required `bool` (non-pointer)** — 거부.
   - default 가 false (Go zero value) 로 설정되면 v1alpha1 호환 깨짐.
   - `*bool` + `kubebuilder:default:=true` 가 Kubernetes API convention
     (omitempty + nil-safe).

3. **AuthSpec 통합 (Required + Enabled 단일 필드)** — 거부.
   - v1alpha1 호환 깨짐 — 기존 manifest 의 `enabled: true` 가 새 필드명에
     매치 안 됨. conversion webhook 으로 매핑 가능하나 단순성 우선.

4. **v1beta1 신규 (v1alpha2 건너뛰기)** — 거부.
   - K8s API convention: v1alpha → v1beta → v1 단계적 진행.
   - v1alpha2 는 *실험적 단계* — 추가 변경 (NetworkPolicy/PSS toggle, HPA,
     Custom Modules) 누적 후 v1beta1 승격이 자연스러움.

## Refs

- ADR-0013 (supersede 대상): Auth.Enabled 사실상 항상 enabled.
- ADR-0026 (deferred 회복): Conversion Webhook 의 deferred 결정 재검토.
- Plan §2 D1 (사용자 결정 1: Auth optional, v1alpha2 + conversion webhook).
- Phase 1 비교: 외부 chart (Helm 기준) 의 `auth.enabled` 토글 패턴.
- 글로벌 표준: `standards/adr.md §3` (Nygard 5섹션).
- v1alpha2 type module 위치: `api/v1alpha2/`.
- doc.go: PR-A2.1 anatomy + PR-A2.2 후속 작업 안내.
