/*
Copyright 2026 Keiailab.
*/

package valkey

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
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
	if (currentRole == "slave" || currentRole == "replica") && currentMaster == want {
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

// parseReplicationInfo — INFO replication 응답에서 role / master_host:port 추출.
func parseReplicationInfo(info string) (role, master string) {
	var host, port string
	for _, line := range strings.Split(info, "\n") {
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
