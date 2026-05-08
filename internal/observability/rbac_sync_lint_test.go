// kubebuilder:rbac 마커 ↔ config/rbac/role.yaml 동기 검증.
//
// 사고 패턴: controller 가 신규 API 호출 (e.g., r.Get(ctx, ..., &corev1.Pod{}))
// 추가 후 marker 만 갱신, `make manifests` 미실행 → role.yaml drift.
// production 배포 후 RBAC denied 가 reconcile error 로 silent 누적, 메트릭에는
// reconcile_errors_total 만 증가 — 신규 기능이 *침묵 실패*.
//
// 본 테스트는 *resource 이름* 양방향 동기를 검증 (marker 의 resources= 와
// role.yaml 의 resources: 항목). verb/group 까지 검증하면 false positive
// 위험 (markers 가 group 별로 나눠져 있고 role.yaml 은 통합 — controller-gen
// 이 normalize) — resource set 의 양방향 동치만 강제.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

type rbacRule struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

type rbacRoleFile struct {
	Rules []rbacRule `json:"rules"`
}

func loadRBACRoleResources(t *testing.T) map[string]bool {
	t.Helper()
	candidates := []string{"config/rbac/role.yaml", "../../config/rbac/role.yaml", "../../../config/rbac/role.yaml"}
	var path string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			path = c
			break
		}
	}
	if path == "" {
		t.Fatalf("role.yaml not found: %v", candidates)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var f rbacRoleFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	out := map[string]bool{}
	for _, r := range f.Rules {
		for _, res := range r.Resources {
			out[res] = true
		}
	}
	return out
}

// loadKubebuilderRBACResources — internal/controller/*.go 의 모든 RBAC 마커에서
// resources= 항목 추출.
func loadKubebuilderRBACResources(t *testing.T) map[string]bool {
	t.Helper()
	candidates := []string{"internal/controller", "../../internal/controller", "../../../internal/controller"}
	var dir string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			dir = c
			break
		}
	}
	if dir == "" {
		t.Fatalf("internal/controller dir not found: %v", candidates)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	// 정규식: resources=foo;bar;baz, (콤마 또는 줄 끝까지).
	resRe := regexp.MustCompile(`resources=([^,\s]+)`)
	out := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range resRe.FindAllStringSubmatch(string(raw), -1) {
			for res := range strings.SplitSeq(m[1], ";") {
				out[res] = true
			}
		}
	}
	return out
}

func TestRBACMarkerResourcesInRole(t *testing.T) {
	markerRes := loadKubebuilderRBACResources(t)
	roleRes := loadRBACRoleResources(t)

	if len(markerRes) == 0 {
		t.Fatal("kubebuilder:rbac 마커 0건 — controller 파일 위치 회귀")
	}

	for res := range markerRes {
		if !roleRes[res] {
			t.Errorf("kubebuilder:rbac 마커의 resource %q 가 config/rbac/role.yaml 에 없음 — `make manifests` 실행 필요",
				res)
		}
	}
}

func TestRBACRoleResourcesInMarker(t *testing.T) {
	markerRes := loadKubebuilderRBACResources(t)
	roleRes := loadRBACRoleResources(t)

	if len(roleRes) == 0 {
		t.Fatal("role.yaml resource 0 — YAML 파싱 회귀")
	}

	for res := range roleRes {
		if !markerRes[res] {
			t.Errorf("role.yaml 의 resource %q 가 kubebuilder:rbac 마커에 없음 — orphan 권한 (controller 코드 삭제됐는데 권한 잔재) 가능성",
				res)
		}
	}
}
