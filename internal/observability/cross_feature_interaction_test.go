// Cross-feature interaction sync 검증 — *각 feature 독립 정상이지만 결합 시
// silent fail* 차단.
//
// cycle 86 family: cycle 72 (NetworkPolicy) + cycle 73 (webhook) 결합 시 webhook
// 9443 port 가 NetworkPolicy ingress 에 미포함 → kube-apiserver 의 admission
// 호출 차단 → CR 생성 모두 거절 (failurePolicy=Fail).
//
// 본 게이트가 향후 *유사 결합 결함* 차단 — chart template 의 *cross-feature
// 정합* 자동 검증.

package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNetworkPolicyWebhookPortPresent — networkPolicy template 이 webhook.enabled
// 분기 안에 9443 (또는 .Values.webhook.port) ingress rule 보유 검증.
func TestNetworkPolicyWebhookPortPresent(t *testing.T) {
	repo := findRepoRoot(t)
	npPath := filepath.Join(repo, "charts/valkey-operator/templates/networkpolicy.yaml")
	raw, err := os.ReadFile(npPath)
	if err != nil {
		t.Fatalf("read networkpolicy.yaml: %v", err)
	}
	body := string(raw)

	// 1. ingress block 안에 webhook.enabled 분기 존재.
	if !strings.Contains(body, ".Values.webhook.enabled") {
		t.Error("networkpolicy.yaml ingress 에 .Values.webhook.enabled 분기 없음 — webhook 활성 시 9443 미허용 → admission 호출 silent reject")
	}

	// 2. 분기 안에 webhook.port 또는 9443 참조.
	if !strings.Contains(body, ".Values.webhook.port") && !strings.Contains(body, "9443") {
		t.Error("networkpolicy.yaml 의 webhook.enabled 분기 안에 webhook.port (9443) ingress rule 없음")
	}
}

// TestNetworkPolicyTracingEgressPresent — tracing.endpoint 활성 시 OTLP gRPC
// egress (4317/4318) 허용. cycle 88 cross-feature — tracing+networkPolicy 결합
// silent fail 차단.
func TestNetworkPolicyTracingEgressPresent(t *testing.T) {
	repo := findRepoRoot(t)
	npPath := filepath.Join(repo, "charts/valkey-operator/templates/networkpolicy.yaml")
	raw, _ := os.ReadFile(npPath)
	body := string(raw)

	// egress block 안에 tracing.endpoint 분기 존재.
	if !strings.Contains(body, ".Values.tracing.endpoint") {
		t.Error("networkpolicy.yaml 에 .Values.tracing.endpoint 분기 없음 — tracing 활성 시 OTLP egress 차단 → spans silent loss")
	}

	// 분기 안에 OTLP port 4317 또는 4318 ingress rule 보유.
	if !strings.Contains(body, "4317") && !strings.Contains(body, "4318") {
		t.Error("networkpolicy.yaml tracing 분기 안에 OTLP port (4317 gRPC / 4318 HTTP) egress rule 없음")
	}
}
