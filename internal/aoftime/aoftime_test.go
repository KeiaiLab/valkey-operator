/*
Copyright 2026 Keiailab.
*/

package aoftime

import (
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
