//go:build e2e
// +build e2e

/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Licensed under the MIT License. See the LICENSE file for details.
package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/keiailab/valkey-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// INC-0001 / ADR-0039 regression — ValkeyCluster post-init self-heal.
//
// 시나리오:
//  1. ValkeyCluster bootstrap (3×1 = 6 pods, 16384 slots OK).
//  2. status.clusterInitialized=true 확인.
//  3. Replica 3 pods 에 CLUSTER RESET HARD (no keys 라 성공) → cluster_state:fail
//     강제. master 들은 cluster member 그대로 보존.
//  4. ClusterInitialized=true 가 status 에 보존됨에도 controller 가 *post-init
//     fail* 감지 → ensureClusterMeet 자동 재호출 (ADR-0039 self-heal).
//  5. ~3min 안 cluster_state:ok 회복 + 16384 slots 검증.
//
// 본 테스트는 INC-0001 (운영 keiailab-valkey-prod 19h fail) 가 영구 fix 됐음을
// 보장하는 regression 가드.
var _ = Describe("ValkeyCluster INC-0001 self-heal", Ordered, func() {
	const (
		clusterNamespace = "test-valkey-self-heal-20260510"
		clusterName      = "test-vkc-self-heal"
	)

	var pwd string

	BeforeAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", clusterNamespace, "--ignore-not-found"))
		_, err := utils.Run(exec.Command("kubectl", "create", "ns", clusterNamespace))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkeycluster",
			clusterName, "-n", clusterNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", clusterNamespace, "--ignore-not-found"))
	})

	It("should bootstrap then self-heal after replica CLUSTER RESET (INC-0001/ADR-0039)", func() {
		By("creating a 3 shard x 1 replica ValkeyCluster")
		manifest := fmt.Sprintf(`
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyCluster
metadata:
  name: %s
  namespace: %s
spec:
  shards: 3
  replicasPerShard: 1
  version:
    image: docker.io/valkey/valkey
    version: "9.0.4"
  storage:
    size: 1Gi
`, clusterName, clusterNamespace)
		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for 6 pods Ready + cluster_state=ok + 16384 slots")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "valkeycluster",
				clusterName, "-n", clusterNamespace,
				"-o", "jsonpath={.status.phase}/{.status.clusterState}/{.status.assignedSlots}/{.status.clusterInitialized}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("Running/ok/16384/true"))
		}, 8*time.Minute, 5*time.Second).Should(Succeed())

		By("extracting auth password")
		pwdCmd := fmt.Sprintf(`kubectl get secret %s-auth -n %s -o jsonpath='{.data.password}' | base64 -d`,
			clusterName, clusterNamespace)
		pwdOut, err := utils.Run(exec.Command("sh", "-c", pwdCmd))
		Expect(err).NotTo(HaveOccurred())
		pwd = strings.TrimSpace(pwdOut)
		Expect(pwd).NotTo(BeEmpty())
	})

	It("should auto-recover cluster_state=ok after replica CLUSTER RESET HARD (post-init self-heal)", func() {
		By("forcing cluster fail by CLUSTER RESET HARD on 3 replica pods (no keys → safe)")
		// pods 의 인덱스: 0,1,2 = master 또는 replica (배치 round-robin). 3,4,5 도 동일 패턴.
		// CLUSTER RESET HARD 는 master with keys 거부 → replicas (no keys) 만 reset 됨.
		// master 들은 보존되어 cluster member 그대로.
		anyReset := false
		for i := 0; i < 6; i++ {
			cmd := exec.Command("kubectl", "exec", "-n", clusterNamespace,
				fmt.Sprintf("%s-%d", clusterName, i), "--",
				"valkey-cli", "--no-auth-warning", "-a", pwd,
				"cluster", "reset", "hard")
			out, _ := utils.Run(cmd)
			if strings.Contains(out, "OK") {
				anyReset = true
			}
		}
		Expect(anyReset).To(BeTrue(), "최소 1 pod 에서 CLUSTER RESET HARD 가 성공해야 함 (replicas 는 keys 부재로 reset 가능)")

		By("verifying cluster transitioned to fail state")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "exec", "-n", clusterNamespace,
				clusterName+"-0", "--",
				"valkey-cli", "--no-auth-warning", "-a", pwd, "cluster", "info")
			info, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			// 일부 노드 reset 되면 다른 노드 view 에서 cluster_slots_pfail > 0 또는 state != ok.
			g.Expect(strings.Contains(info, "cluster_state:fail") ||
				strings.Contains(info, "cluster_slots_pfail:") &&
					!strings.Contains(info, "cluster_slots_pfail:0")).To(BeTrue(),
				"cluster fail 상태 가 감지되어야 함 (controller self-heal trigger 의 전제)")
		}, 1*time.Minute, 5*time.Second).Should(Succeed())

		By("ClusterInitialized=true 가 status 에 그대로 보존되어 있는지 확인 (self-heal 진입 조건)")
		out, err := utils.Run(exec.Command("kubectl", "get", "valkeycluster",
			clusterName, "-n", clusterNamespace,
			"-o", "jsonpath={.status.clusterInitialized}"))
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("true"))

		By("waiting for controller post-init self-heal — cluster_state=ok 회복 (~3min)")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "exec", "-n", clusterNamespace,
				clusterName+"-0", "--",
				"valkey-cli", "--no-auth-warning", "-a", pwd, "cluster", "info")
			info, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(info).To(ContainSubstring("cluster_state:ok"))
			g.Expect(info).To(ContainSubstring("cluster_slots_assigned:16384"))
			g.Expect(info).To(ContainSubstring("cluster_slots_ok:16384"))
		}, 5*time.Minute, 10*time.Second).Should(Succeed(),
			"INC-0001 / ADR-0039 self-heal 미동작 — controller 가 cluster fail 자동 회복 못 함")

		By("CR.status 도 healthy 로 갱신되었는지 검증")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "valkeycluster",
				clusterName, "-n", clusterNamespace,
				"-o", "jsonpath={.status.clusterState}/{.status.assignedSlots}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("ok/16384"))
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})
})
