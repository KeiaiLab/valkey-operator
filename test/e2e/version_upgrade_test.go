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

Version upgrade reconcile 회귀 가드 (ROADMAP P0 차단요인 2 영구 영구화).

배경:
- Phase B PoC (2026-05-07) 발견 — `spec.version.version` patch 가 STS template
  image 로 propagate 되지 않는 의심 (가설 A: server-side merge 의 immutable
  field 거부, 가설 B: webhook idempotency 누설, 가설 C: STS rolling update
  partition 보존).
- iteration 7 진단 (2026-05-07): fresh 인스턴스의 8.1.6 → 9.0.4 patch 시나리오
  에서는 *재현 안됨*. STS image propagate + Pod rotation 모두 정상.
- envtest 의 fake client 는 server-side merge 거부 행동을 모사하지 못함 — 본
  e2e (real Kind API server) 가 정확한 회귀 가드.

본 파일은 별도 Describe 로 e2e_test.go / failover_test.go 와 독립 실행 가능.
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
	versionUpgradeNamespace = "valkey-version-upgrade-e2e"
	versionUpgradeCRName    = "vk-upgrade-test"
	versionUpgradeFromTag   = "8.1.6"
	versionUpgradeToTag     = "9.0.4"
)

var _ = Describe("Version Upgrade Reconcile (ROADMAP P0 차단요인 2)", Ordered, func() {
	BeforeAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "create", "ns", versionUpgradeNamespace))

		manifest := fmt.Sprintf(`
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: %s
  namespace: %s
spec:
  mode: Standalone
  replicas: 1
  version:
    image: docker.io/valkey/valkey
    version: %q
  storage:
    size: 1Gi
`, versionUpgradeCRName, versionUpgradeNamespace, versionUpgradeFromTag)

		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Valkey CR apply (8.1.6)")
	})

	AfterAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkey",
			versionUpgradeCRName, "-n", versionUpgradeNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns",
			versionUpgradeNamespace, "--ignore-not-found"))
	})

	Context("초기 부트스트랩 8.1.6", func() {
		It("Phase=Running + STS image=8.1.6", func() {
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					versionUpgradeCRName, "-n", versionUpgradeNamespace,
					"-o", "jsonpath={.status.phase}"))
				return out
			}, 5*time.Minute, 5*time.Second).Should(Equal("Running"))

			stsImage, err := utils.Run(exec.Command("kubectl", "get", "sts",
				versionUpgradeCRName, "-n", versionUpgradeNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].image}"))
			Expect(err).NotTo(HaveOccurred())
			Expect(stsImage).To(Equal("docker.io/valkey/valkey:" + versionUpgradeFromTag))
		})
	})

	Context("spec.version.version patch 8.1.6 → 9.0.4", func() {
		It("STS image 가 9.0.4 로 propagate + Pod 재생성", func() {
			patch := fmt.Sprintf(
				`{"spec":{"version":{"version":%q,"image":"docker.io/valkey/valkey"}}}`,
				versionUpgradeToTag,
			)
			_, err := utils.Run(exec.Command("kubectl", "patch", "valkey",
				versionUpgradeCRName, "-n", versionUpgradeNamespace,
				"--type=merge", "-p", patch))
			Expect(err).NotTo(HaveOccurred(), "patch valkey")

			// 가설 A 회귀 가드 — STS image field 가 새 tag 로 갱신.
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "sts",
					versionUpgradeCRName, "-n", versionUpgradeNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].image}"))
				return out
			}, 60*time.Second, 5*time.Second).Should(
				Equal("docker.io/valkey/valkey:"+versionUpgradeToTag),
				"STS image 가 9.0.4 로 propagate 안됨 — 가설 A 재현",
			)

			// 가설 C 회귀 가드 — Pod 가 실제로 새 image 로 재생성.
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "pod",
					versionUpgradeCRName+"-0", "-n", versionUpgradeNamespace,
					"-o", "jsonpath={.spec.containers[0].image}"))
				return out
			}, 2*time.Minute, 5*time.Second).Should(
				Equal("docker.io/valkey/valkey:"+versionUpgradeToTag),
				"Pod image 가 새 tag 로 재생성 안됨 — 가설 C 재현",
			)

			// 가설 B 회귀 가드 — webhook 가 spec 을 8.1.6 으로 되돌리지 않음.
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					versionUpgradeCRName, "-n", versionUpgradeNamespace,
					"-o", "jsonpath={.spec.version.version}"))
				return out
			}, 30*time.Second, 5*time.Second).Should(
				Equal(versionUpgradeToTag),
				"CR spec.version.version 이 8.1.6 으로 되돌아감 — 가설 B 재현",
			)

			// Phase=Running 복귀.
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					versionUpgradeCRName, "-n", versionUpgradeNamespace,
					"-o", "jsonpath={.status.phase}"))
				return out
			}, 5*time.Minute, 5*time.Second).Should(Equal("Running"))
		})
	})
})
