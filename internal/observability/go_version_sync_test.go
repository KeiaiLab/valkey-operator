// Dockerfile 의 builder Go version ↔ go.mod 의 `go X.Y` directive 동기 검증.
//
// 사고 패턴: go.mod 가 `go 1.26` 으로 bump (신규 std lib 사용) 되었지만 Dockerfile
// builder 는 여전히 `golang:1.25` → `go build` 실패 (`required Go version`
// error) → docker build CrashLoopBackOff. cycle 47 의 go.mod tidy 와 sibling.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestGoVersionDockerfileVsGoMod(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. go.mod 의 `go X.Y[.Z]` directive 추출.
	gomodRaw, _ := os.ReadFile(filepath.Join(repo, "go.mod"))
	goRe := regexp.MustCompile(`(?m)^go\s+(\d+)\.(\d+)`)
	m := goRe.FindStringSubmatch(string(gomodRaw))
	if m == nil {
		t.Fatal("go.mod 의 `go X.Y` directive 추출 실패")
	}
	gomodMajor, _ := strconv.Atoi(m[1])
	gomodMinor, _ := strconv.Atoi(m[2])

	// 2. Dockerfile 의 `FROM golang:X.Y[.Z]` builder image 추출.
	dockerRaw, _ := os.ReadFile(filepath.Join(repo, "Dockerfile"))
	fromRe := regexp.MustCompile(`FROM\s+golang:(\d+)\.(\d+)`)
	dm := fromRe.FindStringSubmatch(string(dockerRaw))
	if dm == nil {
		t.Fatal("Dockerfile 의 FROM golang 추출 실패")
	}
	dockerMajor, _ := strconv.Atoi(dm[1])
	dockerMinor, _ := strconv.Atoi(dm[2])

	// 3. Dockerfile Go version ≥ go.mod 최소 (Go 의 forward compatibility).
	if dockerMajor < gomodMajor || (dockerMajor == gomodMajor && dockerMinor < gomodMinor) {
		t.Errorf("Dockerfile golang:%d.%d < go.mod go %d.%d — docker build 시 'required Go version' 에러. Dockerfile 의 FROM golang:X.Y 갱신 필요",
			dockerMajor, dockerMinor, gomodMajor, gomodMinor)
	}
	// 정확 동등 권장 (Dockerfile 이 *go.mod 와 같은 또는 한 minor 위* 권장).
	if dockerMajor == gomodMajor && dockerMinor > gomodMinor+2 {
		t.Logf("warning: Dockerfile golang:%d.%d 가 go.mod %d.%d 보다 +2 minor 이상 — 의식적 결정 인지 확인 (Go 버전 정합성)",
			dockerMajor, dockerMinor, gomodMajor, gomodMinor)
	}
	if !strings.Contains(string(gomodRaw), "go ") {
		t.Fatal("go.mod 형식 회귀")
	}

	// 4. CONTRIBUTING.md 의 Go 버전 table 동기 — `| Go | X.Y | go.mod 와 일치 |` 형식.
	contribRaw, _ := os.ReadFile(filepath.Join(repo, "CONTRIBUTING.md"))
	contribRe := regexp.MustCompile(`\|\s*Go\s*\|\s*(\d+)\.(\d+)\s*\|`)
	cm := contribRe.FindStringSubmatch(string(contribRaw))
	if cm == nil {
		t.Log("CONTRIBUTING.md 의 Go version table 추출 실패 — 정규식 또는 doc 변경 — skip")
		return
	}
	contribMajor, _ := strconv.Atoi(cm[1])
	contribMinor, _ := strconv.Atoi(cm[2])
	if contribMajor != gomodMajor || contribMinor != gomodMinor {
		t.Errorf("CONTRIBUTING.md 의 Go %d.%d ≠ go.mod %d.%d — 환경 요구사항 table 갱신 필요",
			contribMajor, contribMinor, gomodMajor, gomodMinor)
	}
}

// TestKubernetesVersionSync — Chart.yaml kubeVersion ↔ README 의 K8s badge ↔
// chart README "Kubernetes X.Y+" 동기 (cycle 97).
//
// 사고 패턴: K8s 호환 minimum bump (e.g., 1.28+) 시 *3 표면 동시 갱신* 누락 →
// 사용자가 *어떤 표면 보았는가* 에 따라 *다른 minimum* 로 알게 됨 — 신뢰 저하.
func TestKubernetesVersionSync(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. Chart.yaml kubeVersion: ">=X.Y.Z-W".
	chartRaw, _ := os.ReadFile(filepath.Join(repo, "charts/valkey-operator/Chart.yaml"))
	kvRe := regexp.MustCompile(`kubeVersion:\s*"?>=(\d+)\.(\d+)`)
	cm := kvRe.FindStringSubmatch(string(chartRaw))
	if cm == nil {
		t.Fatal("Chart.yaml kubeVersion 추출 실패")
	}
	chartMajor, _ := strconv.Atoi(cm[1])
	chartMinor, _ := strconv.Atoi(cm[2])

	// 2. README badge: "Kubernetes-X.Y+".
	readmeRaw, _ := os.ReadFile(filepath.Join(repo, "README.md"))
	badgeRe := regexp.MustCompile(`Kubernetes-(\d+)\.(\d+)\+`)
	rm := badgeRe.FindStringSubmatch(string(readmeRaw))
	if rm == nil {
		t.Fatal("README.md K8s badge 추출 실패")
	}
	readmeMajor, _ := strconv.Atoi(rm[1])
	readmeMinor, _ := strconv.Atoi(rm[2])

	// 3. chart README "- Kubernetes X.Y+".
	chartReadmeRaw, _ := os.ReadFile(filepath.Join(repo, "charts/valkey-operator/README.md"))
	listRe := regexp.MustCompile(`Kubernetes\s+(\d+)\.(\d+)\+`)
	crm := listRe.FindStringSubmatch(string(chartReadmeRaw))
	if crm == nil {
		t.Fatal("chart README Kubernetes prerequisite 추출 실패")
	}
	chartReadmeMajor, _ := strconv.Atoi(crm[1])
	chartReadmeMinor, _ := strconv.Atoi(crm[2])

	// 3 표면 모두 동일.
	if chartMajor != readmeMajor || chartMinor != readmeMinor {
		t.Errorf("Chart.yaml kubeVersion %d.%d ≠ README badge %d.%d", chartMajor, chartMinor, readmeMajor, readmeMinor)
	}
	if chartMajor != chartReadmeMajor || chartMinor != chartReadmeMinor {
		t.Errorf("Chart.yaml kubeVersion %d.%d ≠ chart README %d.%d", chartMajor, chartMinor, chartReadmeMajor, chartReadmeMinor)
	}
}
