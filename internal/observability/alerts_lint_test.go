// Prometheus alert rules YAML 정합성 검증 — promtool 미설치 환경 대비 in-process lint.
//
// 검증 항목:
//  1. YAML 파싱 성공 + PrometheusRule 스키마 (apiVersion/kind/spec.groups).
//  2. 각 alert 의 필수 필드: alert / expr / for / labels.severity /
//     annotations.summary + annotations.description.
//  3. severity ∈ {critical, warning, info}.
//  4. for: duration 형식 (Prometheus model.Duration).
//  5. expr 가 참조하는 메트릭 이름이 실제 등록된 메트릭 (subsystem=valkey_cluster) 또는
//     Prometheus 표준 메트릭 (up{...} 등) 인지 화이트리스트 검증.
//  6. alert 이름 prefix "Valkey".
//
// promtool 의 `expr` Prometheus 파싱은 본 lint 의 범위 밖 — promtool 설치 환경에서만
// 가능. 대신 metric name 동기 차단 (metric rename 시 silent alert miss 방지) 에 집중.

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

type promRuleFile struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Groups []promRuleGroup `json:"groups"`
	} `json:"spec"`
}

type promRuleGroup struct {
	Name     string         `json:"name"`
	Interval string         `json:"interval,omitempty"`
	Rules    []promAlertDef `json:"rules"`
}

type promAlertDef struct {
	Alert       string            `json:"alert,omitempty"`
	Expr        string            `json:"expr"`
	For         string            `json:"for,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// 코드 (internal/controller/metrics.go) 에서 등록된 메트릭의 SSOT.
// metricSubsystem="valkey_cluster" + Name → "valkey_cluster_<name>".
// 변경 시 본 슬라이스 + alert-rules.yaml 동기화 필수.
var registeredAppMetrics = map[string]bool{
	"valkey_cluster_state_ok":               true,
	"valkey_cluster_assigned_slots":         true,
	"valkey_cluster_shards":                 true,
	"valkey_cluster_ready_replicas":         true,
	"valkey_cluster_reconcile_total":        true,
	"valkey_cluster_reconcile_errors_total": true,
	"valkey_cluster_phase":                  true,
	"valkey_cluster_backup_total":           true,
	"valkey_cluster_restore_total":          true,
	"valkey_cluster_failover_total":         true,
}

// Prometheus / kube-prometheus-stack 표준 메트릭 화이트리스트.
var standardMetrics = map[string]bool{
	"up":                 true,
	"controller_runtime": true, // controller-runtime 의 *_total 등.
	"kube_pod_status":    true,
	"prometheus_target":  true,
}

func loadAlertRules(t *testing.T) *promRuleFile {
	t.Helper()
	// repo root 의 config/prometheus/alert-rules.yaml 을 찾는다.
	candidates := []string{
		"config/prometheus/alert-rules.yaml",
		"../../config/prometheus/alert-rules.yaml",
		"../../../config/prometheus/alert-rules.yaml",
	}
	var path string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			path = c
			break
		}
	}
	if path == "" {
		t.Fatalf("alert-rules.yaml not found in candidates: %v", candidates)
	}
	abs, _ := filepath.Abs(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	var f promRuleFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		t.Fatalf("yaml parse %s: %v", abs, err)
	}
	return &f
}

func TestAlertRulesSchemaSanity(t *testing.T) {
	f := loadAlertRules(t)
	if f.APIVersion != "monitoring.coreos.com/v1" {
		t.Errorf("apiVersion: %q (want monitoring.coreos.com/v1)", f.APIVersion)
	}
	if f.Kind != "PrometheusRule" {
		t.Errorf("kind: %q (want PrometheusRule)", f.Kind)
	}
	if len(f.Spec.Groups) == 0 {
		t.Fatal("groups 비어 있음")
	}
}

func TestAlertRulesAllFieldsValid(t *testing.T) {
	f := loadAlertRules(t)
	validSeverity := map[string]bool{"critical": true, "warning": true, "info": true}
	alertCount := 0
	for _, g := range f.Spec.Groups {
		for _, a := range g.Rules {
			if a.Alert == "" {
				continue // recording rule (skip).
			}
			alertCount++
			t.Run(a.Alert, func(t *testing.T) {
				if !strings.HasPrefix(a.Alert, "Valkey") {
					t.Errorf("alert name %q prefix 'Valkey' 아님", a.Alert)
				}
				if a.Expr == "" {
					t.Error("expr 비어 있음")
				}
				if a.For == "" {
					t.Error("for 누락 (즉시 알람 → 노이즈 위험)")
				} else if _, err := time.ParseDuration(a.For); err != nil {
					t.Errorf("for=%q duration 파싱 실패: %v", a.For, err)
				}
				if got := a.Labels["severity"]; got == "" || !validSeverity[got] {
					t.Errorf("severity=%q (want critical|warning|info)", got)
				}
				if a.Annotations["summary"] == "" {
					t.Error("annotations.summary 누락")
				}
				if a.Annotations["description"] == "" {
					t.Error("annotations.description 누락")
				}
				if a.Annotations["runbook_url"] == "" {
					t.Error("annotations.runbook_url 누락 — on-call MTTR 위해 필수")
				}
			})
		}
	}
	if alertCount == 0 {
		t.Fatal("alert 0건 — 본 테스트가 무력화됨")
	}
	if alertCount < 10 {
		t.Errorf("alert %d 건 — 10+ 기대 (cluster + operator + business 카테고리)", alertCount)
	}
}

// runbook_url 의 anchor 가 실제 runbook.md 에 존재하는지 검증.
// 섹션 rename 시 silent broken link 차단 (on-call 이 404 페이지 만나는 사고 예방).
func TestAlertRulesRunbookAnchorsExist(t *testing.T) {
	f := loadAlertRules(t)
	// runbook.md 로딩.
	candidates := []string{
		"docs/operations/runbook.md",
		"../../docs/operations/runbook.md",
		"../../../docs/operations/runbook.md",
	}
	var rbPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			rbPath = c
			break
		}
	}
	if rbPath == "" {
		t.Fatalf("runbook.md not found in: %v", candidates)
	}
	rb, err := os.ReadFile(rbPath)
	if err != nil {
		t.Fatalf("read runbook: %v", err)
	}
	// runbook.md 의 모든 heading → GitHub anchor 형식 (lowercase, 공백→하이픈, 영숫자/하이픈만).
	headingRe := regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	anchors := map[string]bool{}
	for _, m := range headingRe.FindAllStringSubmatch(string(rb), -1) {
		anchors[githubAnchor(m[1])] = true
	}

	urlRe := regexp.MustCompile(`runbook\.md#([\w-]+)`)
	for _, g := range f.Spec.Groups {
		for _, a := range g.Rules {
			url := a.Annotations["runbook_url"]
			if url == "" {
				continue // 별도 테스트가 검증.
			}
			match := urlRe.FindStringSubmatch(url)
			if match == nil {
				t.Errorf("alert %q: runbook_url=%q 가 'runbook.md#anchor' 형식 아님", a.Alert, url)
				continue
			}
			anchor := match[1]
			if !anchors[anchor] {
				t.Errorf("alert %q: runbook_url anchor #%q 가 docs/operations/runbook.md 에 없음", a.Alert, anchor)
			}
		}
	}
}

