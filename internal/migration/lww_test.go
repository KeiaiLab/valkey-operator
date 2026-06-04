/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Online schema-less migration 의 LWW(Last-Write-Wins) 충돌 해소 회귀 보호 (ROADMAP 2.x).
// RDB diff merge 중 동일 key 가 source/target 양쪽에 있을 때 timestamp 최신 우선,
// 동률 시 source 사전순(결정론적)으로 split-brain 없이 수렴.

package migration

import "testing"

func vv(val string, ts int64, src string) VersionedValue {
	return VersionedValue{Value: val, Timestamp: ts, Source: src}
}

func TestResolveLWW(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b VersionedValue
		want string // 기대 Value
	}{
		{"a 가 최신", vv("x", 200, "s1"), vv("y", 100, "s2"), "x"},
		{"b 가 최신", vv("x", 100, "s1"), vv("y", 200, "s2"), "y"},
		{"동률 → source 사전순(s1<s2 → s1 값)", vv("x", 100, "s2"), vv("y", 100, "s1"), "y"},
		{"동률 + source 동일 → a(안정)", vv("x", 100, "s1"), vv("y", 100, "s1"), "x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveLWW(c.a, c.b)
			if got.Value != c.want {
				t.Fatalf("got %q, want %q", got.Value, c.want)
			}
			// 교환법칙: ResolveLWW(a,b) == ResolveLWW(b,a) (결정론적 수렴)
			if ResolveLWW(c.b, c.a).Value != got.Value {
				t.Fatalf("교환법칙 위반: (a,b)=%q (b,a)=%q", got.Value, ResolveLWW(c.b, c.a).Value)
			}
		})
	}
}
