// 모든 markdown 파일의 *상대 경로 .md link* 가 실재하는지 검증.
//
// 사고 패턴: ADR/runbook/README 에서 다른 doc 을 link 한 후 그 파일이
// rename / 삭제 되면 broken link. GitHub UI 에서는 보이지 않다가 사용자가
// 클릭 시 404. 본 테스트가 lefthook pre-push 에서 차단.
//
// scope: 상대 경로 .md link 만 (절대 URL https:// 은 외부 도메인 책임).
// fragment (#anchor) 는 무시 — 별도 alerts_lint / readme_anchors 테스트에서 검증.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMarkdownRelativeLinksResolve(t *testing.T) {
	repoRoot := findRepoRoot(t)

	mdLinkRe := regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	var files []string
	skipDirs := map[string]bool{".git": true, "node_modules": true, "vendor": true, "bin": true, "dist": true}
	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("markdown 파일 0건 — 본 테스트가 무력")
	}

	checked := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		dir := filepath.Dir(f)
		for _, m := range mdLinkRe.FindAllStringSubmatch(string(raw), -1) {
			target := m[2]
			// 절대 URL skip.
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "ftp://") {
				continue
			}
			// fragment-only (#anchor) skip — 동일 파일 내부.
			if strings.HasPrefix(target, "#") {
				continue
			}
			// fragment 분리.
			pathPart := target
			if i := strings.Index(target, "#"); i >= 0 {
				pathPart = target[:i]
			}
			// .md 만 검증.
			if !strings.HasSuffix(pathPart, ".md") {
				continue
			}
			// 절대 경로 → repoRoot 기준, 상대 → 현재 dir 기준.
			var resolved string
			if strings.HasPrefix(pathPart, "/") {
				resolved = filepath.Join(repoRoot, pathPart)
			} else {
				resolved = filepath.Join(dir, pathPart)
			}
			if _, err := os.Stat(resolved); err != nil {
				rel, _ := filepath.Rel(repoRoot, f)
				t.Errorf("%s: broken link → %q (resolved=%s)", rel, target, resolved)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("검증한 .md link 0건 — 정규식 회귀")
	}
	t.Logf("verified %d markdown .md links across %d files", checked, len(files))
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	// go.mod 가 있는 디렉토리를 root 로 간주.
	candidates := []string{".", "..", "../..", "../../.."}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "go.mod")); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	t.Fatal("repo root (go.mod) not found")
	return ""
}
