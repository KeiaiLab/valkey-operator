/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Multi-cluster federation 라우팅 결정(SelectPrimary) 회귀 보호 (ROADMAP 2.x).
// healthy member 중 최고 weight 를 primary 로 선택, 동률 시 이름 순(결정론적).
// 순수 함수 — kubeconfig/cross-cluster 인프라 없이 핵심 결정을 검증.

package federation

import "testing"

func TestSelectPrimary(t *testing.T) {
	t.Parallel()
	members := []Member{
		{Name: "east", Weight: 10, Region: "us-east"},
		{Name: "west", Weight: 20, Region: "us-west"},
		{Name: "eu", Weight: 20, Region: "eu-central"},
	}
	cases := []struct {
		name    string
		healthy map[string]bool
		want    string
		wantOK  bool
	}{
		{"최고 weight + 동률 시 이름 순", map[string]bool{"east": true, "west": true, "eu": true}, "eu", true},
		{"unhealthy 제외 → 남은 최고", map[string]bool{"east": true, "west": false, "eu": false}, "east", true},
		{"단일 healthy", map[string]bool{"west": true}, "west", true},
		{"healthy 없음 → false", map[string]bool{}, "", false},
		{"전부 unhealthy → false", map[string]bool{"east": false, "west": false, "eu": false}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok := SelectPrimary(members, c.healthy)
			if ok != c.wantOK {
				t.Fatalf("ok: got %v, want %v (primary=%q)", ok, c.wantOK, got)
			}
			if ok && got != c.want {
				t.Fatalf("primary: got %q, want %q", got, c.want)
			}
		})
	}
}

func TestSelectPrimary_EmptyMembers(t *testing.T) {
	t.Parallel()
	if _, ok := SelectPrimary(nil, map[string]bool{"x": true}); ok {
		t.Fatal("members 가 비면 primary 선택 불가")
	}
}
