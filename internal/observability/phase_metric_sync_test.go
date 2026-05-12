/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// metrics.go::allPhases (MetricPhase label values) ↔ api/v1alpha1 의 ValkeyPhase
// + ClusterPhase enum 동기 검증.
//
// 사고 패턴: 신규 phase (e.g., "Migrating") 가 ValkeyPhase 또는 ClusterPhase 에
// 추가되었지만 metrics.go::allPhases 갱신 누락 → SetPhaseMetric 이 *알 수 없는
// phase* 인식 못 함 → MetricPhase 시계열 incomplete → Grafana dashboard 의
// "phase=Unknown" 같은 placeholder 표시.
//
// 본 게이트가 *최소 superset* 검증 — allPhases ⊇ union(ValkeyPhase, ClusterPhase).

package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMetricPhaseLabelsSync(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. metrics.go::allPhases 추출.
	mRaw, _ := os.ReadFile(filepath.Join(repo, "internal/controller/metrics.go"))
	allPhasesRe := regexp.MustCompile(`allPhases\s*:?=\s*\[\]string\{([^}]+)\}`)
	m := allPhasesRe.FindStringSubmatch(string(mRaw))
	if m == nil {
		t.Fatal("metrics.go::allPhases 추출 실패")
	}
	metricPhases := map[string]bool{}
	for p := range strings.SplitSeq(m[1], ",") {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			metricPhases[p] = true
		}
	}

	// 2. api/v1alpha1 의 ValkeyPhase + ClusterPhase 추출.
	apiPhases := map[string]bool{}
	enumRe := regexp.MustCompile(`(?:ValkeyPhase|ClusterPhase)\s*=\s*"([^"]+)"`)
	for _, fname := range []string{"valkey_types.go", "valkeycluster_types.go"} {
		raw, err := os.ReadFile(filepath.Join(repo, "api/v1alpha1", fname))
		if err != nil {
			continue
		}
		for _, mm := range enumRe.FindAllStringSubmatch(string(raw), -1) {
			apiPhases[mm[1]] = true
		}
	}
	if len(apiPhases) == 0 {
		t.Fatal("api Phase enum 0건 — 정규식 회귀")
	}

	// 3. metricPhases ⊇ apiPhases 검증.
	for p := range apiPhases {
		if !metricPhases[p] {
			t.Errorf("api Phase %q 가 metrics.go::allPhases 에 없음 — Grafana dashboard 시계열 incomplete (cycle 25 의 alert SSOT 와 sibling)",
				p)
		}
	}
}
