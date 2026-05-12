/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package aoftime — Valkey AOF (Append-Only File) format 의 timestamp marker 파싱.
//
// Valkey 8.0+ 의 `aof-timestamp-enabled yes` 활성 시, AOF 파일은 RESP commands
// 사이에 `#TS:<unix-seconds>\r\n` marker 포함. 본 패키지는 marker 위치를 식별해
// *Point-In-Time Recovery 의 truncation 위치* 를 반환.
//
// PITR 사용:
//
//	bytes, _ := os.ReadFile("dump.aof")
//	cutoff := aoftime.TruncateOffset(bytes, time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC))
//	// cutoff bytes 까지 만 valkey-cli --pipe 로 replay → 해당 시각까지의 데이터 복원
//
// ADR-0040 §6 의 PITR phase 2 enterprise-tier 항목 — phase 1 (PR #54 API/webhook)
// 의 *replay 측 missing piece*. 외부 `valkey-aof-trim` 도구 의존 제거.
//
// 본 패키지는 *parse-only* — 실제 truncate 는 caller 가 byte slice 자르기.
// reconciler 통합은 별도 후속 (controller 가 본 패키지 호출 → truncated bytes 를
// PVC 에 write → init container 가 valkey-cli --pipe).
package aoftime

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"time"
)

// timestampPrefix — Valkey AOF 의 timestamp marker 시작 byte sequence.
// `aof-timestamp-enabled yes` 활성 시 RESP commands 앞에 `#TS:<unix-seconds>\r\n` 삽입.
var timestampPrefix = []byte("#TS:")

// crlf — RESP / AOF 의 line terminator.
var crlf = []byte("\r\n")

// TruncateOffset — AOF byte slice 에서 *cutoff 시각까지의 데이터* 를 보존하는
// truncation offset 반환.
//
// 동작:
//   - cutoff 직후 #TS: marker 를 발견하면 그 marker 의 *시작 위치* 반환
//     (해당 marker 와 그 후속 commands 모두 제외)
//   - cutoff 이후 marker 가 없으면 len(aof) 반환 (전체 보존)
//   - aof 가 timestamp marker 없으면 len(aof) 반환 (truncate 불가, 전체 replay)
//
// 반환된 offset 까지 ([0:offset]) 만 valkey-cli --pipe 에 stream → cutoff 시각
// 까지의 데이터 만 복원.
//
// 가정:
//   - AOF 가 `aof-timestamp-enabled yes` 로 작성됨 (Valkey 8.0+)
//   - timestamp 단위는 unix seconds (Valkey 표준)
//   - cutoff 보다 *이전 또는 같은* marker 의 commands 는 보존
//
// 미가정:
//   - 외부 도구 의존 (valkey-aof-trim 불필요)
//   - RESP command 의 *내부* 파싱 — marker 만 식별 (RESP-level skip 없음 — caller
//     의 valkey-cli --pipe 가 RESP 복원 책임)
func TruncateOffset(aof []byte, cutoff time.Time) int {
	if len(aof) == 0 {
		return 0
	}

	cutoffUnix := cutoff.Unix()
	pos := 0
	for pos < len(aof) {
		// 다음 #TS: marker 위치.
		idx := bytes.Index(aof[pos:], timestampPrefix)
		if idx < 0 {
			// marker 없음 — 전체 보존 (이미 cutoff 이전 commands 만).
			return len(aof)
		}
		markerStart := pos + idx
		// `#TS:` 이후 \r\n 까지의 byte 가 unix seconds 문자열.
		valueStart := markerStart + len(timestampPrefix)
		crlfIdx := bytes.Index(aof[valueStart:], crlf)
		if crlfIdx < 0 {
			// 손상된 marker (truncated file end) — 그 위치까지 보존.
			return markerStart
		}
		valueEnd := valueStart + crlfIdx
		ts, err := strconv.ParseInt(string(aof[valueStart:valueEnd]), 10, 64)
		if err != nil {
			// invalid timestamp — 다음 marker 검색 (skip).
			pos = valueEnd + len(crlf)
			continue
		}
		if ts > cutoffUnix {
			// 이 marker 부터의 commands 는 cutoff 이후 — 본 marker 시작 위치까지
			// 만 보존.
			return markerStart
		}
		// 본 marker 는 cutoff 이전 — 다음 marker 검색.
		pos = valueEnd + len(crlf)
	}
	return len(aof)
}

// HasTimestamps — AOF 가 timestamp marker 를 포함하는지. false 면 PITR 불가
// (전체 AOF replay 만 가능, RDB 와 동일 semantic).
func HasTimestamps(aof []byte) bool {
	return bytes.Contains(aof, timestampPrefix)
}

// TruncateAOFFile — file-level helper. srcPath 의 AOF 를 읽어 cutoff 시각까지
// 보존한 truncated AOF 를 dstPath 에 write. PITR phase 2 reconciler 통합 진입점.
//
// backup-aware 변형은 TruncateAOFFileWithBackup. 본 함수는 thin wrapper.
func TruncateAOFFile(srcPath, dstPath string, cutoff time.Time) (bytesWritten int, truncated bool, err error) {
	return TruncateAOFFileWithBackup(srcPath, dstPath, "", cutoff)
}

// TruncateAOFFileWithBackup — TruncateAOFFile 의 *rollback-aware* 버전.
//
// backupPath != "" 시 truncate 전 *원본* AOF 를 backupPath 에 복사. PITR
// rollback (replay 실패 시 backup 시점 fallback) 진입점.
//
// 운영 시나리오:
//   - reconciler 가 backupPath = `<dstPath>.original` 명시
//   - init container 가 truncated AOF 로 부팅 시도
//   - CrashLoopBackOff 감지 시 reconciler 가 RollbackFromBackup 호출
//     (전체 AOF replay 로 fallback, PITR 효과 미달성하지만 데이터 보존)
//
// backupPath="" 시 동작은 TruncateAOFFile 와 동일 (backup 미생성).
func TruncateAOFFileWithBackup(srcPath, dstPath, backupPath string, cutoff time.Time) (bytesWritten int, truncated bool, err error) {
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return 0, false, fmt.Errorf("read src AOF %s: %w", srcPath, err)
	}
	if backupPath != "" {
		if err := os.WriteFile(backupPath, src, 0o600); err != nil {
			return 0, false, fmt.Errorf("write backup AOF %s: %w", backupPath, err)
		}
	}
	if !HasTimestamps(src) {
		// timestamps 부재 — 전체 복사 (PITR 효과 없음, 사용자 안내).
		if err := os.WriteFile(dstPath, src, 0o600); err != nil {
			return 0, false, fmt.Errorf("write dst AOF %s: %w", dstPath, err)
		}
		return len(src), false, nil
	}
	offset := TruncateOffset(src, cutoff)
	out := src[:offset]
	if err := os.WriteFile(dstPath, out, 0o600); err != nil {
		return 0, true, fmt.Errorf("write truncated AOF %s: %w", dstPath, err)
	}
	return len(out), true, nil
}

// RollbackFromBackup — PITR replay 실패 시 backup 원본을 dstPath 로 복원.
// 본 함수는 backup 파일 *읽기 + dst 덮어쓰기* 만 — caller 가 init container
// 재시작 책임 (kubectl rollout restart 또는 STS pod delete).
func RollbackFromBackup(backupPath, dstPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup AOF %s: %w", backupPath, err)
	}
	if err := os.WriteFile(dstPath, data, 0o600); err != nil {
		return fmt.Errorf("rollback write %s: %w", dstPath, err)
	}
	return nil
}
