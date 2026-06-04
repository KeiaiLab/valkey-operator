/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Cross-region backup 의 자동 lifecycle(retention) 정책 회귀 보호 (ROADMAP 2.x).
// maxCount(개수 상한) + maxAge(보존 기간) 초과 backup 을 만료 대상으로 선택.
// 순수 함수 — S3/SSE-KMS 인프라 없이 retention 결정을 단위 검증.

package backuplifecycle

import "testing"

func bk(name string, createdAt int64) BackupInfo {
	return BackupInfo{Name: name, CreatedAt: createdAt}
}

func TestSelectExpired(t *testing.T) {
	t.Parallel()
	now := int64(1000)
	// 오래된 순: old(100) < mid(500) < recent(900)
	backups := []BackupInfo{bk("recent", 900), bk("old", 100), bk("mid", 500)}

	t.Run("maxCount 초과 → 오래된 것부터 만료", func(t *testing.T) {
		t.Parallel()
		// maxCount=2 → 3개 중 1개(가장 오래된 old) 만료. maxAge=0(무제한)
		got := SelectExpired(backups, 2, 0, now)
		if len(got) != 1 || got[0] != "old" {
			t.Fatalf("maxCount 초과: got %v, want [old]", got)
		}
	})

	t.Run("maxAge 초과 → 만료", func(t *testing.T) {
		t.Parallel()
		// maxAge=600 → now-createdAt > 600 인 old(900>600) 만료. maxCount=0(무제한)
		got := SelectExpired(backups, 0, 600, now)
		if len(got) != 1 || got[0] != "old" {
			t.Fatalf("maxAge 초과: got %v, want [old]", got)
		}
	})

	t.Run("maxCount + maxAge 합집합", func(t *testing.T) {
		t.Parallel()
		// maxCount=1 → recent 제외 2개(old,mid) 만료. maxAge=600 → old 도 포함. 합집합 {old,mid}
		got := SelectExpired(backups, 1, 600, now)
		if len(got) != 2 {
			t.Fatalf("합집합: got %v, want 2개(old,mid)", got)
		}
	})

	t.Run("정책 비활성(둘 다 0) → 만료 없음", func(t *testing.T) {
		t.Parallel()
		if got := SelectExpired(backups, 0, 0, now); len(got) != 0 {
			t.Fatalf("비활성: got %v, want []", got)
		}
	})
}
