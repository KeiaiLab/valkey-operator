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

// chart features.* ↔ operator ENABLE_*_RECONCILER env 동기 검증.
//
// 사고 패턴 (cycle 80 발견):
// - chart 의 features.{cluster,backup}.enabled 가 RBAC clusterrole 만 gating.
// - operator 코드 가 *항상 reconciler 등록* 시 features=false → cache.WaitForCacheSync
//   실패 → CrashLoopBackOff.
// - 본 cycle 80 가 ENABLE_*_RECONCILER env 추가로 정합 — 본 게이트 가 향후 drift
//   차단 (chart features 추가 시 operator env / chart deployment env 함께 갱신).

package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChartFeaturesReconcilerEnvSync(t *testing.T) {
	repo := findRepoRoot(t)

	// 1. cmd/main.go 의 ENABLE_*_RECONCILER env 사용 추출.
	mainRaw, _ := os.ReadFile(filepath.Join(repo, "cmd/main.go"))
	mainContent := string(mainRaw)
	expectedEnvs := map[string]bool{
		"ENABLE_CLUSTER_RECONCILER": false, // false = 발견 안 됨, true 로 set.
		"ENABLE_BACKUP_RECONCILER":  false,
	}
	for env := range expectedEnvs {
		if strings.Contains(mainContent, `os.Getenv("`+env+`")`) {
			expectedEnvs[env] = true
		}
	}
	for env, found := range expectedEnvs {
		if !found {
			t.Errorf("cmd/main.go 가 %q env 를 읽지 않음 — features gating 회귀", env)
		}
	}

	// 2. chart deployment.yaml 이 동일 env 를 features.* 조건부 주입하는지 검증.
	deployRaw, _ := os.ReadFile(filepath.Join(repo, "charts/valkey-operator/templates/deployment.yaml"))
	deployContent := string(deployRaw)
	mappings := map[string]string{
		"ENABLE_CLUSTER_RECONCILER": ".Values.features.cluster.enabled",
		"ENABLE_BACKUP_RECONCILER":  ".Values.features.backup.enabled",
	}
	for env, valueRef := range mappings {
		// chart 가 env block 에 본 env 주입 + features 조건부 분기 사용해야.
		hasEnvName := strings.Contains(deployContent, "name: "+env)
		hasFeatureRef := strings.Contains(deployContent, valueRef)
		if !hasEnvName {
			t.Errorf("chart deployment.yaml 의 env block 에 %q 미주입 — operator 가 본 env 읽지만 chart 가 미설정 → drift",
				env)
		}
		if !hasFeatureRef {
			t.Errorf("chart deployment.yaml 가 %q 참조 안 함 — features gating 미배선",
				valueRef)
		}
	}
}
