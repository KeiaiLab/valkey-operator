/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func minimalSTSParamsTSC(replicas int32) STSParams {
	return STSParams{
		CRName:      "vk",
		Namespace:   "ns",
		Replicas:    replicas,
		Image:       "valkey:9.0.4",
		StorageSize: resource.MustParse("8Gi"),
	}
}

func TestDefaultTopologySpread_replicas_ge_2_injected(t *testing.T) {
	p := minimalSTSParamsTSC(3)
	sts := BuildStatefulSet(p)
	tsc := sts.Spec.Template.Spec.TopologySpreadConstraints
	if len(tsc) != 2 {
		t.Fatalf("expected 2 default TSCs (zone + hostname), got %d", len(tsc))
	}
	if tsc[0].TopologyKey != "topology.kubernetes.io/zone" {
		t.Errorf("first TSC topologyKey: %q want zone", tsc[0].TopologyKey)
	}
	if tsc[1].TopologyKey != "kubernetes.io/hostname" {
		t.Errorf("second TSC topologyKey: %q want hostname", tsc[1].TopologyKey)
	}
	for _, c := range tsc {
		if c.MaxSkew != 1 {
			t.Errorf("MaxSkew: got %d want 1", c.MaxSkew)
		}
		if c.WhenUnsatisfiable != corev1.ScheduleAnyway {
			t.Errorf("WhenUnsatisfiable: %q want ScheduleAnyway", c.WhenUnsatisfiable)
		}
	}
}

func TestDefaultTopologySpread_replicas_1_no_inject(t *testing.T) {
	p := minimalSTSParamsTSC(1)
	sts := BuildStatefulSet(p)
	if len(sts.Spec.Template.Spec.TopologySpreadConstraints) != 0 {
		t.Errorf("Standalone (replicas=1) should not auto-inject TSC, got %d",
			len(sts.Spec.Template.Spec.TopologySpreadConstraints))
	}
}

func TestDefaultTopologySpread_user_override_preserved(t *testing.T) {
	p := minimalSTSParamsTSC(3)
	p.Pod = &cachev1alpha1.PodSpec{
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{
				MaxSkew:           2,
				TopologyKey:       "rack",
				WhenUnsatisfiable: corev1.DoNotSchedule,
			},
		},
	}
	sts := BuildStatefulSet(p)
	tsc := sts.Spec.Template.Spec.TopologySpreadConstraints
	if len(tsc) != 1 {
		t.Fatalf("user override should preserve only 1 TSC, got %d", len(tsc))
	}
	if tsc[0].TopologyKey != "rack" {
		t.Errorf("user TSC overridden: %v", tsc[0])
	}
}

func TestDefaultTopologySpread_label_selector_matches_sts_pods(t *testing.T) {
	p := minimalSTSParamsTSC(3)
	sts := BuildStatefulSet(p)
	stsSelector := sts.Spec.Selector.MatchLabels
	tsc := sts.Spec.Template.Spec.TopologySpreadConstraints[0]
	for k, v := range stsSelector {
		if got, ok := tsc.LabelSelector.MatchLabels[k]; !ok || got != v {
			t.Errorf("TSC label selector missing %q=%q (got %q)", k, v, got)
		}
	}
}
