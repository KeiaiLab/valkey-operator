/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// conversion_test.go 는 Valkey / ValkeyCluster 두 타입만 덮고 있었고, 나머지 세
// 타입(ValkeyBackup / ValkeyBackupTarget / ValkeyRestore)의 ConvertTo·ConvertFrom
// 은 커버리지 0% 였다. 변환은 JSON byte-copy 라 **양쪽 JSON tag 가 어긋나면
// 조용히 필드가 사라진다** — 백업 대상이나 복원 지점이 유실돼도 에러가 나지
// 않고, 그 사실은 실제로 복원해 보기 전까지 드러나지 않는다. API 버전 승격의
// 안전장치가 바로 이 변환이므로 세 타입 모두 왕복 보존을 고정한다.
package v1alpha1_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/api/v1alpha2"
)

// TestValkeyBackup_Convert_왕복에서_필드가_보존된다 — 백업 스펙이 유실되면
// 엉뚱한 대상/보존정책으로 백업이 돌고, 그 사실은 복원 시점에야 드러난다.
func TestValkeyBackup_Convert_왕복에서_필드가_보존된다(t *testing.T) {
	src := &v1alpha1.ValkeyBackup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-test", Namespace: "ns"},
		Spec: v1alpha1.ValkeyBackupSpec{
			ClusterRef:              v1alpha1.ClusterReference{Kind: "ValkeyCluster", Name: "vk"},
			Type:                    v1alpha1.BackupType("Full"),
			TargetPVC:               &corev1.LocalObjectReference{Name: "backup-pvc"},
			StorageSize:             "10Gi",
			RetainPVC:               true,
			TTL:                     "168h",
			VolumeSnapshotClassName: "csi-snapclass",
		},
	}

	dst := &v1alpha2.ValkeyBackup{}
	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if dst.Spec.ClusterRef.Name != "vk" || dst.Spec.ClusterRef.Kind != "ValkeyCluster" {
		t.Errorf("ClusterRef 유실: %+v", dst.Spec.ClusterRef)
	}
	if dst.Spec.TargetPVC == nil || dst.Spec.TargetPVC.Name != "backup-pvc" {
		t.Errorf("TargetPVC 유실: %+v", dst.Spec.TargetPVC)
	}
	if dst.Spec.StorageSize != "10Gi" || dst.Spec.TTL != "168h" {
		t.Errorf("StorageSize/TTL 유실: %q / %q", dst.Spec.StorageSize, dst.Spec.TTL)
	}
	if !dst.Spec.RetainPVC {
		t.Error("RetainPVC=true 가 false 로 뒤집혔다 — 백업 PVC 가 조기 삭제된다")
	}
	if dst.Spec.VolumeSnapshotClassName != "csi-snapclass" {
		t.Errorf("VolumeSnapshotClassName 유실: %q", dst.Spec.VolumeSnapshotClassName)
	}

	back := &v1alpha1.ValkeyBackup{}
	if err := back.ConvertFrom(dst); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if back.Spec.TargetPVC == nil || back.Spec.TargetPVC.Name != "backup-pvc" {
		t.Errorf("역방향 TargetPVC 유실: %+v", back.Spec.TargetPVC)
	}
	if back.Spec.TTL != "168h" || !back.Spec.RetainPVC {
		t.Errorf("역방향 TTL/RetainPVC 유실: %q / %v", back.Spec.TTL, back.Spec.RetainPVC)
	}
}

// TestValkeyBackupTarget_Convert_왕복에서_대상이_보존된다 — 타깃이 유실되면
// 백업이 조용히 다른 곳(또는 아무 데도)으로 간다.
func TestValkeyBackupTarget_Convert_왕복에서_대상이_보존된다(t *testing.T) {
	src := &v1alpha1.ValkeyBackupTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "target-test", Namespace: "ns"},
		Spec: v1alpha1.ValkeyBackupTargetSpec{
			Type: v1alpha1.BackupTargetType("S3"),
		},
	}

	dst := &v1alpha2.ValkeyBackupTarget{}
	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if string(dst.Spec.Type) != "S3" {
		t.Errorf("Type 유실: %q", dst.Spec.Type)
	}

	back := &v1alpha1.ValkeyBackupTarget{}
	if err := back.ConvertFrom(dst); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if string(back.Spec.Type) != "S3" {
		t.Errorf("역방향 Type 유실: %q", back.Spec.Type)
	}
}

// TestValkeyRestore_Convert_왕복에서_복원지점이_보존된다 — PointInTime 이
// 유실되면 의도한 시점이 아닌 데이터로 복원된다. 조용한 데이터 손실이다.
func TestValkeyRestore_Convert_왕복에서_복원지점이_보존된다(t *testing.T) {
	pit := metav1.NewTime(metav1.Now().Rfc3339Copy().Time)
	src := &v1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{Name: "restore-test", Namespace: "ns"},
		Spec: v1alpha1.ValkeyRestoreSpec{
			ClusterRef:  v1alpha1.ClusterReference{Kind: "ValkeyCluster", Name: "vk"},
			RestoreType: v1alpha1.RestoreType("PointInTime"),
			PointInTime: &pit,
			Source: v1alpha1.RestoreSource{
				PVC: &v1alpha1.RestoreSourcePVC{Name: "snap-pvc"},
			},
		},
	}

	dst := &v1alpha2.ValkeyRestore{}
	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("ConvertTo: %v", err)
	}
	if dst.Spec.PointInTime == nil || !dst.Spec.PointInTime.Equal(&pit) {
		t.Errorf("PointInTime 유실/변형: %v (want %v)", dst.Spec.PointInTime, pit)
	}
	if dst.Spec.Source.PVC == nil || dst.Spec.Source.PVC.Name != "snap-pvc" {
		t.Errorf("Source.PVC 유실: %+v", dst.Spec.Source.PVC)
	}
	if dst.Spec.ClusterRef.Name != "vk" {
		t.Errorf("ClusterRef 유실: %+v", dst.Spec.ClusterRef)
	}

	back := &v1alpha1.ValkeyRestore{}
	if err := back.ConvertFrom(dst); err != nil {
		t.Fatalf("ConvertFrom: %v", err)
	}
	if back.Spec.PointInTime == nil || !back.Spec.PointInTime.Equal(&pit) {
		t.Errorf("역방향 PointInTime 유실/변형: %v", back.Spec.PointInTime)
	}
	if back.Spec.Source.PVC == nil || back.Spec.Source.PVC.Name != "snap-pvc" {
		t.Errorf("역방향 Source.PVC 유실: %+v", back.Spec.Source.PVC)
	}
}
