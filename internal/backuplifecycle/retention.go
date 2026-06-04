/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package backuplifecycle — cross-region backup 의 자동 lifecycle(retention) 결정
// 로직 (ROADMAP 2.x). ValkeyBackup controller / S3 lifecycle 적용기가 이 결정을
// 사용해 만료 대상을 고른다. S3/SSE-KMS 인프라와 분리된 순수 함수.
package backuplifecycle

import "sort"

// BackupInfo — 보존 판정에 필요한 backup 메타데이터.
type BackupInfo struct {
	Name      string // backup 식별자(삭제 대상 키)
	CreatedAt int64  // 생성 시각(unix sec)
}

// SelectExpired — retention 정책 초과 backup 을 만료 대상으로 선택한다(오래된 순).
//
//	maxCount > 0: 개수가 maxCount 초과 시 가장 오래된 (n-maxCount)개 만료
//	maxAge  > 0: now-CreatedAt 가 maxAge(초) 초과인 backup 만료
//
// 두 조건은 합집합. 둘 다 0이면 정책 비활성(만료 없음).
func SelectExpired(backups []BackupInfo, maxCount int, maxAgeSec int64, now int64) []string {
	sorted := make([]BackupInfo, len(backups))
	copy(sorted, backups)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CreatedAt < sorted[j].CreatedAt })

	expired := make(map[string]bool, len(sorted))
	if maxCount > 0 && len(sorted) > maxCount {
		for i := 0; i < len(sorted)-maxCount; i++ {
			expired[sorted[i].Name] = true
		}
	}
	if maxAgeSec > 0 {
		for _, b := range sorted {
			if now-b.CreatedAt > maxAgeSec {
				expired[b.Name] = true
			}
		}
	}

	var result []string
	for _, b := range sorted { // 오래된 순 결정론적 출력
		if expired[b.Name] {
			result = append(result, b.Name)
		}
	}
	return result
}
