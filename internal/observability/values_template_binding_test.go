// charts/valkey-operator/values.yaml 의 value key ↔ templates/*.yaml 의 사용
// 동기 검증.
//
// 사고 패턴 (cycle 65/68/69 family):
// - cycle 65: tracing.endpoint — values 에 있지만 chart deployment 미사용 → 추가.
// - cycle 69: logging.format — values 에 있지만 deployment 미배선 → 추가.
// - 본 cycle 70: 모든 *유효 value* 가 *최소 1 template 에서 참조* 되는지 검증
//   *어느 template 에서도 미사용 value* = silent ignore — 사용자가 설정해도 무효.
//
// scope: top-level value 키만 검증 (중첩 키는 false positive 위험 — Helm
// `with` block 안에서 동적 참조 시). 본 게이트가 *grossly unused value* 차단.

package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestValuesTemplateBindingCoverage(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. values.yaml 의 top-level keys 추출.
	valuesRaw, err := os.ReadFile(filepath.Join(repo, "charts/valkey-operator/values.yaml"))
	if err != nil {
		t.Fatalf("read values: %v", err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(valuesRaw, &values); err != nil {
		t.Fatalf("parse values: %v", err)
	}

	// 2. templates/ 안의 모든 .yaml 파일 본문 합본.
	tmplDir := filepath.Join(repo, "charts/valkey-operator/templates")
	entries, err := os.ReadDir(tmplDir)
	if err != nil {
		t.Fatalf("readdir tmpl: %v", err)
	}
	var allTemplate strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(tmplDir, e.Name()))
		if err != nil {
			continue
		}
		allTemplate.Write(raw)
		allTemplate.WriteByte('\n')
	}
	// _helpers.tpl 도 포함.
	helpersRaw, _ := os.ReadFile(filepath.Join(tmplDir, "_helpers.tpl"))
	allTemplate.Write(helpersRaw)

	allBody := allTemplate.String()

	// 3. 각 top-level key 가 *최소 한 번* 참조되는지 검증.
	// `.Values.<key>` 또는 `.Values "<key>"` (Helm template syntax) 또는 `with .Values.<key>`.
	// 일부 key 는 의도적 *future-proofing* (e.g., 향후 사용 예정) — exempted set.
	exempted := map[string]bool{
		// alpha 단계: chart 미구현 — values.yaml 에 명시 (NOT YET IMPLEMENTED 주석).
		// 본 4 항목은 *향후 cycle 에서 chart template 추가 후 exempted 제거*.
		// values.yaml 에 *명시적 미구현 주석* 가 있으므로 silent ignore 위험 0.
		"webhook":       true, // ValidatingWebhookConfig + MutatingWebhookConfig + Service + Cert 미구현.
		"crds":          true, // Helm 3 built-in CRD 동작 사용 (crds/ 디렉토리, install only).
		"watch":         true, // namespace-scoped watch 옵션 — controller-runtime DefaultNamespaces 통합 필요.
		"networkPolicy": true, // operator pod NetworkPolicy template 미구현.
	}

	for key := range values {
		if exempted[key] {
			continue
		}
		// 검색 패턴: ".Values.<key>" 또는 ".Values \"<key>\"".
		patterns := []string{
			".Values." + key,
			`.Values "` + key + `"`,
		}
		found := false
		for _, p := range patterns {
			if strings.Contains(allBody, p) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("values.yaml top-level key %q 가 templates/ 어디에서도 참조되지 않음 — silent ignore (사용자 설정 무효)",
				key)
		}
	}
}
