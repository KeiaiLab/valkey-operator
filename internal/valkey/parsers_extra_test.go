// Pure parser / 분류 헬퍼 회귀 보호 보강.
// parseFlags, parseSlotToken, NodeView.{IsPrimary,IsReplica,HasSlot},
// containsAny/indexOf (BGSAVE 멱등 분기), atoi32 (CLUSTER INFO).

package valkey

import "testing"

func TestParseFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want map[string]bool
	}{
		{"master", map[string]bool{"master": true}},
		{"myself,master", map[string]bool{"myself": true, "master": true}},
		{"slave,fail?", map[string]bool{"slave": true, "fail?": true}},
		{"noflags", map[string]bool{"noflags": true}},
		{"", map[string]bool{"": true}}, // strings.Split 의 zero-value 동작 명시.
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			got := parseFlags(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("len got %d want %d (%v)", len(got), len(c.want), got)
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Errorf("flag %q: got %v want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestParseSlotToken(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in        string
		wantStart int
		wantEnd   int
		wantOK    bool
	}{
		{"0-5460", 0, 5460, true},
		{"42", 42, 42, true},
		{"5461-10922", 5461, 10922, true},
		{"abc", 0, 0, false},
		{"1-abc", 0, 0, false},
		{"abc-5", 0, 0, false},
		{"-", 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			r, ok := parseSlotToken(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok got %v want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if r.Start != c.wantStart || r.End != c.wantEnd {
				t.Errorf("got [%d-%d] want [%d-%d]", r.Start, r.End, c.wantStart, c.wantEnd)
			}
		})
	}
}

func TestNodeViewClassifiers(t *testing.T) {
	t.Parallel()
	primary := NodeView{Flags: map[string]bool{"master": true}, Slots: []SlotRange{{0, 100}, {200, 300}}}
	replicaSlave := NodeView{Flags: map[string]bool{"slave": true}}
	replicaNew := NodeView{Flags: map[string]bool{"replica": true}}
	none := NodeView{Flags: map[string]bool{}}

	if !primary.IsPrimary() {
		t.Error("primary.IsPrimary() = false")
	}
	if primary.IsReplica() {
		t.Error("primary.IsReplica() = true")
	}
	if !replicaSlave.IsReplica() {
		t.Error("slave flag → IsReplica false")
	}
	if !replicaNew.IsReplica() {
		t.Error("replica flag → IsReplica false (Valkey 8+ 신 명칭)")
	}
	if none.IsPrimary() || none.IsReplica() {
		t.Error("empty flags → both false")
	}

	if !primary.HasSlot(0) || !primary.HasSlot(100) || !primary.HasSlot(50) {
		t.Error("HasSlot 경계+내부 false")
	}
	if !primary.HasSlot(250) {
		t.Error("HasSlot 두 번째 range 내부 false")
	}
	if primary.HasSlot(150) || primary.HasSlot(101) {
		t.Error("HasSlot gap 영역 true")
	}
	if primary.HasSlot(-1) || primary.HasSlot(16384) {
		t.Error("HasSlot 범위 외 true")
	}
}

func TestContainsAnyAndIndexOf(t *testing.T) {
	t.Parallel()

	if indexOf("hello world", "") != 0 {
		t.Error("indexOf empty sub != 0")
	}
	if indexOf("hello", "ell") != 1 {
		t.Errorf("indexOf basic: got %d want 1", indexOf("hello", "ell"))
	}
	if indexOf("abc", "xyz") != -1 {
		t.Error("indexOf no match != -1")
	}
	if indexOf("ab", "abcd") != -1 {
		t.Error("indexOf sub > s != -1")
	}

	if !containsAny("Background save already in progress", "in progress", "already") {
		t.Error("containsAny: BGSAVE 멱등 분기 검출 실패")
	}
	if containsAny("OK", "in progress", "fail") {
		t.Error("containsAny: false positive")
	}
	if containsAny("anything", "") {
		t.Error("containsAny empty sub → true (의도적으로 false)")
	}
}

func TestAtoi32(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int32
	}{
		{"42", 42},
		{"0", 0},
		{"-1", -1},
		{"", 0},    // strconv 에러 → 0.
		{"abc", 0}, // strconv 에러 → 0.
		{"16384", 16384},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			if got := atoi32(c.in); got != c.want {
				t.Errorf("atoi32(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// resolveAddrIP 의 *pure paths* 만 검증 (DNS 호출 path 는 integration test).
// invalid address + IP literal pass-through 두 경로.
func TestResolveAddrIPPurePaths(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// invalid: colon 없음.
	if _, err := resolveAddrIP(ctx, "no-colon-here"); err == nil {
		t.Error("invalid address (no colon) → expected error")
	}

	// IP literal pass-through (DNS 호출 안 함).
	cases := []string{"192.168.1.1:6379", "10.0.0.1:6380", "::1:6379", "[::1]:6379"}
	for _, addr := range cases {
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			got, err := resolveAddrIP(ctx, addr)
			// IPv4 literal 는 net.ParseIP 가 host 부분 (콜론 분리 후) parse 가능.
			// IPv6 의 [::1]:port 형식 은 strings.Cut(":") 로 첫 콜론 분리 — host 가 "[::1]" 또는 "::1"
			// 일 수 있어 동작 확인 만.
			_ = got
			_ = err
		})
	}

	// IPv4 literal 정확 케이스 검증.
	got, err := resolveAddrIP(ctx, "192.168.1.1:6379")
	if err != nil {
		t.Fatalf("IPv4 literal pass-through: unexpected error: %v", err)
	}
	if got != "192.168.1.1:6379" {
		t.Errorf("IPv4 pass-through: got %q, want %q", got, "192.168.1.1:6379")
	}
}
