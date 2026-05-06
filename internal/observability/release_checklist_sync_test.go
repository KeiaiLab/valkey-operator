// docs/operations/release-checklist.md §2 의 게이트 목록 ↔ 실제 internal/observability
// 의 Test* 함수 동기 검증.
//
// 사고 패턴: 신규 게이트 추가 후 release-checklist 갱신 누락 → release 시점
// "체계 가시화" 문서가 *실제 검증 능력의 부분만 표시* — 사용자/maintainer 가
// production-grade 수준을 *과소* 평가. 또는 게이트 삭제 후 doc 잔재 → *실제로는
// 없는 게이트* 를 광고하는 사고.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReleaseChecklistGatesSyncWithActualTests(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. release-checklist.md 의 §2 표 에서 backtick 으로 둘러싼 Test* 함수 이름 추출.
	docPath := filepath.Join(repo, "docs/operations/release-checklist.md")
	docRaw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read release-checklist: %v", err)
	}
	// `TestXxx` 패턴 (markdown table 안의 backtick).
	docTestRe := regexp.MustCompile("`(Test\\w+)`")
	docTests := map[string]bool{}
	for _, m := range docTestRe.FindAllStringSubmatch(string(docRaw), -1) {
		docTests[m[1]] = true
	}
	if len(docTests) == 0 {
		t.Fatal("release-checklist 에서 Test* 추출 0건 — 정규식 또는 doc 회귀")
	}

	// 2. internal/observability/ 의 *_test.go 안의 모든 func TestXxx(t *testing.T) 추출.
	// SSOT 게이트가 *아닌* 일반 unit test 파일은 제외 — tracing_test.go 는 OTEL
	// tracer wrapper 의 단위 test (release-checklist 광고 대상 외).
	dir := filepath.Join(repo, "internal/observability")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	excludedFiles := map[string]bool{
		"tracing_test.go": true, // OTEL wrapper 단위 test, SSOT gate 아님.
	}
	codeTests := map[string]bool{}
	// (?m) multi-line + ^ anchor → 주석 안의 "func TestXxx" 같은 잡음 제거.
	funcRe := regexp.MustCompile(`(?m)^func\s+(Test\w+)\(t\s*\*testing\.T\)`)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if excludedFiles[e.Name()] {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range funcRe.FindAllStringSubmatch(string(raw), -1) {
			codeTests[m[1]] = true
		}
	}
	if len(codeTests) == 0 {
		t.Fatal("observability Test* 추출 0건 — 패키지 회귀")
	}

	// 3. doc → code: release-checklist 가 광고하는 모든 게이트가 실재.
	for name := range docTests {
		if !codeTests[name] {
			t.Errorf("release-checklist 가 %q 를 광고하지만 internal/observability 에 함수 없음 — 게이트 삭제 후 doc 잔재 또는 typo",
				name)
		}
	}

	// 4. code → doc (cycle 60 신규): SSOT gate 신규 추가 시 release-checklist
	// 갱신 누락 차단. 본 sync test 자체와 excludedFiles 의 일반 unit test 는 제외.
	for name := range codeTests {
		// 본 sync test 자체는 release-checklist 가 광고할 필요 없음.
		if name == "TestReleaseChecklistGatesSyncWithActualTests" {
			continue
		}
		if !docTests[name] {
			t.Errorf("SSOT gate %q 가 internal/observability 에 추가되었지만 release-checklist §2 에 없음 — doc 갱신 필수 (cycle 60: code → doc 양방향 강제)",
				name)
		}
	}
}
