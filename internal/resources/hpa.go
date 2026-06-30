/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"

	commonshpa "github.com/keiailab/keiailab-commons/pkg/hpa"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// HPAName — HorizontalPodAutoscaler CR 이름. STS 와 동일 (CR 이름).
func HPAName(crName string) string { return crName }

// BuildHorizontalPodAutoscaler — Spec.Autoscaling.Enabled=true 일 때 생성.
// 미활성 시 nil 반환 (caller 가 기존 HPA 삭제 책임).
//
// ADR-0027:
//   - target: StatefulSet (CRName)
//   - mode=Replication 만 사용 의도 — caller 가 사전 검증 (webhook + reconciler).
//   - CPU + Memory metric source (HPA v2 표준).
//
// 빌드 골격(ScaleTargetRef / MinReplicas clamp / Max 보정)은 keiailab-commons/pkg/hpa.Build
// 에 위임. metric 조립은 valkey 도메인(CPU 기본 70 + opt-in Memory)으로 잔류 —
// commons CPUUtilization/MemoryUtilization 헬퍼 사용. MinFloor=2 (valkey 정책).
func BuildHorizontalPodAutoscaler(v *cachev1alpha1.Valkey) *autoscalingv2.HorizontalPodAutoscaler {
	if v.Spec.Autoscaling == nil || !v.Spec.Autoscaling.Enabled {
		return nil
	}
	a := v.Spec.Autoscaling

	cpuTarget := a.TargetCPUUtilizationPercentage
	if cpuTarget == 0 {
		cpuTarget = 70
	}
	metrics := []autoscalingv2.MetricSpec{commonshpa.CPUUtilization(cpuTarget)}
	if a.TargetMemoryUtilizationPercentage > 0 {
		metrics = append(metrics, commonshpa.MemoryUtilization(a.TargetMemoryUtilizationPercentage))
	}

	return commonshpa.Build(commonshpa.Params{
		Name:        HPAName(v.Name),
		Namespace:   v.Namespace,
		Labels:      CommonLabels(v.Name, "valkey"),
		TargetKind:  "StatefulSet",
		TargetName:  StatefulSetName(v.Name),
		MinReplicas: a.MinReplicas,
		MaxReplicas: a.MaxReplicas,
		MinFloor:    2,
		Metrics:     metrics,
	})
}
