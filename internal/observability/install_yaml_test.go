// dist/install.yaml (make build-installer 출력) 의 구조 정합성 검증.
//
// release pipeline 이 `kubectl apply -f dist/install.yaml` 을 사용자에게 권장 —
// 본 파일은 *kustomize 사용자의 단일 install file*. 본 파일이 valid YAML 이
// 아니거나 핵심 K8s kind 누락 시 사용자 install 실패.
//
// 본 테스트는 *artifact 가 미리 빌드되어 있을 때만* 실행 (make build-installer
// 후) — CI/local 환경에 build artifact 부재 시 자동 skip.

package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestInstallYAMLStructure(t *testing.T) {
	repo := findRepoRoot(t)
	path := filepath.Join(repo, "dist/install.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("dist/install.yaml 부재 — `make build-installer` 후 재실행. skip (%v)", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// kustomize 출력은 multi-doc YAML — `---` 로 분리.
	docs := strings.Split(string(raw), "\n---\n")

	type k8sMeta struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	kindCount := map[string]int{}
	parsed := 0
	for i, doc := range docs {
		body := strings.TrimSpace(doc)
		if body == "" {
			continue
		}
		var m k8sMeta
		if err := yaml.Unmarshal([]byte(body), &m); err != nil {
			t.Errorf("doc[%d] YAML parse 실패: %v", i, err)
			continue
		}
		if m.Kind == "" {
			continue // doc separator 또는 frag.
		}
		kindCount[m.Kind]++
		parsed++
	}

	if parsed < 10 {
		t.Errorf("dist/install.yaml 의 K8s 객체 %d 개 (expect ≥ 10) — 빌드 회귀 의심", parsed)
	}

	// 핵심 kind 가 모두 있는지 검증.
	requiredKinds := []string{
		"CustomResourceDefinition", // 5 CRD
		"Deployment",
		"ServiceAccount",
		"ClusterRole",
		"ClusterRoleBinding",
		"Service",
	}
	for _, k := range requiredKinds {
		if kindCount[k] == 0 {
			t.Errorf("dist/install.yaml 에 %q kind 없음 — kustomize config 회귀", k)
		}
	}

	// CRD 개수 검증 — 5 (Valkey/ValkeyCluster/ValkeyBackup/ValkeyBackupTarget/ValkeyRestore).
	if kindCount["CustomResourceDefinition"] != 5 {
		t.Errorf("CRD 개수 = %d (expect 5)", kindCount["CustomResourceDefinition"])
	}
	t.Logf("dist/install.yaml: %d K8s objects, kinds=%v", parsed, kindCount)
}
