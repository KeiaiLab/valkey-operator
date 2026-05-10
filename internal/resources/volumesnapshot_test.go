/*
Copyright 2026 Keiailab.
*/

package resources

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func backupOfType(t cachev1alpha1.BackupType, snapshotClass string) *cachev1alpha1.ValkeyBackup {
	return &cachev1alpha1.ValkeyBackup{
		ObjectMeta: metav1.ObjectMeta{Name: "vk-backup", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyBackupSpec{
			Type:                    t,
			VolumeSnapshotClassName: snapshotClass,
		},
	}
}

func TestBuildVolumeSnapshot_RDB_returns_nil(t *testing.T) {
	b := backupOfType(cachev1alpha1.BackupTypeRDB, "")
	if u := BuildVolumeSnapshotForBackup(b, "vk"); u != nil {
		t.Error("RDB type should NOT produce VolumeSnapshot")
	}
}

func TestBuildVolumeSnapshot_AOF_returns_nil(t *testing.T) {
	b := backupOfType(cachev1alpha1.BackupTypeAOF, "")
	if u := BuildVolumeSnapshotForBackup(b, "vk"); u != nil {
		t.Error("AOF type should NOT produce VolumeSnapshot")
	}
}

func TestBuildVolumeSnapshot_VolumeSnapshot_minimal(t *testing.T) {
	b := backupOfType(cachev1alpha1.BackupTypeVolumeSnapshot, "")
	u := BuildVolumeSnapshotForBackup(b, "vk")
	if u == nil {
		t.Fatal("VolumeSnapshot type should produce CR")
	}
	if u.GetName() != "vk-backup-snap" {
		t.Errorf("name: %q", u.GetName())
	}
	if u.GetNamespace() != "ns" {
		t.Errorf("namespace: %q", u.GetNamespace())
	}
	if u.GetAPIVersion() != "snapshot.storage.k8s.io/v1" || u.GetKind() != "VolumeSnapshot" {
		t.Errorf("GVK: %s/%s", u.GetAPIVersion(), u.GetKind())
	}
	spec := u.Object["spec"].(map[string]any)
	source := spec["source"].(map[string]any)
	if source["persistentVolumeClaimName"] != "data-vk-0" {
		t.Errorf("source PVC: %v", source["persistentVolumeClaimName"])
	}
	if _, ok := spec["volumeSnapshotClassName"]; ok {
		t.Errorf("volumeSnapshotClassName 미명시 시 spec field 미설정 (cluster default 사용), got %v", spec)
	}
}

func TestBuildVolumeSnapshot_with_class_name(t *testing.T) {
	b := backupOfType(cachev1alpha1.BackupTypeVolumeSnapshot, "ebs-snapshot-class")
	u := BuildVolumeSnapshotForBackup(b, "vk")
	if u == nil {
		t.Fatal("expected CR")
	}
	spec := u.Object["spec"].(map[string]any)
	if spec["volumeSnapshotClassName"] != "ebs-snapshot-class" {
		t.Errorf("snapshot class: %v", spec["volumeSnapshotClassName"])
	}
}

func TestBuildVolumeSnapshot_labels(t *testing.T) {
	b := backupOfType(cachev1alpha1.BackupTypeVolumeSnapshot, "")
	u := BuildVolumeSnapshotForBackup(b, "vk-prod")
	labels := u.GetLabels()
	if labels[LabelInstanceName] != "vk-prod" {
		t.Errorf("instance label: %v", labels)
	}
	if labels[LabelComponent] != "backup-snapshot" {
		t.Errorf("component label: %v", labels)
	}
	if labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("managed-by label: %v", labels)
	}
}

func TestDataPVCName(t *testing.T) {
	cases := map[int]string{0: "data-vk-0", 1: "data-vk-1", 5: "data-vk-5"}
	for ord, want := range cases {
		if got := dataPVCName("vk", ord); got != want {
			t.Errorf("ordinal %d: got %q want %q", ord, got, want)
		}
	}
}
