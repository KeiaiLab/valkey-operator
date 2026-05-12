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

//go:build e2e
// +build e2e

/*
Copyright 2026 Keiailab.

Replication mode 자동 failover e2e 시나리오 (ADR-0017).

전제:
- e2e_test.go 의 BeforeAll 가 operator 배포 + cert-manager 활성화.
- kind cluster 가 setup-test-e2e 로 생성됨.

본 파일은 별도 Describe 로 e2e_test.go 와 독립 실행 가능.
*/

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/keiailab/valkey-operator/test/utils"
)

const (
	failoverNamespace = "valkey-failover-e2e"
	failoverCRName    = "vk-failover-test"
)

var _ = Describe("Replication Failover (ADR-0017)", Ordered, func() {
	BeforeAll(func() {
		// 전용 namespace.
		_, _ = utils.Run(exec.Command("kubectl", "create", "ns", failoverNamespace))

		// Replication mode 3 replicas Valkey CR 생성.
		manifest := fmt.Sprintf(`
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: %s
  namespace: %s
spec:
  mode: Replication
  replicas: 3
  version:
    image: docker.io/valkey/valkey
    version: "8.1.6"
  storage:
    size: 1Gi
  autoFailover: true
`, failoverCRName, failoverNamespace)

		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Valkey CR apply")
	})

	AfterAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkey",
			failoverCRName, "-n", failoverNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns",
			failoverNamespace, "--ignore-not-found"))
	})

	Context("초기 부트스트랩", func() {
		It("Phase=Running 도달 + Status.CurrentPrimary=pod-0", func() {
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					failoverCRName, "-n", failoverNamespace,
					"-o", "jsonpath={.status.phase}"))
				return out
			}, 5*time.Minute, 5*time.Second).Should(Equal("Running"))

			out, err := utils.Run(exec.Command("kubectl", "get", "valkey",
				failoverCRName, "-n", failoverNamespace,
				"-o", "jsonpath={.status.currentPrimary}"))
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(Equal(failoverCRName + "-0"))
		})

		It("3 pod 모두 Ready", func() {
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					failoverCRName, "-n", failoverNamespace,
					"-o", "jsonpath={.status.readyReplicas}"))
				return out
			}, 3*time.Minute, 5*time.Second).Should(Equal("3"))
		})
	})

	Context("Primary kill → 자동 failover", func() {
		It("primary pod (pod-0) 강제 삭제", func() {
			cmd := exec.Command("kubectl", "delete", "pod",
				failoverCRName+"-0", "-n", failoverNamespace,
				"--force", "--grace-period=0")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("30s+ 후 Status.CurrentPrimary 가 pod-1 또는 pod-2 로 변경", func() {
			// Failover threshold (30s) + reconcile interval 추가 마진.
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					failoverCRName, "-n", failoverNamespace,
					"-o", "jsonpath={.status.currentPrimary}"))
				return out
			}, 3*time.Minute, 10*time.Second).ShouldNot(Equal(failoverCRName + "-0"))
		})

		It("새 primary pod Ready", func() {
			out, err := utils.Run(exec.Command("kubectl", "get", "valkey",
				failoverCRName, "-n", failoverNamespace,
				"-o", "jsonpath={.status.currentPrimary}"))
			Expect(err).NotTo(HaveOccurred())
			newPrimary := strings.TrimSpace(out)

			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "pod",
					newPrimary, "-n", failoverNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}"))
				return out
			}, 2*time.Minute, 5*time.Second).Should(Equal("True"))
		})

		It("새 primary 의 INFO replication 의 role=master", func() {
			out, err := utils.Run(exec.Command("kubectl", "get", "valkey",
				failoverCRName, "-n", failoverNamespace,
				"-o", "jsonpath={.status.currentPrimary}"))
			Expect(err).NotTo(HaveOccurred())
			newPrimary := strings.TrimSpace(out)

			pwd, err := utils.Run(exec.Command("kubectl", "get", "secret",
				failoverCRName+"-auth", "-n", failoverNamespace,
				"-o", "jsonpath={.data.password}"))
			Expect(err).NotTo(HaveOccurred())
			pwd = strings.TrimSpace(pwd)

			// kubectl exec valkey-cli -a $pwd info replication | grep role:
			info, err := utils.Run(exec.Command("kubectl", "exec", "-n",
				failoverNamespace, newPrimary, "--",
				"sh", "-c",
				fmt.Sprintf("PWD=$(echo %s | base64 -d) && valkey-cli -a $PWD info replication", pwd)))
			Expect(err).NotTo(HaveOccurred())
			Expect(info).To(ContainSubstring("role:master"))
		})
	})
})
