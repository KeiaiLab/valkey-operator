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

func TestChartReadmeYAMLCodeblocksValid(t *testing.T) {
	repo := findRepoRoot(t)
	mdPath := filepath.Join(repo, "charts/valkey-operator/README.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Skipf("chart README 없음 — skip")
	}
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(raw)

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

	// markdown 내의 ```yaml ... ``` 블록 추출.
	yamlBlockRe := regexp.MustCompile("(?s)```yaml\n(.+?)\n```")
	blocks := yamlBlockRe.FindAllStringSubmatch(content, -1)
	if len(blocks) == 0 {
		// chart README 가 yaml 블록 없을 수도 있음 — info 로 처리.
		return
	}

	modeRe := regexp.MustCompile(`(?m)^\s*mode:\s+(\S+)`)
	checked := 0
	for _, block := range blocks {
		body := block[1]
		for _, m := range modeRe.FindAllStringSubmatch(body, -1) {
			checked++
			val := strings.Trim(m[1], "\"'")
			if strings.HasPrefix(val, "{{") {
				continue // helm template 변수 — skip.
			}
			if !validSet[val] {
				t.Errorf("chart README 의 YAML 블록 mode: %q 가 ValkeyMode enum (%v) 에 없음 — 사용자 카피 시 admission reject",
					val, validValues)
			}
		}
	}
	if checked == 0 {
		t.Log("chart README 에 mode: literal 없음 — ValkeyCluster 만 다루는 것으로 추정")
	}
}
