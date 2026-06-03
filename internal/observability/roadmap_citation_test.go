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

// docs/ROADMAP.md 의 *백틱으로 감싼 경로 인용* 이 실재하는 파일/디렉토리를
// 가리키는지 검증.
//
// 사고 패턴 (2026-05-27 truth-up): `[ ]`→`[x]` 마커 flip 은 정확했으나 *인용
// 경로의 실재 검증을 누락*. 기능은 실재하나 인용 경로가 phantom (예:
// `internal/resources/security.go` 부재 — 실제는 statefulset.go 등에 분산;
// `internal/webhook/v1alpha2/` 부재 — 전부 v1alpha1/; `internal/controller/
// pvc_resize.go` 는 ADR-0049 로 삭제되고 commonspvc.ExpandDataPVCs 로 대체).
// ROADMAP 독자가 인용 경로를 클릭/검색 시 빈손 → 신뢰 저하. 본 테스트가
// lefthook pre-push 에서 phantom 인용 차단.
//
// scope: docs/ROADMAP.md (영어 canonical) 만. i18n 번역본은 동일 경로를 미러
// 하므로 canonical 1건만 게이트하면 충분 (번역 텍스트 diff 노이즈 회피).

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRoadmapCitationsResolve(t *testing.T) {
	repo := findRepoRoot(t)

	roadmap := filepath.Join(repo, "docs", "ROADMAP.md")
	raw, err := os.ReadFile(roadmap)
	if err != nil {
		t.Fatalf("docs/ROADMAP.md 읽기 실패: %v", err)
	}

	// 백틱(`...`) 안의 토큰을 추출.
	backtickRe := regexp.MustCompile("`([^`]+)`")

	// 경로형 토큰의 1번째 segment 가 source-tree top-level dir 인지 판정.
	// 이 prefix 로 시작 + 슬래시 포함 = repo 내 실 경로 인용으로 간주.
	pathPrefixes := []string{
		"internal/", "charts/", "scripts/", "hack/",
		"api/", "cmd/", "config/", "test/",
	}

	checked := 0
	var phantom []string
	seen := map[string]bool{}

	for _, m := range backtickRe.FindAllStringSubmatch(string(raw), -1) {
		tok := strings.TrimSpace(m[1])

		// 휴리스틱 — 경로 인용으로 *간주하지 않는* 토큰을 배제:
		//   1) 슬래시 없는 bare 토큰 (예: `statefulset.go`, `pvc_resize.go`,
		//      `failover.go`, `autoscaling.enabled`, `[x]`) — 파일명/심볼/플래그
		//      이지 경로 인용이 아님. 변경 이력의 "no `pvc_resize.go`" 같은
		//      *부재를 명시한 bare 파일명* 도 여기서 안전하게 제외됨.
		//   2) 명령/CLI 인용 (예: `make ...`, `kubectl ...`,
		//      `go test ./internal/webhook/v1alpha1/`,
		//      `bash scripts/release-smoke-test.sh <tag>`,
		//      `--set features.cluster.enabled=false`) — 공백 포함.
		if !strings.Contains(tok, "/") {
			continue
		}
		if strings.ContainsAny(tok, " \t") {
			continue
		}
		// 3) glob / brace 패턴 인용 (예:
		//    `internal/controller/*_controller.go` — 5 controller 集합;
		//    `charts/valkey-operator/dashboards/{cluster-overview,...}.json`
		//    — 4 dashboard 集合). 단일 파일이 아닌 *파일 집합* 을 가리키므로
		//    os.Stat 으로 검증 불가 → skip. 본 테스트의 표적은 *literal 경로*
		//    phantom (H3/H4/H5/H5b 류) 이며, 이는 모두 metachar 없는 경로였음.
		if strings.ContainsAny(tok, "*{}?") {
			continue
		}
		// pathPrefixes 중 하나로 시작해야 source-tree 경로 인용.
		isPath := false
		for _, p := range pathPrefixes {
			if strings.HasPrefix(tok, p) {
				isPath = true
				break
			}
		}
		if !isPath {
			// 예: `keiailab.github.io/valkey-operator` (외부 URL host),
			// `cache.keiailab.io/v1alpha2` (GVK) 등은 top-level dir 아님 → skip.
			continue
		}

		if seen[tok] {
			continue
		}
		seen[tok] = true

		// 디렉토리 인용은 trailing `/` 를 허용 (os.Stat 은 trailing slash 무관).
		resolved := filepath.Join(repo, tok)
		if _, err := os.Stat(resolved); err != nil {
			phantom = append(phantom, tok)
		}
		checked++
	}

	if checked == 0 {
		t.Fatal("docs/ROADMAP.md 에서 검증한 경로 인용 0건 — 정규식 회귀")
	}
	for _, p := range phantom {
		t.Errorf("docs/ROADMAP.md: phantom 경로 인용 %q — repo 에 실재하지 않음 (파일 rename/삭제 또는 오타). 실 위치로 정정 필요", p)
	}
	t.Logf("verified %d path citations in docs/ROADMAP.md (%d phantom)", checked, len(phantom))
}
