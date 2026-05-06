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
}
