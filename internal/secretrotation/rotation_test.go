/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 자체 시크릿 로테이션 결정 로직 회귀 보호.
// ESO 위임이 아닌 operator-managed 주기 로테이션 — interval 경과 판정.
// 첫 reconcile(lastRotation zero)은 baseline 기록만 하고 로테이션하지 않는다(안전).

package secretrotation

import (
	"testing"
	"time"
)

func TestShouldRotate(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	cases := []struct {
		name         string
		lastRotation time.Time
		interval     time.Duration
		now          time.Time
		want         bool
	}{
		{"interval 0 → 비활성", base, 0, base.Add(100 * day), false},
		{"interval 음수 → 비활성", base, -day, base.Add(100 * day), false},
		{"lastRotation zero → baseline(로테이션 X)", time.Time{}, 30 * day, base, false},
		{"경과 미달 → false", base, 30 * day, base.Add(29 * day), false},
		{"경계 정확 도달 → true", base, 30 * day, base.Add(30 * day), true},
		{"경과 초과 → true", base, 30 * day, base.Add(31 * day), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ShouldRotate(c.lastRotation, c.interval, c.now); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestGeneratePassword(t *testing.T) {
	t.Parallel()
	a, err := GeneratePassword()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(a) < 40 { // 32바이트 base64(raw url) ≈ 43자
		t.Fatalf("비밀번호가 너무 짧음: %d자", len(a))
	}
	b, _ := GeneratePassword()
	if a == b {
		t.Fatal("두 호출이 동일 — 암호학적 난수가 아니다")
	}
}
