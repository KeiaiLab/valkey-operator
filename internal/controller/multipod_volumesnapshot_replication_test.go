/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

func multiPodScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("corev1: %v", err)
	}
	if err := cachev1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("cachev1alpha1: %v", err)
	}
	return s
}

func multipodVSRestore(name string) *cachev1alpha1.ValkeyRestore {
	return &cachev1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: "test"},
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk-rep"},
			Source: cachev1alpha1.RestoreSource{
				VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap-2026"},
			},
		},
	}
}

func TestEnsureMultiPodVolumeSnapshotSources_creates_N_PVCs(t *testing.T) {
	rest := multipodVSRestore("rest-mp")
	scheme := multiPodScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rest).Build()
	r := &ValkeyRestoreReconciler{Client: c, Scheme: scheme}

	_, ok, err := r.ensureMultiPodVolumeSnapshotSources(testCtx(), rest, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("first call should not be ready (PVCs just created)")
	}

	// 검증: 3개 PVC 가 생성됐는지.
	for o := range 3 {
		ord := int32(o)
		pvc := &corev1.PersistentVolumeClaim{}
		expectName := resources.MultiPodRestoredPVCName("rest-mp", ord)
		if err := c.Get(testCtx(), types.NamespacedName{
			Name: expectName, Namespace: "ns",
		}, pvc); err != nil {
			t.Errorf("ordinal %d PVC not created: %v", ord, err)
		}
		if pvc.Spec.DataSource == nil || pvc.Spec.DataSource.Name != "snap-2026" {
			t.Errorf("ordinal %d DataSource: %v", ord, pvc.Spec.DataSource)
		}
	}
}

func TestEnsureMultiPodVolumeSnapshotSources_all_bound_returns_ready(t *testing.T) {
	rest := multipodVSRestore("rest-mp")
	scheme := multiPodScheme(t)

	// 미리 3 PVC 를 Bound 상태로 생성.
	objs := make([]client.Object, 0, 4)
	objs = append(objs, rest)
	for o := range 3 {
		ord := int32(o)
		pvc := resources.BuildPVCFromVolumeSnapshotForOrdinal(
			"rest-mp", "ns", "snap-2026", ord, nil,
			multiPodSize8Gi(),
		)
		pvc.Status.Phase = corev1.ClaimBound
		objs = append(objs, pvc)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	r := &ValkeyRestoreReconciler{Client: c, Scheme: scheme}

	_, ok, err := r.ensureMultiPodVolumeSnapshotSources(testCtx(), rest, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Error("all PVCs Bound → expected ok=true")
	}
	// Status.Message 가 phase 2 안내를 포함해야.
	if rest.Status.Message == "" {
		t.Error("Status.Message should be populated with phase 2 guidance")
	}
}

func TestEnsureMultiPodVolumeSnapshotSources_partial_bound_not_ready(t *testing.T) {
	rest := multipodVSRestore("rest-mp")
	scheme := multiPodScheme(t)

	// 2개 Bound, 1개 Pending.
	pvc0 := resources.BuildPVCFromVolumeSnapshotForOrdinal("rest-mp", "ns", "snap", 0, nil, multiPodSize8Gi())
	pvc0.Status.Phase = corev1.ClaimBound
	pvc1 := resources.BuildPVCFromVolumeSnapshotForOrdinal("rest-mp", "ns", "snap", 1, nil, multiPodSize8Gi())
	pvc1.Status.Phase = corev1.ClaimBound
	pvc2 := resources.BuildPVCFromVolumeSnapshotForOrdinal("rest-mp", "ns", "snap", 2, nil, multiPodSize8Gi())
	pvc2.Status.Phase = corev1.ClaimPending

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(rest, pvc0, pvc1, pvc2).Build()
	r := &ValkeyRestoreReconciler{Client: c, Scheme: scheme}

	_, ok, err := r.ensureMultiPodVolumeSnapshotSources(testCtx(), rest, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("partial Bound (2/3) should not be ready")
	}
}

func TestEnsureMultiPodVolumeSnapshotSources_replicaCount_lt_2_returns_not_ok(t *testing.T) {
	rest := multipodVSRestore("rest-mp")
	scheme := multiPodScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rest).Build()
	r := &ValkeyRestoreReconciler{Client: c, Scheme: scheme}

	_, ok, err := r.ensureMultiPodVolumeSnapshotSources(testCtx(), rest, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("replicaCount=1 should return ok=false (defensive guard)")
	}
}

func TestMultiPodRestoredPVCName(t *testing.T) {
	cases := map[int32]string{
		0: "my-rest-restored-0",
		1: "my-rest-restored-1",
		5: "my-rest-restored-5",
	}
	for ord, want := range cases {
		if got := resources.MultiPodRestoredPVCName("my-rest", ord); got != want {
			t.Errorf("ord %d: got %q want %q", ord, got, want)
		}
	}
}

// multiPodSize8Gi — 테스트 helper.
func multiPodSize8Gi() resource.Quantity {
	return resource.MustParse("8Gi")
}
