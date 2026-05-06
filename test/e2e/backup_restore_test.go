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
})
