// charts/valkey-operator/README.md (ArtifactHub 에 렌더) 안의 YAML codeblock 검증.
//
// 사고 패턴: chart README 의 example YAML 이 ValkeyMode enum 또는 api 필드와
// 어긋남 → ArtifactHub UI 사용자 카피-페이스트 → admission reject. cycle 36
// (crdsExamples) + cycle 41 (NOTES.txt) 와 *동일 패턴, 다른 위치*.
//
// 본 테스트는 ```yaml ... ``` 블록 추출 → ValkeyMode enum / api default 와
// 정합성 검증. lightweight (helm template 미사용).

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestChartReadmeYAMLCodeblocksValid — 단일 chart README + 전 markdown 의 YAML
// 블록 망라. README/runbook/ADR/templates/NOTES 등 모든 *.md 안의 mode: literal
// 이 ValkeyMode enum 안에 있는지 검증. cycle 36/41/42 의 sibling 결함 family
// 가 *모든 user-facing markdown surface* 에 잠복하지 않도록 차단.
func TestChartReadmeYAMLCodeblocksValid(t *testing.T) {
	repo := findRepoRoot(t)

	// ValkeyMode enum 추출.
	apiPath := filepath.Join(repo, "api/v1alpha1/valkey_types.go")
	apiRaw, _ := os.ReadFile(apiPath)
	enumRe := regexp.MustCompile(`\+kubebuilder:validation:Enum=([\w;]+)`)
	enumM := enumRe.FindStringSubmatch(string(apiRaw))
	if enumM == nil {
		t.Fatal("ValkeyMode enum 추출 실패")
	}
	validValues := strings.Split(enumM[1], ";")
	validSet := map[string]bool{}
	for _, v := range validValues {
		validSet[v] = true
	}

	// 모든 *.md 파일 walk (skip dirs: .git, vendor, node_modules, bin, dist).
	skipDirs := map[string]bool{".git": true, "vendor": true, "node_modules": true, "bin": true, "dist": true}
	var mdFiles []string
	err := filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
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
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	yamlBlockRe := regexp.MustCompile("(?s)```yaml\n(.+?)\n```")
	modeRe := regexp.MustCompile(`(?m)^\s*mode:\s+(\S+)`)
	checked := 0
	for _, f := range mdFiles {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		blocks := yamlBlockRe.FindAllStringSubmatch(string(raw), -1)
		for _, block := range blocks {
			body := block[1]
			for _, m := range modeRe.FindAllStringSubmatch(body, -1) {
				checked++
				val := strings.Trim(m[1], "\"'")
				if strings.HasPrefix(val, "{{") {
					continue
				}
				if !validSet[val] {
					rel, _ := filepath.Rel(repo, f)
					t.Errorf("%s: YAML 블록 mode: %q 가 ValkeyMode enum (%v) 에 없음 — 사용자 카피 시 admission reject",
						rel, val, validValues)
				}
			}
		}
	}
	t.Logf("verified %d 'mode:' literals across %d markdown files", checked, len(mdFiles))
}
