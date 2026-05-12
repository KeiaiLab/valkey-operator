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

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

func restoreScheme(t *testing.T) *runtime.Scheme {
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

func volumeSnapshotRestore(name, snapshotName string) *cachev1alpha1.ValkeyRestore {
	return &cachev1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ns",
			UID:       "test-uid",
		},
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk"},
			Source: cachev1alpha1.RestoreSource{
				VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: snapshotName},
			},
		},
	}
}

func TestEnsureVolumeSnapshotSource_creates_new_pvc(t *testing.T) {
	rest := volumeSnapshotRestore("my-restore", "snap-2026")
	scheme := restoreScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rest).Build()

	r := &ValkeyRestoreReconciler{Client: c, Scheme: scheme}
	result, ok, err := r.ensureVolumeSnapshotSource(testCtx(), rest)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("first call should not be ready (PVC just created)")
	}
	if result.RequeueAfter == 0 {
		t.Error("expected RequeueAfter")
	}

	// 검증: PVC 가 cluster 에 생성됐는지.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.Get(testCtx(), types.NamespacedName{
		Name: "my-restore-restored", Namespace: "ns",
	}, pvc); err != nil {
		t.Fatalf("PVC not created: %v", err)
	}
	if pvc.Spec.DataSource == nil || pvc.Spec.DataSource.Name != "snap-2026" {
		t.Errorf("DataSource: %v", pvc.Spec.DataSource)
	}
	if pvc.Spec.DataSource.Kind != "VolumeSnapshot" {
		t.Errorf("DataSource.Kind: %q", pvc.Spec.DataSource.Kind)
	}
}

func TestEnsureVolumeSnapshotSource_pending_pvc_not_ready(t *testing.T) {
	rest := volumeSnapshotRestore("my-restore", "snap-2026")
	scheme := restoreScheme(t)

	// 이미 PVC 가 존재하나 phase=Pending (CSI 복사 진행 중).
	pvc := resources.BuildPVCFromVolumeSnapshot("my-restore", "ns", "snap-2026", nil,
		resource.MustParse("8Gi"))
	pvc.Status.Phase = corev1.ClaimPending
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rest, pvc).Build()

	r := &ValkeyRestoreReconciler{Client: c, Scheme: scheme}
	_, ok, err := r.ensureVolumeSnapshotSource(testCtx(), rest)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("Pending PVC should not be ready")
	}
}

func TestEnsureVolumeSnapshotSource_bound_pvc_ready(t *testing.T) {
	rest := volumeSnapshotRestore("my-restore", "snap-2026")
	scheme := restoreScheme(t)

	pvc := resources.BuildPVCFromVolumeSnapshot("my-restore", "ns", "snap-2026", nil,
		resource.MustParse("8Gi"))
	pvc.Status.Phase = corev1.ClaimBound
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rest, pvc).Build()

	r := &ValkeyRestoreReconciler{Client: c, Scheme: scheme}
	_, ok, err := r.ensureVolumeSnapshotSource(testCtx(), rest)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Error("Bound PVC should be ready (ok=true)")
	}
}

func TestEnsureVolumeSnapshotSource_nil_volumesnapshot_returns_not_ok(t *testing.T) {
	rest := &cachev1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk"},
		},
	}
	scheme := restoreScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rest).Build()
	r := &ValkeyRestoreReconciler{Client: c, Scheme: scheme}

	_, ok, err := r.ensureVolumeSnapshotSource(testCtx(), rest)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("nil VolumeSnapshot should return ok=false (defensive)")
	}
}

func TestSourcePVCName_VolumeSnapshot_uses_RestoredPVCName(t *testing.T) {
	rest := volumeSnapshotRestore("my-r", "snap")
	if got := sourcePVCName(rest); got != "my-r-restored" {
		t.Errorf("VolumeSnapshot path → sourcePVCName: %q want my-r-restored", got)
	}
}
