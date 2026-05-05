/*
Copyright 2026 Keiailab.
*/

package valkey

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// LastSaveTime — `LASTSAVE` 응답 (마지막 RDB 스냅샷 epoch 초).
func LastSaveTime(ctx context.Context, c *redis.Client) (time.Time, error) {
	t, err := c.LastSave(ctx).Result()
	if err != nil {
		return time.Time{}, fmt.Errorf("lastsave: %w", err)
	}
	return time.Unix(t, 0), nil
}

// BgSave — `BGSAVE` 비동기 RDB 스냅샷 트리거.
//
// 멱등성: 이미 BGSAVE 진행 중이면 server 가 "Background save already in progress"
// 에러 응답 — 본 함수는 *해당 에러를 무시* (사실상 idempotent). 외부 호출자는
// LASTSAVE timestamp 변동으로 완료 감지.
func BgSave(ctx context.Context, c *redis.Client) error {
	_, err := c.BgSave(ctx).Result()
	if err == nil {
		return nil
	}
	// "Background save already in progress" — 이전 BGSAVE 가 진행 중. 멱등.
	if msg := err.Error(); msg != "" && containsAny(msg, "in progress", "Background save already") {
		return nil
	}
	return fmt.Errorf("bgsave: %w", err)
}

// BgRewriteAOF — `BGREWRITEAOF` (AOF 모드 백업).
func BgRewriteAOF(ctx context.Context, c *redis.Client) error {
	_, err := c.BgRewriteAOF(ctx).Result()
	if err == nil {
		return nil
	}
	if msg := err.Error(); msg != "" && containsAny(msg, "in progress", "already in progress") {
		return nil
	}
	return fmt.Errorf("bgrewriteaof: %w", err)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	if m == 0 {
		return 0
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}
