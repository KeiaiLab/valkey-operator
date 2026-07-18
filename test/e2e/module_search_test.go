//go:build e2e
// +build e2e

/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package e2e

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/keiailab/valkey-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// 모듈 프리셋 로딩 characterization e2e — ROADMAP "Valkey official module presets"
// 의 미검증 항목(`valkey-search` round-trip)을 실증한다.
//
// 검증 대상 (internal/resources/module_init.go BuildModuleInitContainers):
//  1. spec.modules:[{name: valkey-search}] → init-container 가
//     docker.io/valkey/valkey-bundle:9.0 의 /usr/lib/valkey/libsearch.so 를
//     /modules/valkey-search.so 로 추출.
//  2. valkey 컨테이너가 --loadmodule /modules/valkey-search.so 로 로드.
//  3. MODULE LIST 에 search 노출 + 벡터 인덱스(FT.CREATE VECTOR) + KNN(FT.SEARCH) 왕복.
//
// NOTE: valkey-search 는 RediSearch 대체가 아니라 *벡터 검색* 엔진이다 — 인덱스에
// VECTOR 필드가 필수이며 SCHEMA TEXT 는 미지원("At least one attribute must be
// indexed as a vector", 실측). 따라서 본 테스트는 HNSW/FLOAT32 벡터 인덱스 +
// KNN 쿼리로 왕복을 검증한다. presets.go 의 "e2e 검증 의무" NOTE 를 충족.
var _ = Describe("Valkey module preset valkey-search", Ordered, func() {
	const (
		ns   = "test-valkey-module-search"
		name = "test-vk-search"
	)

	var pwd string

	// vec4 — float32 4차원 벡터를 valkey VECTOR 필드용 little-endian raw bytes 로.
	vec4 := func(a, b, c, d float32) string {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(a))
		binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(b))
		binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(c))
		binary.LittleEndian.PutUint32(buf[12:], math.Float32bits(d))
		return string(buf)
	}

	// cli — valkey-cli 를 valkey 컨테이너에서 실행 (init-container 있어 -c valkey 명시).
	cli := func(args ...string) (string, error) {
		full := append([]string{"exec", "-n", ns, name + "-0", "-c", "valkey", "--",
			"valkey-cli", "--no-auth-warning", "-a", pwd}, args...)
		return utils.Run(exec.Command("kubectl", full...))
	}

	// cliStdin — valkey-cli -x (마지막 인자를 stdin 으로). VECTOR raw bytes 전달용
	// (shell/arg escaping 회피 — bash 검증과 동일 경로).
	cliStdin := func(stdin string, args ...string) (string, error) {
		full := append([]string{"exec", "-i", "-n", ns, name + "-0", "-c", "valkey", "--",
			"valkey-cli", "--no-auth-warning", "-a", pwd, "-x"}, args...)
		cmd := exec.Command("kubectl", full...)
		cmd.Stdin = strings.NewReader(stdin)
		return utils.Run(cmd)
	}

	BeforeAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", ns, "--ignore-not-found"))
		_, err := utils.Run(exec.Command("kubectl", "create", "ns", ns))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "valkey", name, "-n", ns, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", ns, "--ignore-not-found"))
	})

	It("loads the search module and answers a vector KNN round-trip", func() {
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
    version: "9.0.4"
  storage:
    size: 1Gi
  auth:
    enabled: true
  modules:
    - name: valkey-search
`, name, ns)

		By("creating a Standalone Valkey with the valkey-search preset (webhook 준비까지 재시도)")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(manifest)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		By("waiting for the Valkey pod Ready (status.phase=Running)")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "valkey",
				name, "-n", ns, "-o", "jsonpath={.status.phase}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("Running"))
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("extracting the auth password")
		// sh 파이프(| base64 -d) 대신 Go 표준 디코드 — shell 미경유(injection 표면 0).
		pwdB64, err := utils.Run(exec.Command("kubectl", "get", "secret", name+"-auth",
			"-n", ns, "-o", "jsonpath={.data.password}"))
		Expect(err).NotTo(HaveOccurred())
		pwdBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pwdB64))
		Expect(err).NotTo(HaveOccurred())
		pwd = string(pwdBytes)
		Expect(pwd).NotTo(BeEmpty())

		By("MODULE LIST includes the search module")
		Eventually(func(g Gomega) {
			out, err := cli("MODULE", "LIST")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(strings.ToLower(out)).To(ContainSubstring("search"))
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		By("FT.CREATE a HNSW FLOAT32 vector index")
		Eventually(func(g Gomega) {
			out, err := cli("FT.CREATE", "idxvec", "ON", "HASH", "PREFIX", "1", "v:",
				"SCHEMA", "emb", "VECTOR", "HNSW", "6", "TYPE", "FLOAT32", "DIM", "4",
				"DISTANCE_METRIC", "L2")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(ContainSubstring("OK"))
		}, 1*time.Minute, 5*time.Second).Should(Succeed())

		By("HSET a vector document (emb = [1,2,3,4])")
		_, err = cliStdin(vec4(1, 2, 3, 4), "HSET", "v:1", "emb")
		Expect(err).NotTo(HaveOccurred())

		By("FT.SEARCH KNN returns the indexed document v:1")
		Eventually(func(g Gomega) {
			// PARAMS 를 맨 끝에 둬 -x stdin(벡터)이 파라미터 q 값 자리에 위치.
			out, err := cliStdin(vec4(1, 2, 3, 4),
				"FT.SEARCH", "idxvec", "*=>[KNN 1 @emb $q]", "DIALECT", "2", "PARAMS", "2", "q")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(ContainSubstring("v:1"))
		}, 1*time.Minute, 5*time.Second).Should(Succeed())
	})
})
