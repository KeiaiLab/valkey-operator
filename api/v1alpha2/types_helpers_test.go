/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// api/v1alpha1 의 순수 헬퍼 함수 회귀 보호.
// IsAutoFailoverEnabled / DefaultBackupObjectPath / IsTerminal / TotalNodes —
// 컨트롤러 분기 / 객체 경로 / 종료 판정의 contract 으로 외부 호출 다수.

package v1alpha2

import (
	"testing"

	"k8s.io/utils/ptr"
)

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
		replicasPerShard *int32
		want             int32
	}{
		{"3 shards, 1 replica each → 6", 3, ptr.To[int32](1), 6},
		{"6 shards, 2 replicas each → 18", 6, ptr.To[int32](2), 18},
		{"1 shard, explicit 0 replicas → 1 (masters-only)", 1, ptr.To[int32](0), 1},
		{"3 shards, explicit 0 replicas → 3 (masters-only)", 3, ptr.To[int32](0), 3},
		{"3 shards, nil replicas → 12 (nil defaults to 1)", 3, nil, 6},
		{"0 shards → 0", 0, ptr.To[int32](1), 0},
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

// TestValkeyClusterSpecGetReplicasPerShard — defect ④: nil(미지정)→1,
// 명시 0(masters-only)→0, 명시 2→2. 명시 0 이 default 1 로 덮이지 않음을 증명.
func TestValkeyClusterSpecGetReplicasPerShard(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rps  *int32
		want int32
	}{
		{"nil → 1 (omitted defaults to 1)", nil, 1},
		{"explicit 0 → 0 (masters-only preserved)", ptr.To[int32](0), 0},
		{"explicit 2 → 2", ptr.To[int32](2), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &ValkeyClusterSpec{ReplicasPerShard: c.rps}
			if got := s.GetReplicasPerShard(); got != c.want {
				t.Fatalf("GetReplicasPerShard() = %d, want %d", got, c.want)
			}
		})
	}
}

// TestValkeyClusterSpecTotalNodesMastersOnly — 명시 0 의 TotalNodes 가 shards
// 와 같음(masters-only)을 명시 검증. nil 은 shards*2.
func TestValkeyClusterSpecTotalNodesMastersOnly(t *testing.T) {
	t.Parallel()
	mastersOnly := &ValkeyClusterSpec{Shards: 5, ReplicasPerShard: ptr.To[int32](0)}
	if got := mastersOnly.TotalNodes(); got != 5 {
		t.Fatalf("masters-only TotalNodes() = %d, want 5 (== shards)", got)
	}
	omitted := &ValkeyClusterSpec{Shards: 5, ReplicasPerShard: nil}
	if got := omitted.TotalNodes(); got != 10 {
		t.Fatalf("omitted TotalNodes() = %d, want 10 (shards*2, nil→1)", got)
	}
}
