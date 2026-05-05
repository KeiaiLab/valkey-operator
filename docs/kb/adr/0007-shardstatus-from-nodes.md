# ADR-0007: ShardStatus from CLUSTER NODES (supersedes ADR-0004)

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil
- Supersedes: ADR-0004

## Context

ADR-0004 는 Alpha 단계에서 `Status.Shards` 를 *Spec 기반* 으로 빌드하기로 했다. 그러나
이는 failover 시점 거짓말 — replica 가 primary 로 승격되어도 status 가 옛 primary 를
보고. 사용자가 `kubectl get valkeycluster -o yaml` 로 진단 시 혼란.

iter 4 에서 cluster 도메인 통합 테스트 (`TestIntegration_NodesTopology`) 가 동작
가능해지면서 NODES 기반 빌드를 *실제 환경* 에서 검증할 수 있게 됨.

## Decision

`buildShardStatusFromNodes(nodes []vk.NodeView) []ShardStatus` 채택. Reconcile 에서
`pollClusterState` 가 (info, nodes, err) 반환 → nodes 가 있으면 NODES 기반, 없으면
spec 기반 fallback.

매핑 규칙:
- 각 master flag 노드 → ShardStatus 1건.
- replica 노드의 MasterID 로 primary 매핑 → ReplicaPods.
- SlotRange 는 NodeView.Slots 를 "low-high[,low-high]" 형식으로 직렬화.
- Index 는 slot 시작값 기준 정렬 (안정적 ordering).

## Consequences

**긍정:**
- failover 정확 반영 — `TestBuildShardStatusFromNodes_afterFailover` 단위테스트로 검증.
- 실제 cluster 토폴로지가 status 에 그대로 — 운영자가 status 만 보고 정확한 진단 가능.

**부정:**
- NODES 응답이 일시 실패 시 fallback 으로 *부정확한* spec 기반 status — 명확한 표시
  (`Status.ClusterState` 가 ok 인데 Shards 가 spec 기반) 가 없음. M3 에서
  `Status.ShardsSource` (NODES | Spec) 필드 추가 검토.

## 후속 작업

- ShardStatus 의 PrimaryPod / ReplicaPods 가 현재 "ip:port" — 사용자 친숙성 위해 pod
  ordinal 이름 (vk-0) 으로 매핑 검토 (DNS 역조회 또는 Pod 라벨 매칭).
- 통합 테스트 추가: 강제 failover 후 ShardStatus 가 반영되는지.

## Alternatives Considered

- **두 source 병기 (Status.SpecShards + Status.ObservedShards):** API surface 비대화.
- **CLUSTER SHARDS (Redis 7+) 사용:** 더 풍부한 정보 (slots, replication-offset) 제공.
  현 PR 에서는 NODES 응답 파서가 이미 충분.
