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

// Package valkey — operator 가 in-process 로 valkey 인스턴스에 접속해 cluster
// init / replication / health 를 제어하는 client 래퍼.
// internal/mongodb/client.go 패턴을 valkey 도메인에 맞게 차용.
//
// go-redis/v9 사용 — Valkey 는 Redis 7.2 wire-compatible 이므로 그대로 동작.
package valkey

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DialOptions — operator 가 cluster 노드에 접속할 때 사용.
type DialOptions struct {
	Address  string // host:port
	Password string
	UseTLS   bool
	TLSConf  *tls.Config
}

// NewSingleClient — 단일 노드 클라이언트 (Standalone / Replication primary 검증용).
func NewSingleClient(opts DialOptions) *redis.Client {
	o := &redis.Options{
		Addr:        opts.Address,
		Password:    opts.Password,
		DialTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second,
	}
	if opts.UseTLS {
		o.TLSConfig = opts.TLSConf
	}
	return redis.NewClient(o)
}

// Ping — 연결성 + AUTH 검증.
func Ping(ctx context.Context, c *redis.Client) error {
	pong, err := c.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	if pong != "PONG" {
		return fmt.Errorf("unexpected ping response: %q", pong)
	}
	return nil
}
