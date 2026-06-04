/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package valkey

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// resolveAddrIP — Valkey CLUSTER MEET 는 hostname 을 거부하고 IP 만 받기 때문에
// host:port 형태 주소를 IP:port 로 정규화한다. 이미 IP 면 그대로 반환.
// 다중 IP 가 반환되면 첫 IPv4 우선, 없으면 첫 결과 사용.
func resolveAddrIP(ctx context.Context, addr string) (string, error) {
	host, portStr, ok := strings.Cut(addr, ":")
	if !ok {
		return "", fmt.Errorf("invalid address: %q", addr)
	}
	if ip := net.ParseIP(host); ip != nil {
		return addr, nil
	}
	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("resolve %s: %w", host, err)
	}
	chosen := ips[0]
	for _, ip := range ips {
		if v4 := ip.IP.To4(); v4 != nil {
			chosen = ip
			break
		}
	}
	return net.JoinHostPort(chosen.IP.String(), portStr), nil
}

// ClusterInfo — CLUSTER INFO 핵심 필드.
type ClusterInfo struct {
	State         string // "ok" | "fail"
	SlotsAssigned int32
	SlotsOK       int32
	KnownNodes    int32
	Size          int32 // primary 수
}

// QueryClusterInfo — 단일 노드에서 cluster 상태 조회 (멱등 선체크용).
func QueryClusterInfo(ctx context.Context, c *redis.Client) (*ClusterInfo, error) {
	raw, err := c.ClusterInfo(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("cluster info: %w", err)
	}
	return parseClusterInfo(raw), nil
}

func parseClusterInfo(s string) *ClusterInfo {
	out := &ClusterInfo{}
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch k {
		case "cluster_state":
			out.State = v
		case "cluster_slots_assigned":
			out.SlotsAssigned = atoi32(v)
		case "cluster_slots_ok":
			out.SlotsOK = atoi32(v)
		case "cluster_known_nodes":
			out.KnownNodes = atoi32(v)
		case "cluster_size":
			out.Size = atoi32(v)
		}
	}
	return out
}

func atoi32(s string) int32 {
	// gosec G109/G115 — bitsize 32 명시로 overflow 검출 위임 (실패 시 0).
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int32(n)
}

// CreateCluster — 6 노드 슬롯 분배 + replicate. valkey-cli --cluster create 의
// in-process 등가물.
//
// addresses: 모든 노드 host:port. addresses[:shards] = primary, addresses[shards:] = replica.
// shards: primary 수.
// replicasPerShard: shard 당 replica 수.
//
// 단계별 멱등성:
//  1. ensureMeet: 첫 노드의 CLUSTER NODES 응답을 보고 *모르는 주소만* MEET.
//  2. ensureSlots: 각 primary 의 CLUSTER NODES 자체 응답을 보고 *미보유 slot 만* ADDSLOTS.
//  3. ensureReplicas: 각 replica 의 CLUSTER NODES 자체 응답을 보고 *MasterID 가 다를 때만*
//     REPLICATE.
//
// 부분 실패 후 재호출이 안전하다 — 이미 적용된 단계는 자동 skip.
func CreateCluster(ctx context.Context, dial func(addr string) *redis.Client, addresses []string, shards, replicasPerShard int) error {
	if len(addresses) < shards*(1+replicasPerShard) {
		return fmt.Errorf("need %d nodes, got %d", shards*(1+replicasPerShard), len(addresses))
	}
	primaries := addresses[:shards]
	replicas := addresses[shards:]

	if err := ensureMeet(ctx, dial, addresses); err != nil {
		return err
	}
	if err := ensureSlots(ctx, dial, primaries); err != nil {
		return err
	}
	if err := ensureReplicas(ctx, dial, primaries, replicas, shards); err != nil {
		return err
	}
	return nil
}

// ensureMeet — 첫 노드에서 본 알려진 주소 집합을 비교해 미멤버만 MEET 발행. 멱등.
func ensureMeet(ctx context.Context, dial func(addr string) *redis.Client, addresses []string) error {
	first := dial(addresses[0])
	defer func() { _ = first.Close() }()

	known := map[string]bool{addresses[0]: true} // 자기 자신은 항상 멤버.
	if nodes, err := QueryClusterNodes(ctx, first); err == nil {
		for k := range KnownAddrs(nodes) {
			known[k] = true
		}
	}

	for _, addr := range addresses[1:] {
		if known[addr] {
			continue
		}
		ipAddr, err := resolveAddrIP(ctx, addr)
		if err != nil {
			return fmt.Errorf("cluster meet %s: %w", addr, err)
		}
		host, portStr, ok := strings.Cut(ipAddr, ":")
		if !ok {
			return fmt.Errorf("invalid resolved address: %q", ipAddr)
		}
		if err := first.ClusterMeet(ctx, host, portStr).Err(); err != nil {
			return fmt.Errorf("cluster meet %s (resolved %s): %w", addr, ipAddr, err)
		}
	}
	return nil
}

