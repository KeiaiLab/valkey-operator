// internal/webhook/v1alpha1/ 의 SetupXxxWebhookWithManager 함수 ↔ cmd/main.go
// 등록 호출 동기 검증.
//
// 사고 패턴: 신규 CRD 의 admission webhook 정의 추가 → main.go 등록 누락 →
// production 배포 후 *기존 webhook 만 호출* (validation skip) → 잘못된 spec 이
// admission 통과 + 그대로 reconcile → silent corruption.
//
// 본 테스트가 PR 단계에서 차단 — webhook 정의가 추가되면 main.go 도 갱신 강제.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestWebhookSetupFunctionsRegisteredInMain(t *testing.T) {
	// 1. internal/webhook/v1alpha1/ 의 모든 SetupXxxWebhookWithManager 함수 추출.
	candidates := []string{"internal/webhook/v1alpha1", "../../internal/webhook/v1alpha1", "../../../internal/webhook/v1alpha1"}
	var dir string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			dir = c
			break
		}
	}
	if dir == "" {
		t.Fatalf("internal/webhook/v1alpha1 not found: %v", candidates)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	setupRe := regexp.MustCompile(`func\s+(Setup\w+WebhookWithManager)\(`)
	defined := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range setupRe.FindAllStringSubmatch(string(raw), -1) {
			defined[m[1]] = true
		}
	}
	if len(defined) == 0 {
		t.Fatal("Setup*WebhookWithManager 함수 0건 — 패키지 회귀")
	}

	// 2. cmd/main.go 에서 각 함수 호출 검증.
	mainCandidates := []string{"cmd/main.go", "../../cmd/main.go", "../../../cmd/main.go"}
	var mainPath string
	for _, c := range mainCandidates {
		if _, err := os.Stat(c); err == nil {
			mainPath = c
			break
		}
	}
	if mainPath == "" {
		t.Fatalf("cmd/main.go not found: %v", mainCandidates)
	}
	mainRaw, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	mainContent := string(mainRaw)

	for fn := range defined {
		if !strings.Contains(mainContent, fn+"(") {
			t.Errorf("webhook setup 함수 %q 가 정의되었지만 cmd/main.go 에서 호출되지 않음 — 신규 webhook 추가 시 main.go SetupWithManager 블록 갱신 필수",
				fn)
		}
	}
}
