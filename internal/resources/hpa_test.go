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

/*
Copyright 2026 Keiailab.
*/

package resources

import (
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func vkWithAutoscaling(min, max, cpu, mem int32) *cachev1alpha1.Valkey {
	return &cachev1alpha1.Valkey{
		Spec: cachev1alpha1.ValkeySpec{
			Mode: cachev1alpha1.ModeReplication,
			Autoscaling: &cachev1alpha1.AutoscalingSpec{
				Enabled:                           true,
				MinReplicas:                       min,
				MaxReplicas:                       max,
				TargetCPUUtilizationPercentage:    cpu,
				TargetMemoryUtilizationPercentage: mem,
			},
		},
	}
}

func TestBuildHPA_disabled_returns_nil(t *testing.T) {
	v := &cachev1alpha1.Valkey{}
	if h := BuildHorizontalPodAutoscaler(v); h != nil {
		t.Errorf("disabled (Autoscaling nil) → expected nil HPA, got %v", h)
	}
	v.Spec.Autoscaling = &cachev1alpha1.AutoscalingSpec{Enabled: false}
	if h := BuildHorizontalPodAutoscaler(v); h != nil {
		t.Errorf("disabled (Enabled=false) → expected nil HPA")
	}
}

func TestBuildHPA_cpu_only_minimal(t *testing.T) {
	v := vkWithAutoscaling(2, 5, 70, 0)
	v.Name = "vk"
	v.Namespace = "ns"

	hpa := BuildHorizontalPodAutoscaler(v)
	if hpa == nil {
		t.Fatal("expected HPA")
	}
	if hpa.Name != "vk" || hpa.Namespace != "ns" {
		t.Errorf("metadata: %s/%s", hpa.Namespace, hpa.Name)
	}
	if hpa.Spec.ScaleTargetRef.Kind != "StatefulSet" || hpa.Spec.ScaleTargetRef.Name != "vk" {
		t.Errorf("ScaleTargetRef: %v", hpa.Spec.ScaleTargetRef)
	}
	if *hpa.Spec.MinReplicas != 2 || hpa.Spec.MaxReplicas != 5 {
		t.Errorf("min/max: %d/%d", *hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas)
	}
	if len(hpa.Spec.Metrics) != 1 {
		t.Fatalf("expected 1 metric (cpu only), got %d", len(hpa.Spec.Metrics))
	}
	cpu := hpa.Spec.Metrics[0]
	if cpu.Resource == nil || cpu.Resource.Name != corev1.ResourceCPU {
		t.Errorf("cpu metric: %v", cpu)
	}
	if *cpu.Resource.Target.AverageUtilization != 70 {
		t.Errorf("cpu target: %d", *cpu.Resource.Target.AverageUtilization)
	}
}

func TestBuildHPA_with_memory(t *testing.T) {
	v := vkWithAutoscaling(2, 5, 70, 80)
	v.Name = "vk"
	v.Namespace = "ns"

	hpa := BuildHorizontalPodAutoscaler(v)
	if len(hpa.Spec.Metrics) != 2 {
		t.Fatalf("expected 2 metrics (cpu + memory), got %d", len(hpa.Spec.Metrics))
	}
	if hpa.Spec.Metrics[1].Resource.Name != corev1.ResourceMemory {
		t.Errorf("2nd metric not memory: %v", hpa.Spec.Metrics[1])
	}
	if *hpa.Spec.Metrics[1].Resource.Target.AverageUtilization != 80 {
		t.Errorf("memory target: %d", *hpa.Spec.Metrics[1].Resource.Target.AverageUtilization)
	}
}

func TestBuildHPA_default_cpu_target_70(t *testing.T) {
	v := vkWithAutoscaling(2, 5, 0, 0) // CPU target unset.
	hpa := BuildHorizontalPodAutoscaler(v)
	if *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization != 70 {
		t.Errorf("default CPU target: %d", *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization)
	}
}

func TestBuildHPA_min_clamped_to_2(t *testing.T) {
	v := vkWithAutoscaling(0, 5, 70, 0)
	hpa := BuildHorizontalPodAutoscaler(v)
	if *hpa.Spec.MinReplicas != 2 {
		t.Errorf("min clamp: got %d, want 2 (Replication topology)", *hpa.Spec.MinReplicas)
	}
}

func TestBuildHPA_max_clamped_to_min(t *testing.T) {
	v := vkWithAutoscaling(3, 1, 70, 0) // max < min.
	hpa := BuildHorizontalPodAutoscaler(v)
	if hpa.Spec.MaxReplicas != 3 {
		t.Errorf("max clamp: got %d, want 3 (= min)", hpa.Spec.MaxReplicas)
	}
}

// TestBuildHPA_targetRef_apiVersion — k8s autoscaling/v2 표준은 ScaleTargetRef
// 의 APIVersion=apps/v1 (StatefulSet 의 group/version).
func TestBuildHPA_targetRef_apiVersion(t *testing.T) {
	v := vkWithAutoscaling(2, 5, 70, 0)
	v.Name = "vk"
	hpa := BuildHorizontalPodAutoscaler(v)
	if hpa.Spec.ScaleTargetRef.APIVersion != "apps/v1" {
		t.Errorf("APIVersion: %q want apps/v1", hpa.Spec.ScaleTargetRef.APIVersion)
	}
}

// 컴파일 시 type assertion — autoscalingv2 import 사용.
var _ autoscalingv2.HorizontalPodAutoscaler