// ensureSlots — 각 primary 가 *기대 slot 범위* 를 보유했는지 확인하고 미보유분만 ADDSLOTS.
//
// 분배 규칙: 16384 / shards 균등, 나머지 slot 은 마지막 primary 에 부여.
// 이미 다른 노드가 가진 slot 은 본 단계가 회복하지 못함 — partial-state 감지 단계에서
// 별도 처리 (CLUSTER FLUSHSLOTS 또는 SETSLOT).
func ensureSlots(ctx context.Context, dial func(addr string) *redis.Client, primaries []string) error {
	const totalSlots = 16384
	shards := len(primaries)
	per := totalSlots / shards
	idx := 0
	for i, addr := range primaries {
		end := idx + per - 1
		if i == shards-1 {
			end = totalSlots - 1
		}
		c := dial(addr)
		nodes, err := QueryClusterNodes(ctx, c)
		if err != nil {
			_ = c.Close()
			return fmt.Errorf("cluster nodes %s: %w", addr, err)
		}
		myself := findMyself(nodes)
		var missing []int
		for s := idx; s <= end; s++ {
			if myself == nil || !myself.HasSlot(s) {
				missing = append(missing, s)
			}
		}
		if len(missing) > 0 {
			if err := c.ClusterAddSlots(ctx, missing...).Err(); err != nil {
				_ = c.Close()
				return fmt.Errorf("addslots primary %s (%d missing): %w", addr, len(missing), err)
			}
		}
		_ = c.Close()
		idx = end + 1
	}
	return nil
}

// ensureReplicas — 각 replica 가 올바른 primary 를 가리키게 한다. 이미 가리키면 skip.
//
// gossip 수렴 지연 처리: ensureMeet 직후에는 replica 가 아직 primary node id 를
// 모를 수 있다 ("ERR Unknown node"). 짧은 backoff 로 재시도.
func ensureReplicas(ctx context.Context, dial func(addr string) *redis.Client, primaries, replicas []string, shards int) error {
	primaryIDs := make([]string, len(primaries))
	for i, addr := range primaries {
		c := dial(addr)
		id, err := c.ClusterMyID(ctx).Result()
		_ = c.Close()
		if err != nil {
			return fmt.Errorf("get my id %s: %w", addr, err)
		}
		primaryIDs[i] = id
	}
	for i, addr := range replicas {
		want := primaryIDs[i%shards]
		if err := replicateWithRetry(ctx, dial, addr, want); err != nil {
			return err
		}
	}
	return nil
}

// replicateWithRetry — 단일 replica 가 want primary 를 가리키도록 ClusterReplicate.
// gossip 수렴 대기 (Unknown node) 와 일시적 네트워크 에러를 흡수.
func replicateWithRetry(ctx context.Context, dial func(addr string) *redis.Client, addr, want string) error {
	const maxAttempts = 10
	delay := 200 * time.Millisecond
	var lastErr error
	for range maxAttempts {
		c := dial(addr)
		nodes, err := QueryClusterNodes(ctx, c)
		if err != nil {
			_ = c.Close()
			lastErr = fmt.Errorf("cluster nodes %s: %w", addr, err)
		} else {
			myself := findMyself(nodes)
			if myself != nil && myself.IsReplica() && myself.MasterID == want {
				_ = c.Close()
				return nil
			}
			err := c.ClusterReplicate(ctx, want).Err()
			_ = c.Close()
			if err == nil {
				return nil
			}
			lastErr = fmt.Errorf("replicate %s -> %s: %w", addr, want, err)
			// "Unknown node" 외 영구 에러는 즉시 반환.
			if !strings.Contains(err.Error(), "Unknown node") {
				return lastErr
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (last: %v)", ctx.Err(), lastErr)
		case <-time.After(delay):
		}
		if delay < 2*time.Second {
			delay *= 2
		}
	}
	return fmt.Errorf("replicate %s -> %s: %d attempts: %w", addr, want, maxAttempts, lastErr)
}

// findMyself — CLUSTER NODES 결과에서 myself flag 가진 노드 반환.
func findMyself(nodes []NodeView) *NodeView {
	for i := range nodes {
		if nodes[i].Flags["myself"] {
			return &nodes[i]
		}
	}
	return nil
}

// ForgetNode — scale-in 시 node 제거. 모든 잔존 primary 에서 호출 필요.
func ForgetNode(ctx context.Context, c *redis.Client, nodeID string) error {
	if err := c.ClusterForget(ctx, nodeID).Err(); err != nil {
		return fmt.Errorf("cluster forget %s: %w", nodeID, err)
	}
	return nil
}
