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

// .github/ISSUE_TEMPLATE/*.yml 의 README anchor (#roadmap, #readme) 가 실제
// README.md 에 존재하는지 검증.
//
// 사고 패턴: issue template 이 README#roadmap 같은 anchor 를 link 했는데 README
// 의 해당 섹션이 *없거나 rename* → 사용자가 GitHub UI 에서 issue 작성 중 broken
// link 만남. 첫인상이 결정적인 OSS 신뢰 지표.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// README.md 에서 issue template 이 link 하는 anchor 를 추출하는 테스트.
// .github/ISSUE_TEMPLATE/*.yml 의 `valkey-operator#anchor` 또는 `README#anchor` 패턴.
func TestIssueTemplateReadmeAnchorsExist(t *testing.T) {
	// README.md 로딩.
	readmePath := findFileUp(t, "README.md")
	rmRaw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	// README heading → anchor.
	headingRe := regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	readmeAnchors := map[string]bool{
		"readme": true, // README#readme 는 항상 first heading.
	}
	for _, m := range headingRe.FindAllStringSubmatch(string(rmRaw), -1) {
		a := githubAnchor(m[1])
		if a != "" {
			readmeAnchors[a] = true
		}
	}

	// .github/ISSUE_TEMPLATE/ 의 yml 파일 조회.
	candidates := []string{".github/ISSUE_TEMPLATE", "../../.github/ISSUE_TEMPLATE", "../../../.github/ISSUE_TEMPLATE"}
	var dir string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			dir = c
			break
		}
	}
	if dir == "" {
		t.Skip(".github/ISSUE_TEMPLATE 부재 — skip")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	// keiailab/valkey-operator#<anchor> 패턴.
	urlRe := regexp.MustCompile(`keiailab/valkey-operator#(\w[\w-]*)`)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range urlRe.FindAllStringSubmatch(string(raw), -1) {
			anchor := strings.ToLower(m[1])
			if !readmeAnchors[anchor] {
				t.Errorf("%s: README anchor #%s 가 README.md 에 없음 — 섹션 추가 또는 link 제거 필요",
					e.Name(), anchor)
			}
		}
	}
}

func findFileUp(t *testing.T, name string) string {
	t.Helper()
	candidates := []string{name, "../../" + name, "../../../" + name}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Fatalf("%s not found in: %v", name, candidates)
	return ""
}
