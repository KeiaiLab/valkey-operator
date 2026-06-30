/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"fmt"

	policyv1 "k8s.io/api/policy/v1"

	commonspdb "github.com/keiailab/keiailab-commons/pkg/pdb"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// pdbParamsFromSpec — CR 의 PodDisruptionBudgetSpec 을 commons pdb.Params 로 변환한다.
// valkey 의 기존 우선순위(MaxUnavailable 먼저 검사)를 보존하기 위해 *둘 중 하나만*
// commons 로 전달한다 (둘 다 nil 이면 commons 가 DefaultFloor 정책 적용).
func pdbParamsFromSpec(name, namespace string, labels, selector map[string]string, replicas int32, spec *cachev1alpha1.PodDisruptionBudgetSpec) commonspdb.Params {
	p := commonspdb.Params{
		Name:         name,
		Namespace:    namespace,
		Labels:       labels,
		Selector:     selector,
		Replicas:     replicas,
		DefaultFloor: 1, // valkey: primary 항상 보존 (minAvailable >= 1)
	}
	switch {
	case spec != nil && spec.MaxUnavailable != nil:
		p.MaxUnavailable = spec.MaxUnavailable
	case spec != nil && spec.MinAvailable != nil:
		p.MinAvailable = spec.MinAvailable
	}
	return p
}

// BuildPDB — opt-in PodDisruptionBudget. minAvailable / maxUnavailable 둘 중 하나만 사용.
// 둘 다 nil 이면 minAvailable = replicas-1 (3 노드 RS → minAvailable=2) 기본 적용.
//
// 빌드 골격은 keiailab-commons/pkg/pdb.Build 에 위임 (default-floor / min·max 정책
// SSOT). name / labels / selector / floor 는 valkey 도메인 책임으로 잔류.
func BuildPDB(crName, namespace string, replicas int32, spec *cachev1alpha1.PodDisruptionBudgetSpec) *policyv1.PodDisruptionBudget {
	return commonspdb.Build(pdbParamsFromSpec(
		PDBName(crName), namespace, CommonLabels(crName, "valkey"), SelectorLabels(crName), replicas, spec,
	))
}

// ShardPDBName — ValkeyCluster shard-aware PDB name (CDEX-M2).
// `<cr>-shard-<idx>` 패턴.
func ShardPDBName(crName string, shardIdx int) string {
	return fmt.Sprintf("%s-shard-%d", crName, shardIdx)
}

// ShardSelectorLabels — ValkeyCluster shard-aware PDB selector (CDEX-M2).
// SelectorLabels(crName) base + valkey.keiailab.io/shard=<idx>.
func ShardSelectorLabels(crName string, shardIdx int) map[string]string {
	base := SelectorLabels(crName)
	base[LabelValkeyShard] = fmt.Sprintf("%d", shardIdx)
	return base
}

// BuildShardPDB — CDEX-M2 (2026-05-21): ValkeyCluster shard 별 PDB 생성. opt-in
// (`spec.PodDisruptionBudget.PerShard=true`). drain 시 모든 shard primary 동시
// evict 차단.
//
// 동작:
//   - shardReplicas = 1 (primary) + replicasPerShard
//   - minAvailable default = shardReplicas-1 (primary 항상 사용 가능)
//   - selector = `app.kubernetes.io/name=valkey + instance=<cr> + valkey.keiailab.io/shard=<idx>`
//   - spec.MinAvailable / MaxUnavailable 명시 override 가능
//
// 빌드 골격은 keiailab-commons/pkg/pdb.Build 에 위임. caller (reconciler) 가 shard
// 별 loop 의무. orphan PDB cleanup = caller 책임.
func BuildShardPDB(crName, namespace string, shardIdx int, shardReplicas int32, spec *cachev1alpha1.PodDisruptionBudgetSpec) *policyv1.PodDisruptionBudget {
	return commonspdb.Build(pdbParamsFromSpec(
		ShardPDBName(crName, shardIdx), namespace, CommonLabels(crName, "valkey"), ShardSelectorLabels(crName, shardIdx), shardReplicas, spec,
	))
}
