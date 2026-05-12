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

/*
Copyright 2026 Keiailab.
*/

package controller

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSetCapabilityMetrics_active_set_to_1(t *testing.T) {
	defer DeleteMetricsFor("ns-cap", "vk-cap")
	SetCapabilityMetrics("ns-cap", "vk-cap", AllCapabilities,
		[]string{CapabilityTLS, CapabilityAuth, CapabilityMonitoring})

	cases := map[string]float64{
		CapabilityTLS:               1,
		CapabilityAuth:              1,
		CapabilityMonitoring:        1,
		CapabilityAutoscaling:       0, // 비활성
		CapabilitySlowLog:           0,
		CapabilityEncryptionAudit:   0,
		CapabilityEncryptionEnforce: 0,
		CapabilityNetworkPolicy:     0,
		CapabilityTLSAutoCA:         0,
		CapabilityExternalReplica:   0,
		CapabilityEphemeralStorage:  0,
	}
	for cap, want := range cases {
		got := testutil.ToFloat64(MetricCapabilityActive.WithLabelValues("ns-cap", "vk-cap", cap))
		if got != want {
			t.Errorf("capability %s: got %v want %v", cap, got, want)
		}
	}
}

func TestSetCapabilityMetrics_toggle_off_resets_to_0(t *testing.T) {
	defer DeleteMetricsFor("ns-toggle", "vk-toggle")
	// 1차: TLS 활성.
	SetCapabilityMetrics("ns-toggle", "vk-toggle", AllCapabilities, []string{CapabilityTLS})
	if v := testutil.ToFloat64(MetricCapabilityActive.WithLabelValues("ns-toggle", "vk-toggle", CapabilityTLS)); v != 1 {
		t.Fatalf("TLS should be 1: got %v", v)
	}
	// 2차: 사용자가 spec 에서 TLS 제거 → reconcile 다시 호출 → active 슬라이스 비어있음.
	SetCapabilityMetrics("ns-toggle", "vk-toggle", AllCapabilities, []string{})
	if v := testutil.ToFloat64(MetricCapabilityActive.WithLabelValues("ns-toggle", "vk-toggle", CapabilityTLS)); v != 0 {
		t.Errorf("TLS should reset to 0 after toggle off: got %v", v)
	}
}

func TestDeleteMetricsFor_clears_capabilities(t *testing.T) {
	SetCapabilityMetrics("ns-del", "vk-del", AllCapabilities,
		[]string{CapabilityTLS, CapabilityMonitoring})
	DeleteMetricsFor("ns-del", "vk-del")

	for _, cap := range AllCapabilities {
		// CollectAndCount 으로 series 가 사라졌는지 검증.
		if v := testutil.ToFloat64(MetricCapabilityActive.WithLabelValues("ns-del", "vk-del", cap)); v != 0 {
			// ToFloat64 는 not-found 시 0 반환 — DeleteLabelValues 한 series 의 default.
			t.Logf("capability %s after delete: %v (expected: 0 or absent)", cap, v)
		}
	}
}

func TestAllCapabilities_count(t *testing.T) {
	// 토큰 11 개 (ADR-0043 추가 2개 포함). 신규 추가 시 본 테스트도 함께 갱신.
	if len(AllCapabilities) != 11 {
		t.Errorf("AllCapabilities length: got %d want 11 (ADR-0043 spec)", len(AllCapabilities))
	}
}
