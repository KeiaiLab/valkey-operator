/*
Copyright 2026 Keiailab.
*/

package valkey

import "testing"

func TestParseReplicationOffset_master(t *testing.T) {
	raw := "# Replication\nrole:master\nmaster_repl_offset:12345\nconnected_slaves:1\n"
	if got := ParseReplicationOffset(raw); got != 12345 {
		t.Fatalf("expected 12345, got %d", got)
	}
}

func TestParseReplicationOffset_replica(t *testing.T) {
	raw := "# Replication\nrole:slave\nmaster_host:10.0.0.1\nmaster_port:6379\n" +
		"slave_repl_offset:6789\nconnected_slaves:0\n"
	if got := ParseReplicationOffset(raw); got != 6789 {
		t.Fatalf("expected 6789, got %d", got)
	}
}

func TestParseReplicationOffset_missing(t *testing.T) {
	raw := "# Replication\nrole:replica\nmaster_link_status:up\n"
	if got := ParseReplicationOffset(raw); got != 0 {
		t.Fatalf("expected 0 (missing offset), got %d", got)
	}
}

func TestParseReplicationOffset_invalidNumber(t *testing.T) {
	raw := "master_repl_offset:abc\nslave_repl_offset:42\n"
	if got := ParseReplicationOffset(raw); got != 42 {
		t.Fatalf("expected 42 (skip invalid + take valid), got %d", got)
	}
}

func TestParseClusterInfo_ok(t *testing.T) {
	raw := "cluster_state:ok\r\ncluster_slots_assigned:16384\r\n" +
		"cluster_slots_ok:16384\r\ncluster_known_nodes:6\r\ncluster_size:3\r\n"
	got := parseClusterInfo(raw)
	if got.State != "ok" {
		t.Errorf("State: got %q want ok", got.State)
	}
	if got.SlotsAssigned != 16384 || got.SlotsOK != 16384 {
		t.Errorf("Slots: got %+v", got)
	}
	if got.KnownNodes != 6 || got.Size != 3 {
		t.Errorf("Topology: got %+v", got)
	}
}

func TestParseClusterInfo_partial_fail(t *testing.T) {
	raw := "cluster_state:fail\r\ncluster_slots_assigned:8192\r\ncluster_size:2\r\n"
	got := parseClusterInfo(raw)
	if got.State != "fail" {
		t.Errorf("State: got %q want fail", got.State)
	}
	if got.SlotsAssigned != 8192 || got.Size != 2 {
		t.Errorf("Partial: got %+v", got)
	}
}

func TestParseClusterInfo_unknown_keys_ignored(t *testing.T) {
	raw := "cluster_state:ok\r\nfuture_key:42\r\nmangled-line\r\n:value-only\r\n"
	got := parseClusterInfo(raw)
	if got.State != "ok" {
		t.Errorf("expected resilient parse, got %+v", got)
	}
}

func TestParseReplicationInfo_replica_modern(t *testing.T) {
	info := "role:replica\r\nmaster_host:vk-0.svc\r\nmaster_port:6379\r\n"
	role, master := parseReplicationInfo(info)
	if role != "replica" {
		t.Errorf("role: got %q", role)
	}
	if master != "vk-0.svc:6379" {
		t.Errorf("master: got %q", master)
	}
}

func TestParseReplicationInfo_slave_legacy(t *testing.T) {
	info := "role:slave\r\nmaster_host:m\r\nmaster_port:6380\r\n"
	role, master := parseReplicationInfo(info)
	if role != "slave" || master != "m:6380" {
		t.Errorf("legacy parse: role=%q master=%q", role, master)
	}
}

