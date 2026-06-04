/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package aoftime

import (
	"os"
	"strings"
	"testing"
	"time"
)

// makeAOF — test helper. timestamp marker + 간단한 RESP SET 명령들을 결합.
// `*3\r\n$3\r\nSET\r\n$N\r\n<key>\r\n$M\r\n<val>\r\n` 형식.
func makeAOF(entries ...string) string { return strings.Join(entries, "") }

func ts(unixSec int64) string { return "#TS:" + intToStr(unixSec) + "\r\n" }

func intToStr(n int64) string {
	// strconv import 줄이려고 minimal.
	return time.Unix(n, 0).UTC().Format("") + simpleItoa(n)
}

func simpleItoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var s []byte
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = append([]byte{byte('0' + n%10)}, s...)
		n /= 10
	}
	if neg {
		s = append([]byte{'-'}, s...)
	}
	return string(s)
}

// setCommand — RESP SET k v 명령 byte sequence.
func setCommand(k, v string) string {
	return "*3\r\n$3\r\nSET\r\n$" + simpleItoa(int64(len(k))) + "\r\n" + k + "\r\n$" +
		simpleItoa(int64(len(v))) + "\r\n" + v + "\r\n"
}

func TestTruncateOffset_no_timestamps_returns_full_length(t *testing.T) {
	aof := []byte(setCommand("a", "1") + setCommand("b", "2"))
	off := TruncateOffset(aof, time.Unix(1715318400, 0))
	if off != len(aof) {
		t.Errorf("no timestamps → expected full length %d, got %d", len(aof), off)
	}
}

func TestTruncateOffset_empty_aof(t *testing.T) {
	off := TruncateOffset([]byte{}, time.Unix(1715318400, 0))
	if off != 0 {
		t.Errorf("empty AOF → 0, got %d", off)
	}
}

func TestTruncateOffset_cutoff_after_all_markers_keeps_all(t *testing.T) {
	aof := []byte(makeAOF(
		ts(1000), setCommand("a", "1"),
		ts(2000), setCommand("b", "2"),
		ts(3000), setCommand("c", "3"),
	))
	off := TruncateOffset(aof, time.Unix(5000, 0))
	if off != len(aof) {
		t.Errorf("cutoff > all markers → full %d, got %d", len(aof), off)
	}
}

func TestTruncateOffset_cutoff_between_markers_truncates(t *testing.T) {
	// markers 1000, 2000, 3000. cutoff = 2500. 3000 marker 시작 위치 반환.
	aof := []byte(makeAOF(
		ts(1000), setCommand("a", "1"),
		ts(2000), setCommand("b", "2"),
		ts(3000), setCommand("c", "3"),
	))
	off := TruncateOffset(aof, time.Unix(2500, 0))

	// 검증: off 가 #TS:3000 시작 위치.
	expected := strings.Index(string(aof), "#TS:3000")
	if off != expected {
		t.Errorf("cutoff=2500: got off=%d, want %d (#TS:3000 시작)", off, expected)
	}
	// off 까지 의 데이터에 SET a / SET b 만 포함되고 SET c 미포함 검증.
	preserved := string(aof[:off])
	if !strings.Contains(preserved, "SET\r\n$1\r\na") {
		t.Error("preserved 에 SET a 미포함")
	}
	if !strings.Contains(preserved, "SET\r\n$1\r\nb") {
		t.Error("preserved 에 SET b 미포함")
	}
	if strings.Contains(preserved, "SET\r\n$1\r\nc") {
		t.Error("preserved 에 SET c 가 포함됨 (truncate 실패)")
	}
}

func TestTruncateOffset_cutoff_equals_marker_keeps_marker_data(t *testing.T) {
	// cutoff == marker 시각이면 그 marker 의 commands 도 *보존*.
	aof := []byte(makeAOF(
		ts(1000), setCommand("a", "1"),
		ts(2000), setCommand("b", "2"),
	))
	off := TruncateOffset(aof, time.Unix(2000, 0))
	preserved := string(aof[:off])
	if !strings.Contains(preserved, "SET\r\n$1\r\nb") {
		t.Error("cutoff=2000 marker 시점은 b 보존해야 (>= 비교, > 만 cutoff)")
	}
}

func TestTruncateOffset_cutoff_before_first_marker(t *testing.T) {
	// 첫 marker 1000 보다 cutoff 작음 → 첫 marker 시작에서 truncate (전부 제거).
	aof := []byte(makeAOF(
		ts(1000), setCommand("a", "1"),
	))
	off := TruncateOffset(aof, time.Unix(500, 0))
	if off != 0 {
		t.Errorf("cutoff=500 < first marker 1000: expected 0, got %d", off)
	}
}

func TestHasTimestamps_yes(t *testing.T) {
	aof := []byte(ts(1000) + setCommand("a", "1"))
	if !HasTimestamps(aof) {
		t.Error("AOF with #TS: marker should report HasTimestamps=true")
	}
}

func TestHasTimestamps_no(t *testing.T) {
	aof := []byte(setCommand("a", "1"))
	if HasTimestamps(aof) {
		t.Error("AOF without #TS: marker should report HasTimestamps=false")
	}
}

