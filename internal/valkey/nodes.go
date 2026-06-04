/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package valkey

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// NodeView — 단일 CLUSTER NODES 라인 파싱 결과.
//
// 라인 형식 (Valkey 7.x 기준):
//
//	<id> <ip:port@bus[,hostname]> <flags> <master-id|-> <ping-sent> <pong-recv>
//	<config-epoch> <link-state> [slot|slot-range|migration]...
//
// 예:
//
//	abc... 10.0.0.1:6379@16379 myself,master - 0 0 1 connected 0-5460
//	def... 10.0.0.2:6379@16379 slave abc... 0 0 1 connected
type NodeView struct {
	ID       string
	Addr     string // "ip:port" — bus 포트와 hostname 제거.
	Flags    map[string]bool
	MasterID string // replica 의 경우 primary id, primary 면 "" 또는 "-".
	LinkOK   bool
	Slots    []SlotRange // primary 만 보유.
}

// SlotRange — [Start, End] inclusive. 단일 슬롯이면 Start==End.
type SlotRange struct {
	Start int
	End   int
}

// IsPrimary — myself,master 또는 master flag.
func (n *NodeView) IsPrimary() bool { return n.Flags["master"] }

// IsReplica — slave / replica flag.
func (n *NodeView) IsReplica() bool { return n.Flags[roleSlave] || n.Flags[roleReplica] }

// HasSlot — slot s 가 본 노드에 할당되어 있는가.
func (n *NodeView) HasSlot(s int) bool {
	for _, r := range n.Slots {
		if s >= r.Start && s <= r.End {
			return true
		}
	}
	return false
}

// QueryClusterNodes — CLUSTER NODES 응답을 NodeView 슬라이스로 파싱.
func QueryClusterNodes(ctx context.Context, c *redis.Client) ([]NodeView, error) {
	raw, err := c.ClusterNodes(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("cluster nodes: %w", err)
	}
	return parseClusterNodes(raw), nil
}

func parseClusterNodes(raw string) []NodeView {
	var out []NodeView
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 8 {
			continue
		}
		nv := NodeView{
			ID:       parts[0],
			Addr:     extractAddr(parts[1]),
			Flags:    parseFlags(parts[2]),
			MasterID: parts[3],
			LinkOK:   parts[7] == "connected",
		}
		// slot ranges: parts[8:] — migrating/importing 표기 ([N-<id>]) 는 무시.
		for _, tok := range parts[8:] {
			if strings.HasPrefix(tok, "[") {
				continue // 마이그레이션 표기.
			}
			if r, ok := parseSlotToken(tok); ok {
				nv.Slots = append(nv.Slots, r)
			}
		}
		out = append(out, nv)
	}
	return out
}

// extractAddr — "ip:port@bus[,hostname]" → "ip:port".
func extractAddr(s string) string {
	if i := strings.IndexAny(s, "@,"); i >= 0 {
		return s[:i]
	}
	return s
}

func parseFlags(s string) map[string]bool {
	out := map[string]bool{}
	for f := range strings.SplitSeq(s, ",") {
		out[f] = true
	}
	return out
}

func parseSlotToken(s string) (SlotRange, bool) {
	if before, after, ok := strings.Cut(s, "-"); ok {
		a, errA := strconv.Atoi(before)
		b, errB := strconv.Atoi(after)
		if errA != nil || errB != nil {
			return SlotRange{}, false
		}
		// Reject malformed ranges where Start > End. Valkey itself
		// should never emit these, but the fuzz suite confirmed the
		// parser previously accepted "2-0" as a valid range.
		if a > b {
			return SlotRange{}, false
		}
		return SlotRange{Start: a, End: b}, true
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return SlotRange{}, false
	}
	return SlotRange{Start: n, End: n}, true
}

// FindByAddr — addr ("ip:port") 로 NodeView 검색. 없으면 nil.
func FindByAddr(nodes []NodeView, addr string) *NodeView {
	for i := range nodes {
		if nodes[i].Addr == addr {
			return &nodes[i]
		}
	}
	return nil
}

// KnownAddrs — 클러스터 멤버 주소 집합 (ensureMeet 의 사전 점검용).
func KnownAddrs(nodes []NodeView) map[string]bool {
	out := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		out[n.Addr] = true
	}
	return out
}
