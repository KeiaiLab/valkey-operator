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
	for doc := range strings.SplitSeq(string(mgrRaw), "\n---\n") {
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

// TestKustomizeChartProbesSync — liveness/readiness probe 의 initialDelaySeconds /
// periodSeconds 가 kustomize manager Deployment ↔ chart values.yaml 정합.
//
// 사고 패턴: kustomize 사용자가 cluster 에서 *느린 startup* 감지 → liveness
// probe 가 너무 짧아 무한 restart loop. Helm 사용자는 정상. cycle 61 (resources)
// 와 동일 패턴.
func TestKustomizeChartProbesSync(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. config/manager/manager.yaml 의 probes.
	mgrPath := filepath.Join(repo, "config/manager/manager.yaml")
	mgrRaw, err := os.ReadFile(mgrPath)
	if err != nil {
		t.Fatalf("read manager: %v", err)
	}
	deployProbes := map[string]map[string]int{}
	for doc := range strings.SplitSeq(string(mgrRaw), "\n---\n") {
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
				for _, probe := range []string{"livenessProbe", "readinessProbe"} {
					if p, ok := cm[probe].(map[string]any); ok {
						deployProbes[probe] = map[string]int{
							"initialDelaySeconds": toInt(p["initialDelaySeconds"]),
							"periodSeconds":       toInt(p["periodSeconds"]),
						}
					}
				}
			}
		}
	}
	if len(deployProbes) != 2 {
		t.Fatal("manager Deployment 의 liveness + readiness probe 추출 실패")
	}

	// 2. chart values.yaml 의 probes.liveness / probes.readiness.
	chartPath := filepath.Join(repo, "charts/valkey-operator/values.yaml")
	chartRaw, _ := os.ReadFile(chartPath)
	var chartValues map[string]any
	if err := yaml.Unmarshal(chartRaw, &chartValues); err != nil {
		t.Fatalf("parse values: %v", err)
	}
	probes, _ := chartValues["probes"].(map[string]any)
	mapping := map[string]string{"livenessProbe": "liveness", "readinessProbe": "readiness"}
	for deployKey, valuesKey := range mapping {
		v, _ := probes[valuesKey].(map[string]any)
		for _, field := range []string{"initialDelaySeconds", "periodSeconds"} {
			cv := toInt(v[field])
			dv := deployProbes[deployKey][field]
			if cv != dv {
				t.Errorf("probe drift: %s.%s — kustomize=%d ≠ chart=%d", deployKey, field, dv, cv)
			}
		}
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// TestKustomizeChartSecurityContextInvariants — 보안 critical 필드가 *양쪽 모두*
// 적용되었는지 검증. strict equality 아님 — chart 는 추가 redundancy 가능
// (e.g., container-level 에 runAsNonRoot 추가는 pod-level 위에 무해 redundancy).
//
// 검증 invariant (Pod Security Standards "restricted" 준수):
// 1. Pod 레벨: runAsNonRoot=true + seccompProfile=RuntimeDefault.
// 2. Container 레벨: allowPrivilegeEscalation=false + readOnlyRootFilesystem=true
//   - capabilities.drop 에 ALL 포함.
//
// 한쪽에서 누락 시 *Pod Security Admission 거절* (restricted PSS namespace 에서)
// 또는 *컨테이너 권한 상승 가능* (privileged 침투 위험).
func TestKustomizeChartSecurityContextInvariants(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. kustomize manager.yaml 의 pod + container securityContext.
	mgrRaw, _ := os.ReadFile(filepath.Join(repo, "config/manager/manager.yaml"))
	var mgrPodSC, mgrContainerSC map[string]any
	for doc := range strings.SplitSeq(string(mgrRaw), "\n---\n") {
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
		mgrPodSC, _ = podSpec["securityContext"].(map[string]any)
		containers, _ := podSpec["containers"].([]any)
		for _, c := range containers {
			cm, _ := c.(map[string]any)
			if cm["name"] == "manager" {
				mgrContainerSC, _ = cm["securityContext"].(map[string]any)
			}
		}
	}

	// 2. chart values.yaml.
	chartRaw, _ := os.ReadFile(filepath.Join(repo, "charts/valkey-operator/values.yaml"))
	var chartValues map[string]any
	_ = yaml.Unmarshal(chartRaw, &chartValues)
	chartPodSC, _ := chartValues["podSecurityContext"].(map[string]any)
	chartContainerSC, _ := chartValues["securityContext"].(map[string]any)

	// 3. Pod 레벨 invariant: runAsNonRoot=true.
	for _, pair := range []struct {
		name, side string
		sc         map[string]any
	}{
		{"kustomize", "pod", mgrPodSC},
		{"chart", "pod", chartPodSC},
	} {
		if v, _ := pair.sc["runAsNonRoot"].(bool); !v {
			t.Errorf("%s.%s.runAsNonRoot = false (want true) — Pod Security Standards restricted 위반",
				pair.name, pair.side)
		}
		seccomp, _ := pair.sc["seccompProfile"].(map[string]any)
		if seccomp["type"] != "RuntimeDefault" {
			t.Errorf("%s.%s.seccompProfile.type=%v (want RuntimeDefault)", pair.name, pair.side, seccomp["type"])
		}
	}

	// 4. Container 레벨 invariant.
	for _, pair := range []struct {
		name string
		sc   map[string]any
	}{
		{"kustomize", mgrContainerSC},
		{"chart", chartContainerSC},
	} {
		if v, _ := pair.sc["allowPrivilegeEscalation"].(bool); v {
			t.Errorf("%s.container.allowPrivilegeEscalation=true (want false) — privilege escalation 차단 필수", pair.name)
		}
		if v, _ := pair.sc["readOnlyRootFilesystem"].(bool); !v {
			t.Errorf("%s.container.readOnlyRootFilesystem=false (want true)", pair.name)
		}
		caps, _ := pair.sc["capabilities"].(map[string]any)
		dropList, _ := caps["drop"].([]any)
		hasAll := false
		for _, d := range dropList {
			if s, _ := d.(string); s == "ALL" {
				hasAll = true
				break
			}
		}
		if !hasAll {
			t.Errorf("%s.container.capabilities.drop 에 'ALL' 없음 — 모든 capability drop 필수", pair.name)
		}
	}
}
