/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// AutoUpdate 버전 결정 순수 로직 회귀 보호.
// channel 제약(patch/minor) 내 catalog 최신 안전 버전 선택 + maintenance window
// 판정 + 둘의 통합(ResolveVersion). major 상승은 절대 자동화하지 않는다.

package autoupdate

import (
	"testing"
	"time"
)

func TestResolveTarget(t *testing.T) {
	t.Parallel()
	catalog := []string{"8.0.9", "8.1.6", "8.1.7", "9.0.4", "9.0.7", "9.1.2", "10.0.0"}
	cases := []struct {
		name       string
		current    string
		channel    string
		wantTarget string
		wantUpdate bool
	}{
		{"patch: 9.0.4 → 9.0.7 동일 major.minor 최신", "9.0.4", "patch", "9.0.7", true},
		{"patch: minor 상승(9.1.2) 무시", "9.0.7", "patch", "", false},
		{"minor: 9.0.4 → 9.1.2 동일 major 최신", "9.0.4", "minor", "9.1.2", true},
		{"minor: major(10.0.0) 자동 금지", "9.1.2", "minor", "", false},
		{"minor: 9.0.7 → 9.1.2", "9.0.7", "minor", "9.1.2", true},
		{"다운그레이드 없음", "9.1.2", "patch", "", false},
		{"patch: 8.1.6 → 8.1.7", "8.1.6", "patch", "8.1.7", true},
		{"빈 channel 은 patch 취급", "9.0.4", "", "9.0.7", true},
		{"current 가 카탈로그에 없어도 비교", "9.0.5", "patch", "9.0.7", true},
		{"형식 위반 current 거부", "garbage", "patch", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			target, has := ResolveTarget(c.current, c.channel, catalog)
			if has != c.wantUpdate {
				t.Fatalf("hasUpdate: got %v, want %v (target=%q)", has, c.wantUpdate, target)
			}
			if has && target != c.wantTarget {
				t.Fatalf("target: got %q, want %q", target, c.wantTarget)
			}
		})
	}
}

func TestIsWithinWindow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		window string
		now    string
		want   bool
	}{
		{"빈 window 는 항상 허용", "", "03:00", true},
		{"02:00-04:00 안", "02:00-04:00", "03:00", true},
		{"02:00-04:00 밖", "02:00-04:00", "05:00", false},
		{"시작 경계 포함", "02:00-04:00", "02:00", true},
		{"끝 경계 배타", "02:00-04:00", "04:00", false},
		{"자정 넘김 (23시) 안", "22:00-02:00", "23:00", true},
		{"자정 넘김 (01시) 안", "22:00-02:00", "01:00", true},
		{"자정 넘김 (12시) 밖", "22:00-02:00", "12:00", false},
		{"형식 위반 거부", "garbage", "03:00", false},
		{"부분 형식 위반 거부", "02:00-xx", "03:00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			now, err := time.Parse("15:04", c.now)
			if err != nil {
				t.Fatalf("bad test now: %v", err)
			}
			if got := IsWithinWindow(c.window, now); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestResolveVersion(t *testing.T) {
	t.Parallel()
	catalog := []string{"9.0.4", "9.0.7", "9.1.2"}
	in, _ := time.Parse("15:04", "03:00")  // 02:00-04:00 안
	out, _ := time.Parse("15:04", "05:00") // 02:00-04:00 밖
	cases := []struct {
		name        string
		base        string
		channel     string
		window      string
		now         time.Time
		wantVer     string
		wantApplied bool
	}{
		{"window 안 patch → 9.0.7", "9.0.4", "patch", "02:00-04:00", in, "9.0.7", true},
		{"window 밖 → base 유지", "9.0.4", "patch", "02:00-04:00", out, "9.0.4", false},
		{"상시 window minor → 9.1.2", "9.0.4", "minor", "", out, "9.1.2", true},
		{"이미 최신 → base 유지", "9.1.2", "minor", "", in, "9.1.2", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ver, applied := ResolveVersion(c.base, c.channel, c.window, catalog, c.now)
			if ver != c.wantVer || applied != c.wantApplied {
				t.Fatalf("got (%q,%v), want (%q,%v)", ver, applied, c.wantVer, c.wantApplied)
			}
		})
	}
}

func TestIsMajorUpgrade(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		old, new string
		want     bool
	}{
		{"major 상승 9→10 차단", "9.0.4", "10.0.0", true},
		{"minor 상승은 허용", "9.0.4", "9.1.0", false},
		{"patch 상승은 허용", "9.0.4", "9.0.5", false},
		{"동일 버전", "9.0.4", "9.0.4", false},
		{"major 하락은 별도 검증 위임", "10.0.0", "9.0.4", false},
		{"new 파싱 불가 → false(다른 검증 위임)", "9.0.4", "bogus", false},
		{"old 빈 값 → false", "", "10.0.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := IsMajorUpgrade(c.old, c.new); got != c.want {
				t.Fatalf("IsMajorUpgrade(%q,%q): got %v, want %v", c.old, c.new, got, c.want)
			}
		})
	}
}
