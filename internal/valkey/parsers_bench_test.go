/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// 핫 path parser 의 microbenchmark — performance 회귀 차단.
// 본 parser 들은 매 reconcile / 매 health check 시점 호출 — 큰 cluster (100+ shards)
// 에서 parseClusterNodes 가 GC pressure 의 주범이 되면 reconcile loop 지연 →
// 자동 failover SLA 영향. 본 benchmark 가 baseline.
//
// 실행: go test -bench=. -benchmem ./internal/valkey/

package valkey

import (
	"strings"
	"testing"
)

// 100 노드 cluster 의 CLUSTER NODES 응답 시뮬레이션 — primary 50 + replica 50.
func generateClusterNodesRaw(nodeCount int) string {
	var b strings.Builder
	for i := range nodeCount {
		role := "master"
		if i >= nodeCount/2 {
			role = "slave"
		}
		// id ip:port@bus flags master link epoch state slots...
		// node{i} 10.0.0.{i+1}:6379@16379 myself,{role} - 0 0 epoch connected 0-100
		_, _ = b.WriteString("node")
		_, _ = b.WriteString(string(rune('0' + (i % 10))))
		_, _ = b.WriteString(" 10.0.0.1:6379@16379 ")
		_, _ = b.WriteString(role)
		_, _ = b.WriteString(" - 0 0 1 connected 0-100 [101-200]\n")
	}
	return b.String()
}

func BenchmarkParseClusterNodes_6(b *testing.B) {
	raw := generateClusterNodesRaw(6)
	b.ReportAllocs()

	for b.Loop() {
		_ = parseClusterNodes(raw)
	}
}

func BenchmarkParseClusterNodes_100(b *testing.B) {
	raw := generateClusterNodesRaw(100)
	b.ReportAllocs()

	for b.Loop() {
		_ = parseClusterNodes(raw)
	}
}

func BenchmarkParseClusterInfo(b *testing.B) {
	raw := "cluster_state:ok\ncluster_slots_assigned:16384\ncluster_slots_ok:16384\ncluster_slots_pfail:0\ncluster_slots_fail:0\ncluster_known_nodes:6\ncluster_size:3\ncluster_current_epoch:6\ncluster_my_epoch:1\ncluster_stats_messages_sent:1000\ncluster_stats_messages_received:1000\n"
	b.ReportAllocs()

	for b.Loop() {
		_ = parseClusterInfo(raw)
	}
}

func BenchmarkParseReplicationOffset(b *testing.B) {
	raw := "# Replication\nrole:master\nconnected_slaves:1\nslave0:ip=10.0.0.2,port=6379,state=online,offset=12345,lag=0\nmaster_replid:abc123\nmaster_repl_offset:12345\n"
	b.ReportAllocs()

	for b.Loop() {
		_ = ParseReplicationOffset(raw)
	}
}

func BenchmarkParseSlotToken(b *testing.B) {
	tokens := []string{"0-5460", "5461-10922", "10923-16383", "42", "abc", "0-100", "200"}
	b.ReportAllocs()

	for b.Loop() {
		for _, tok := range tokens {
			_, _ = parseSlotToken(tok)
		}
	}
}
