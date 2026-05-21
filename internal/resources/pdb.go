/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"fmt"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// BuildPDB — opt-in PodDisruptionBudget. minAvailable / maxUnavailable 둘 중 하나만 사용.
// 둘 다 nil 이면 minAvailable = replicas-1 (3 노드 RS → minAvailable=2) 기본 적용.
func BuildPDB(crName, namespace string, replicas int32, spec *cachev1alpha1.PodDisruptionBudgetSpec) *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PDBName(crName),
			Namespace: namespace,
			Labels:    CommonLabels(crName, "valkey"),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: SelectorLabels(crName)},
		},
	}
	switch {
	case spec != nil && spec.MaxUnavailable != nil:
		pdb.Spec.MaxUnavailable = spec.MaxUnavailable
	case spec != nil && spec.MinAvailable != nil:
		pdb.Spec.MinAvailable = spec.MinAvailable
	default:
		min := max(int(replicas-1), 1)
		v := intstr.FromInt(min)
		pdb.Spec.MinAvailable = &v
	}
	return pdb
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
// (`spec.PodDisruptionBudget.PerShard=true`). mongodb sharded `builder.go:2105`
// per-shard PDB pattern 정합 — drain 시 모든 shard primary 동시 evict 차단.
//
// 동작:
//   - shardReplicas = 1 (primary) + replicasPerShard
//   - minAvailable default = shardReplicas-1 (primary 항상 사용 가능)
//   - selector = `app.kubernetes.io/name=valkey + instance=<cr> + valkey.keiailab.io/shard=<idx>`
//   - spec.MinAvailable / MaxUnavailable 명시 override 가능
//
// caller (reconciler) 가 shard 별 loop 의무. orphan PDB cleanup = caller 책임.
func BuildShardPDB(crName, namespace string, shardIdx int, shardReplicas int32, spec *cachev1alpha1.PodDisruptionBudgetSpec) *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ShardPDBName(crName, shardIdx),
			Namespace: namespace,
			Labels:    CommonLabels(crName, "valkey"),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: ShardSelectorLabels(crName, shardIdx)},
		},
	}
	switch {
	case spec != nil && spec.MaxUnavailable != nil:
		pdb.Spec.MaxUnavailable = spec.MaxUnavailable
	case spec != nil && spec.MinAvailable != nil:
		pdb.Spec.MinAvailable = spec.MinAvailable
	default:
		min := max(int(shardReplicas-1), 1)
		v := intstr.FromInt(min)
		pdb.Spec.MinAvailable = &v
	}
	return pdb
}
