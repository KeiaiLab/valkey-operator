/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 결함 ③ — 멤버십 재합류 *순수 결정 로직* 단위테스트.
// 실제 Valkey gossip 없이 (envtest 불가 영역) 어떤 pod 가 재합류 대상인지 +
// 각 replica 가 어느 master 를 따라야 하는지의 알고리즘 회귀를 차단한다.
package controller

import (
	"reflect"
	"testing"

	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

func TestDesiredMasterOrdinal(t *testing.T) {
	// 3 shards × 2 rps: replica ordinal 3..8 → master ordinal.
	//   ord 3 (j=0) → 0,  ord 4 (j=1) → 1,  ord 5 (j=2) → 2
	//   ord 6 (j=3) → 0,  ord 7 (j=4) → 1,  ord 8 (j=5) → 2
	cases := map[int]int{3: 0, 4: 1, 5: 2, 6: 0, 7: 1, 8: 2}
	for ord, want := range cases {
		if got := desiredMasterOrdinal(ord, 3); got != want {
			t.Errorf("desiredMasterOrdinal(%d,3)=%d want %d", ord, got, want)
		}
	}
}

func TestDetectReintegration_allHealthy_noop(t *testing.T) {
	// 3×1, 모두 멤버 + replica 가 올바른 master.
	observed := map[int]observedMember{
		0: {IsMember: true, MasterOrdinal: -1},
		1: {IsMember: true, MasterOrdinal: -1},
		2: {IsMember: true, MasterOrdinal: -1},
		3: {IsMember: true, MasterOrdinal: 0},
		4: {IsMember: true, MasterOrdinal: 1},
		5: {IsMember: true, MasterOrdinal: 2},
	}
	if got := detectReintegration(3, 1, observed); len(got) != 0 {
		t.Fatalf("healthy cluster should yield no actions, got %v", got)
	}
}

func TestDetectReintegration_missingReplica(t *testing.T) {
	// 3×1, replica ord 4 가 멤버에서 이탈 (재시작 후 새 id).
	observed := map[int]observedMember{
		0: {IsMember: true, MasterOrdinal: -1},
		1: {IsMember: true, MasterOrdinal: -1},
		2: {IsMember: true, MasterOrdinal: -1},
		3: {IsMember: true, MasterOrdinal: 0},
		4: {IsMember: false, MasterOrdinal: -1}, // 이탈.
		5: {IsMember: true, MasterOrdinal: 2},
	}
	got := detectReintegration(3, 1, observed)
	want := []reintegrationAction{
		{Ordinal: 4, IsReplica: true, ReplicateTargetOrdinal: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestDetectReintegration_replicaWrongMaster(t *testing.T) {
	// replica ord 5 가 멤버지만 틀린 master(0)를 따름 → 2 로 교정.
	observed := map[int]observedMember{
		0: {IsMember: true, MasterOrdinal: -1},
		1: {IsMember: true, MasterOrdinal: -1},
		2: {IsMember: true, MasterOrdinal: -1},
		3: {IsMember: true, MasterOrdinal: 0},
		4: {IsMember: true, MasterOrdinal: 1},
		5: {IsMember: true, MasterOrdinal: 0}, // 틀림 — 2 여야.
	}
	got := detectReintegration(3, 1, observed)
	want := []reintegrationAction{
		{Ordinal: 5, IsReplica: true, ReplicateTargetOrdinal: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestDetectReintegration_missingPrimaryOrderedFirst(t *testing.T) {
	// primary ord 1 이탈 + replica ord 4 (그 primary 의 replica) 이탈.
	// primary 가 먼저 MEET 되도록 actions 순서가 primary→replica 여야 한다.
	observed := map[int]observedMember{
		0: {IsMember: true, MasterOrdinal: -1},
		1: {IsMember: false, MasterOrdinal: -1},
		2: {IsMember: true, MasterOrdinal: -1},
		3: {IsMember: true, MasterOrdinal: 0},
		4: {IsMember: false, MasterOrdinal: -1},
		5: {IsMember: true, MasterOrdinal: 2},
	}
	got := detectReintegration(3, 1, observed)
	want := []reintegrationAction{
		{Ordinal: 1, IsReplica: false, ReplicateTargetOrdinal: 0},
		{Ordinal: 4, IsReplica: true, ReplicateTargetOrdinal: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	// 첫 액션은 반드시 primary (replica 가 붙기 전 master 가 멤버여야 함).
	if got[0].IsReplica {
		t.Fatal("primary action must come before replica action")
	}
}

func TestDetectReintegration_mastersOnly_noReplicaActions(t *testing.T) {
	// rps=0 — replica 자체가 없으므로 누락 primary 만 MEET, replica 액션 없음.
	observed := map[int]observedMember{
		0: {IsMember: true, MasterOrdinal: -1},
		1: {IsMember: false, MasterOrdinal: -1},
		2: {IsMember: true, MasterOrdinal: -1},
	}
	got := detectReintegration(3, 0, observed)
	want := []reintegrationAction{
		{Ordinal: 1, IsReplica: false, ReplicateTargetOrdinal: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestBuildObservedMembers_mapsMasterOrdinal(t *testing.T) {
	// 3×1, 모든 노드 멤버. NODES 가 replica 의 MasterID 로 master ordinal 을 역참조.
	addrByOrdinal := map[int]string{
		0: "10.0.0.1:6379", 1: "10.0.0.2:6379", 2: "10.0.0.3:6379",
		3: "10.0.0.4:6379", 4: "10.0.0.5:6379", 5: "10.0.0.6:6379",
	}
	nodes := []vk.NodeView{
		{ID: "p0", Addr: "10.0.0.1:6379", Flags: map[string]bool{"master": true}},
		{ID: "p1", Addr: "10.0.0.2:6379", Flags: map[string]bool{"master": true}},
		{ID: "p2", Addr: "10.0.0.3:6379", Flags: map[string]bool{"master": true}},
		{ID: "r0", Addr: "10.0.0.4:6379", Flags: map[string]bool{"slave": true}, MasterID: "p0"},
		{ID: "r1", Addr: "10.0.0.5:6379", Flags: map[string]bool{"slave": true}, MasterID: "p1"},
		{ID: "r2", Addr: "10.0.0.6:6379", Flags: map[string]bool{"slave": true}, MasterID: "p2"},
	}
	observed, ids := buildObservedMembers(3, 1, nodes, addrByOrdinal)

	// replica ordinal 3 → master ordinal 0, 4 → 1, 5 → 2.
	for ord, wantMaster := range map[int]int{3: 0, 4: 1, 5: 2} {
		if !observed[ord].IsMember {
			t.Errorf("ordinal %d should be member", ord)
		}
		if observed[ord].MasterOrdinal != wantMaster {
			t.Errorf("ordinal %d masterOrdinal=%d want %d", ord, observed[ord].MasterOrdinal, wantMaster)
		}
	}
	if ids[0] != "p0" || ids[5] != "r2" {
		t.Errorf("nodeIDByOrdinal mapping wrong: %v", ids)
	}
	// 멤버 + 올바른 master → 재합류 액션 0.
	if acts := detectReintegration(3, 1, observed); len(acts) != 0 {
		t.Errorf("healthy → no actions, got %v", acts)
	}
}

func TestBuildObservedMembers_missingPodIsNonMember(t *testing.T) {
	// ordinal 4 의 pod IP 미상 (재시작 직후) → 비멤버.
	addrByOrdinal := map[int]string{
		0: "10.0.0.1:6379", 1: "10.0.0.2:6379", 2: "10.0.0.3:6379",
		3: "10.0.0.4:6379", 5: "10.0.0.6:6379",
	}
	nodes := []vk.NodeView{
		{ID: "p0", Addr: "10.0.0.1:6379", Flags: map[string]bool{"master": true}},
		{ID: "p1", Addr: "10.0.0.2:6379", Flags: map[string]bool{"master": true}},
		{ID: "p2", Addr: "10.0.0.3:6379", Flags: map[string]bool{"master": true}},
		{ID: "r0", Addr: "10.0.0.4:6379", Flags: map[string]bool{"slave": true}, MasterID: "p0"},
		{ID: "r2", Addr: "10.0.0.6:6379", Flags: map[string]bool{"slave": true}, MasterID: "p2"},
	}
	observed, _ := buildObservedMembers(3, 1, nodes, addrByOrdinal)
	if observed[4].IsMember {
		t.Fatal("ordinal 4 (no pod IP) must be non-member")
	}
	got := detectReintegration(3, 1, observed)
	want := []reintegrationAction{{Ordinal: 4, IsReplica: true, ReplicateTargetOrdinal: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestOrdinalFromPodName(t *testing.T) {
	cases := []struct {
		name, prefix string
		want         int
		ok           bool
	}{
		{"vk-0", "vk-", 0, true},
		{"vk-3", "vk-", 3, true},
		{"vk-12", "vk-", 12, true},
		{"vk-", "vk-", 0, false},
		{"other-1", "vk-", 0, false},
		{"vk-x", "vk-", 0, false},
	}
	for _, c := range cases {
		got, ok := ordinalFromPodName(c.name, c.prefix)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ordinalFromPodName(%q,%q)=(%d,%v) want (%d,%v)", c.name, c.prefix, got, ok, c.want, c.ok)
		}
	}
}
