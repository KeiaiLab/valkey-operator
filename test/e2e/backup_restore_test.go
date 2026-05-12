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

ValkeyBackup + ValkeyRestore (Standalone PVC) e2e 시나리오.

Track A 검증: backup → 데이터 변경 → restore → 원래 데이터 복원 확인.
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
	brNamespace = "valkey-backup-restore-e2e"
	brValkey    = "vk-br-test"
	brBackup    = "vk-br-backup-1"
	brRestore   = "vk-br-restore-1"
)

var _ = Describe("ValkeyBackup + ValkeyRestore (Standalone PVC)", Ordered, func() {
	BeforeAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "create", "ns", brNamespace))

		// Standalone Valkey CR.
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
    version: "8.1.6"
  storage:
    size: 1Gi
`, brValkey, brNamespace)
		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Valkey CR apply")
	})

	AfterAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkeybackup",
			brBackup, "-n", brNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkeyrestore",
			brRestore, "-n", brNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkey",
			brValkey, "-n", brNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns",
			brNamespace, "--ignore-not-found"))
	})

	getPwdEnvCmd := func() string {
		// shell 한 줄 — Auth Secret 의 password 추출 후 valkey-cli 호출.
		return fmt.Sprintf(`PWD=$(kubectl get secret %s-auth -n %s `+
			`-o jsonpath='{.data.password}' | base64 -d) && echo $PWD`,
			brValkey, brNamespace)
	}

	Context("부트스트랩 + 데이터 set", func() {
		It("Phase=Running 도달", func() {
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					brValkey, "-n", brNamespace,
					"-o", "jsonpath={.status.phase}"))
				return out
			}, 5*time.Minute, 5*time.Second).Should(Equal("Running"))
		})

		It("초기 데이터 SET (foo=bar1)", func() {
			pwd, err := utils.Run(exec.Command("sh", "-c", getPwdEnvCmd()))
			Expect(err).NotTo(HaveOccurred())
			pwd = strings.TrimSpace(pwd)

			_, err = utils.Run(exec.Command("kubectl", "exec", "-n",
				brNamespace, brValkey+"-0", "--",
				"valkey-cli", "-a", pwd, "set", "foo", "bar1"))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("ValkeyBackup → Completed", func() {
		It("ValkeyBackup CR 생성 + Phase=Completed", func() {
			manifest := fmt.Sprintf(`
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata:
  name: %s
  namespace: %s
spec:
  clusterRef:
    kind: Valkey
    name: %s
  type: RDB
  retainPVC: true
`, brBackup, brNamespace, brValkey)
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(manifest)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkeybackup",
					brBackup, "-n", brNamespace,
					"-o", "jsonpath={.status.phase}"))
				return out
			}, 5*time.Minute, 5*time.Second).Should(Equal("Completed"))
		})

		It("backup PVC 가 생성됨", func() {
			Eventually(func() error {
				_, err := utils.Run(exec.Command("kubectl", "get", "pvc",
					brBackup+"-backup", "-n", brNamespace))
				return err
			}, 1*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	Context("데이터 변경 후 Restore → 원래 복원", func() {
		It("foo=bar2 로 변경 (backup 시점 이후)", func() {
			pwd, err := utils.Run(exec.Command("sh", "-c", getPwdEnvCmd()))
			Expect(err).NotTo(HaveOccurred())
			pwd = strings.TrimSpace(pwd)

			_, err = utils.Run(exec.Command("kubectl", "exec", "-n",
				brNamespace, brValkey+"-0", "--",
				"valkey-cli", "-a", pwd, "set", "foo", "bar2"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("ValkeyRestore CR 생성 + Phase=Completed", func() {
			manifest := fmt.Sprintf(`
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata:
  name: %s
  namespace: %s
spec:
  clusterRef:
    kind: Valkey
    name: %s
  source:
    pvc:
      name: %s
      path: dump.rdb
  restoreType: RDB
