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
package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestRestoredPVCName(t *testing.T) {
	if got := RestoredPVCName("vk-restore"); got != "vk-restore-restored" {
		t.Errorf("name: %q", got)
	}
}

func TestBuildPVCFromVolumeSnapshot_minimal(t *testing.T) {
	pvc := BuildPVCFromVolumeSnapshot(
		"my-restore", "ns", "snap-2026-05",
		nil, resource.MustParse("16Gi"),
	)
	if pvc.Name != "my-restore-restored" {
		t.Errorf("name: %q", pvc.Name)
	}
	if pvc.Namespace != "ns" {
		t.Errorf("namespace: %q", pvc.Namespace)
	}
	if pvc.Spec.DataSource == nil {
		t.Fatal("DataSource not set")
	}
	if pvc.Spec.DataSource.Name != "snap-2026-05" {
		t.Errorf("DataSource.Name: %q", pvc.Spec.DataSource.Name)
	}
	if pvc.Spec.DataSource.Kind != "VolumeSnapshot" {
		t.Errorf("DataSource.Kind: %q", pvc.Spec.DataSource.Kind)
	}
	if pvc.Spec.DataSource.APIGroup == nil || *pvc.Spec.DataSource.APIGroup != "snapshot.storage.k8s.io" {
		t.Errorf("DataSource.APIGroup: %v", pvc.Spec.DataSource.APIGroup)
	}
	q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if q.String() != "16Gi" {
		t.Errorf("size: %s", q.String())
	}
}

func TestBuildPVCFromVolumeSnapshot_storageClass_set(t *testing.T) {
	sc := "ebs-gp3"
	pvc := BuildPVCFromVolumeSnapshot("r", "ns", "snap",
		new(sc), resource.MustParse("8Gi"))
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != sc {
		t.Errorf("StorageClass: %v", pvc.Spec.StorageClassName)
	}
}

func TestBuildPVCFromVolumeSnapshot_default_size(t *testing.T) {
	pvc := BuildPVCFromVolumeSnapshot("r", "ns", "snap", nil, resource.Quantity{})
	q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if q.String() != "8Gi" {
		t.Errorf("default size: %s want 8Gi", q.String())
	}
}

func TestIsVolumeSnapshotRestore_nil_spec_false(t *testing.T) {
	if IsVolumeSnapshotRestore(nil) {
		t.Error("nil spec should not be VolumeSnapshot restore")
	}
}

func TestIsVolumeSnapshotRestore_PVC_source_false(t *testing.T) {
	spec := &cachev1alpha1.ValkeyRestoreSpec{
		Source: cachev1alpha1.RestoreSource{PVC: &cachev1alpha1.RestoreSourcePVC{Name: "x"}},
	}
	if IsVolumeSnapshotRestore(spec) {
		t.Error("PVC source should not be VolumeSnapshot restore")
	}
}

func TestIsVolumeSnapshotRestore_VolumeSnapshot_set_true(t *testing.T) {
	spec := &cachev1alpha1.ValkeyRestoreSpec{
		Source: cachev1alpha1.RestoreSource{
			VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap"},
		},
	}
	if !IsVolumeSnapshotRestore(spec) {
		t.Error("VolumeSnapshot source should be detected")
	}
}

func TestIsVolumeSnapshotRestore_VolumeSnapshot_empty_name_false(t *testing.T) {
	spec := &cachev1alpha1.ValkeyRestoreSpec{
		Source: cachev1alpha1.RestoreSource{
			VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: ""},
		},
	}
	if IsVolumeSnapshotRestore(spec) {
		t.Error("empty name should not count")
	}
}
