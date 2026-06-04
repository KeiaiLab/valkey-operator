/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// charts/valkey-operator/templates/deployment.yaml 의 args ↔ cmd/main.go 의
// flag.{StringVar,BoolVar} 정의 동기 검증.
//
// 사고 패턴 (cycle 68 발견): chart 가 `--enable-cluster-controller=true` 같은
// 옛 flag 를 args 에 포함 → cmd/main.go 가 parse 안 함 → operator immediate exit
// → CrashLoopBackOff. *Helm 사용자가 features.cluster.enabled=true 설정 시 정확
// 히 broken*.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestChartArgsMatchOperatorFlags(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. cmd/main.go 의 모든 flag 이름 추출.
	mainRaw, _ := os.ReadFile(filepath.Join(repo, "cmd/main.go"))
	flagRe := regexp.MustCompile(`flag\.(?:String|Bool|Int|Duration)Var\(\&\w+,\s*"([\w-]+)"`)
	definedFlags := map[string]bool{}
	for _, m := range flagRe.FindAllStringSubmatch(string(mainRaw), -1) {
		definedFlags[m[1]] = true
	}
	// controller-runtime / zap.Options 등이 추가하는 표준 flag 화이트리스트.
	standardFlags := map[string]bool{
		"zap-devel":            true,
		"zap-encoder":          true,
		"zap-log-level":        true,
		"zap-stacktrace-level": true,
		"zap-time-encoding":    true,
		"kubeconfig":           true,
		"v":                    true, // klog
		"add_dir_header":       true,
		"alsologtostderr":      true,
		"log_backtrace_at":     true,
		"log_dir":              true,
		"log_file":             true,
		"log_file_max_size":    true,
		"logtostderr":          true,
		"one_output":           true,
		"skip_headers":         true,
		"skip_log_headers":     true,
		"stderrthreshold":      true,
		"vmodule":              true,
	}
	if len(definedFlags) == 0 {
		t.Fatal("cmd/main.go 의 flag 정의 0건 — 정규식 회귀")
	}

	// 2. chart deployment.yaml 의 args 추출 (`- --<flag>=...` 패턴, helm template
	// 변수 {{...}} 부분은 placeholder 처리).
	deployRaw, _ := os.ReadFile(filepath.Join(repo, "charts/valkey-operator/templates/deployment.yaml"))
	argRe := regexp.MustCompile(`-\s*--([\w-]+)`)
	for _, m := range argRe.FindAllStringSubmatch(string(deployRaw), -1) {
		flag := m[1]
		if definedFlags[flag] {
			continue
		}
		if standardFlags[flag] {
			continue
		}
		t.Errorf("chart deployment.yaml args 의 --%s 가 cmd/main.go 에 정의된 flag 가 아님 (operator 즉시 exit + CrashLoopBackOff)", flag)
	}

	// 3. config/manager/manager.yaml 의 args 도 검증 (kustomize 사용자 동일 위험).
	mgrRaw, _ := os.ReadFile(filepath.Join(repo, "config/manager/manager.yaml"))
	for _, m := range argRe.FindAllStringSubmatch(string(mgrRaw), -1) {
		flag := m[1]
		// helm template 변수 부분은 mgr.yaml 에 없음 — 모두 literal.
		if !definedFlags[flag] && !standardFlags[flag] {
			t.Errorf("config/manager/manager.yaml args 의 --%s 가 cmd/main.go 에 정의된 flag 가 아님",
				flag)
		}
	}
}

// 보조: chart 가 placeholder 변수 ({{ .Values.* }}) 사용 시 args 패턴 가시화.
var _ = strings.TrimSpace