func TestTruncateOffset_invalid_timestamp_skipped(t *testing.T) {
	// `#TS:not-a-number\r\n` 은 invalid → skip 하고 다음 marker 검색.
	aof := []byte("#TS:NOTANUMBER\r\n" + setCommand("a", "1") +
		ts(3000) + setCommand("b", "2"))
	off := TruncateOffset(aof, time.Unix(2000, 0))
	// invalid skip → 3000 marker 가 cutoff 초과 → 그 위치 반환.
	expected := strings.Index(string(aof), "#TS:3000")
	if off != expected {
		t.Errorf("invalid marker skip + 3000 marker truncate: got %d want %d", off, expected)
	}
}

func TestTruncateAOFFile_truncates_to_cutoff(t *testing.T) {
	srcPath := t.TempDir() + "/src.aof"
	dstPath := t.TempDir() + "/dst.aof"

	aof := makeAOF(
		ts(1000), setCommand("a", "1"),
		ts(2000), setCommand("b", "2"),
		ts(3000), setCommand("c", "3"),
	)
	if err := writeFile(srcPath, []byte(aof)); err != nil {
		t.Fatalf("write src: %v", err)
	}

	written, truncated, err := TruncateAOFFile(srcPath, dstPath, time.Unix(2500, 0))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !truncated {
		t.Error("expected truncated=true (timestamps present)")
	}
	expected := strings.Index(aof, "#TS:3000")
	if written != expected {
		t.Errorf("written: got %d want %d", written, expected)
	}
}

func TestTruncateAOFFile_no_timestamps_full_copy(t *testing.T) {
	srcPath := t.TempDir() + "/src.aof"
	dstPath := t.TempDir() + "/dst.aof"
	aof := setCommand("a", "1") + setCommand("b", "2")
	if err := writeFile(srcPath, []byte(aof)); err != nil {
		t.Fatalf("write src: %v", err)
	}

	written, truncated, err := TruncateAOFFile(srcPath, dstPath, time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if truncated {
		t.Error("AOF without timestamps should report truncated=false (full copy fallback)")
	}
	if written != len(aof) {
		t.Errorf("full copy: written %d want %d", written, len(aof))
	}
}

func TestTruncateAOFFile_src_not_found(t *testing.T) {
	dstPath := t.TempDir() + "/dst.aof"
	_, _, err := TruncateAOFFile("/nonexistent/path.aof", dstPath, time.Now())
	if err == nil {
		t.Fatal("expected err for nonexistent src")
	}
}

// writeFile — test helper.
func writeFile(path string, data []byte) error {
	return osWriteFile(path, data, 0o600)
}

// 별도 alias — strconv import 줄임 + os.WriteFile 사용.
var osWriteFile = func(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}

func TestTruncateAOFFileWithBackup_creates_backup_before_truncate(t *testing.T) {
	dir := t.TempDir()
	srcPath := dir + "/src.aof"
	dstPath := dir + "/dst.aof"
	backupPath := dir + "/dst.aof.original"

	aof := makeAOF(
		ts(1000), setCommand("a", "1"),
		ts(2000), setCommand("b", "2"),
		ts(3000), setCommand("c", "3"),
	)
	if err := osWriteFile(srcPath, []byte(aof), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	written, truncated, err := TruncateAOFFileWithBackup(srcPath, dstPath, backupPath, time.Unix(2500, 0))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !truncated {
		t.Error("expected truncated=true")
	}

	// backup 은 *원본 전체*.
	bk, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(bk) != aof {
		t.Errorf("backup mismatch: got %d bytes, want %d (full original)", len(bk), len(aof))
	}

	// dst 는 truncated.
	if written >= len(aof) {
		t.Errorf("dst should be truncated (< %d), got %d", len(aof), written)
	}
}

func TestTruncateAOFFileWithBackup_no_backup_when_path_empty(t *testing.T) {
	dir := t.TempDir()
	srcPath := dir + "/src.aof"
	dstPath := dir + "/dst.aof"
	aof := ts(1000) + setCommand("a", "1")
	_ = osWriteFile(srcPath, []byte(aof), 0o600)

	_, _, err := TruncateAOFFileWithBackup(srcPath, dstPath, "", time.Unix(500, 0))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// backupPath 빈 문자열 — backup 파일 미생성 확인 (.original 부재).
	if _, err := os.Stat(srcPath + ".original"); err == nil {
		t.Error("backup file should not exist when backupPath empty")
	}
}

func TestRollbackFromBackup_restores_original(t *testing.T) {
	dir := t.TempDir()
	dstPath := dir + "/dump.aof"
	backupPath := dir + "/dump.aof.original"

	originalAOF := ts(1000) + setCommand("a", "1") + ts(2000) + setCommand("b", "2")
	truncatedAOF := ts(1000) + setCommand("a", "1") // 시뮬레이션: PITR truncated state.

	_ = osWriteFile(backupPath, []byte(originalAOF), 0o600)
	_ = osWriteFile(dstPath, []byte(truncatedAOF), 0o600)

	if err := RollbackFromBackup(backupPath, dstPath); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// dst 가 이제 *원본 전체* 와 동일해야 (rollback 성공).
	dst, _ := os.ReadFile(dstPath)
	if string(dst) != originalAOF {
		t.Errorf("dst after rollback mismatch:\n got: %q\nwant: %q", dst, originalAOF)
	}
}

func TestRollbackFromBackup_backup_not_found(t *testing.T) {
	err := RollbackFromBackup("/nonexistent/backup.aof", "/tmp/dst.aof")
	if err == nil {
		t.Fatal("expected err for nonexistent backup")
	}
}
