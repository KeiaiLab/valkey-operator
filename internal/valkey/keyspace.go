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

// INFO keyspace 호출 + 응답 파싱. ValkeyRestore 의 데이터 plane 검증 (Verifying
// phase) 에서 RestoredKeys 추정에 사용.
package valkey

import (
	"context"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// CountKeyspaceKeys — INFO keyspace 호출 후 모든 db 의 keys 합산.
//
// 응답 형식 (예시):
//
//	# Keyspace
//	db0:keys=42,expires=0,avg_ttl=0
//	db1:keys=10,expires=2,avg_ttl=0
//
// 위 케이스에서 합산 결과 = 52.
//
// 호출 실패 시 0 반환 (caller 가 에러 분기). 빈 keyspace (어떤 db 도 키 없음)
// 는 정상 — 0 반환.
func CountKeyspaceKeys(ctx context.Context, c *redis.Client) (int64, error) {
	info, err := c.Info(ctx, "keyspace").Result()
	if err != nil {
		return 0, err
	}
	return parseKeyspaceKeys(info), nil
}

// parseKeyspaceKeys — INFO keyspace 출력에서 모든 db 의 keys 합산.
//
// "dbN:keys=M,..." 패턴 line 만 인식. 그 외 line (헤더, 빈 줄) 무시.
// 정수 파싱 실패 시 해당 line 만 skip — 부분 결과 보존.
func parseKeyspaceKeys(info string) int64 {
	var total int64
	for line := range strings.SplitSeq(info, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "db") {
			continue
		}
		_, after, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.SplitSeq(after, ",")
		for kv := range fields {
			kv = strings.TrimSpace(kv)
			if !strings.HasPrefix(kv, "keys=") {
				continue
			}
			if n, err := strconv.ParseInt(strings.TrimPrefix(kv, "keys="), 10, 64); err == nil && n >= 0 {
				// Reject negative key counts — Valkey never emits them,
				// but the fuzz suite confirmed the parser previously
				// accepted `keys=-1` as a valid negative contribution.
				total += n
			}
			break
		}
	}
	return total
}
