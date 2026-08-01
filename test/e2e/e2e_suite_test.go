//go:build e2e
// +build e2e

/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/keiailab/valkey-operator/test/utils"
)

var (
	// managerImage is the manager image to be built and loaded for testing.
	// cycle 99: kubebuilder default placeholder ("example.com/...") → 실제 GHCR
	// 표준 image. IMG env 로 override 가능 (e.g., local registry test).
	managerImage = func() string {
		if v := os.Getenv("IMG"); v != "" {
			return v
		}
		return "ghcr.io/keiailab/valkey-operator:e2e-dev"
	}()
	// shouldCleanupCertManager tracks whether CertManager was installed by this suite.
	shouldCleanupCertManager = false
	// shouldCleanupPrometheusCRDs tracks whether monitoring.coreos.com CRDs
	// were installed by this suite.
	shouldCleanupPrometheusCRDs = false
)

// TestE2E runs the e2e test suite to validate the solution in an isolated environment.
// The default setup requires Kind and CertManager.
//
// To enable kubectl kuberc (use custom kubectl configurations), set: KUBECTL_KUBERC=true
// By default, kuberc is disabled to ensure consistent test behavior across different environments.
// To skip CertManager installation, set: CERT_MANAGER_INSTALL_SKIP=true
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting valkey-operator e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("building the manager image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", managerImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager image")

	// TODO(user): If you want to change the e2e test vendor from Kind,
	// ensure the image is built and available, then remove the following block.
	By("loading the manager image on Kind")
	err = utils.LoadImageToKindClusterWithName(managerImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager image into Kind")

	configureKubectlKubeRC()
	setupCertManager()
	setupPrometheusOperatorCRDs()

	// CRD + operator 설치는 **모든 spec 보다 먼저** 일어나야 한다.
	// 구 구조는 e2e_test.go 의 한 Ordered 컨테이너가 BeforeAll 에서 install/deploy 하고
	// AfterAll 에서 undeploy/uninstall 까지 했다. Ginkgo 는 최상위 컨테이너 순서를
	// 무작위화하므로 그 컨테이너보다 **먼저** 뽑힌 컨테이너는 CRD 없이 돌고,
	// **나중에** 뽑힌 컨테이너는 방금 지워진 CRD 위에서 돈다 — 어느 쪽이든 깨진다.
	By("installing CRDs")
	_, err = utils.Run(exec.Command("make", "install"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to install CRDs")

	By("deploying the controller-manager")
	_, err = utils.Run(exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", managerImage)))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

	// deploy 직후 바로 CR 을 apply 하면 mutating webhook 이 아직 안 떠서
	// `failed calling webhook "mvalkey-v1alpha1.kb.io": connection refused` 로
	// 죽는다 (첫 CI 실행에서 failover/version_upgrade BeforeAll 이 이 이유로 실패).
	// rollout 완료 + webhook endpoint 주소 확보까지 기다린다.
	By("waiting for controller-manager rollout")
	_, err = utils.Run(exec.Command("kubectl", "-n", "valkey-operator-system", "rollout", "status",
		"deploy/valkey-operator-controller-manager", "--timeout=180s"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "controller-manager rollout")

	// endpoint 에 주소가 잡혔다는 것만으로는 부족하다 — readiness probe 는 healthz(8081)
	// 를 볼 뿐 webhook TLS 포트(9443)를 보지 않으므로, Pod 가 Ready 이고 endpoint 에
	// 주소가 있어도 webhook 서버가 아직 안 듣고 있을 수 있다. 실측: endpoint 대기 통과
	// **0.5초 뒤** 첫 CR apply 가 `connection refused` 로 죽었다.
	//
	// 그래서 **실제로 webhook 을 호출해** 응답하는지 본다. server-side dry-run 은
	// admission 체인을 그대로 태우면서 아무것도 저장하지 않으므로 정확한 신호다.
	// 검증 거부(4xx)는 "webhook 이 응답했다"는 뜻이라 성공으로 친다 — 연결 실패만 재시도.
	By("waiting for the webhook to actually answer (server-side dry-run)")
	probe := `apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: webhook-readiness-probe
  namespace: default
spec:
  mode: Standalone
  replicas: 1
`
	EventuallyWithOffset(1, func() string {
		cmd := exec.Command("kubectl", "apply", "--dry-run=server", "-f", "-")
		cmd.Stdin = strings.NewReader(probe)
		out, err := utils.Run(cmd)
		if err == nil {
			return ""
		}
		msg := out + err.Error()
		// webhook 에 닿지 못한 경우만 재시도 대상.
		if strings.Contains(msg, "failed calling webhook") ||
			strings.Contains(msg, "connection refused") ||
			strings.Contains(msg, "context deadline exceeded") {
			return msg
		}
		// 그 외(검증 거부 등)는 webhook 이 살아서 응답한 것 — 준비 완료로 본다.
		return ""
	}, 3*time.Minute, 3*time.Second).Should(BeEmpty(), "webhook never became reachable")
})

var _ = AfterSuite(func() {
	teardownPrometheusOperatorCRDs()
	teardownCertManager()
})

// Disable kubectl kuberc by default for test isolation.
// This prevents local kubectl configurations from affecting test behavior.
// To enable kuberc, set: KUBECTL_KUBERC=true
func configureKubectlKubeRC() {
	if os.Getenv("KUBECTL_KUBERC") != "true" {
		By("disabling kubectl kuberc for test isolation")
		err := os.Setenv("KUBECTL_KUBERC", "false")
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to disable kubectl kuberc")
		_, _ = fmt.Fprintf(GinkgoWriter,
			"kubectl kuberc disabled for consistent test behavior (override with KUBECTL_KUBERC=true)\n")
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "kubectl kuberc enabled (KUBECTL_KUBERC=true)\n")
	}
}

// setupCertManager installs CertManager if needed for webhook tests.
// Skips installation if CERT_MANAGER_INSTALL_SKIP=true or if already present.
func setupCertManager() {
	if os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager installation (CERT_MANAGER_INSTALL_SKIP=true)\n")
		return
	}

	By("checking if CertManager is already installed")
	if utils.IsCertManagerCRDsInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "CertManager is already installed. Skipping installation.\n")
		return
	}

	// Mark for cleanup before installation to handle interruptions and partial installs.
	shouldCleanupCertManager = true

	By("installing CertManager")
	Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
}

