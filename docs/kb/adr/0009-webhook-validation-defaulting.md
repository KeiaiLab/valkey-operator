# ADR-0009: Validating + Mutating Webhook (supersedes ADR-0001)

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil
- Supersedes: ADR-0001 (operator-side defaulting)

## Context

ADR-0001 은 Alpha 단계 (M1) 에서 webhook 을 보류하고 operator-side `applyDefaults()`
+ CRD marker 조합으로 defaulting 을 처리하기로 했다. 트리거 조건:

- 사용자 수 > 1
- CR yaml 정합성 보장 요구
- 조합 검증 필요한 필드 ≥ 5개

iter 5 시점에 다음 *조합 검증* 시나리오가 누적됨:

1. `AutoFailover=true` + `ReplicasPerShard=0` 모순 (failover 불가능).
2. `Spec.TLS.Enabled=true` 시 `CertManager` 또는 `CustomCert` 둘 중 하나 필수.
3. `CertManager` + `CustomCert` 동시 명시 금지 (mutually exclusive).
4. `auth.users` 사용 시 `auth.enabled=true` 필수.
5. `Mode=Standalone` + `Replicas > 1` 모순.
6. `Mode=Replication` + `Replicas < 2` 모순.
7. `Storage.StorageClassName`, `Spec.Mode`, `TLS.Enabled` 등 immutable 필드 보호.
8. `Shards * (1 + ReplicasPerShard) > 100` 운영 한계.

→ 8건 → ADR-0001 의 trigger 충족. webhook 도입 정당화.

## Decision

`kubebuilder create webhook --defaulting --programmatic-validation` 으로 양 CRD 에
webhook scaffold 생성 + 위 8건 조합 검증 + immutable 가드 구현.

defaulting 은 *조건부 derived* 만 — 단순 zero → 상수 기본값은 CRD marker 가 처리.

## Consequences

**긍정:**
- admission 시점 보장 — `kubectl apply` 에서 즉시 reject (사용자 즉시 피드백).
- *Reconcile 진입 전* 모순 차단 → operator log noise 감소, status condition 폭주 회피.
- immutable 필드 가드 — `kubectl edit` 으로 우발적 데이터 손실 방지.

**부정:**
- cert-manager 의존성 (또는 self-signed cert + admission cert rotation 자동화).
- webhook 가용성이 cluster admission 의 critical path — 다운 시 모든 valkey CR 변경
  차단. `failurePolicy=Fail` 가 기본 (잘못된 안전망 보다 strict 한 fail).
- envtest 만으로는 webhook 동작 검증 어려움 — 단위테스트 + e2e Kind 이중 검증 필요.

## 후속 작업

- M5: webhook 자체 e2e 테스트 (Kind + cert-manager 설치 + admission reject 시나리오 검증).
- M6: `failurePolicy=Ignore` 옵션 검토 (webhook 다운 시에도 admission 허용 — 일부
  사용 케이스에서 가용성 우선).

## Alternatives Considered

- **operator-side validation 만 유지:** 사용자 피드백 지연 (Reconcile 진입 후 status
  condition 으로만 통지) → ADR-0001 단점 누적.
- **CEL validation rules (CRD marker 의 `+kubebuilder:validation:XValidation`):**
  가능. 단 Kubernetes 1.28+ 필수, 표현력 제한 (예: 다른 필드 immutable 체크는 가능
  하지만 CR 외부 자원 - Secret 의 ca.crt 키 존재 여부 - 검증 불가). 본 webhook 은
  내부 자원 검증도 미래에 통합 가능 (ADR-0008 의 CA bundle 유효성 검증 등).
- **OpenAPI schema 만 사용:** 조합 검증 불가능.
