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

// Valkey replication role 식별자 — Valkey/Redis 는 legacy "slave" 또는 modern
// "replica" 로 role 을 보고. 둘 다 허용.
const (
	roleSlave   = "slave"
	roleReplica = "replica"
)

// EnsureReplicaOf — replica 가 primary 를 가리키도록 REPLICAOF 명령 발행.
// 이미 가리키고 있으면 no-op (멱등).
func EnsureReplicaOf(ctx context.Context, c *redis.Client, primaryHost string, primaryPort int) error {
	info, err := c.Info(ctx, "replication").Result()
	if err != nil {
		return fmt.Errorf("info replication: %w", err)
	}
	currentRole, currentMaster := parseReplicationInfo(info)
	want := fmt.Sprintf("%s:%d", primaryHost, primaryPort)
	// Valkey/Redis 는 role 을 "slave" (legacy) 또는 "replica" 로 보고. 둘 다 허용.
	if (currentRole == roleSlave || currentRole == roleReplica) && currentMaster == want {
		return nil
	}
	if err := c.ReplicaOf(ctx, primaryHost, fmt.Sprintf("%d", primaryPort)).Err(); err != nil {
		return fmt.Errorf("replicaof %s %d: %w", primaryHost, primaryPort, err)
	}
	return nil
}

// PromoteToPrimary — REPLICAOF NO ONE.
func PromoteToPrimary(ctx context.Context, c *redis.Client) error {
	if err := c.ReplicaOf(ctx, "no", "one").Err(); err != nil {
		return fmt.Errorf("replicaof no one: %w", err)
	}
	return nil
}

// ClusterFailoverTakeover — replica 에서 CLUSTER FAILOVER TAKEOVER 발행 (결함 ⑤).
//
// 일반 CLUSTER FAILOVER 는 master 와의 handshake (offset 동기 + 합의)를 요구하므로
// master 가 *이미 fail* 인 partial-slot outage 에서는 동작하지 않는다. TAKEOVER 옵션은
// 합의 없이 replica 가 즉시 slot 소유권 + epoch 를 승계해 stuck slot 을 ok 로 되돌린다.
//
// 주의: TAKEOVER 는 split-brain 위험이 있어 *master 가 확실히 fail* 일 때만 호출해야
// 한다 (caller 의 보수적 게이트 책임). go-redis 의 ClusterFailover 는 옵션 인자를
// 받지 않으므로 raw Do 로 발행한다. 본 명령은 replica 노드에 직접 보내야 한다.
func ClusterFailoverTakeover(ctx context.Context, c *redis.Client) error {
	if err := c.Do(ctx, "CLUSTER", "FAILOVER", "TAKEOVER").Err(); err != nil {
		return fmt.Errorf("cluster failover takeover: %w", err)
	}
	return nil
}

// ParseReplicationOffset — INFO replication 응답에서 master_repl_offset 또는
// slave_repl_offset 추출. ADR-0017 의 failover 후보 선출에 사용.
//
// primary 노드: master_repl_offset 가 *총 commit 시점*. replica 노드:
// slave_repl_offset 가 *replica 가 받아간* 시점. failover 시 *가장 큰 slave
// offset* replica 가 가장 latest.
//
// 둘 다 부재 또는 invalid 시 0 반환 (보수적 — failover 후보에서 사실상 제외).
// 첫 valid 매칭만 사용 — replica 노드는 보통 slave_repl_offset 만, primary
// 는 master_repl_offset 만 가지므로 OR 의미.
func ParseReplicationOffset(info string) int64 {
	for line := range strings.SplitSeq(info, "\n") {
		line = strings.TrimSpace(line)
		var raw string
		switch {
		case strings.HasPrefix(line, "master_repl_offset:"):
			raw = strings.TrimPrefix(line, "master_repl_offset:")
		case strings.HasPrefix(line, "slave_repl_offset:"):
			raw = strings.TrimPrefix(line, "slave_repl_offset:")
		default:
			continue
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil {
			return n
		}
	}
	return 0
}

// parseReplicationInfo — INFO replication 응답에서 role / master_host:port 추출.
func parseReplicationInfo(info string) (role, master string) {
	var host, port string
	for line := range strings.SplitSeq(info, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "role:"):
			role = strings.TrimPrefix(line, "role:")
		case strings.HasPrefix(line, "master_host:"):
			host = strings.TrimPrefix(line, "master_host:")
		case strings.HasPrefix(line, "master_port:"):
			port = strings.TrimPrefix(line, "master_port:")
		}
	}
	if host != "" && port != "" {
		master = fmt.Sprintf("%s:%s", host, port)
	}
	return
}
