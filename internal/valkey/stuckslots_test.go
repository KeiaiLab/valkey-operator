/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package valkey

import "testing"

// 결함 ⑤ — partial-slot outage 감지 *순수 로직* 단위테스트.
// 실 Valkey gossip 없이, CLUSTER INFO/NODES 스냅샷만으로 (1) degraded 판정,
// (2) takeover 대상 선정 알고리즘의 회귀를 차단한다.

// 실 incident snapshot: cluster_state:ok, slots_assigned:16384, slots_ok:10922.
// ccc shard 의 master 가 fail flag 인 채 slot [10922-16383] 을 여전히 소유.
const incidentNodes = `aaa11111111111111111111111111111111111111 10.0.0.1:6379@16379 myself,master - 0 0 1 connected 0-5460
bbb22222222222222222222222222222222222222 10.0.0.2:6379@16379 master - 0 1700000000000 2 connected 5461-10921
ccc33333333333333333333333333333333333333 10.0.0.3:6379@16379 master,fail - 0 1700000000000 3 connected 10922-16383
ddd44444444444444444444444444444444444444 10.0.0.4:6379@16379 slave aaa11111111111111111111111111111111111111 0 1700000000000 1 connected
eee55555555555555555555555555555555555555 10.0.0.5:6379@16379 slave bbb22222222222222222222222222222222222222 0 1700000000000 2 connected
fff66666666666666666666666666666666666666 10.0.0.6:6379@16379 slave ccc33333333333333333333333333333333333333 0 1700000000000 3 connected
`

func TestIsClusterDegraded_partialSlotOutage(t *testing.T) {
	// state=ok, assigned=16384 이지만 slots_ok<16384 → degraded (결함 ⑤ 핵심).
	info := &ClusterInfo{State: "ok", SlotsAssigned: 16384, SlotsOK: 10922}
	nodes := parseClusterNodes(incidentNodes)
	if !IsClusterDegraded(info, nodes) {
		t.Fatalf("partial-slot outage must be degraded (slots_ok=%d)", info.SlotsOK)
	}
}

func TestIsClusterDegraded_failMasterOwnsSlots_evenIfSlotsOKFull(t *testing.T) {
	// slots_ok 가 16384 로 보고돼도 fail master 가 slot 을 소유하면 degraded.
	// (slots_ok 보고와 NODES flag 간 일시적 불일치 방어.)
	info := &ClusterInfo{State: "ok", SlotsAssigned: 16384, SlotsOK: 16384}
	nodes := parseClusterNodes(incidentNodes)
	if !IsClusterDegraded(info, nodes) {
		t.Fatalf("fail master owning slots must be degraded")
	}
}

func TestIsClusterDegraded_healthy_notDegraded(t *testing.T) {
	info := &ClusterInfo{State: "ok", SlotsAssigned: 16384, SlotsOK: 16384}
	nodes := parseClusterNodes(`aaa11111111111111111111111111111111111111 10.0.0.1:6379@16379 myself,master - 0 0 1 connected 0-5460
bbb22222222222222222222222222222222222222 10.0.0.2:6379@16379 master - 0 1700000000000 2 connected 5461-10921
ccc33333333333333333333333333333333333333 10.0.0.3:6379@16379 master - 0 1700000000000 3 connected 10922-16383
`)
	if IsClusterDegraded(info, nodes) {
		t.Fatalf("fully healthy cluster must not be degraded")
	}
}

func TestIsClusterDegraded_stateFail(t *testing.T) {
	info := &ClusterInfo{State: "fail", SlotsAssigned: 16384, SlotsOK: 16384}
	if !IsClusterDegraded(info, nil) {
		t.Fatalf("cluster_state=fail must be degraded")
	}
}

func TestIsClusterDegraded_nilInfo(t *testing.T) {
	if IsClusterDegraded(nil, nil) {
		t.Fatalf("nil info must be conservatively non-degraded (handled elsewhere)")
	}
}

