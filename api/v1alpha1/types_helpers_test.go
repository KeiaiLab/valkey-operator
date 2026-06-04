/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// api/v1alpha1 의 순수 헬퍼 함수 회귀 보호.
// IsAutoFailoverEnabled / DefaultBackupObjectPath / IsTerminal / TotalNodes —
// 컨트롤러 분기 / 객체 경로 / 종료 판정의 contract 으로 외부 호출 다수.

package v1alpha1

import "testing"

func TestValkeySpecIsAutoFailoverEnabled(t *testing.T) {
	t.Parallel()
	tr, fa := true, false
	cases := []struct {
		name string
		ptr  *bool
		want bool
	}{
		{"nil → default true", nil, true},
		{"explicit true", &tr, true},
		{"explicit false", &fa, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &ValkeySpec{AutoFailover: c.ptr}
			if got := s.IsAutoFailoverEnabled(); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestDefaultBackupObjectPath(t *testing.T) {
	t.Parallel()
	got := DefaultBackupObjectPath("nightly", "2026-05-06T00-00-00Z")
	want := "nightly/2026-05-06T00-00-00Z/dump.rdb"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValkeyBackupIsTerminal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		phase BackupPhase
		want  bool
	}{
		{BackupPhaseCompleted, true},
		{BackupPhaseFailed, true},
		{BackupPhase("Pending"), false},
		{BackupPhase("Running"), false},
		{BackupPhase(""), false},
	}
	for _, c := range cases {
		t.Run(string(c.phase), func(t *testing.T) {
			t.Parallel()
			b := &ValkeyBackup{}
			b.Status.Phase = c.phase
			if got := b.IsTerminal(); got != c.want {
				t.Fatalf("phase=%q got %v, want %v", c.phase, got, c.want)
			}
		})
	}
}

func TestValkeyRestoreIsTerminal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		phase RestorePhase
		want  bool
	}{
		{RestorePhaseCompleted, true},
		{RestorePhaseFailed, true},
		{RestorePhase("Pending"), false},
		{RestorePhase("Restoring"), false},
		{RestorePhase(""), false},
	}
	for _, c := range cases {
		t.Run(string(c.phase), func(t *testing.T) {
			t.Parallel()
			r := &ValkeyRestore{}
			r.Status.Phase = c.phase
			if got := r.IsTerminal(); got != c.want {
				t.Fatalf("phase=%q got %v, want %v", c.phase, got, c.want)
			}
		})
	}
}

func TestValkeyClusterSpecTotalNodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		shards           int32
		replicasPerShard int32
		want             int32
	}{
		{"3 shards, 1 replica each → 6", 3, 1, 6},
		{"6 shards, 2 replicas each → 18", 6, 2, 18},
		{"1 shard, 0 replicas → 1", 1, 0, 1},
		{"0 shards → 0", 0, 1, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &ValkeyClusterSpec{Shards: c.shards, ReplicasPerShard: c.replicasPerShard}
			if got := s.TotalNodes(); got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}
