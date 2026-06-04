/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package autoupdate — Valkey 자동 버전 업데이트 정책의 순수 결정 로직.
// API 버전 패키지(v1alpha1/v1alpha2)와 controller 가 공유한다(중복/순환 import 회피).
package autoupdate

import (
	"strconv"
	"strings"
	"time"
)

// parseSemver — "9.0.4" / "8.1" 을 (major, minor, patch) 로 분해. patch 생략 시 0.
// 형식 위반 시 ok=false.
func parseSemver(v string) (maj, min, pat int, ok bool) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	var err error
	if maj, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, 0, false
	}
	if min, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, 0, false
	}
	if len(parts) == 3 {
		if pat, err = strconv.Atoi(parts[2]); err != nil {
			return 0, 0, 0, false
		}
	}
	return maj, min, pat, true
}

// semverLess — (amaj,amin,apat) < (bmaj,bmin,bpat).
func semverLess(amaj, amin, apat, bmaj, bmin, bpat int) bool {
	if amaj != bmaj {
		return amaj < bmaj
	}
	if amin != bmin {
		return amin < bmin
	}
	return apat < bpat
}

// ResolveTarget — current 버전에서 channel 제약 내 catalog 최신 안전 버전을 고른다.
//
//	channel "patch"(또는 ""): 동일 major.minor 내 최신 patch
//	channel "minor":          동일 major 내 최신 minor.patch
//
// major 상승은 절대 자동화하지 않는다(운영자 명시 필요). 다운그레이드도 하지 않는다.
// hasUpdate=false 면 target 은 무의미.
func ResolveTarget(current, channel string, catalog []string) (target string, hasUpdate bool) {
	cmaj, cmin, cpat, ok := parseSemver(current)
	if !ok {
		return "", false
	}
	if channel == "" {
		channel = "patch"
	}

	bestMaj, bestMin, bestPat := cmaj, cmin, cpat // current 보다 높아야 채택
	for _, cand := range catalog {
		pmaj, pmin, ppat, ok := parseSemver(cand)
		if !ok {
			continue
		}
		if pmaj != cmaj { // major 자동 상승 금지
			continue
		}
		if channel == "patch" && pmin != cmin { // patch 채널은 minor 고정
			continue
		}
		if !semverLess(bestMaj, bestMin, bestPat, pmaj, pmin, ppat) { // 더 높은 것만
			continue
		}
		target, bestMaj, bestMin, bestPat = cand, pmaj, pmin, ppat
	}
	return target, target != ""
}

// IsWithinWindow — now(시:분만 사용)가 window 안인지 판정한다.
// window 형식 "HH:MM-HH:MM"(UTC). 빈 값이면 항상 허용. 자정 넘김(22:00-02:00) 지원.
// 시작 경계 포함, 끝 경계 배타. 형식 위반은 안전하게 false(업데이트 보류).
func IsWithinWindow(window string, now time.Time) bool {
	if window == "" {
		return true
	}
	parts := strings.SplitN(window, "-", 2)
	if len(parts) != 2 {
		return false
	}
	start, err1 := time.Parse("15:04", strings.TrimSpace(parts[0]))
	end, err2 := time.Parse("15:04", strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return false
	}
	nowMin := now.Hour()*60 + now.Minute()
	startMin := start.Hour()*60 + start.Minute()
	endMin := end.Hour()*60 + end.Minute()
	if startMin <= endMin {
		return nowMin >= startMin && nowMin < endMin
	}
	// 자정 넘김: [start, 24:00) ∪ [00:00, end)
	return nowMin >= startMin || nowMin < endMin
}

// ResolveVersion — AutoUpdate 정책 전체(window + channel + catalog)를 적용해 최종 버전을 결정.
// enabled 여부는 호출 측이 거른다(spec 헬퍼). window 밖 / 업데이트 없음 이면 (base, false).
func ResolveVersion(base, channel, window string, catalog []string, now time.Time) (version string, applied bool) {
	if !IsWithinWindow(window, now) {
		return base, false
	}
	target, has := ResolveTarget(base, channel, catalog)
	if !has {
		return base, false
	}
	return target, true
}
