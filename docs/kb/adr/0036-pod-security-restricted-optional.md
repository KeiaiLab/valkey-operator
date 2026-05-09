# ADR-0036: PodSecurity Restricted Optional Toggle (v1alpha2)

- Date: 2026-05-09
- Status: Accepted (PR-A3.2 type module — controller 분기는 PR-A3.2.2 후속)
- Authors: @eightynine01
- Refs: Plan §2 D3 (`~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-synthetic-gem.md`), ADR-0034 / ADR-0035 (v1alpha2 patterns)

## Context

ArtifactHub Helm 차트 비교 (Plan §1 Phase 1) 결과 외부 두 redis Helm
차트 (Bitnami v25.5.2, Cloudpirates v0.27.6) 모두 PodSecurityContext
*opt-in restricted* + 사용자 *override* 허용. valkey-operator v1alpha1
은 *PSA restricted 강제* — 외부 PSA policy (Kyverno / Gatekeeper /
custom admission controller) 사용자에게 *충돌* 가능.

사용자 결정 (Plan AskUserQuestion §1): Auth + NetworkPolicy + PSS *3종
모두 토글*, default=true 유지 (secure-by-default 보존).

## Decision

1. **`api/v1alpha2/common_types.go` 의 `PodSpec`** 에
   `PodSecurityRestricted *bool` 필드 신규. `omitempty` +
   `kubebuilder:default:=true`.

2. **동작 매핑**:
   - `nil` (legacy 호환): true 처리.
   - `true` (default): operator 가 restricted SecurityContext 강제 적용.
     `SecurityContext` / `ContainerSecurityContext` 사용자 정의는
     *enforced fields* (capabilities.drop=ALL, runAsNonRoot=true,
     readOnlyRootFilesystem=true) 외 영역만 적용 — v1alpha1 동작 동등.
   - `false`: 사용자 정의 `SecurityContext` / `ContainerSecurityContext`
     우선. 외부 PSA policy (Kyverno / Gatekeeper) 또는 K8s 1.25+ PSA
     label namespace 분리 시나리오 지원.

3. **PodSpec 통합 vs SecuritySpec 분리**:
   - 본 PR 은 *PodSpec 안에 통합* — 외과 수술적 변경 우선.
   - Plan §3 의 `SecuritySpec` 신규 type 분리는 *v1alpha3 또는 별 PR*
     (의미 분리 완성도 vs Spec 변경 비용 trade-off).

4. **PR 분할**:
   - **PR-A3.2** (본 ADR): v1alpha2 type module 에 필드 추가. controller
     미수정.
   - **PR-A3.2.2** (별도): controller 의 statefulset.go 분기
     (`PodSecurityRestricted=false` 시 사용자 SecurityContext 그대로
     적용). PR-A2.2 (v1alpha2 hub 전환) 의존.

## Consequences

### Positive

- 외부 PSA policy 사용자 (Kyverno / Gatekeeper / custom admission)
  시나리오 지원 — Bitnami / Cloudpirates 동등 유연성.
- default=true 유지로 v1alpha1 secure-by-default 보존.
- v1alpha1 사용자 무중단 (PodSecurityRestricted nil 시 true).

### Negative

- v1alpha2 PodSpec 표면 +1 필드. doc + chart values 갱신 의무 (PR-A3.2.2).
- false 시 *사용자 SecurityContext 책임* — 운영 복잡도 사용자 측.
  완화: doc + lint 권고 (Pod Security Admission label).

### Trade-offs

- *PodSpec 안에 통합* (본 ADR) vs *SecuritySpec 신규 type 분리* — 후자
  는 Plan §3 명시이지만 Spec 표면 +1 type + 호환성 고려. 본 ADR 의
  통합이 *외과 수술적*.

## Alternatives Considered

1. **SecuritySpec 신규 type + ValkeySpec 에 Security 필드 추가** — 거부
   (본 PR 한정).
   - Plan §3 명시 패턴이지만 Spec 표면 +1 type + Spec 필드 추가 → v1alpha2
     CRD 변화 큼. v1alpha3 또는 별 PR 로 분리.
   - PodSpec 통합이 *최소 변경*.

2. **`Enforced *bool` 단일 필드** — 거부.
   - "PodSecurityRestricted" 가 *PSA restricted profile 명시* — Kubernetes
     Pod Security Standards (PSS) 컨벤션 정합. "Enforced" 는 모호.

3. **PodSpec 그대로 + chart values 만 분기** — 거부.
   - chart values 만으로는 *operator behavior* 변경 불가 — controller
     코드 분기 필요. 본 PR 은 type 추가 (controller 분기는 PR-A3.2.2).

## Refs

- Plan §2 D3 (사용자 결정 1: PSS optional, default true).
- ADR-0034 (Auth Optional v1alpha2) / ADR-0035 (NetworkPolicy.AutoCreate)
  와 동일 패턴 — 3종 보안 토글 cross-PR.
- 외부 비교: Bitnami v25.5.2 / Cloudpirates v0.27.6 의 PodSecurityContext opt-in.
- K8s Pod Security Standards: <https://kubernetes.io/docs/concepts/security/pod-security-standards/>
- 후속 PR-A3.2.2: controller statefulset.go 분기.