`, brRestore, brNamespace, brValkey, brBackup+"-backup")
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(manifest)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkeyrestore",
					brRestore, "-n", brNamespace,
					"-o", "jsonpath={.status.phase}"))
				return out
			}, 10*time.Minute, 10*time.Second).Should(Equal("Completed"))
		})

		It("foo=bar1 로 복원 확인 (backup 시점 데이터)", func() {
			pwd, err := utils.Run(exec.Command("sh", "-c", getPwdEnvCmd()))
			Expect(err).NotTo(HaveOccurred())
			pwd = strings.TrimSpace(pwd)

			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "exec", "-n",
					brNamespace, brValkey+"-0", "--",
					"valkey-cli", "-a", pwd, "get", "foo"))
				return strings.TrimSpace(out)
			}, 2*time.Minute, 5*time.Second).Should(Equal("bar1"))
		})
	})

	// iteration 18 (Phase 2 V2) — narrow scope: restore → version patch chain.
	// ROADMAP P0 차단요인 2 (iteration 7 진단) 의 narrow scope 검증 — *fresh*
	// 시나리오는 iteration 7 에서 정상 동작 확인됨. *restored 인스턴스* 의
	// version patch 시 STS image propagate + Pod rotation + RDB 호환성 회귀 가드.
	Context("Restored 인스턴스의 8.1.6 → 9.0.4 version patch chain (V2)", func() {
		It("spec.version.version 8.1.6 → 9.0.4 patch (restored 후)", func() {
			patch := `{"spec":{"version":{"version":"9.0.4","image":"docker.io/valkey/valkey"}}}`
			_, err := utils.Run(exec.Command("kubectl", "patch", "valkey",
				brValkey, "-n", brNamespace,
				"--type=merge", "-p", patch))
			Expect(err).NotTo(HaveOccurred(), "patch valkey to 9.0.4 (restored 인스턴스)")
		})

		It("STS image propagate to 9.0.4 (가설 A 회귀 가드)", func() {
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "sts",
					brValkey, "-n", brNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].image}"))
				return out
			}, 60*time.Second, 5*time.Second).Should(
				Equal("docker.io/valkey/valkey:9.0.4"),
				"restored 인스턴스의 STS image 가 9.0.4 로 propagate (가설 A)")
		})

		It("Pod 가 9.0.4 image 로 재생성 (가설 C 회귀 가드)", func() {
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "pod",
					brValkey+"-0", "-n", brNamespace,
					"-o", "jsonpath={.spec.containers[0].image}"))
				return out
			}, 3*time.Minute, 10*time.Second).Should(
				Equal("docker.io/valkey/valkey:9.0.4"),
				"restored 인스턴스의 Pod 가 9.0.4 로 재생성 (가설 C)")
		})

		It("CR spec.version.version 9.0.4 보존 (가설 B 회귀 가드)", func() {
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					brValkey, "-n", brNamespace,
					"-o", "jsonpath={.spec.version.version}"))
				return out
			}, 30*time.Second, 5*time.Second).Should(
				Equal("9.0.4"),
				"webhook defaulter 가 9.0.4 → 8.1.6 reverting 안함 (가설 B)")
		})

		It("Phase=Running 복귀 + 복원된 데이터 (foo=bar1) 보존 검증", func() {
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
					brValkey, "-n", brNamespace,
					"-o", "jsonpath={.status.phase}"))
				return out
			}, 5*time.Minute, 10*time.Second).Should(Equal("Running"))

			pwd, err := utils.Run(exec.Command("sh", "-c", getPwdEnvCmd()))
			Expect(err).NotTo(HaveOccurred())
			pwd = strings.TrimSpace(pwd)

			// 9.0.4 의 RDB 형식 호환성 확인 — 8.1.6 시점 데이터 (foo=bar1) 가
			// 9.0.4 startup 시 정상 load 되어야 함 (RDB v80 마이그레이션).
			Eventually(func() string {
				out, _ := utils.Run(exec.Command("kubectl", "exec", "-n",
					brNamespace, brValkey+"-0", "--",
					"valkey-cli", "-a", pwd, "get", "foo"))
				return strings.TrimSpace(out)
			}, 2*time.Minute, 5*time.Second).Should(
				Equal("bar1"),
				"8.1.6 → 9.0.4 upgrade 후 RDB v80 호환 (ROADMAP narrow scope)")
		})
	})
})