// CLUSTER NODES 응답 — 3 primary + 3 replica 부트스트랩 직후의 전형적 형태.
func TestParseClusterNodes_3x1(t *testing.T) {
	raw := `aaa11111111111111111111111111111111111111 10.0.0.1:6379@16379 myself,master - 0 0 1 connected 0-5460
bbb22222222222222222222222222222222222222 10.0.0.2:6379@16379 master - 0 1700000000000 2 connected 5461-10921
ccc33333333333333333333333333333333333333 10.0.0.3:6379@16379 master - 0 1700000000000 3 connected 10922-16383
ddd44444444444444444444444444444444444444 10.0.0.4:6379@16379 slave aaa11111111111111111111111111111111111111 0 1700000000000 1 connected
eee55555555555555555555555555555555555555 10.0.0.5:6379@16379 replica bbb22222222222222222222222222222222222222 0 1700000000000 2 connected
fff66666666666666666666666666666666666666 10.0.0.6:6379@16379 slave ccc33333333333333333333333333333333333333 0 1700000000000 3 connected
`
	nodes := parseClusterNodes(raw)
	if len(nodes) != 6 {
		t.Fatalf("want 6 nodes, got %d", len(nodes))
	}
	if !nodes[0].IsPrimary() {
		t.Errorf("node 0 should be primary")
	}
	if !nodes[0].Flags["myself"] {
		t.Errorf("node 0 myself flag missing")
	}
	if nodes[3].MasterID != "aaa11111111111111111111111111111111111111" {
		t.Errorf("node 3 master id: %q", nodes[3].MasterID)
	}
	if !nodes[3].IsReplica() {
		t.Errorf("node 3 should be replica (slave flag)")
	}
	if !nodes[4].IsReplica() {
		t.Errorf("node 4 should be replica (replica flag)")
	}

	// slot 검증: 0번 노드는 [0,5460] 보유.
	if !nodes[0].HasSlot(0) || !nodes[0].HasSlot(5460) || nodes[0].HasSlot(5461) {
		t.Errorf("node 0 slot range wrong: %+v", nodes[0].Slots)
	}
	if !nodes[2].HasSlot(16383) {
		t.Errorf("node 2 should hold 16383: %+v", nodes[2].Slots)
	}

	// extractAddr 동작 — bus port 제거.
	if nodes[0].Addr != "10.0.0.1:6379" {
		t.Errorf("addr extract: %q", nodes[0].Addr)
	}
}

func TestParseClusterNodes_migrationToken_ignored(t *testing.T) {
	raw := `aaa 10.0.0.1:6379@16379 myself,master - 0 0 1 connected 0-100 [101-<-bbb]
bbb 10.0.0.2:6379@16379 master - 0 0 2 connected
`
	nodes := parseClusterNodes(raw)
	if len(nodes) != 2 {
		t.Fatalf("want 2, got %d", len(nodes))
	}
	if !nodes[0].HasSlot(100) || nodes[0].HasSlot(101) {
		t.Errorf("migration token leaked: %+v", nodes[0].Slots)
	}
}

func TestParseClusterNodes_singleSlot(t *testing.T) {
	raw := `aaa 1.1.1.1:6379@16379 master - 0 0 1 connected 42`
	nodes := parseClusterNodes(raw)
	if len(nodes) != 1 || !nodes[0].HasSlot(42) || nodes[0].HasSlot(41) {
		t.Errorf("single slot parse: %+v", nodes)
	}
}

func TestExtractAddr(t *testing.T) {
	cases := map[string]string{
		"10.0.0.1:6379@16379":          "10.0.0.1:6379",
		"10.0.0.1:6379@16379,hostname": "10.0.0.1:6379",
		"10.0.0.1:6379":                "10.0.0.1:6379",
	}
	for in, want := range cases {
		if got := extractAddr(in); got != want {
			t.Errorf("extractAddr(%q): got %q want %q", in, got, want)
		}
	}
}

func TestKnownAddrs(t *testing.T) {
	nodes := []NodeView{{Addr: "a:1"}, {Addr: "b:2"}, {Addr: "a:1"}}
	got := KnownAddrs(nodes)
	if len(got) != 2 || !got["a:1"] || !got["b:2"] {
		t.Errorf("KnownAddrs: %v", got)
	}
}

func TestFindByAddr(t *testing.T) {
	nodes := []NodeView{{Addr: "a:1", ID: "x"}, {Addr: "b:2", ID: "y"}}
	if FindByAddr(nodes, "b:2").ID != "y" {
		t.Errorf("FindByAddr miss")
	}
	if FindByAddr(nodes, "z:9") != nil {
		t.Errorf("FindByAddr should return nil for unknown addr")
	}
}

func TestParseReplicationInfo_master_no_master_fields(t *testing.T) {
	info := "role:master\r\nconnected_slaves:2\r\n"
	role, master := parseReplicationInfo(info)
	if role != "master" {
		t.Errorf("role: got %q", role)
	}
	if master != "" {
		t.Errorf("master should be empty for primary role, got %q", master)
	}
}
