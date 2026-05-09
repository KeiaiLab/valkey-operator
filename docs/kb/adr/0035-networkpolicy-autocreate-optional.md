# ADR-0035: NetworkPolicy.AutoCreate Optional Toggle (v1alpha2)

- Date: 2026-05-09
- Status: Accepted (PR-A3.1 type module — controller 분기는 PR-A3.1.2 후속)
- Authors: @eightynine01
- Refs: Plan §2 D2 (`~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-synthetic-gem.md`), ADR-0057 (NetworkPolicy 자동생성 강제 — 본 ADR 이 의미 분리)

## Context

ArtifactHub Helm 차트 비교 분석 (Plan §1 Phase 1) 결과 외부 두 redis
Helm 차트 (Bitnami v25.5.2, Cloudpirates v0.27.6) 모두 NetworkPolicy
*opt-in* + cluster-wide policy engine (Calico / Cilium / Antrea) 와의
*공존* 패턴 채택. valkey-operator 의 ADR-0057 은 *operator 자동 생성
강제* — 외부 NP 관리 사용자에게 *충돌* 가능.

사용자 결정 (Plan AskUserQuestion §1): *Auth + NetworkPolicy + PSS 3종
모두 토글*, default=true 유지 (secure-by-default 보존), v1alpha2 신규
+ conversion webhook (사용자 결정 §4).

## Decision

1. **`api/v1alpha2/common_types.go` 의 `NetworkPolicySpec`** 에
   `AutoCreate *bool` 필드 신규. `omitempty` + `kubebuilder:default:=true`.
   - **Enabled 와 의미 분리**: Enabled = *정책 사용 여부* (사용자 의도);
     AutoCreate = *operator 가 NP 리소스 생성/갱신/삭제 책임 가질지*.
   - default=true 유지로 v1alpha1 ADR-0057 동작 동등 (secure-by-default).

2. **사용자 시나리오**:
   - `AutoCreate=true` (default): operator 가 NP 리소스 생성/관리.
     v1alpha1 ADR-0057 동등.
   - `AutoCreate=false` + `Enabled=true`: 사용자가 *외부 NP 관리 책임*.
     예: Calico GlobalNetworkPolicy / Cilium ClusterwideNetworkPolicy /
     Antrea ClusterNetworkPolicy 사용 시나리오.
   - `Enabled=false`: NetworkPolicy 미사용. AutoCreate 무관.

3. **PR 분할**:
   - **PR-A3.1** (본 ADR — 본 PR): v1alpha2 type module 에 AutoCreate
     필드 추가. controller 미수정.
   - **PR-A3.1.2** (별도): controller 의 reconcile 분기 (`AutoCreate=false`
     시 NP 빌드 스킵). PR-A2.2 (v1alpha2 hub 전환) 후 활성.

## Consequences

### Positive

- 외부 NP 관리 사용자 (Calico / Cilium / Antrea) 시나리오 지원 — Bitnami
  / Cloudpirates 와 동등 유연성.
- default=true 유지로 ADR-0057 의 *secure-by-default* 철학 보존.
- v1alpha1 사용자 무중단 (AutoCreate nil 시 true 처리).

### Negative

- v1alpha2 Spec 표면 +1 필드. doc + chart values 갱신 의무 (PR-A3.1.2).
- AutoCreate=false + Enabled=true 시 사용자 *외부 NP 관리 책임* — 운영
  복잡도 사용자 측. 완화: doc 명시.

### Trade-offs

- *Enabled 와 의미 분리* (본 ADR) vs *Enabled 단일 토글* — 후자는
  Bitnami 패턴이지만 *운영 책임* 모호. 본 ADR 의 분리가 *명시 의미*.

## Alternatives Considered

1. **Enabled 만 사용 (AutoCreate 미추가)** — 거부.
   - Enabled=false 가 *NP 미사용* + *operator NP 미관리* 두 의미 혼재.
     사용자가 *외부 NP 관리 시 Enabled 어떻게 설정?* 모호.

2. **AutoCreate 를 Enabled 로 통합 + ManagementSource enum** — 거부.
   - `ManagementSource: operator | external | none` 등 enum 도 가능하나
     기존 `Enabled` 필드와 혼합 시 호환성 깨짐.

3. **PR-A3.1 + PR-A3.1.2 통합** — 거부.
   - controller 분기는 PR-A2.2 (v1alpha2 hub 전환) 의존 — 분리가 자연.

## Refs

- Plan §2 D2 (사용자 결정 1: NetworkPolicy optional, default true).
- ADR-0057 (NetworkPolicy 자동생성 강제 — 본 ADR 이 의미 분리, supersede
  아님 단 *opt-out path* 추가).
- ADR-0034 (Auth Optional v1alpha2 — 동일 패턴 cross-PR).
- 외부 비교: Bitnami v25.5.2 / Cloudpirates v0.27.6 의 NetworkPolicy opt-in.
- 후속 PR-A3.1.2: controller reconcile 분기.
