//go:build e2e
// +build e2e

/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
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

var _ = Describe("Valkey Redis Stack-compatible modules", Ordered, func() {
	const (
		modulesNamespace = "test-valkey-modules-20260610"
		modulesName      = "vk-modules"
	)

	BeforeAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", modulesNamespace, "--ignore-not-found"))
		_, err := utils.Run(exec.Command("kubectl", "create", "ns", modulesNamespace))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkey", modulesName,
			"-n", modulesNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", modulesNamespace, "--ignore-not-found"))
	})

	It("loads Valkey official modules and serves JSON, Search, and Bloom commands", func() {
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
    version: "9.0.4"
  storage:
    ephemeral: true
  modules:
    - name: valkey-json
    - name: valkey-search
    - name: valkey-bloom
`, modulesName, modulesNamespace)

		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() string {
			out, _ := utils.Run(exec.Command("kubectl", "get", "valkey", modulesName,
				"-n", modulesNamespace, "-o", "jsonpath={.status.phase}"))
			return strings.TrimSpace(out)
		}, 5*time.Minute, 2*time.Second).Should(Equal("Running"))

		pwdCmd := fmt.Sprintf(`kubectl get secret %s-auth -n %s -o jsonpath='{.data.password}' | base64 -d`,
			modulesName, modulesNamespace)
		pwd, err := utils.Run(exec.Command("sh", "-c", pwdCmd))
		Expect(err).NotTo(HaveOccurred())
		pwd = strings.TrimSpace(pwd)
		Expect(pwd).NotTo(BeEmpty())

		runCLI := func(args ...string) (string, error) {
			base := []string{"exec", "-n", modulesNamespace, modulesName + "-0", "--",
				"valkey-cli", "--no-auth-warning", "-a", pwd}
			base = append(base, args...)
			return utils.Run(exec.Command("kubectl", base...))
		}

		Eventually(func(g Gomega) {
			out, err := runCLI("MODULE", "LIST")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(ContainSubstring("json"))
			g.Expect(out).To(ContainSubstring("search"))
			g.Expect(out).To(ContainSubstring("bloom"))
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		_, err = runCLI("JSON.SET", "doc:1", "$", `{"title":"hello modules"}`)
		Expect(err).NotTo(HaveOccurred())
		jsonOut, err := runCLI("JSON.GET", "doc:1", "$.title")
		Expect(err).NotTo(HaveOccurred())
		Expect(jsonOut).To(ContainSubstring("hello modules"))

		_, err = runCLI("FT.CREATE", "idx:docs", "ON", "HASH", "PREFIX", "1", "doc:", "SCHEMA", "title", "TEXT")
		Expect(err).NotTo(HaveOccurred())
		_, err = runCLI("HSET", "doc:2", "title", "searchable modules")
		Expect(err).NotTo(HaveOccurred())
		searchOut, err := runCLI("FT.SEARCH", "idx:docs", "searchable")
		Expect(err).NotTo(HaveOccurred())
		Expect(searchOut).To(ContainSubstring("doc:2"))

		bloomOut, err := runCLI("BF.ADD", "bf:docs", "module-item")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(bloomOut)).To(Or(Equal("1"), ContainSubstring("1")))
	})
})
