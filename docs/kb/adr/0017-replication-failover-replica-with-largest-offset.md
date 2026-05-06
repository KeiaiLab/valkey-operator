# ADR-0017: Replication Mode Failover — Replica with Largest master_repl_offset

- Date: 2026-05-06
- Status: Accepted
- Authors: @phil

## Context

Plan §3 Track B (Failover) 의 사전조건. Valkey Replication mode 에서 primary
pod 가 NotReady 가 되어도 *operator 는 자동 failover 수행 안 함* — 현재
ValkeyController 가 항상 pod-0 을 primary 로 hardcode (`Status.CurrentPrimary
= <name>-0`).

요구사항:
- primary pod NotReady 30s+ 감지 → 자동 failover
- 데이터 손실 최소화 (가장 latest replica 선출)
- replication 분기 안정성 (split-brain 회피)
- 기존 reconcile loop 와 충돌 없음

ValkeyCluster mode 는 *valkey native cluster mode* 의 cluster_replica_validity_factor
+ Spec.AutoFailover 로 cluster bus 가 자동 failover 처리 — 본 ADR 범위 외.

## Decision

**Replica 의 master_repl_offset 가장 큰 선출** + REPLICAOF NO ONE 발행.

알고리즘:
1. primary pod (Status.CurrentPrimary) Ready 상태 30s+ NotReady 감지.
2. 모든 replica pod 의 INFO replication 호출 → master_repl_offset 추출.
3. 가장 큰 offset 의 replica 선출 (tie 시 ordinal 작은 것).
4. 선출된 replica 에 REPLICAOF NO ONE 발행 → 새 primary.
5. 나머지 replica 들에 REPLICAOF <new-primary-host> <port> 발행.
6. Status.CurrentPrimary = 새 pod 이름.

## Consequences

긍정:
- **데이터 손실 최소화** — replication offset 가장 latest replica 가 *가장 적은*
  데이터 손실 (master 의 마지막 commit 시점에 가장 가까움).
- **단순 알고리즘** — Sentinel 의 quorum / odd-replica 요구사항 회피.
  3 replicas 환경에서도 동작.
- **기존 ensureReplication 와 호환** — failover 후 새 primary 기준으로
  ensureReplication 가 다음 reconcile 에 자동 동기화.
- **Standalone (replicas=1) 에는 N/A** — replica 0 → failover 불가능.
  PVC 손실 = 데이터 손실, restore 만 가능.

부정:
- **Split-brain 위험** — operator 가 primary 와 replica 를 *동시에* 보지
  못한 시점 (network partition) 에 failover 발행 시 두 primary 발생.
  *완화*: NotReady 30s+ 임계값 + operator 단일 인스턴스 (leader-elect).
  Stronger 보장은 별개 (Sentinel/Raft 추후).
- **Replication offset 신뢰성** — replica 가 primary 와 lag 큰 경우 offset
  비교가 *대략적*. 그러나 latest 라는 사실은 변함.
- **Quorum 미지원** — 3 replicas 중 1 pod 만 살아있어도 failover. 이는
  *availability 우선* 디자인. Stronger 일관성 요구 시 별개 정책.

## Alternatives Considered

1. **Sentinel 스타일 quorum** (옵션 b 거절):
   - quorum > N/2+1 replicas 합의 필요 → 복잡 + 5+ replica 환경 강제.
   - 거절: 본 프로젝트의 *3 replicas* 일반 환경에 부적합.

2. **Manual only (Spec.AutoFailover=false)** (옵션 c 부분 채택):
   - 사용자 명시 트리거만 failover. CR annotation 또는 별도 명령.
   - 본 ADR 의 *default 동작* 외 *fallback 옵션* 으로 채택.
   - Spec.AutoFailover (CRD field) — 추가 ADR 또는 본 ADR 의 Action Items.

3. **Primary pod-0 hardcode + 사용자 수동 STS scale** (현재 동작 유지):
   - Failover 미지원. 사용자가 STS replicas 변경으로 primary 재배치.
   - 거절: 자동 복구 부재 = production 부적합.

## Action Items

- [ ] AI-001: ValkeyController 의 `determinePrimary()` helper 추출 (pod-0
      hardcode → Status.CurrentPrimary 우선 + Ready 검증 fallback). 기존
      동작 보존.
- [ ] AI-002: Spec.AutoFailover *bool* 필드 추가 (default true). false 시
      automatic failover 비활성.
- [ ] AI-003: `reconcileFailover()` — 새 phase 또는 reconcile loop 분기.
      INFO replication 호출 + offset 비교 + REPLICAOF NO ONE.
- [ ] AI-004: 단위 테스트 — fake redis 클라이언트 + offset 시뮬레이션.
- [ ] AI-005: e2e — kind cluster 에서 primary pod kill → failover 검증.
- [ ] AI-006: README 운영 시나리오 표 갱신 — failover 검증 결과.

## 예외 / 한계

- **Split-brain 강력 보장**: 본 ADR 은 *availability 우선*. 강력 일관성은
  별개 ADR (Sentinel 또는 Raft 도입) 추후.
- **ValkeyCluster mode**: cluster bus 의 native auto-failover 활용. 본
  ADR 범위 외 — Spec.AutoFailover (ValkeyCluster) 는 valkey configmap
  의 cluster-replica-validity-factor 로 매핑.
- **Disaster Recovery 시나리오 (전체 cluster 손실)**: failover 가 아닌
  ValkeyRestore 영역.

Refs: Plan §3 Track B, ADR-0015 (Restore — 동일 Replication mode 검증
패턴).
