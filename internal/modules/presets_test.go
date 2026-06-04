/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 공식 module preset allow-list contract 회귀 보호 (ADR-0032).
// 라이선스 안전: BSD-3 Valkey 공식 module(valkey-search/json/bloom)만 turnkey
// 허용하고 외부 Redis Stack(RediSearch/RedisJSON, RSALv2/SSPL)은 거부한다.

package modules

import "testing"

func TestResolveModulePreset(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		wantOK bool
	}{
		{"valkey-search", true},
		{"valkey-json", true},
		{"valkey-bloom", true},
		{"redisearch", false},         // 외부 Redis Stack — RSALv2/SSPL 라이선스 거부
		{"redisjson", false},          // 외부 Redis Stack 거부
		{"RedisBloom", false},         // 대소문자 변형도 거부
		{"", false},                   // 빈 이름 거부
		{"valkey-search-evil", false}, // allow-list 엄격 일치(부분 매칭 금지)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			preset, ok := ResolveModulePreset(c.name)
			if ok != c.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, c.wantOK)
			}
			if ok && (preset.Image == "" || preset.SOPath == "") {
				t.Fatalf("공식 preset 은 Image+SOPath 필수: %+v", preset)
			}
		})
	}
}

func TestIsOfficialPreset(t *testing.T) {
	t.Parallel()
	if !IsOfficialPreset("valkey-search") {
		t.Fatal("valkey-search 는 공식 BSD preset")
	}
	if IsOfficialPreset("redisearch") {
		t.Fatal("redisearch 는 외부 Redis Stack — 거부해야 한다")
	}
}
