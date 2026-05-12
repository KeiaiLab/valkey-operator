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

chaos engineering e2e — ADR-0041 chaos-mesh 기반.

전제 조건:
  - Kind cluster (또는 임의 K8s) 활성, kubeconfig 가 가리키는 context 가 *test
    가능* 환경 (production 사용 금지).
  - chaos-mesh CRD + controller 설치 (`make chaos-mesh-install`).
  - cert-manager 설치 (cert-manager auto-discovery 통합 시).
  - valkey-operator 가 kind cluster 에 deploy 된 상태 (`make deploy`).

실행:
  go test -tags=chaos ./test/chaos/... -v -timeout=20m

또는:
  make chaos-e2e
*/

package chaos

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	// chaosMeshAPIVersion — chaos-mesh CRD 의 GroupVersion.
	// 본 패키지는 SDK 의존성 추가하지 않고 unstructured 만 사용 (cert-manager 와
	// 동일 패턴, ADR-0010).
	chaosMeshAPIVersion = "chaos-mesh.org/v1alpha1"

	// chaosTestNamespace — 본 suite 가 valkey CR + chaos CR 을 배포하는 namespace.
	// 환경변수 CHAOS_TEST_NAMESPACE 로 override 가능.
	defaultChaosTestNamespace = "valkey-chaos-e2e"

	// targetCRName — 테스트 대상 ValkeyCluster CR 이름.
	targetCRName = "vc-chaos"
)

func chaosTestNamespace() string {
	if v := os.Getenv("CHAOS_TEST_NAMESPACE"); v != "" {
		return v
	}
	return defaultChaosTestNamespace
}

func TestChaos(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "chaos-mesh based chaos e2e suite — namespace=%s\n",
		chaosTestNamespace())
	RunSpecs(t, "valkey-operator chaos suite")
}
