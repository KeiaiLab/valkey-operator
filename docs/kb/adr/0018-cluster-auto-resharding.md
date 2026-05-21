# ADR-0018: Cluster Auto-Resharding (SlotMigrationPolicy Auto 활성)

- Date: 2026-05-09
- Status: Accepted (PR-B8.1 — ADR 정식 작성, controller 구현은 PR-B8.2 후속)
- Authors: @eightynine01
- Refs: Plan §2 D7 (`~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-synthetic-gem.md`), ADR-0006 (ScalePolicy.Deliberate), ADR-0027 (HPA deferred)

## Context

`api/v1alpha1/valkeycluster_types.go` 가 `SlotMigrationPolicy` enum
(`Auto` / `Manual`) + `ValkeyClusterSpec.SlotMigration` 필드 (default
Auto) 를 *이미 정의*. `ClusterPhase` 가 `Resharding` 도 enum 에 포함.

그러나 *controller 가 SlotMigration=Auto 시 자동 slot 재분배 안 함* —
plans/ethereal-fluttering-wand 에서 ADR-0018 슬롯 *예약* 만 했고 실
구현 deferred. 본 ADR 은 *정식 결정* 보존 + controller 구현을 PR-B8.2
별 PR 로 분리.

ArtifactHub 비교 분석 (Plan §1 Phase 1) 결과 *외부 chart 도
auto-resharding 미지원* (수동 redis-cli MIGRATE 필요). 본 결정 implementation
시 valkey-operator 가 *4-repo 중 유일* 한 auto-resharding 보유 — 차별점.

## Decision

1. **`ValkeyClusterSpec.SlotMigration=Auto`** 의미 정식 보존:
   - default Auto — 사용자가 `spec.shards` 변경 시 operator 가 자동
     slot 재분배 (controller 구현 — PR-B8.2).
   - Manual — 사용자가 명시적으로 redis-cli MIGRATE 실행. operator 무영향.

2. **controller 구현 (PR-B8.2 후속)**:
   - `ClusterPhase=Resharding` 진입 — `spec.shards` 변경 감지 시.
   - 16384 slot 을 N batch (예: 256 slot/batch) 로 MIGRATE +
     `SETSLOT <slot> IMPORTING <node>` + ASKING redirect 처리.
   - `vk.ClusterMigrateSlots` helper 신규 (internal/valkey/cluster.go).
   - `Status.ReshardingProgress` 필드 신규 (current batch / 전체 batch) —
     본 PR-B8.1 외 (별 PR-B8.2 commit).
   - ScalePolicy.Deliberate=false 시 자동 trigger (ADR-0006 정합).

3. **회귀 위험 + 완화** (PR-B8.2 commit 의 검증 항목):
   - MIGRATE 중 client `MOVED` 응답 → ASKING redirect 자동 처리.
   - 데이터 손실 (CLUSTER FORGET 타이밍) → ADR-0005 graceful teardown 패턴.
   - e2e: 3-shard → 5-shard scale up + 16384 slot 무손실 (MGET 100%).

4. **HPA prerequisite**: ADR-0027 (HPA deferred) 가 본 ADR 활성을
   trigger 로 명시. PR-B8.2 머지 후 PR-C5 (HPA) 가 활성.

## Consequences

### Positive

- valkey-operator 가 *4-repo 중 유일한 auto-resharding* 보유 — Plan §1
  Phase 1 Gap F 해소. 외부 chart 와의 차별점.
- HPA (ADR-0027 deferred) 의 prerequisite 충족.
- 사용자 운영 부담 감소 — `kubectl scale valkeycluster --shards=5` 만으로
  완료.

### Negative

- controller 구현 비용 (PR-B8.2): 16384 slot batch + ASKING 처리 +
  retry 로직. T3 작업.
- MIGRATE 중 일시 client error — ASKING 처리로 완화하나 client lib 가
  *redirect 자동 처리* 의무. 운영 매뉴얼에 명시.

### Trade-offs

- *Auto default* (본 ADR) vs *Manual default* — 후자는 사용자 명시 의도.
  본 ADR 은 *secure-by-operator-managed* — ScalePolicy.Deliberate
  (ADR-0006) 와 정합 (auto failover + auto resharding).

## Alternatives Considered

1. **외부 redis-cluster-rebalance 도구 의존** — 거부.
   - operator 가 *single source* — 외부 도구 의존 시 운영 책임 분산.

2. **slot batch 미사용 (전체 한 번 MIGRATE)** — 거부.
   - 16384 slot 동시 migration 시 client error 폭주. batch 가 표준.

3. **Auto 제거 + Manual 만 지원** — 거부.
   - HPA (ADR-0027) prerequisite 부재. Plan §1 Phase 1 Gap F 미해소.

## Refs

- Plan §2 D7 (Sprint B PR-B8).
- ADR-0006 (ScalePolicy.Deliberate=false 기본값) — auto failover/resharding 결합.
- ADR-0027 (HPA deferred) — 본 ADR 활성을 trigger 로 명시.
- ADR-0005 (Graceful cluster teardown) — CLUSTER FORGET 패턴.
- 후속 PR-B8.2: controller `vk.ClusterMigrateSlots` + `reconcileResharding` phase + e2e 3→5 shard.
