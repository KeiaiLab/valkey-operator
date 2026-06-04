/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// AutoUpdate spec 헬퍼 contract 회귀 보호(v1alpha2 = conversion hub).
// 순수 버전 결정 로직은 internal/autoupdate 에서 별도 테스트.

package v1alpha2

import "testing"

func TestValkeySpecIsAutoUpdateEnabled(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		au   *AutoUpdateSpec
		want bool
	}{
		{"nil → false", nil, false},
		{"enabled=false → false", &AutoUpdateSpec{Enabled: false}, false},
		{"enabled=true → true", &AutoUpdateSpec{Enabled: true}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &ValkeySpec{AutoUpdate: c.au}
			if got := s.IsAutoUpdateEnabled(); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestValkeySpecAutoUpdateChannel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		au   *AutoUpdateSpec
		want string
	}{
		{"nil → patch 기본", nil, "patch"},
		{"빈 channel → patch", &AutoUpdateSpec{Enabled: true}, "patch"},
		{"minor 명시", &AutoUpdateSpec{Enabled: true, Channel: "minor"}, "minor"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &ValkeySpec{AutoUpdate: c.au}
			if got := s.AutoUpdateChannel(); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
