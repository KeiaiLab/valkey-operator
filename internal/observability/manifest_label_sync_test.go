// kustomize manifest 의 selector chain 정합성 검증.
//
// chain: Deployment.spec.template.metadata.labels (pod labels) ⊇
//        Deployment.spec.selector.matchLabels ⊇
//        Service.spec.selector ⊇
//        ServiceMonitor.spec.selector.matchLabels
//
// 한 link 라도 *불일치* 시:
//  - Deployment selector ⊋ pod labels: pod 가 Deployment 에 안 잡힘 (CrashLoop).
//  - Service selector ⊋ pod labels: Prometheus 가 endpoint discovery 실패 → metrics 미수집.
//  - ServiceMonitor 가 Service 못 찾음: 동일 결과.
//
// 본 테스트는 핵심 라벨 (control-plane, app.kubernetes.io/name) 의 *모든
// manifest 일치* 검증.

package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestKustomizeManifestLabelChainSync(t *testing.T) {
	repo := findRepoRoot(t)

	loadAll := func(path string) []map[string]any {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var out []map[string]any
		// multi-doc YAML — `---` 분리.
		for doc := range strings.SplitSeq(string(raw), "\n---\n") {
			body := strings.TrimSpace(doc)
			if body == "" {
				continue
			}
			var m map[string]any
			if err := yaml.Unmarshal([]byte(body), &m); err != nil {
				continue
			}
			out = append(out, extractRelevantLabels(m))
		}
		return out
	}

	// 1. Deployment pod template labels.
	mgrPath := filepath.Join(repo, "config/manager/manager.yaml")
	mgrLabels := loadAll(mgrPath)
	deploy := findDocByKind(mgrLabels, "Deployment")
	if deploy == nil {
		t.Fatal("config/manager/manager.yaml 에 Deployment 없음")
	}
	podLabels := deploy["pod_labels"].(map[string]string)
	deploySelector := deploy["selector"].(map[string]string)

	for k, v := range deploySelector {
		if podLabels[k] != v {
			t.Errorf("Deployment selector[%q]=%q ≠ pod labels[%q]=%q (selector ⊋ pod labels — pod 미잡힘)",
				k, v, k, podLabels[k])
		}
	}

	// 2. Metrics Service selector.
	svcPath := filepath.Join(repo, "config/default/metrics_service.yaml")
	svcLabels := loadAll(svcPath)
	svc := findDocByKind(svcLabels, "Service")
	if svc == nil {
		t.Fatal("metrics_service.yaml 에 Service 없음")
	}
	svcSelector := svc["selector"].(map[string]string)
	for k, v := range svcSelector {
		if podLabels[k] != v {
			t.Errorf("Service selector[%q]=%q ≠ pod labels[%q]=%q (Prometheus endpoint discovery 실패)",
				k, v, k, podLabels[k])
		}
	}

	// 3. ServiceMonitor selector.
	smPath := filepath.Join(repo, "config/prometheus/monitor.yaml")
	smLabels := loadAll(smPath)
	sm := findDocByKind(smLabels, "ServiceMonitor")
	if sm == nil {
		t.Fatal("monitor.yaml 에 ServiceMonitor 없음")
	}
	smSelector := sm["selector"].(map[string]string)
	// ServiceMonitor 는 *Service* 의 metadata.labels 와 매칭. metrics_service.yaml 의
	// 상단 metadata.labels 가 SSOT.
	svcMetaLabels := svc["meta_labels"].(map[string]string)
	for k, v := range smSelector {
		if svcMetaLabels[k] != v {
			t.Errorf("ServiceMonitor selector[%q]=%q ≠ Service metadata.labels[%q]=%q (Prometheus 가 Service 못 찾음)",
				k, v, k, svcMetaLabels[k])
		}
	}

	// 핵심 라벨 둘 다 존재 검증.
	core := []string{"control-plane", "app.kubernetes.io/name"}
	for _, k := range core {
		if podLabels[k] == "" {
			t.Errorf("핵심 라벨 %q 가 pod labels 에 없음", k)
		}
	}
}

func extractRelevantLabels(m map[string]any) map[string]any {
	out := map[string]any{}
	if k, ok := m["kind"].(string); ok {
		out["kind"] = k
	}
	if md, ok := m["metadata"].(map[string]any); ok {
		if labels, ok := md["labels"].(map[string]any); ok {
			out["meta_labels"] = toStringMap(labels)
		}
	}
	if spec, ok := m["spec"].(map[string]any); ok {
		// Deployment: spec.selector.matchLabels + spec.template.metadata.labels.
		if sel, ok := spec["selector"].(map[string]any); ok {
			if ml, ok := sel["matchLabels"].(map[string]any); ok {
				out["selector"] = toStringMap(ml)
			} else {
				// Service: spec.selector 는 직접 map.
				out["selector"] = toStringMap(sel)
			}
		}
		if tmpl, ok := spec["template"].(map[string]any); ok {
			if meta, ok := tmpl["metadata"].(map[string]any); ok {
				if labels, ok := meta["labels"].(map[string]any); ok {
					out["pod_labels"] = toStringMap(labels)
				}
			}
		}
	}
	return out
}

func toStringMap(m map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func findDocByKind(docs []map[string]any, kind string) map[string]any {
	for _, d := range docs {
		if k, ok := d["kind"].(string); ok && k == kind {
			return d
		}
	}
	return nil
}
