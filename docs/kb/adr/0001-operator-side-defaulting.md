# ADR-0001: Operator-side defaulting (vs admission webhook)

- Date: 2026-05-05
- Status: Superseded by ADR-0009
- Authors: @phil

## Context

Valkey / ValkeyCluster CR 의 누락 필드 (`Mode`, `Replicas`, `Shards`,
`ReplicasPerShard`, `NodeTimeoutMillis`, `Version.Version`, `Version.Image`)
는 기본값이 필요하다. Kubebuilder 는 두 가지 defaulting 메커니즘을 지원한다:

1. **CRD `+kubebuilder:default=` marker** — apiserver 가 Create/Update 시점 적용.
2. **Mutating admission webhook** — 모든 변경 시점에 controller-manager 가 자동 적용.
3. **Operator-side `applyDefaults()`** — Reconcile 안에서 in-memory 보정 (영속화 안 됨).

## Decision

**현재 단계 (M1, Alpha):** CRD marker + operator-side `applyDefaults()` 조합 채택.
admission webhook 은 도입하지 않는다.

근거:
- CRD marker 는 단순 기본값에 충분 (`Shards=3` 등 이미 적용됨).
- Webhook 은 cert-manager 통합 + Service / ValidatingWebhookConfig / MutatingWebhookConfig
  배포가 추가 필요 — 단일 작업자 (RFC 0001 환경) 에서 *조작 변수 1개* 늘리는 부담.
- `applyDefaults()` 는 Reconcile 내부 안전망으로 동작 — CRD marker 가 미적용된 기존
  CR (예: 다른 controller-gen 버전으로 만든) 도 reconcile 시 보정됨.

## Consequences

**긍정:**
- 인프라 의존성 0 — Kind 클러스터 간단 설치 가능.
- Reconcile 단일 진입점 보장 — admission 시점 추가 표면 없음.

**부정:**
- API 클라이언트가 CR 을 *조회* 했을 때 보정 전 값을 본다 (in-memory only). status 갱신
  시점에서 보정값이 영속화되지 않음 — 사용자가 `kubectl get valkeycluster -o yaml` 로
  보면 `shards: 0` 같은 누락이 그대로 보일 수 있음.
- ValidatingWebhook 부재 — 잘못된 조합 (예: `replicasPerShard: 0` + `autoFailover: true`)
  은 reconcile 단계에서야 에러 처리.

**후속 작업 (M3, Beta):**
- Mutating webhook 추가 시 본 ADR 을 Superseded 처리.
- Webhook 도입 트리거: ① 사용자 수 > 1, ② CR yaml 정합성 보장 요구, ③ 조합 검증 필요한
  필드 ≥ 5개 이상.

## Alternatives Considered

- **CRD marker 만 사용**: 새 필드 추가 시 마이그레이션 부담 + 기존 CR 호환성 미보장.
- **Webhook 만 사용**: Alpha 단계에서 운영 부담 과다.
- **두 메커니즘 모두 즉시 도입**: Surgical Changes 위반 + ROI 낮음.
