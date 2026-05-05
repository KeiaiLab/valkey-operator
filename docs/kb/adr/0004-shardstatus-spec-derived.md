# ADR-0004: ShardStatus derived from Spec (not CLUSTER NODES)

- Date: 2026-05-05
- Status: Superseded by ADR-0007
- Authors: @phil

## Context

`Status.Shards[]` 는 각 shard 의 primary pod / replica pods / slot range 를 보고한다.
구현 옵션:

1. **Spec 기반 (현재):** `buildShardStatus(vc)` — pod ordinal 매핑 규칙 + 균등 slot
   분배 알고리즘으로 계산. *예상* 토폴로지를 보고.
2. **CLUSTER NODES 기반:** primary 노드에 접속해 `CLUSTER NODES` 응답 파싱 — *실제*
   토폴로지를 보고.

## Decision

**Alpha (M1):** Spec 기반 채택. CLUSTER NODES 통합은 M2 (Beta) 에서.

근거:
- 초기 부트스트랩 시점에는 spec 과 실제가 일치 (멱등성 검증된 `CreateCluster` 호출 후).
- CLUSTER NODES 파싱은 추가 코드 + 테스트 ~150 줄 (per-line: id ip:port@bus flags
  master ping pong epoch link slots) — Alpha 범위 초과.
- envtest 만으로는 검증 불가 → 실 Valkey 컨테이너 통합 테스트 동반 필요.

## Consequences

**긍정:**
- 외부 의존성 0 — `make test` 만으로 100% 검증.
- 단위테스트 (`TestBuildShardStatus_*`) 로 알고리즘 회귀 차단.

**부정 (중요):**
- **failover 시 status 거짓말**: replica → primary 자동 승격이 일어나면
  `Status.Shards[i].PrimaryPod` 가 *과거 primary* 를 보고 → 사용자 / 운영자 혼란.
- **slot migration 미반영**: SlotMigration 정책에 따른 reshard 진행 중에도 spec 기반
  range 가 그대로 보고됨.

## 후속 작업

- Trigger: ① 사용자가 failover 시나리오 신뢰 보고 요청, 또는 ② SlotMigration 정책
  자동화 도입.
- 작업: `buildShardStatus` 를 `pollShardTopology` 로 교체 — CLUSTER NODES 응답 기반.
- 회귀 차단: 본 ADR 의 단위테스트 들은 *bootstrap 직후 시나리오* 한정으로 retain
  (Spec=실제 일 때 정합성).

## Alternatives Considered

- **두 source 모두 보고 (`Status.SpecShards` + `Status.ObservedShards`):** Status 비대화
  + 클라이언트 혼란.
- **CLUSTER NODES 부터 시작:** Alpha 단계 over-engineering, envtest 검증 불가.
