/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

// findCondition — Type 으로 condition 검색.
func findCondition(conds []metav1.Condition, t string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == t {
			return &conds[i]
		}
	}
	return nil
}

func TestApplyClusterConditions_running_state(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Generation = 5
	vc.Status.Phase = cachev1alpha1.ClusterPhaseRunning
	conds := []metav1.Condition{}
	info := &vk.ClusterInfo{State: "ok", SlotsAssigned: 16384, SlotsOK: 16384}

	applyClusterConditions(&conds, vc, info, 6, 6)

	// 5개 condition 모두 존재.
	for _, want := range []string{
		CondTypeReady, CondTypeClusterReady, CondTypeScalePending,
		CondTypeUpgradeInProgress, CondTypeCertReady,
	} {
		if findCondition(conds, want) == nil {
			t.Errorf("missing condition %s", want)
		}
	}

	if c := findCondition(conds, CondTypeReady); c.Status != metav1.ConditionTrue {
		t.Errorf("Ready: %v", c.Status)
	}
	if c := findCondition(conds, CondTypeClusterReady); c.Status != metav1.ConditionTrue {
		t.Errorf("ClusterReady: %v %s", c.Status, c.Message)
	}
	if c := findCondition(conds, CondTypeScalePending); c.Status != metav1.ConditionFalse {
		t.Errorf("ScalePending should be False when Status.PendingScale==nil: %v", c.Status)
	}
	if c := findCondition(conds, CondTypeUpgradeInProgress); c.Status != metav1.ConditionFalse {
		t.Errorf("UpgradeInProgress: %v", c.Status)
	}
	if c := findCondition(conds, CondTypeCertReady); c.Status != metav1.ConditionTrue {
		t.Errorf("CertReady (TLS disabled) should be True: %v", c.Status)
	}
	// ObservedGeneration 전파.
	if c := findCondition(conds, CondTypeReady); c.ObservedGeneration != 5 {
		t.Errorf("ObservedGeneration: %d", c.ObservedGeneration)
	}
}

func TestApplyClusterConditions_failed_state(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Status.Phase = cachev1alpha1.ClusterPhaseFailed
	conds := []metav1.Condition{}

	applyClusterConditions(&conds, vc, nil, 6, 0)

	if c := findCondition(conds, CondTypeReady); c.Status != metav1.ConditionFalse {
		t.Errorf("Failed phase → Ready=False, got %v", c.Status)
	}
	if c := findCondition(conds, CondTypeClusterReady); c.Status != metav1.ConditionFalse {
		t.Errorf("ClusterReady: %v", c.Status)
	}
}

func TestApplyClusterConditions_resharding(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Status.Phase = cachev1alpha1.ClusterPhaseResharding
	conds := []metav1.Condition{}
	info := &vk.ClusterInfo{State: "ok", SlotsAssigned: 8192}

	applyClusterConditions(&conds, vc, info, 6, 6)

	c := findCondition(conds, CondTypeClusterReady)
	if c.Status != metav1.ConditionFalse {
		t.Errorf("ClusterReady during resharding: %v", c.Status)
	}
	if c.Reason != "ClusterNotConverged" {
		t.Errorf("Reason: %q", c.Reason)
	}
}

// 결함 ⑤ — partial-slot outage 는 ClusterReady=False (Reason=PartialSlotOutage).
func TestApplyClusterConditions_partialSlotOutage(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Status.Phase = cachev1alpha1.ClusterPhaseResharding
	conds := []metav1.Condition{}
	// state=ok + assigned=16384 이지만 slots_ok<16384.
	info := &vk.ClusterInfo{State: "ok", SlotsAssigned: 16384, SlotsOK: 10922}

	applyClusterConditions(&conds, vc, info, 6, 6)

	c := findCondition(conds, CondTypeClusterReady)
	if c.Status != metav1.ConditionFalse {
		t.Errorf("ClusterReady during partial outage must be False: %v", c.Status)
	}
	if c.Reason != "PartialSlotOutage" {
		t.Errorf("Reason: %q want PartialSlotOutage", c.Reason)
	}
}

