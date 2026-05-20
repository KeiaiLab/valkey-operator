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
package controller

import (
	"context"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// shouldAutoCreatePDB — HA default 정책. ADR-0040 commercial parity.
//
// 진리표:
//
//	spec=nil + replicas>=2 → true  (HA default — auto PDB minAvailable=N-1)
//	spec=nil + replicas<2  → false (Standalone — PDB 의미 없음)
//	spec.Enabled=true      → true  (사용자 명시)
//	spec.Enabled=false     → false (사용자 explicit opt-out)
//
// 자동 생성 시 BuildPDB 가 spec=nil 처리: minAvailable = replicas-1 (HA 보호).
func shouldAutoCreatePDB(spec *cachev1alpha1.PodDisruptionBudgetSpec, replicas int32) bool {
	if spec == nil {
		return replicas >= 2
	}
	return spec.Enabled
}

// EnsurePDBDeleted — CDEX-M1 fix (Codex stage 3 finding, ai-dev `cleanup-supercycle-2026-05-21` plan defer 5).
//
// shouldAutoCreatePDB=false 시 *기존 PDB 가 cluster 에 잔존* 하면 삭제 보장 (spec ↔ cluster state 동기화).
// mongodb-operator `reconcilePDB` (`mongodb_controller.go:313` sister pattern) 정합.
//
// 트리거 사고: spec.Enabled=true 로 PDB 자동 생성 후 사용자가 spec.Enabled=false 로 변경 시
// 기존 PDB 가 orphan 으로 잔존 → drain blocker.
//
// 동작:
//
//	PDB 존재 → Delete (success path)
//	PDB 없음 → nil (idempotent, 첫 reconcile / 이미 cleanup 후 등)
//	기타 error → caller 에 전달 (caller 가 applyErrorCondition wrap)
func EnsurePDBDeleted(ctx context.Context, c client.Client, name, namespace string) error {
	pdb := &policyv1.PodDisruptionBudget{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, pdb)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := c.Delete(ctx, pdb); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