func setupPrometheusOperatorCRDs() {
	By("checking if Prometheus Operator CRDs are already installed")
	if utils.IsPrometheusOperatorCRDsInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "Prometheus Operator CRDs are already installed. Skipping installation.\n")
		return
	}

	shouldCleanupPrometheusCRDs = true

	By("installing Prometheus Operator CRDs")
	Expect(utils.InstallPrometheusOperatorCRDs()).To(Succeed(), "Failed to install Prometheus Operator CRDs")
}

// teardownCertManager uninstalls CertManager if it was installed by setupCertManager.
// This ensures we only remove what we installed.
func teardownCertManager() {
	if !shouldCleanupCertManager {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager cleanup (not installed by this suite)\n")
		return
	}

	By("uninstalling CertManager")
	utils.UninstallCertManager()
}

func teardownPrometheusOperatorCRDs() {
	if !shouldCleanupPrometheusCRDs {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping Prometheus Operator CRD cleanup (not installed by this suite)\n")
		return
	}

	By("uninstalling Prometheus Operator CRDs")
	utils.UninstallPrometheusOperatorCRDs()
}

// dumpNamespaceOnFailure — spec 이 실패했을 때 해당 namespace 의 상태를 남긴다.
//
// 각 spec 의 AfterAll 이 namespace 를 지우므로, 워크플로 말미의 `kubectl get pods -A`
// 덤프에는 **테스트 파드가 하나도 남지 않는다** — 실측으로 확인했다(실패 덤프에
// kube-system 과 operator 만 있었다). 그래서 진단은 실패 *직후* 여기서 해야 한다.
//
// 남은 타임아웃 실패(backup 이 Completed 에 못 감 / failover 가 primary 를 못 바꿈)가
// 러너 자원 부족인지 제품 결함인지 가르려면 이 정보가 필요하다.
func dumpNamespaceOnFailure(ns string) {
	if !CurrentSpecReport().Failed() {
		return
	}
	for _, probe := range []struct {
		label string
		args  []string
	}{
		{"pods", []string{"get", "pods", "-n", ns, "-o", "wide"}},
		{"pvc", []string{"get", "pvc", "-n", ns}},
		{"jobs", []string{"get", "jobs", "-n", ns}},
		{"valkey CRs", []string{"get", "valkey,valkeycluster,valkeybackup,valkeyrestore", "-n", ns, "-o", "wide"}},
		{"events", []string{"get", "events", "-n", ns, "--sort-by=.lastTimestamp"}},
	} {
		out, err := utils.Run(exec.Command("kubectl", probe.args...))
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "[dump] %s (%s): %v\n", probe.label, ns, err)
			continue
		}
		_, _ = fmt.Fprintf(GinkgoWriter, "[dump] %s (%s):\n%s\n", probe.label, ns, out)
	}

	// operator 로그 tail — reconcile 이 왜 멈췄는지의 1차 단서.
	out, err := utils.Run(exec.Command("kubectl", "-n", "valkey-operator-system", "logs",
		"deploy/valkey-operator-controller-manager", "--tail=120"))
	if err == nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "[dump] operator logs (tail 120):\n%s\n", out)
	}
}
