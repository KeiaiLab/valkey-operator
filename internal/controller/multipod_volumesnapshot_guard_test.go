/*
Copyright 2026 Keiailab.
*/

package controller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// 본 unit 은 isMultiPodTarget 의 fake-client driven 분기 가 아닌, handlePending 진입
// 직전 단계 의 *Source.VolumeSnapshot + multi-pod target* 조합 reject path 만 검증.
//
// envtest 통합 회귀는 별도 (test/e2e 의 cluster_recovery_test 패턴).
func TestHandlePending_VolumeSnapshot_with_Replication_target_rejected(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1: %v", err)
	}
	if err := cachev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("cachev1alpha1: %v", err)
	}

	// Replication target (replicas=3) — multi-pod.
	target := &cachev1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: "vk-rep", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeySpec{
			Mode:     cachev1alpha1.ModeReplication,
			Replicas: 3,
		},
	}
	rest := &cachev1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{Name: "rest-vs-rep", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk-rep"},
			Source: cachev1alpha1.RestoreSource{
				VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap"},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(target, rest).
		WithStatusSubresource(rest).
		Build()

	r := &ValkeyRestoreReconciler{
		Client:   c,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	_, _ = r.handlePending(testCtx(), rest)

	// rest 의 status 가 Failed phase 로 전이됐는지 검증.
	updated := &cachev1alpha1.ValkeyRestore{}
	if err := c.Get(testCtx(), client.ObjectKeyFromObject(rest), updated); err != nil {
		t.Fatalf("get rest: %v", err)
	}
	if updated.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Errorf("expected Failed phase, got %q", updated.Status.Phase)
	}
	if !strings.Contains(updated.Status.Message, "phase 1 미지원") &&
		!strings.Contains(updated.Status.Message, "Standalone 만 지원") {
		t.Errorf("Status.Message should explain phase 1 limitation, got: %q",
			updated.Status.Message)
	}
}

func TestHandlePending_VolumeSnapshot_with_Standalone_target_passes(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1: %v", err)
	}
	if err := cachev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("cachev1alpha1: %v", err)
	}

	target := &cachev1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: "vk-stand", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeySpec{
			Mode:     cachev1alpha1.ModeStandalone,
			Replicas: 1,
		},
	}
	rest := &cachev1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{Name: "rest-vs-stand", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk-stand"},
			Source: cachev1alpha1.RestoreSource{
				VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap"},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(target, rest).
		WithStatusSubresource(rest).
		Build()

	r := &ValkeyRestoreReconciler{
		Client:   c,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	_, _ = r.handlePending(testCtx(), rest)

	updated := &cachev1alpha1.ValkeyRestore{}
	if err := c.Get(testCtx(), client.ObjectKeyFromObject(rest), updated); err != nil {
		t.Fatalf("get rest: %v", err)
	}
	// Standalone + VolumeSnapshot → Mounting 으로 전이 (Failed 아님).
	if updated.Status.Phase == cachev1alpha1.RestorePhaseFailed {
		t.Errorf("Standalone + VolumeSnapshot should not Failed, got Failed: %q",
			updated.Status.Message)
	}
}
