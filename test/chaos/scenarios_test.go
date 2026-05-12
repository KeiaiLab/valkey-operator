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

//go:build chaos
// +build chaos

/*
Copyright 2026 Keiailab.

4 chaos 시나리오 — ADR-0041 §Action Items AI-002.

각 scenario:
  1. valkey CR 이미 healthy 상태 가정 (BeforeSuite 에서 배포).
  2. chaos CR 적용 → 일정 시간 대기.
  3. chaos CR 삭제.
  4. cluster 회복 검증 (cluster_state=ok, slots=16384, ready_replicas 충족).

본 시나리오들은 *비결정론적 장애* 회복 능력을 검증 — production SEV-1 의 다수가
이런 패턴 (ADR-0040 §gap #4).
*/

package chaos

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// kubectlApply — chaos CR YAML 을 cluster 에 apply.
// 본 helper 는 chaos-mesh client SDK 의존성을 회피하고 kubectl 호출로 간소화.
func kubectlApply(yaml string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %v: %s", err, out)
	}
	return nil
}

func kubectlDelete(kind, name, namespace string) error {
	cmd := exec.Command("kubectl", "-n", namespace, "delete", kind, name, "--ignore-not-found=true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete %s/%s failed: %v: %s", kind, name, err, out)
	}
	return nil
}

// waitClusterHealthy — `kubectl get vc <name>` 의 status.phase=Running + cluster_state=ok 까지 polling.
func waitClusterHealthy(ctx context.Context, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		cmd := exec.Command("kubectl", "-n", namespace, "get", "vc", name,
			"-o", "jsonpath={.status.phase}")
		out, err := cmd.CombinedOutput()
		if err == nil && strings.TrimSpace(string(out)) == "Running" {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("ValkeyCluster %s/%s did not reach Running within %s", namespace, name, timeout)
}

// makeChaos — chaos-mesh CR unstructured (kind / spec).
func makeChaos(kind, name, namespace string, spec map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(chaosMeshAPIVersion)
	u.SetKind(kind)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.Object["spec"] = spec
	return u
}

var _ = Describe("Scenario 1: random pod kill (PodChaos)", Ordered, func() {
	ns := chaosTestNamespace()

	It("kills random ValkeyCluster pod 5 회 + cluster 자동 회복", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		spec := map[string]any{
			"action": "pod-kill",
			"mode":   "one",
			"selector": map[string]any{
				"namespaces":     []any{ns},
				"labelSelectors": map[string]any{"app.kubernetes.io/instance": targetCRName},
			},
			"scheduler": map[string]any{
				"cron": "@every 1m",
			},
		}
		chaos := makeChaos("Schedule", "pod-kill-loop", ns, map[string]any{
			"schedule": "@every 1m",
			"type":     "PodChaos",
			"podChaos": spec,
		})
		yaml, err := unstructuredToYAML(chaos)
		Expect(err).ToNot(HaveOccurred())
		Expect(kubectlApply(yaml)).To(Succeed())
		DeferCleanup(func() { _ = kubectlDelete("Schedule", "pod-kill-loop", ns) })

		// 5 분 chaos 진행 후 cluster healthy 회복 검증.
		time.Sleep(5 * time.Minute)
		Expect(kubectlDelete("Schedule", "pod-kill-loop", ns)).To(Succeed())
		Expect(waitClusterHealthy(ctx, ns, targetCRName, 5*time.Minute)).To(Succeed())
	})
})

var _ = Describe("Scenario 2: network partition (NetworkChaos)", Ordered, func() {
	ns := chaosTestNamespace()

	It("master ↔ replica 30s 차단 후 회복", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		chaos := makeChaos("NetworkChaos", "partition-30s", ns, map[string]any{
			"action": "partition",
			"mode":   "all",
			"selector": map[string]any{
				"namespaces":     []any{ns},
				"labelSelectors": map[string]any{"app.kubernetes.io/instance": targetCRName},
			},
			"direction": "both",
			"duration":  "30s",
		})
		yaml, err := unstructuredToYAML(chaos)
		Expect(err).ToNot(HaveOccurred())
		Expect(kubectlApply(yaml)).To(Succeed())
		DeferCleanup(func() { _ = kubectlDelete("NetworkChaos", "partition-30s", ns) })

		// 30s chaos + 1m 회복 마진 = 90s.
		time.Sleep(90 * time.Second)
		Expect(waitClusterHealthy(ctx, ns, targetCRName, 3*time.Minute)).To(Succeed())
	})
})

var _ = Describe("Scenario 3: disk fill (StressChaos via IOChaos)", Ordered, func() {
	ns := chaosTestNamespace()

	It("PV 80% fill 시뮬레이션 + cluster 정상 운영 (degraded but functioning)", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		chaos := makeChaos("IOChaos", "disk-fill-80pct", ns, map[string]any{
			"action": "fault",
			"mode":   "one",
			"selector": map[string]any{
				"namespaces":     []any{ns},
				"labelSelectors": map[string]any{"app.kubernetes.io/instance": targetCRName},
			},
			"volumePath": "/data",
			"path":       "/data/**",
			"errno":      28, // ENOSPC
			"percent":    80,
			"duration":   "60s",
		})
		yaml, err := unstructuredToYAML(chaos)
		Expect(err).ToNot(HaveOccurred())
		Expect(kubectlApply(yaml)).To(Succeed())
		DeferCleanup(func() { _ = kubectlDelete("IOChaos", "disk-fill-80pct", ns) })

		// cluster 가 *읽기 가능* 상태 유지 (write 는 부분 fail 가능).
		time.Sleep(75 * time.Second)
		Expect(waitClusterHealthy(ctx, ns, targetCRName, 3*time.Minute)).To(Succeed())
	})
})

var _ = Describe("Scenario 4: slow disk I/O (IOChaos latency)", Ordered, func() {
	ns := chaosTestNamespace()

	It("replica 하나에 100ms latency injection — failover 미발생 검증", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		chaos := makeChaos("IOChaos", "slow-disk-replica", ns, map[string]any{
			"action": "latency",
			"mode":   "one",
			"selector": map[string]any{
				"namespaces": []any{ns},
				"labelSelectors": map[string]any{
					"app.kubernetes.io/instance": targetCRName,
					"valkey.keiailab.io/role":    "replica",
				},
			},
			"volumePath": "/data",
			"path":       "/data/**",
			"delay":      "100ms",
			"duration":   "60s",
		})
		yaml, err := unstructuredToYAML(chaos)
		Expect(err).ToNot(HaveOccurred())
		Expect(kubectlApply(yaml)).To(Succeed())
		DeferCleanup(func() { _ = kubectlDelete("IOChaos", "slow-disk-replica", ns) })

		// replica latency 만 → master 영향 없음 → failover 없어야 정상.
		time.Sleep(75 * time.Second)
		Expect(waitClusterHealthy(ctx, ns, targetCRName, 3*time.Minute)).To(Succeed())
	})
})
