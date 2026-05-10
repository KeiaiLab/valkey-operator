/*
Copyright 2026 Keiailab.
*/

package controller

import (
	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
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