func TestApplyClusterConditions_scalePending(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Status.Phase = cachev1alpha1.ClusterPhaseRunning
	vc.Status.PendingScale = &cachev1alpha1.PendingScale{
		CurrentReplicas: 6,
		DesiredReplicas: 9,
		Reason:          "Topology change deferred",
	}
	conds := []metav1.Condition{}

	applyClusterConditions(&conds, vc, &vk.ClusterInfo{State: "ok", SlotsAssigned: 16384, SlotsOK: 16384}, 6, 6)

	c := findCondition(conds, CondTypeScalePending)
	if c.Status != metav1.ConditionTrue {
		t.Errorf("ScalePending: %v", c.Status)
	}
	if c.Reason != "DeliberateRequired" {
		t.Errorf("Reason: %q", c.Reason)
	}
}

func TestApplyClusterConditions_upgrading(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Status.Phase = cachev1alpha1.ClusterPhaseUpgrading
	vc.Status.Version = "8.1.6"
	vc.Spec.Version.Version = "8.2.0"
	conds := []metav1.Condition{}

	applyClusterConditions(&conds, vc, nil, 6, 3)

	c := findCondition(conds, CondTypeUpgradeInProgress)
	if c.Status != metav1.ConditionTrue {
		t.Errorf("UpgradeInProgress: %v", c.Status)
	}
	if c.Reason != "VersionTransition" {
		t.Errorf("Reason: %q", c.Reason)
	}
}

func TestApplyClusterConditions_TLS_certManager_configured(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Status.Phase = cachev1alpha1.ClusterPhaseRunning
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:     true,
		CertManager: &cachev1alpha1.CertManagerSpec{IssuerRef: cachev1alpha1.CertIssuerRef{Name: "letsencrypt"}},
	}
	conds := []metav1.Condition{}

	applyClusterConditions(&conds, vc, &vk.ClusterInfo{State: "ok", SlotsAssigned: 16384}, 6, 6)

	c := findCondition(conds, CondTypeCertReady)
	if c.Status != metav1.ConditionTrue {
		t.Errorf("CertReady (cert-manager configured): %v", c.Status)
	}
	if c.Reason != "CABundleConfigured" {
		t.Errorf("Reason: %q", c.Reason)
	}
}

func TestApplyClusterConditions_TLS_no_CA_falsepositive(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Status.Phase = cachev1alpha1.ClusterPhaseRunning
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true} // CA 미설정.
	conds := []metav1.Condition{}

	applyClusterConditions(&conds, vc, &vk.ClusterInfo{State: "ok", SlotsAssigned: 16384}, 6, 6)

	c := findCondition(conds, CondTypeCertReady)
	if c.Status != metav1.ConditionFalse {
		t.Errorf("TLS enabled without CA → CertReady=False: %v", c.Status)
	}
	if c.Reason != "FallbackInsecureSkipVerify" {
		t.Errorf("Reason: %q", c.Reason)
	}
}

// setCondition — 동일 Type 두 번째 set 시 *교체* + status 변경 시에만 LastTransitionTime 갱신.
func TestSetCondition_replace_and_transitionTime(t *testing.T) {
	conds := []metav1.Condition{}

	c1 := metav1.Condition{Type: "X", Status: metav1.ConditionTrue, Reason: "A"}
	setCondition(&conds, c1)
	if len(conds) != 1 {
		t.Fatalf("len: %d", len(conds))
	}
	t1 := conds[0].LastTransitionTime
	if t1.IsZero() {
		t.Fatal("LastTransitionTime should be set on first add")
	}

	// 같은 status — LastTransitionTime 보존.
	c2 := metav1.Condition{Type: "X", Status: metav1.ConditionTrue, Reason: "B"}
	setCondition(&conds, c2)
	if len(conds) != 1 {
		t.Fatalf("len after same status: %d", len(conds))
	}
	if !conds[0].LastTransitionTime.Equal(&t1) {
		t.Errorf("LastTransitionTime should be preserved when status unchanged")
	}
	if conds[0].Reason != "B" {
		t.Errorf("Reason should update: %q", conds[0].Reason)
	}

	// 다른 status — LastTransitionTime 갱신.
	c3 := metav1.Condition{Type: "X", Status: metav1.ConditionFalse, Reason: "C"}
	setCondition(&conds, c3)
	if conds[0].LastTransitionTime.Equal(&t1) {
		t.Errorf("LastTransitionTime should change when status changes")
	}
}

func TestBoolToConditionStatus(t *testing.T) {
	if boolToConditionStatus(true) != metav1.ConditionTrue {
		t.Error("true → ConditionTrue")
	}
	if boolToConditionStatus(false) != metav1.ConditionFalse {
		t.Error("false → ConditionFalse")
	}
}
