/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// ClusterReference.Kind 의 kubebuilder Enum 마커 ↔ controller switch case 동기 검증.
//
// 사고 패턴: API 에 "ValkeyReplication" 같은 신규 kind 를 enum 추가하고
// controller 의 switch ref.Kind { case "Valkey": ... case "ValkeyCluster": ... }
// 에 case 추가 누락 → 사용자가 신규 kind 사용 시 default 분기 (보통 error)
// 또는 silent skip. PR 에서는 컴파일/lint 통과 (switch 가 기본값 처리하므로).

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ClusterRef Kind 의 SSOT — kubebuilder Enum + controller switch 모두 일치해야.
// 신규 kind 추가 시 본 슬라이스 + api Enum 마커 + controller switch 3 곳 동시 갱신.
var clusterRefKinds = []string{"Valkey", "ValkeyCluster"}

func TestClusterRefKindEnumMatchesSSOT(t *testing.T) {
	candidates := []string{"api/v1alpha1/valkeybackup_types.go", "../../api/v1alpha1/valkeybackup_types.go", "../../../api/v1alpha1/valkeybackup_types.go"}
	var path string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			path = c
			break
		}
	}
	if path == "" {
		t.Fatalf("valkeybackup_types.go not found: %v", candidates)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// 마커 형식: `+kubebuilder:validation:Enum=Valkey;ValkeyCluster`.
	enumRe := regexp.MustCompile(`\+kubebuilder:validation:Enum=([\w;]+)\s*\n\s*Kind\s+string`)
	m := enumRe.FindStringSubmatch(string(raw))
	if m == nil {
		t.Fatal("ClusterReference.Kind 위의 Enum 마커를 찾을 수 없음 — api 타입 회귀")
	}
	got := strings.Split(m[1], ";")
	gotMap := map[string]bool{}
	for _, k := range got {
		gotMap[k] = true
	}
	wantMap := map[string]bool{}
	for _, k := range clusterRefKinds {
		wantMap[k] = true
	}
	for k := range wantMap {
		if !gotMap[k] {
			t.Errorf("SSOT kind %q 가 kubebuilder Enum 에 없음 — Enum 갱신 필요", k)
		}
	}
	for k := range gotMap {
		if !wantMap[k] {
			t.Errorf("kubebuilder Enum 의 %q 가 SSOT (clusterRefKinds) 에 없음 — 본 테스트의 SSOT 슬라이스 갱신 + controller switch 추가 확인", k)
		}
	}
}

func TestClusterRefKindAllHaveSwitchCase(t *testing.T) {
	// internal/controller/*.go 에서 "ref.Kind" 또는 "ClusterRef.Kind" 를 switch 하는 부분의
	// case "<Kind>": 토큰 모두 수집. SSOT 의 각 kind 가 *최소 1회* 등장해야.
	candidates := []string{"internal/controller", "../../internal/controller", "../../../internal/controller"}
	var dir string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			dir = c
			break
		}
	}
	if dir == "" {
		t.Fatalf("internal/controller not found: %v", candidates)
	}
	entries, _ := os.ReadDir(dir)
	// case "<Kind>": (string literal) 또는 case <pkg>.Kind<Name>: (const reference) 둘 다 catch.
	// 2026-05-09 audit (RFC-0017 §3.2 goconst 추출) 후 const reference 패턴이 도입됨 —
	// 본 정규식은 string literal "Valkey" 와 const cachev1alpha1.KindValkey 모두 인식.
	caseRe := regexp.MustCompile(`case\s+(?:"([\w]+)"|[\w.]*\.?Kind(\w+)):`)
	switchKindRe := regexp.MustCompile(`switch\s+\S*\.?Kind\s*\{`)

	caseCounts := map[string]int{}
	for _, k := range clusterRefKinds {
		caseCounts[k] = 0
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		s := string(raw)
		// switch ref.Kind { ... case "X": ... case "Y": ... } block 추출.
		// 단순화: switch 키워드가 Kind 와 함께 등장하는 파일 안의 모든 case "X": 를 카운트.
		// (false positive 가능: 다른 string switch 의 case 도 잡힘 — but kind 가 SSOT 와
		// 동치 비교이므로 SSOT 외 kind 는 무시됨.)
		if !switchKindRe.MatchString(s) {
			continue
		}
		for _, m := range caseRe.FindAllStringSubmatch(s, -1) {
			// m[1] = string literal kind, m[2] = const Kind<Name> 의 <Name>.
			kind := m[1]
			if kind == "" {
				kind = m[2]
			}
			if _, tracked := caseCounts[kind]; tracked {
				caseCounts[kind]++
			}
		}
	}
	for _, k := range clusterRefKinds {
		if caseCounts[k] == 0 {
			t.Errorf("kind %q 에 대응하는 controller switch case 없음 — 신규 kind 추가 시 dial_helpers.go / valkeybackup_controller.go / valkeyrestore_controller.go 갱신 필요",
				k)
		}
	}
}
