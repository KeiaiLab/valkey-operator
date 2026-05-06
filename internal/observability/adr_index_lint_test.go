// docs/kb/adr/ 디렉토리의 ADR 파일과 INDEX.md 엔트리 동기 검증.
//
// 표준 (~/.../standards/adr.md): 신규 ADR 추가 시 INDEX.md 갱신 필수. 누락은
// 글로벌 게이트가 막지만 본 in-process lint 가 *로컬* PR 단계에서 동일 차단.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func adrDir(t *testing.T) string {
	t.Helper()
	candidates := []string{"docs/kb/adr", "../../docs/kb/adr", "../../../docs/kb/adr"}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	t.Fatalf("docs/kb/adr 디렉토리를 찾을 수 없음: %v", candidates)
	return ""
}

// ADR 파일 ID (4 자리) 추출. "0024-helm-...md" → "0024".
var adrFileRe = regexp.MustCompile(`^(\d{4})-[\w-]+\.md$`)

// INDEX.md 의 row: `| [0024](0024-...md) | Title | Status | Date |`.
var adrIndexRowRe = regexp.MustCompile(`\|\s*\[(\d{4})\]\(\d{4}-[\w-]+\.md\)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|`)

func TestADRFilesAllInIndex(t *testing.T) {
	dir := adrDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	fileIDs := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := adrFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		fileIDs[m[1]] = e.Name()
	}
	if len(fileIDs) == 0 {
		t.Fatal("ADR 파일 0건 — 본 테스트가 무의미")
	}

	idxPath := filepath.Join(dir, "INDEX.md")
	idxRaw, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read INDEX.md: %v", err)
	}
	indexIDs := map[string]bool{}
	for _, m := range adrIndexRowRe.FindAllStringSubmatch(string(idxRaw), -1) {
		indexIDs[m[1]] = true
	}

	for id, fname := range fileIDs {
		if !indexIDs[id] {
			t.Errorf("ADR %s (%s) 가 INDEX.md 에 없음 — INDEX 갱신 필수", id, fname)
		}
	}
	for id := range indexIDs {
		if _, ok := fileIDs[id]; !ok {
			t.Errorf("INDEX.md 에 ADR %s 가 있지만 파일이 없음 — 삭제된 ADR? rename?", id)
		}
	}
}

func TestADRIndexStatusValid(t *testing.T) {
	dir := adrDir(t)
	idxPath := filepath.Join(dir, "INDEX.md")
	idxRaw, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read INDEX.md: %v", err)
	}
	rows := adrIndexRowRe.FindAllStringSubmatch(string(idxRaw), -1)
	if len(rows) == 0 {
		t.Fatal("INDEX.md row 0 — 정규식 회귀 또는 INDEX 손상")
	}

	// 본 테스트 컬럼 매핑: [1]=ID, [2]=Title, [3]=Status, [4]=Date.
	allowed := []string{"Accepted", "Proposed", "Deprecated", "Superseded by"}
	for _, m := range rows {
		id, status := m[1], m[3]
		ok := false
		for _, p := range allowed {
			if strings.HasPrefix(status, p) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("ADR %s status=%q 가 알려진 상태 (Accepted/Proposed/Deprecated/Superseded by NNNN) 가 아님", id, status)
		}
	}
}

func TestADRIndexSupersededReferencesExist(t *testing.T) {
	dir := adrDir(t)
	idxPath := filepath.Join(dir, "INDEX.md")
	idxRaw, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read INDEX.md: %v", err)
	}
	// 모든 file ID 수집.
	entries, _ := os.ReadDir(dir)
	fileIDs := map[string]bool{}
	for _, e := range entries {
		if m := adrFileRe.FindStringSubmatch(e.Name()); m != nil {
			fileIDs[m[1]] = true
		}
	}

	// "Superseded by NNNN" 의 NNNN 추출 — 존재해야 함.
	supersededRe := regexp.MustCompile(`Superseded by (\d{4})`)
	for _, m := range supersededRe.FindAllStringSubmatch(string(idxRaw), -1) {
		if !fileIDs[m[1]] {
			t.Errorf("'Superseded by %s' 참조하지만 ADR-%s 파일 없음", m[1], m[1])
		}
	}
}