// githubAnchor — GitHub markdown 의 heading-to-anchor 변환.
// "9.1 ValkeyClusterStateNotOK" → "91-valkeyclusterstatenotok".
func githubAnchor(heading string) string {
	s := strings.ToLower(heading)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
		// 그 외 (점, 슬래시 등) skip.
	}
	// 연속 hyphen → 단일.
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

// 코드 (metrics.go) 가 등록한 메트릭과 alert expr 의 동기 검증.
// metric rename 시 silent alert miss 차단.
func TestAlertRulesMetricNamesRegistered(t *testing.T) {
	f := loadAlertRules(t)
	// expr 에서 "valkey_cluster_<word>" 형태 메트릭 추출.
	metricRe := regexp.MustCompile(`valkey_cluster_[a-z_]+`)
	for _, g := range f.Spec.Groups {
		for _, a := range g.Rules {
			if a.Alert == "" {
				continue
			}
			matches := metricRe.FindAllString(a.Expr, -1)
			for _, m := range matches {
				if !registeredAppMetrics[m] {
					t.Errorf("alert %q: expr 의 메트릭 %q 가 internal/controller/metrics.go 에 등록되지 않음 — registeredAppMetrics 또는 metrics.go 동기화 필요",
						a.Alert, m)
				}
			}
			// up{...} 같은 표준 메트릭은 화이트리스트.
			// 본 테스트는 valkey_cluster_* 만 검사 — 표준 메트릭 알람 (ValkeyOperatorDown) 은 skip.
			_ = standardMetrics
		}
	}
}
