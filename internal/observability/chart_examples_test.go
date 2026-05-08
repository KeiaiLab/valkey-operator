// charts/valkey-operator/Chart.yaml 의 artifacthub.io/crdsExamples 와
// artifacthub.io/images annotation 의 정확성 검증.
//
// 사고 패턴 1 (실제 발견): crdsExamples 가 잘못된 enum / 필드 이름 사용 →
// ArtifactHub UI 에서 사용자가 copy → kubectl apply → admission reject.
// 사용자가 "처음 시도해보는 시점" 의 신뢰 손상.
//
// 사고 패턴 2: artifacthub.io/images 의 image tag 가 Chart.AppVersion 과 drift
// → ArtifactHub UI 가 *틀린 version 의 image* 를 사용자에게 안내.
//
// 사고 패턴 3: Chart icon URL 이 외부 사이트 개편으로 404 → ArtifactHub
// tracking warning 과 package logo 누락.

package observability

import (
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

type chartFile struct {
	Version     string            `json:"version"`
	AppVersion  string            `json:"appVersion"`
	Icon        string            `json:"icon"`
	Annotations map[string]string `json:"annotations"`
}

func loadChart(t *testing.T) *chartFile {
	t.Helper()
	candidates := []string{"charts/valkey-operator/Chart.yaml", "../../charts/valkey-operator/Chart.yaml", "../../../charts/valkey-operator/Chart.yaml"}
	var path string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			path = c
			break
		}
	}
	if path == "" {
		t.Fatalf("Chart.yaml not found: %v", candidates)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var ch chartFile
	if err := yaml.Unmarshal(raw, &ch); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &ch
}

func TestChartImagesAnnotationMatchesAppVersion(t *testing.T) {
	ch := loadChart(t)
	imagesYaml := ch.Annotations["artifacthub.io/images"]
	if imagesYaml == "" {
		t.Fatal("artifacthub.io/images annotation 비어 있음")
	}
	// 첫 번째 image (operator 자체) 가 AppVersion 을 tag 로 사용해야.
	expected := "ghcr.io/keiailab/valkey-operator:" + ch.AppVersion
	if !strings.Contains(imagesYaml, expected) {
		t.Errorf("artifacthub.io/images 가 %q 를 포함하지 않음 — Chart.AppVersion=%q 와 drift",
			expected, ch.AppVersion)
	}
}

func TestChartIconURLUsesCurrentValkeyAsset(t *testing.T) {
	ch := loadChart(t)
	if ch.Icon == "" {
		t.Fatal("Chart icon URL 비어 있음")
	}
	if ch.Icon == "https://valkey.io/img/Valkey-Logo-RGB-Color.svg" {
		t.Fatal("Chart icon 이 Artifact Hub 에서 404 를 반환한 과거 Valkey logo URL 을 사용함")
	}
	if ch.Icon != "https://valkey.io/img/valkey-horizontal.svg" {
		t.Fatalf("Chart icon=%q, want current Valkey logo asset", ch.Icon)
	}
}

// crdsExamples 를 strict 하게 Go 타입과 매칭. 각 entry 의 kind 별로 다른 타입.
func TestChartCRDExamplesStrictUnmarshal(t *testing.T) {
	ch := loadChart(t)
	examplesYaml := ch.Annotations["artifacthub.io/crdsExamples"]
	if examplesYaml == "" {
		t.Fatal("artifacthub.io/crdsExamples annotation 비어 있음")
	}
	// YAML 의 여러 docs 가 [...] 슬라이스 형태.
	var examples []map[string]any
	if err := yaml.Unmarshal([]byte(examplesYaml), &examples); err != nil {
		t.Fatalf("crdsExamples slice 파싱 실패: %v", err)
	}
	if len(examples) == 0 {
		t.Fatal("crdsExamples 0 entries")
	}

	for i, ex := range examples {
		kind, _ := ex["kind"].(string)
		t.Run(kind, func(t *testing.T) {
			rawEntry, err := yaml.Marshal(ex)
			if err != nil {
				t.Fatalf("re-marshal entry %d: %v", i, err)
			}
			var obj any
			switch kind {
			case "Valkey":
				obj = &cachev1alpha1.Valkey{}
			case "ValkeyCluster":
				obj = &cachev1alpha1.ValkeyCluster{}
			case "ValkeyBackup":
				obj = &cachev1alpha1.ValkeyBackup{}
			case "ValkeyBackupTarget":
				obj = &cachev1alpha1.ValkeyBackupTarget{}
			case "ValkeyRestore":
				obj = &cachev1alpha1.ValkeyRestore{}
			default:
				t.Fatalf("알 수 없는 kind=%q", kind)
			}
			if err := yaml.UnmarshalStrict(rawEntry, obj); err != nil {
				t.Errorf("strict unmarshal 실패 (api 타입과 example drift) — %v", err)
			}
		})
	}
}
