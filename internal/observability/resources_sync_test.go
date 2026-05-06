// config/manager/manager.yaml 의 resources ↔ charts/valkey-operator/values.yaml
// 의 resources 동기 검증.
//
// 사고 패턴 (cycle 61 발견): kustomize 사용자와 Helm 사용자가 *다른 resource
// limits* — 한쪽 production 환경에서 OOM 발생, 다른 쪽 정상. 동일 operator 가
// *deploy 방법에 따라 다른 SLA*. cycle 37 의 CRD chart sync (Helm vs kustomize
// 다른 기능) 와 동일 패턴.

package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestKustomizeChartResourcesSync(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. config/manager/manager.yaml 의 Deployment.spec.template.spec.containers[0].resources.
	mgrPath := filepath.Join(repo, "config/manager/manager.yaml")
	mgrRaw, err := os.ReadFile(mgrPath)
	if err != nil {
		t.Fatalf("read manager: %v", err)
	}
	var deployRes map[string]map[string]string
	for _, doc := range strings.Split(string(mgrRaw), "\n---\n") {
		body := strings.TrimSpace(doc)
		if body == "" {
			continue
		}
		var m map[string]any
		if err := yaml.Unmarshal([]byte(body), &m); err != nil {
			continue
		}
		if k, _ := m["kind"].(string); k != "Deployment" {
			continue
		}
		spec, _ := m["spec"].(map[string]any)
		tmpl, _ := spec["template"].(map[string]any)
		podSpec, _ := tmpl["spec"].(map[string]any)
		containers, _ := podSpec["containers"].([]any)
		for _, c := range containers {
			cm, _ := c.(map[string]any)
			if cm["name"] == "manager" {
				if r, ok := cm["resources"].(map[string]any); ok {
					deployRes = toResourceMap(r)
				}
			}
		}
	}
	if deployRes == nil {
		t.Fatal("config/manager/manager.yaml 의 manager 컨테이너 resources 추출 실패")
	}

	// 2. charts/valkey-operator/values.yaml 의 resources.
	chartPath := filepath.Join(repo, "charts/valkey-operator/values.yaml")
	chartRaw, _ := os.ReadFile(chartPath)
	var chartValues map[string]any
	if err := yaml.Unmarshal(chartRaw, &chartValues); err != nil {
		t.Fatalf("parse values: %v", err)
	}
	resAny, _ := chartValues["resources"].(map[string]any)
	if resAny == nil {
		t.Fatal("charts values.yaml 의 resources 추출 실패")
	}
	chartRes := toResourceMap(resAny)

	// 3. 4 값 모두 동일 검증 (limits.cpu, limits.memory, requests.cpu, requests.memory).
	for _, kind := range []string{"limits", "requests"} {
		for _, key := range []string{"cpu", "memory"} {
			d := deployRes[kind][key]
			c := chartRes[kind][key]
			if d != c {
				t.Errorf("resources.%s.%s drift: kustomize=%q ≠ chart=%q — 사용자가 deploy 방법에 따라 다른 SLA 받음",
					kind, key, d, c)
			}
		}
	}
}

func toResourceMap(r map[string]any) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, kind := range []string{"limits", "requests"} {
		out[kind] = map[string]string{}
		if sub, ok := r[kind].(map[string]any); ok {
			for k, v := range sub {
				if s, ok := v.(string); ok {
					out[kind][k] = s
				}
			}
		}
	}
	return out
}
