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

// Valkey 공식 module preset (PR-C6.2, ADR-0032 / ADR-0062) e2e.
//
// 시나리오: Valkey CR 에 spec.modules:[{name: valkey-search}] 지정 → operator 가
// init container 로 valkey-bundle 에서 libsearch.so 를 공유 emptyDir 로 cp →
// 메인 valkey 가 --loadmodule 로 적재 → MODULE LIST 에 search 등장 + FT.CREATE /
// FT.SEARCH 라운드트립 동작.
//
// operator 배포는 e2e_test.go 의 "Manager" Describe BeforeAll 에 의존 (suite 공유).
var _ = Describe("Valkey Module Preset (valkey-search)", Ordered, func() {
	const (
		modNamespace = "valkey-module-e2e"
		modValkey    = "vk-mod"
	)

	BeforeAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "create", "ns", modNamespace))

		// valkey-search 는 8.1+ GA. BundleTagOrDefault("8.1.6") → "8.1" →
		// valkey-bundle:8.1 (libsearch.so 보유).
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
  modules:
    - name: valkey-search
`, modValkey, modNamespace)
		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Valkey CR (modules) apply")
	})

	AfterAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkey",
			modValkey, "-n", modNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns",
			modNamespace, "--ignore-not-found"))
	})

	// valkey-cli 호출 (auth 활성 — <name>-auth secret 의 password 사용).
	cli := func(args ...string) (string, error) {
		pwdOut, _ := utils.Run(exec.Command("sh", "-c", fmt.Sprintf(
			`kubectl get secret %s-auth -n %s -o jsonpath='{.data.password}' | base64 -d`,
			modValkey, modNamespace)))
		pwd := strings.TrimSpace(pwdOut)
		full := append([]string{
			"exec", "-n", modNamespace, modValkey + "-0", "--",
			"valkey-cli", "-a", pwd,
		}, args...)
		return utils.Run(exec.Command("kubectl", full...))
	}

	It("STS 에 module init container 가 주입됨", func() {
		Eventually(func() string {
			out, _ := utils.Run(exec.Command("kubectl", "get", "statefulset",
				modValkey, "-n", modNamespace,
				"-o", "jsonpath={.spec.template.spec.initContainers[*].name}"))
			return out
		}, 2*time.Minute, 5*time.Second).Should(ContainSubstring("module-valkey-search"))
	})

	It("Phase=Running 도달 (init container .so cp + 메인 --loadmodule)", func() {
		Eventually(func() string {
			out, _ := utils.Run(exec.Command("kubectl", "get", "valkey",
				modValkey, "-n", modNamespace, "-o", "jsonpath={.status.phase}"))
			return out
		}, 5*time.Minute, 5*time.Second).Should(Equal("Running"))
	})

	It("MODULE LIST 에 search 모듈 적재 확인", func() {
		Eventually(func() string {
			out, _ := cli("MODULE", "LIST")
			return out
		}, 2*time.Minute, 5*time.Second).Should(ContainSubstring("search"))
	})

	It("FT.CREATE vector 인덱스 + FT._LIST 동작", func() {
		// valkey-search 는 vector 검색 엔진 — 모든 인덱스에 VECTOR 속성 필수
		// (RediSearch 의 TEXT/TAG-only 와 다름, 라이브 실측 2026-06-16).
		Eventually(func() error {
			_, err := cli("FT.CREATE", "idx", "ON", "HASH", "PREFIX", "1", "doc:",
				"SCHEMA", "vec", "VECTOR", "FLAT", "6",
				"TYPE", "FLOAT32", "DIM", "2", "DISTANCE_METRIC", "L2")
			return err
		}, time.Minute, 5*time.Second).Should(Succeed())

		// 인덱스가 생성·조회됨 — 모듈 기능 동작 증명.
		Eventually(func() string {
			out, _ := cli("FT._LIST")
			return out
		}, time.Minute, 3*time.Second).Should(ContainSubstring("idx"))
	})
})
