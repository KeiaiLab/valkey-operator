/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package valkey

// 결함 ⑤ — partial-slot outage 감지 (순수 로직).
//
// 증상 (실 incident): node churn 후 cluster_state:ok, cluster_slots_assigned:16384
// 이지만 cluster_slots_ok:10922 — 한 shard(약 5462 slot)가 `fail` flag 노드 소유라
// 키스페이스의 1/3 이 실제로 DOWN. 기존 health gate 는 cluster_state / slots_assigned
// 만 검사해 이 상태를 "정상"으로 오판하고 self-heal 이 동작하지 않았다.
//
// 본 파일은 CLUSTER INFO + CLUSTER NODES 스냅샷만으로 (1) cluster 가 degraded
// (heal 필요) 인지, (2) 어느 fail master 의 slot 이 stuck 이고 어느 healthy replica 가
// takeover 대상인지를 판정한다 — gossip / 실 명령 없이 단위테스트 가능한 순수 함수.

// ClusterTotalSlots — Valkey cluster 의 전체 slot 수 (도메인 [0,16383]).
const ClusterTotalSlots = 16384

// StuckSlotHeal — partial-slot outage 한 건의 heal 계획.
//
// FailedMaster: slot 을 여전히 소유한 채 `fail` flag 가 붙은 primary.
// TakeoverReplica: 해당 master 를 따르는 healthy replica 중 takeover 대상.
// 둘 다 NodeView 의 *복사본* — caller 는 Addr / ID 로 명령을 발행한다.
type StuckSlotHeal struct {
	FailedMasterID    string
	FailedMasterAddr  string
	TakeoverReplicaID string
	// TakeoverReplicaAddr — CLUSTER FAILOVER TAKEOVER 를 발행할 replica 주소.
	TakeoverReplicaAddr string
}

// IsClusterDegraded — health gate 강화 (결함 ⑤). 다음 중 하나라도 참이면 degraded.
//
//  1. info == nil — 상태 미상 (호출측에서 별도 처리하지만 보수적으로 false).
//  2. cluster_state != ok.
//  3. cluster_slots_ok < 16384 — 일부 slot 이 ok 가 아님 (pfail/fail 소유).
//  4. `fail` flag 가 붙은 master 가 여전히 slot 을 소유.
//
// 기존 게이트는 (2) 와 slots_assigned 만 봤다. (3)/(4) 가 본 결함의 핵심 — assigned
// 는 16384 인데 ok 가 16384 미만인 partial outage 를 잡는다.
func IsClusterDegraded(info *ClusterInfo, nodes []NodeView) bool {
	if info == nil {
		return false
	}
	if info.State != "ok" {
		return true
	}
	if info.SlotsOK < ClusterTotalSlots {
		return true
	}
	for i := range nodes {
		if nodeOwnsSlotsWhileFailed(&nodes[i]) {
			return true
		}
	}
	return false
}

// nodeOwnsSlotsWhileFailed — `master` + `fail` flag 인데 slot 을 여전히 소유.
//
// Valkey 는 master 가 fail 판정돼도 replica failover 가 일어나기 전까지 slot 소유권을
// 유지한다. takeover 가 일어나지 않으면 이 상태가 영구히 stuck — 그 slot 은 DOWN.
func nodeOwnsSlotsWhileFailed(n *NodeView) bool {
	return n.IsPrimary() && n.Flags["fail"] && len(n.Slots) > 0
}

// DetectStuckSlotHeals — slot 을 stuck 시킨 fail master 마다 takeover 가능한 healthy
// replica 를 1개 선정해 heal 계획을 만든다. 순수 함수.
//
// 선정 규칙:
//   - master 가 `master` + `fail` flag 이고 slot 을 소유.
//   - 그 master 를 따르는 replica 중 *healthy* (fail/pfail flag 없음 && link connected)
//     한 것을 takeover 대상으로. ID 사전순으로 안정 선택 (idempotent / 결정적).
//   - healthy replica 가 없는 master 는 takeover 불가 → 계획에서 제외 (데이터 보존
//     우선, 호출측이 별도 알림). stuck 이지만 promote 할 안전한 후보가 없는 케이스.
//
// 반환은 FailedMasterID 사전순 정렬 — 결정적 출력.
func DetectStuckSlotHeals(nodes []NodeView) []StuckSlotHeal {
	// master id → replica NodeView 목록.
	replicasByMaster := make(map[string][]*NodeView)
	for i := range nodes {
		n := &nodes[i]
		if n.IsReplica() && n.MasterID != "" && n.MasterID != "-" {
			replicasByMaster[n.MasterID] = append(replicasByMaster[n.MasterID], n)
		}
	}

	var heals []StuckSlotHeal
	for i := range nodes {
		m := &nodes[i]
		if !nodeOwnsSlotsWhileFailed(m) {
			continue
		}
		repl := pickHealthyReplica(replicasByMaster[m.ID])
		if repl == nil {
			// stuck 이지만 안전한 takeover 후보 없음 — 제외.
			continue
		}
		heals = append(heals, StuckSlotHeal{
			FailedMasterID:      m.ID,
			FailedMasterAddr:    m.Addr,
			TakeoverReplicaID:   repl.ID,
			TakeoverReplicaAddr: repl.Addr,
		})
	}
	sortHealsByMasterID(heals)
	return heals
}

// pickHealthyReplica — fail/pfail flag 가 없고 link 가 connected 인 replica 중
// ID 사전순 최솟값. 없으면 nil.
func pickHealthyReplica(replicas []*NodeView) *NodeView {
	var best *NodeView
	for _, r := range replicas {
		if r.Flags["fail"] || r.Flags["fail?"] || r.Flags["pfail"] {
			continue
		}
		if !r.LinkOK {
			continue
		}
		if best == nil || r.ID < best.ID {
			best = r
		}
	}
	return best
}

// sortHealsByMasterID — 결정적 출력을 위한 단순 삽입정렬 (heal 건수는 shard 수 — 작다).
func sortHealsByMasterID(h []StuckSlotHeal) {
	for i := 1; i < len(h); i++ {
		for j := i; j > 0 && h[j].FailedMasterID < h[j-1].FailedMasterID; j-- {
			h[j], h[j-1] = h[j-1], h[j]
		}
	}
}
