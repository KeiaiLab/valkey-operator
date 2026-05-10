/*
Copyright 2026 Keiailab.
*/

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func pvcResizeScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1 add: %v", err)
	}
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("storagev1 add: %v", err)
	}
	return scheme
}

func boundPVC(name, ns, scName, size string) *corev1.PersistentVolumeClaim {
	q := resource.MustParse(size)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: q},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
}

func storageClass(name string, allowExpansion bool) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta:           metav1.ObjectMeta{Name: name},
		Provisioner:          "test",
		AllowVolumeExpansion: &allowExpansion,
	}
}

func getPVCSize(t *testing.T, c client.Client, ns, name string) string {
	t.Helper()
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.Get(testCtx(), types.NamespacedName{Namespace: ns, Name: name}, pvc); err != nil {
		t.Fatalf("get PVC %s: %v", name, err)
	}
	q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	return q.String()
}

func TestExpandDataPVCs_grows_when_SC_allows_expansion(t *testing.T) {
	scheme := pvcResizeScheme(t)
	sc := storageClass("gp3", true)
	pvc0 := boundPVC("data-vk-0", "ns", "gp3", "8Gi")
	pvc1 := boundPVC("data-vk-1", "ns", "gp3", "8Gi")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, pvc0, pvc1).Build()

	if err := expandDataPVCs(testCtx(), c, "ns", "vk", resource.MustParse("16Gi")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := getPVCSize(t, c, "ns", "data-vk-0"); got != "16Gi" {
		t.Errorf("data-vk-0 size = %s, want 16Gi", got)
	}
	if got := getPVCSize(t, c, "ns", "data-vk-1"); got != "16Gi" {
		t.Errorf("data-vk-1 size = %s, want 16Gi", got)
	}
}

func TestExpandDataPVCs_skips_when_SC_disallows_expansion(t *testing.T) {
	scheme := pvcResizeScheme(t)
	sc := storageClass("standard", false)
	pvc := boundPVC("data-vk-0", "ns", "standard", "8Gi")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, pvc).Build()

	if err := expandDataPVCs(testCtx(), c, "ns", "vk", resource.MustParse("16Gi")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := getPVCSize(t, c, "ns", "data-vk-0"); got != "8Gi" {
		t.Errorf("data-vk-0 size = %s, want 8Gi (no expansion)", got)
	}
}

func TestExpandDataPVCs_noop_when_size_already_at_or_above_desired(t *testing.T) {
	scheme := pvcResizeScheme(t)
	sc := storageClass("gp3", true)
	pvc := boundPVC("data-vk-0", "ns", "gp3", "16Gi")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, pvc).Build()

	if err := expandDataPVCs(testCtx(), c, "ns", "vk", resource.MustParse("16Gi")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := getPVCSize(t, c, "ns", "data-vk-0"); got != "16Gi" {
		t.Errorf("data-vk-0 size = %s, want 16Gi (unchanged)", got)
	}
}

func TestExpandDataPVCs_skips_pvcs_for_other_CRs(t *testing.T) {
	scheme := pvcResizeScheme(t)
	sc := storageClass("gp3", true)
	mine := boundPVC("data-vk-0", "ns", "gp3", "8Gi")
	other := boundPVC("data-other-0", "ns", "gp3", "8Gi")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, mine, other).Build()

	if err := expandDataPVCs(testCtx(), c, "ns", "vk", resource.MustParse("16Gi")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := getPVCSize(t, c, "ns", "data-vk-0"); got != "16Gi" {
		t.Errorf("data-vk-0 size = %s, want 16Gi", got)
	}
	if got := getPVCSize(t, c, "ns", "data-other-0"); got != "8Gi" {
		t.Errorf("data-other-0 size = %s, want 8Gi (untouched)", got)
	}
}

func TestExpandDataPVCs_skips_non_bound_pvc(t *testing.T) {
	scheme := pvcResizeScheme(t)
	sc := storageClass("gp3", true)
	pending := boundPVC("data-vk-0", "ns", "gp3", "8Gi")
	pending.Status.Phase = corev1.ClaimPending
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, pending).Build()

	if err := expandDataPVCs(testCtx(), c, "ns", "vk", resource.MustParse("16Gi")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := getPVCSize(t, c, "ns", "data-vk-0"); got != "8Gi" {
		t.Errorf("data-vk-0 size = %s, want 8Gi (Pending PVC untouched)", got)
	}
}

func TestExpandDataPVCs_zero_desired_is_noop(t *testing.T) {
	scheme := pvcResizeScheme(t)
	sc := storageClass("gp3", true)
	pvc := boundPVC("data-vk-0", "ns", "gp3", "8Gi")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, pvc).Build()

	if err := expandDataPVCs(testCtx(), c, "ns", "vk", resource.Quantity{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := getPVCSize(t, c, "ns", "data-vk-0"); got != "8Gi" {
		t.Errorf("data-vk-0 size = %s, want 8Gi", got)
	}
}
