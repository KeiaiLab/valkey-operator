# ADR-0039: ValkeyCluster post-init self-heal (INC-0001 영구 fix)

- Date: 2026-05-10
- Status: Accepted
- Authors: @eightynine01

## Context

INC-0001 (2026-05-09 19h cluster fail) 의 root cause: controller 의 cluster
bootstrap 분기 (`internal/controller/valkeycluster_controller.go:227`) 가
`!ClusterInitialized` 만 검증하여 *post-init cluster fail* 상태에서 자가치유
불가. Pod 재시작 후 nodes.conf 의 myself IP stale → gossip fail → state=fail.
controller 는 ClusterInitialized=true 만 보고 bootstrap skip → 19h stuck.

수동 수습 (INC-0001):
1. PVC wipe (data + nodes.conf 삭제) + pods 재시작.
2. `kubectl patch --subresource=status` 로 ClusterInitialized=false 강제.
3. controller 가 즉시 ensureClusterMeet 재실행 → 16384 slots OK 회복.

본 ADR 은 *동일 시나리오에서 controller 자가치유* 가 가능하도록 코드 fix.

## Decision

bootstrap 분기 조건 변경:

```go
// (기존)
allReady := stsObj.readyReplicas == totalReplicas && totalReplicas > 0
if allReady && !vc.Status.ClusterInitialized {
    ensureClusterMeet(...)
    vc.Status.ClusterInitialized = true
}

// (신규)
allReady := ...
shouldBootstrap := allReady && !vc.Status.ClusterInitialized
if allReady && vc.Status.ClusterInitialized {
    preInfo, _, _ := pollClusterState(...)
    if preInfo != nil && (preInfo.State != "ok" || preInfo.SlotsAssigned != 16384) {
        shouldBootstrap = true  // INC-0001 self-heal
    }
}
if shouldBootstrap {
    ensureClusterMeet(...)
    vc.Status.ClusterInitialized = true
}
```

핵심 보장:
1. **첫 bootstrap**: 기존 동작 그대로 (`!ClusterInitialized` 분기).
2. **post-init fail**: `ClusterInitialized=true` 라도 cluster_state != ok 또는
   slots != 16384 면 ensureClusterMeet 재호출.
3. **무한 호출 방지**: `ensureClusterMeet` 의 사전 가드 (`queryAnyNode → state=ok`
   && slots=16384 시 skip) 가 *recovered cluster* 에서는 즉시 return → 무한 loop
   불가능.
4. **partial recovery**: CLUSTER MEET 은 멱등, ADDSLOTS 은 *이미 owner 있는 slot
   은 거부* 되지만 *unassigned slot 은 새로 assign* — partial recovery + 다음
   reconcile 에 수렴.

## Consequences

긍정:
- INC-0001 시나리오 자동 회복: pod 재시작 후 nodes.conf stale → 자동 re-bootstrap.
- 운영자 manual intervention 불필요 (현 INC-0001 이 19h stuck 한 시간 → 0).
- ensureClusterMeet 의 멱등성 보장이 무한 loop 방지.

부정:
- post-init reconcile 마다 *추가 pollClusterState* 호출 — `queryAnyNode` 1회 (모든
  reconcile 의 ~50ms latency 추가). 단 reconcile 빈도 (RequeueAfter 30s+) 고려
  시 mongos / API server 부하 무시 가능.
- partial recovery 시도 후에도 fail 지속 가능 — 그 경우 PrometheusRule alert
  (AI-0002) 가 운영자 호출.

## Alternatives Considered

1. **status.clusterInitialized flag 자체 제거**: 거절 사유. 멱등성 가드 역할 +
   첫 init detection 신호. 제거 시 매 reconcile 마다 ensureClusterMeet 호출 →
   불필요 부하.
2. **알람만 발행 + 자동 fix 안 함**: 거절 사유. 운영자 24/7 대응 부담. INC-0001
   이 19h stuck 의 *시간 손실 자체* 가 비용.
3. **CLUSTER RESET HARD 자동 호출**: 거절 사유. master with keys 거부 (INC-0001
   에서 검증). 데이터 손실 위험. ensureClusterMeet 의 partial recovery 로 충분.

## References

- INC-0001: `docs/kb/incident/INC-0001-cluster-fail-bootstrap-skip.md`.
- 코드 변경: `internal/controller/valkeycluster_controller.go:226-251`.
- 관련 함수: `ensureClusterMeet` (line 569), `pollClusterState` (line 601).
- 후속 (Action Items): AI-0002 PrometheusRule alert, AI-0003 e2e regression
  test (pod IP 변경 시나리오), AI-0004 runbook.