func TestDetectStuckSlotHeals_incident_picksHealthyReplica(t *testing.T) {
	nodes := parseClusterNodes(incidentNodes)
	heals := DetectStuckSlotHeals(nodes)
	if len(heals) != 1 {
		t.Fatalf("want 1 heal (ccc shard), got %d: %+v", len(heals), heals)
	}
	h := heals[0]
	if h.FailedMasterID != "ccc33333333333333333333333333333333333333" {
		t.Errorf("failed master: %q", h.FailedMasterID)
	}
	if h.TakeoverReplicaID != "fff66666666666666666666666666666666666666" {
		t.Errorf("takeover replica: %q", h.TakeoverReplicaID)
	}
	if h.TakeoverReplicaAddr != "10.0.0.6:6379" {
		t.Errorf("takeover replica addr: %q", h.TakeoverReplicaAddr)
	}
}

func TestDetectStuckSlotHeals_noFailMaster_empty(t *testing.T) {
	nodes := parseClusterNodes(`aaa 10.0.0.1:6379@16379 myself,master - 0 0 1 connected 0-8191
bbb 10.0.0.2:6379@16379 master - 0 0 2 connected 8192-16383
`)
	if got := DetectStuckSlotHeals(nodes); len(got) != 0 {
		t.Fatalf("no fail master → no heals, got %+v", got)
	}
}

func TestDetectStuckSlotHeals_failMasterNoHealthyReplica_excluded(t *testing.T) {
	// fail master 의 유일한 replica 도 fail → 안전한 takeover 후보 없음 → 제외.
	nodes := parseClusterNodes(`ccc 10.0.0.3:6379@16379 master,fail - 0 0 3 connected 10922-16383
fff 10.0.0.6:6379@16379 slave,fail ccc 0 0 3 disconnected
`)
	if got := DetectStuckSlotHeals(nodes); len(got) != 0 {
		t.Fatalf("fail master with no healthy replica → excluded, got %+v", got)
	}
}

func TestDetectStuckSlotHeals_skipsUnhealthyReplica_picksHealthyOne(t *testing.T) {
	// ccc 가 fail. replica 둘 중 하나(ggg)는 pfail, 다른 하나(fff)는 healthy.
	nodes := parseClusterNodes(`ccc 10.0.0.3:6379@16379 master,fail - 0 0 3 connected 10922-16383
ggg 10.0.0.7:6379@16379 slave,fail? ccc 0 0 3 connected
fff 10.0.0.6:6379@16379 slave ccc 0 0 3 connected
`)
	heals := DetectStuckSlotHeals(nodes)
	if len(heals) != 1 {
		t.Fatalf("want 1 heal, got %d: %+v", len(heals), heals)
	}
	if heals[0].TakeoverReplicaID != "fff" {
		t.Errorf("should pick healthy replica fff, got %q", heals[0].TakeoverReplicaID)
	}
}

func TestDetectStuckSlotHeals_failMasterNoSlots_excluded(t *testing.T) {
	// fail master 이지만 slot 을 이미 잃음 (failover 일부 진행) → stuck 아님 → 제외.
	nodes := parseClusterNodes(`ccc 10.0.0.3:6379@16379 master,fail - 0 0 3 connected
fff 10.0.0.6:6379@16379 slave ccc 0 0 3 connected
`)
	if got := DetectStuckSlotHeals(nodes); len(got) != 0 {
		t.Fatalf("fail master without slots is not stuck, got %+v", got)
	}
}

func TestDetectStuckSlotHeals_multipleStuckShards_deterministicOrder(t *testing.T) {
	// 두 master 가 동시에 fail + slot 소유. 출력은 FailedMasterID 사전순.
	nodes := parseClusterNodes(`zzz 10.0.0.9:6379@16379 master,fail - 0 0 9 connected 0-5460
aaa 10.0.0.1:6379@16379 master,fail - 0 0 1 connected 5461-10921
zr1 10.0.0.8:6379@16379 slave zzz 0 0 9 connected
ar1 10.0.0.7:6379@16379 slave aaa 0 0 1 connected
`)
	heals := DetectStuckSlotHeals(nodes)
	if len(heals) != 2 {
		t.Fatalf("want 2 heals, got %d", len(heals))
	}
	if heals[0].FailedMasterID != "aaa" || heals[1].FailedMasterID != "zzz" {
		t.Errorf("heals not sorted by master id: %+v", heals)
	}
}
