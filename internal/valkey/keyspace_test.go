/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// INFO keyspace 응답 파싱 단위 테스트. redis client 의존 없는 순수 함수만.
package valkey

import "testing"

func TestParseKeyspaceKeys_singleDB(t *testing.T) {
	in := "# Keyspace\ndb0:keys=42,expires=0,avg_ttl=0\n"
	if got := parseKeyspaceKeys(in); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestParseKeyspaceKeys_multiDB(t *testing.T) {
	in := "# Keyspace\ndb0:keys=42,expires=0,avg_ttl=0\ndb1:keys=10,expires=2,avg_ttl=0\ndb15:keys=1,expires=0,avg_ttl=0\n"
	if got := parseKeyspaceKeys(in); got != 53 {
		t.Fatalf("expected 53 (42+10+1), got %d", got)
	}
}

func TestParseKeyspaceKeys_empty(t *testing.T) {
	in := "# Keyspace\n"
	if got := parseKeyspaceKeys(in); got != 0 {
		t.Fatalf("expected 0 for empty keyspace, got %d", got)
	}
}

func TestParseKeyspaceKeys_emptyString(t *testing.T) {
	if got := parseKeyspaceKeys(""); got != 0 {
		t.Fatalf("expected 0 for empty string, got %d", got)
	}
}

func TestParseKeyspaceKeys_invalidFormat(t *testing.T) {
	// 정수 파싱 실패 시 해당 line 만 skip.
	in := "# Keyspace\ndb0:keys=abc,expires=0\ndb1:keys=5,expires=0\n"
	if got := parseKeyspaceKeys(in); got != 5 {
		t.Fatalf("expected 5 (skip invalid db0, count db1), got %d", got)
	}
}

func TestParseKeyspaceKeys_unrelatedSection(t *testing.T) {
	// "db" 로 시작하지 않는 line 은 무시.
	in := "# Server\nredis_version:7.4.0\n# Keyspace\ndb0:keys=3,expires=0,avg_ttl=0\n"
	if got := parseKeyspaceKeys(in); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}

func TestParseKeyspaceKeys_caseSensitiveDBPrefix(t *testing.T) {
	// 대소문자 정확히 "db" — 다른 prefix 무시.
	in := "DB0:keys=10,expires=0\nDb0:keys=10,expires=0\nfoo:keys=10,expires=0\n"
	if got := parseKeyspaceKeys(in); got != 0 {
		t.Fatalf("expected 0 (case-sensitive), got %d", got)
	}
}

func TestParseKeyspaceKeys_keysFieldNotFirst(t *testing.T) {
	// keys= 가 두 번째 field 에 있어도 인식.
	in := "db0:expires=0,keys=7,avg_ttl=0\n"
	if got := parseKeyspaceKeys(in); got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}
