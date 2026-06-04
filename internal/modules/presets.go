/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package modules — Valkey 공식 BSD module preset 해석 (ADR-0032).
//
// 외부 Redis Stack(RediSearch/RedisJSON/RedisBloom/RedisTimeSeries, RSALv2/SSPL)은
// Valkey BSD-3 와 라이선스 비호환이므로 *자체 재설계* 로 동등 기능을 제공하는 Valkey
// 공식 module(valkey-search=FT.*, valkey-json=JSON.*, valkey-bloom=BF.*)만 turnkey
// 로딩한다. allow-list 정확 일치로 위장 이미지를 차단한다.
package modules

import (
	"sort"
	"strings"
)

// DefaultBundleImage — Valkey 공식 module bundle(valkey 코어 + search/json/bloom).
// BSD-3. 개별 module image 대신 bundle 에서 .so 를 init-container 로 추출한다.
//
// NOTE(e2e 검증 의무): bundle 내 .so 경로는 9.x 기준. major 변경 시 SOPath 를
// `valkey-cli MODULE LIST` e2e 로 재확인한다(ADR-0032 검증 항목).
const DefaultBundleImage = "docker.io/valkey/valkey-bundle:9.0"

// ModulePreset — 공식 preset 의 출처 이미지 + 그 안의 .so 절대 경로.
// init-container 가 Image 의 SOPath 를 공유 emptyDir(/modules/<name>.so)로 cp 하고,
// valkey 컨테이너가 `--loadmodule /modules/<name>.so` 로 적재한다.
type ModulePreset struct {
	Image  string // .so 를 포함한 출처 이미지(init-container cp source image)
	SOPath string // 출처 이미지 안의 .so 절대 경로
}

// officialModulePresets — ADR-0032 BSD allow-list. 외부 Redis Stack 미포함.
var officialModulePresets = map[string]ModulePreset{
	"valkey-search": {Image: DefaultBundleImage, SOPath: "/usr/lib/valkey/libsearch.so"},
	"valkey-json":   {Image: DefaultBundleImage, SOPath: "/usr/lib/valkey/libjson.so"},
	"valkey-bloom":  {Image: DefaultBundleImage, SOPath: "/usr/lib/valkey/libvalkeybloom.so"},
}

// ResolveModulePreset — 공식 preset name → 이미지+경로. allow-list 엄격 일치.
// 미등록(외부 Redis Stack 포함) 이면 ok=false → 호출 측은 BYO Image 경로로 분기하거나 거부.
func ResolveModulePreset(name string) (ModulePreset, bool) {
	p, ok := officialModulePresets[name]
	return p, ok
}

// IsOfficialPreset — name 이 공식 BSD allow-list 에 있는지.
func IsOfficialPreset(name string) bool {
	_, ok := officialModulePresets[name]
	return ok
}

// OfficialPresetNames — allow-list 전체(정렬). webhook 거부 메시지/문서용.
func OfficialPresetNames() []string {
	names := make([]string, 0, len(officialModulePresets))
	for n := range officialModulePresets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// externalRedisStackModules — RSALv2/SSPL 라이선스 module 이름(소문자 정확 일치).
// Valkey BSD-3 와 비호환 (ADR-0032). preset 이름 우회를 차단한다.
var externalRedisStackModules = map[string]struct{}{
	"redisearch":      {},
	"redisjson":       {},
	"rejson":          {},
	"redisbloom":      {},
	"redistimeseries": {},
	"redisgraph":      {},
	"redisgears":      {},
}

// externalRedisStackImageMarkers — BYO image 경로로 외부 Redis Stack 을 끼워넣는
// 우회를 차단할 substring marker(소문자). registry/repo 어디에 등장해도 거부.
var externalRedisStackImageMarkers = []string{
	"redislabs/",
	"redis-stack",
	"/redisearch",
	"/rejson",
	"/redisjson",
	"/redisbloom",
	"/redistimeseries",
	"/redisgraph",
}

// IsExternalRedisStackModule — name 이 외부 Redis Stack module 인지(대소문자 무관).
// 공식 BSD preset 이름 우회 + BYO image 동반 우회 양쪽 차단용 (ADR-0032).
func IsExternalRedisStackModule(name string) bool {
	_, ok := externalRedisStackModules[strings.ToLower(name)]
	return ok
}

// IsExternalRedisStackImage — image 가 외부 Redis Stack 출처인지(대소문자 무관).
// 공식 valkey-bundle 및 임의 사내 이미지는 false (허용).
func IsExternalRedisStackImage(image string) bool {
	lower := strings.ToLower(image)
	for _, m := range externalRedisStackImageMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
