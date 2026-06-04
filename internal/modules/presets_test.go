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

// 외부 Redis Stack module 이름 판별 — preset 이름 우회 + BYO image 우회 양쪽 차단.
// RSALv2/SSPL 라이선스는 Valkey BSD-3 와 비호환 (ADR-0032). 사용자가 BYO image 로
// redislabs/redisearch 를 끼워넣어 라이선스를 회피하는 경로까지 거부해야 한다.
func TestIsExternalRedisStackModule(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want bool
	}{
		{"redisearch", true},
		{"redisjson", true},
		{"rejson", true},
		{"redisbloom", true},
		{"redistimeseries", true},
		{"redisgraph", true},
		{"redisgears", true},
		{"RediSearch", true},     // 대소문자 무관
		{"valkey-search", false}, // 공식 BSD preset
		{"valkey-json", false},
		{"valkey-bloom", false},
		{"custom-mod", false}, // 임의 BYO 이름은 외부 Stack 아님
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := IsExternalRedisStackModule(c.name); got != c.want {
				t.Fatalf("IsExternalRedisStackModule(%q): got %v, want %v", c.name, got, c.want)
			}
		})
	}
}

// 외부 Redis Stack image 판별 — BYO image 경로로 라이선스 비호환 이미지를 끼워넣는
// 우회를 차단한다(ADR-0032). 공식 valkey-bundle 및 임의 사내 이미지는 허용.
func TestIsExternalRedisStackImage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		image string
		want  bool
	}{
		{"redislabs/redisearch:latest", true},
		{"docker.io/redislabs/rejson:2.6", true},
		{"redis/redis-stack-server:7.2.0", true},
		{"redis-stack:latest", true},
		{"docker.io/valkey/valkey-bundle:9.0", false},     // 공식 BSD bundle
		{"ghcr.io/keiailab/custom-valkey-mod:1.0", false}, // 임의 사내 BYO 허용
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.image, func(t *testing.T) {
			t.Parallel()
			if got := IsExternalRedisStackImage(c.image); got != c.want {
				t.Fatalf("IsExternalRedisStackImage(%q): got %v, want %v", c.image, got, c.want)
			}
		})
	}
}
