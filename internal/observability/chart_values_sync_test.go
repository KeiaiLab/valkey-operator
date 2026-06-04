/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// charts/valkey-operator/values.yaml 의 valkey.version 기본값 ↔ api/v1alpha1
// ValkeyVersion 의 +kubebuilder:default="..." 마커 동기 검증.
//
// 사고 패턴: api 타입의 default 가 "8.1.6" 로 bump 되었는데 chart values
// 가 옛 "8.0" 인 채로 → Helm 사용자가 NOTES.txt 가 제안하는 명령 실행 시 *옛
// 버전 으로 CR 생성*. operator 가 동작은 하지만 사용자가 *최신 권장 버전*
// 을 받지 못함. 잘못된 권장값을 통한 silent 사용성 저하.
//
// 추가: NOTES.txt 의 mode: <value> 가 실제 enum 과 일치하는지 검증.
// (cycle 36/41 의 sample-style drift 와 동일 사고 패턴.)

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

type chartValues struct {
	Valkey struct {
		Version string `json:"version"`
	} `json:"valkey"`
}

func TestChartValuesValkeyVersionMatchesAPIDefault(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. api default 추출 (kubebuilder marker).
	apiPath := filepath.Join(repo, "api/v1alpha1/common_types.go")
	apiRaw, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatalf("read api: %v", err)
	}
	// `+kubebuilder:default="X"` 를 ValkeyVersion struct 의 Version 필드 위에서 찾음.
	// pattern: `kubebuilder:default="<X>"` followed by `Version string`.
	re := regexp.MustCompile(`\+kubebuilder:default="([^"]+)"\s*\n\s*Version\s+string`)
	m := re.FindStringSubmatch(string(apiRaw))
	if m == nil {
		t.Fatal("ValkeyVersion.Version 의 kubebuilder default 마커 추출 실패 — api 타입 회귀")
	}
	apiDefault := m[1]

	// 2. chart values.yaml 추출.
	valuesPath := filepath.Join(repo, "charts/valkey-operator/values.yaml")
	valuesRaw, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("read values: %v", err)
	}
	var v chartValues
	if err := yaml.Unmarshal(valuesRaw, &v); err != nil {
		t.Fatalf("parse values: %v", err)
	}

	if v.Valkey.Version != apiDefault {
		t.Errorf("chart values.yaml valkey.version=%q ≠ api/v1alpha1 ValkeyVersion default=%q — 양쪽 동기 필요",
			v.Valkey.Version, apiDefault)
	}
}

// NOTES.txt 안의 `mode: <value>` 가 실제 ValkeyMode enum 의 valid value 인지.
// helm template 렌더링 없이도 정적 분석 가능 — mode 라인은 Go template 변수
// (`{{ .Values... }}`) 가 아닌 plain literal.
func TestChartNotesTxtModeValueValidEnum(t *testing.T) {
	repo := findRepoRoot(t)
	notesPath := filepath.Join(repo, "charts/valkey-operator/templates/NOTES.txt")
	if _, err := os.Stat(notesPath); err != nil {
		t.Skipf("NOTES.txt 없음 — skip")
	}
	raw, err := os.ReadFile(notesPath)
	if err != nil {
		t.Fatalf("read NOTES.txt: %v", err)
	}
	// `mode: X` literal 추출 (Helm template 변수 패턴 아닌 것만).
	modeRe := regexp.MustCompile(`(?m)^\s*mode:\s+(\S+)`)
	matches := modeRe.FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		// mode 가 없으면 ValkeyCluster 만 다루는 것 — OK.
		return
	}
	// ValkeyMode enum: api/v1alpha1/valkey_types.go 의 마커.
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

	for _, m := range matches {
		val := m[1]
		// helm template 변수 ({{ ... }}) 는 skip.
		if strings.HasPrefix(val, "{{") {
			continue
		}
		if !validSet[val] {
			t.Errorf("NOTES.txt 의 mode: %q 가 ValkeyMode enum (%v) 에 없음 — 사용자 카피 시 admission reject",
				val, validValues)
		}
	}
}
