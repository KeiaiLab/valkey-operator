# ADR-0027: HPA — Replication Mode 만 + Operator-managed (deferred)

- Date: 2026-05-06 (impl: 2026-05-10)
- Status: Accepted
- Authors: @phil, @eightynine01 (impl)
- Impl: deferred → done (ValkeySpec.Autoscaling + HPA reconcile + webhook)

## Context

Plan §3 Track F 후속 — Day-N₂ 진입 단계. *Horizontal Pod Autoscaler* 지원
검토.

설계 분기:

1. **Native HPA** (사용자 책임): 사용자가 HPA CRD 를 직접 작성. operator
   는 *target STS* 가 *autoscaler 영향* 받음을 인식해 *Spec.Replicas
   override* 를 회피해야 함.

2. **Operator-managed HPA**: ValkeySpec.Autoscaling 필드 → reconcile 가
   HPA CRD 자동 생성. 사용자는 minReplicas/maxReplicas/CPU target 만 명시.

추가 분기:
- **Replication mode 만 지원** vs **ValkeyCluster shard scale 도 지원**.
  ValkeyCluster 는 *slot 재분배* (MIGRATE/ASKING) 동반 — Track B Resharding
  미구현 시 데이터 손실 위험.

## Decision

**옵션 2 채택 — Operator-managed HPA**, **Replication mode 만 지원**.
*deferred* — 본 ADR 은 결정만 기록, 실제 구현은 별개 cycle.

### Spec 디자인 (예상):

```yaml
spec:
  mode: Replication
  replicas: 3                       # default (Autoscaling 미지정 시)
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70
    behavior:                       # K8s HPA v2 표준 — optional
      scaleDown:
        stabilizationWindowSeconds: 300
```

### Reconcile 통합:

1. Spec.Autoscaling.Enabled=true → operator 가 HPA CRD 자동 생성 (target =
   STS).
2. ScalePolicy.Deliberate 와 *상호배타* — Autoscaling 활성 시 Deliberate
   무시 (HPA 가 즉시 scale).
3. Spec.Replicas 는 *default 값* — HPA 활성 시 무시 (HPA 결정).

### 제약:

- **ValkeyCluster mode 미지원** — Track B Resharding 의존. shard 수 변경 =
  16384 slot 재분배. autoscaling 으로 *자동 변경* 위험.
- **Standalone mode 미지원** — replicas=1 강제. autoscaling 의미 없음.

## Consequences

긍정:
- **사용자 진입 장벽 낮음** — HPA YAML 작성 + STS 매핑 직접 안 함.
- **operator 의 Status.PendingScale 와 통합** — Autoscaling 활성 시
  PendingScale 부재 (즉시 적용).
- **failover 와 호환** — primary NotReady → reconcileFailover (cycle 7).
  HPA 의 scale 결정은 STS 의 readyReplicas 기준 — failover 후 새 primary
  반영.

부정:
- **추가 reconcile 분기** — HPA 생성/갱신/삭제 로직.
- **CRD 의존성 추가** — autoscaling/v2 (HPA v2 표준) — kubectl 1.23+ 강제.
  현재 kubectl 1.34+ 요구사항 (README L23) 호환.
- **PodDisruptionBudget 와 상호작용** — 사용자가 PDB.minAvailable 와
  Autoscaling.minReplicas 를 일관 설정해야. 별개 webhook validation 필요.

## Alternatives Considered

1. **Native HPA** (옵션 1 거절):
   - 사용자가 HPA YAML 작성 + STS 이름 / namespace 직접 매핑.
   - operator 의 *Spec.Replicas reconcile 와 충돌* — 사용자가 ScalePolicy.
     Deliberate=true 로 항상 설정해야 (UX 부담).
   - 거절: operator 가 *통합 진입점* 역할 못 함.

2. **VPA (Vertical Pod Autoscaler)**: VPA 는 *Spec.Resources* 변경 동반 —
   Pod 재시작 필요. valkey 의 데이터 plane 영향 큼. 본 ADR 범위 외.

3. **KEDA (event-driven)**: 외부 event source 기반 scale. valkey 의 *내장
   metric* (memory / connections) 만으로 충분 — KEDA 의존성 추가 불필요.
   거절.

## Action Items (deferred)

본 ADR 은 *deferred* — Action Items 는 *Track B Resharding 완료 후* 작성:

- [ ] Trigger: Track B Scale apply + Resharding 완료 (Replication mode
      Spec.Replicas 변경 자동 적용 검증).
- [ ] AI-001: ValkeySpec.Autoscaling 필드 정의 (HPA v2 표준 호환).
- [ ] AI-002: HPA reconcile 통합 — Autoscaling.Enabled=true 시 HPA CRD
      자동 생성/갱신/삭제 (owner-ref).
- [ ] AI-003: ScalePolicy 와 상호배타 webhook validation.
- [ ] AI-004: Spec.PodDisruptionBudget 와의 일관성 webhook warning.
- [ ] AI-005: 단위 테스트 + e2e (kind cluster 의 HPA controller 통합).
- [ ] AI-006: README 운영 가이드 — Autoscaling 활성화 + Behavior 튜닝.

## 관련 결정

- **ADR-0006**: ScalePolicy.Deliberate=false default. HPA 와 상호배타.
- **Track B Resharding** (Plan §3): ValkeyCluster shard scale 의 *사전조건*.
- **PDB**: Spec.PodDisruptionBudget 와 Autoscaling.minReplicas 일관성.

Refs: Plan §3 Track F (Day-N₂ 진입), HANDOFF.md cycle 15 §6,
ADR-0006 (Scale Policy), ADR-0026 (Conversion Webhook deferred).
