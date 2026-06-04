/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 자체 시크릿 로테이션 결정(decideRotation) 회귀 보호.
// 순수 함수 — interval 형식/baseline/경과 를 "none"/"baseline"/"rotate" 로 판정.
// 실제 Secret 갱신은 reconcile 가 이 결정에 따라 수행(envtest).

package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDecideRotation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *metav1.Time { return &metav1.Time{Time: now.Add(-d)} }
	cases := []struct {
		name     string
		interval string
		last     *metav1.Time
		want     string
	}{
		{"빈 interval → none(비활성)", "", ago(100 * time.Hour), "none"},
		{"형식 위반 → none(안전)", "garbage", ago(100 * time.Hour), "none"},
		{"음수 → none", "-1h", ago(100 * time.Hour), "none"},
		{"last nil → baseline", "1h", nil, "baseline"},
		{"last zero → baseline", "1h", &metav1.Time{}, "baseline"},
		{"경과 초과 → rotate", "1h", ago(2 * time.Hour), "rotate"},
		{"경계 도달 → rotate", "1h", ago(1 * time.Hour), "rotate"},
		{"미경과 → none", "1h", ago(30 * time.Minute), "none"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := decideRotation(c.interval, c.last, now); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
