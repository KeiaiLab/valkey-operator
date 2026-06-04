/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
func BuildHorizontalPodAutoscaler(v *cachev1alpha1.Valkey) *autoscalingv2.HorizontalPodAutoscaler {
	if v.Spec.Autoscaling == nil || !v.Spec.Autoscaling.Enabled {
		return nil
	}
	a := v.Spec.Autoscaling

	minR := max(a.MinReplicas, int32(2))
	maxR := max(a.MaxReplicas, minR)

	cpuTarget := a.TargetCPUUtilizationPercentage
	if cpuTarget == 0 {
		cpuTarget = 70
	}

	metrics := []autoscalingv2.MetricSpec{
		{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: new(cpuTarget),
				},
			},
		},
	}
	if a.TargetMemoryUtilizationPercentage > 0 {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: new(a.TargetMemoryUtilizationPercentage),
				},
			},
		})
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HPAName(v.Name),
			Namespace: v.Namespace,
			Labels:    CommonLabels(v.Name, "valkey"),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       StatefulSetName(v.Name),
			},
			MinReplicas: new(minR),
			MaxReplicas: maxR,
			Metrics:     metrics,
		},
	}
}
