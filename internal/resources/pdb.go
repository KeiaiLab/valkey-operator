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
package resources

import (
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
