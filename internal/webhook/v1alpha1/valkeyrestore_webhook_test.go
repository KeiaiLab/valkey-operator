/*
Copyright 2026 Keiailab.
*/

package v1alpha1

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func validRestorePVCSource() *cachev1alpha1.ValkeyRestore {
	return &cachev1alpha1.ValkeyRestore{
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk"},
			Source:     cachev1alpha1.RestoreSource{PVC: &cachev1alpha1.RestoreSourcePVC{Name: "src-pvc"}},
		},
	}
}

func validRestoreTargetRefSource() *cachev1alpha1.ValkeyRestore {
	return &cachev1alpha1.ValkeyRestore{
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk"},
			Source: cachev1alpha1.RestoreSource{
				TargetRef: &cachev1alpha1.RestoreSourceTargetRef{Name: "s3-prod", Path: "p/dump.rdb"},
			},
		},
	}
}

func validRestoreVolumeSnapshotSource() *cachev1alpha1.ValkeyRestore {
	return &cachev1alpha1.ValkeyRestore{
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk"},
			Source: cachev1alpha1.RestoreSource{
				VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap-2026"},
			},
		},
	}
}

func TestRestoreValidate_PVC_only_passes(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), validRestorePVCSource()); err != nil {
		t.Errorf("PVC-only should pass: %v", err)
	}
}

func TestRestoreValidate_TargetRef_only_passes(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), validRestoreTargetRefSource()); err != nil {
		t.Errorf("TargetRef-only should pass: %v", err)
	}
}

func TestRestoreValidate_VolumeSnapshot_only_passes(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	if _, err := v.ValidateCreate(context.Background(), validRestoreVolumeSnapshotSource()); err != nil {
		t.Errorf("VolumeSnapshot-only should pass: %v", err)
	}
}

func TestRestoreValidate_no_source_rejected(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	obj := &cachev1alpha1.ValkeyRestore{
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk"},
		},
	}
	_, err := v.ValidateCreate(context.Background(), obj)
	if err == nil || !strings.Contains(err.Error(), "exactly one of") {
		t.Errorf("expected 'exactly one of' reject, got %v", err)
	}
}

func TestRestoreValidate_PVC_and_TargetRef_rejected(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	obj := validRestorePVCSource()
	obj.Spec.Source.TargetRef = &cachev1alpha1.RestoreSourceTargetRef{Name: "s3", Path: "p"}

	_, err := v.ValidateCreate(context.Background(), obj)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual exclusion reject, got %v", err)
	}
}

func TestRestoreValidate_PVC_and_VolumeSnapshot_rejected(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	obj := validRestorePVCSource()
	obj.Spec.Source.VolumeSnapshot = &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap"}

	_, err := v.ValidateCreate(context.Background(), obj)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual exclusion reject, got %v", err)
	}
}

func TestRestoreValidate_all_three_rejected(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	obj := validRestorePVCSource()
	obj.Spec.Source.TargetRef = &cachev1alpha1.RestoreSourceTargetRef{Name: "s3", Path: "p"}
	obj.Spec.Source.VolumeSnapshot = &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap"}

	_, err := v.ValidateCreate(context.Background(), obj)
	if err == nil {
		t.Fatal("expected mutual exclusion reject for all 3 sources")
	}
}

func TestRestoreValidate_PointInTime_with_RDB_rejected(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	obj := validRestoreTargetRefSource()
	obj.Spec.RestoreType = cachev1alpha1.RestoreTypeRDB
	obj.Spec.PointInTime = &metav1.Time{Time: time.Now()}

	_, err := v.ValidateCreate(context.Background(), obj)
	if err == nil || !strings.Contains(err.Error(), "AOF") {
		t.Errorf("expected RDB-PointInTime reject, got %v", err)
	}
}

func TestRestoreValidate_PointInTime_with_AOF_passes(t *testing.T) {
	v := &ValkeyRestoreCustomValidator{}
	obj := validRestoreTargetRefSource()
	obj.Spec.RestoreType = cachev1alpha1.RestoreTypeAOF
	obj.Spec.PointInTime = &metav1.Time{Time: time.Now()}

	if _, err := v.ValidateCreate(context.Background(), obj); err != nil {
		t.Errorf("AOF + PointInTime should pass: %v", err)
	}
}

func TestRestoreValidate_PointInTime_default_RDB_rejected(t *testing.T) {
	// RestoreType 미명시 → CRD default RDB → PointInTime 명시 시 reject.
	v := &ValkeyRestoreCustomValidator{}
	obj := validRestoreTargetRefSource()
	obj.Spec.PointInTime = &metav1.Time{Time: time.Now()}

	_, err := v.ValidateCreate(context.Background(), obj)
	if err == nil {
		t.Fatal("expected default-RDB + PointInTime reject")
	}
}

func TestRestoreValidate_no_PointInTime_AOF_passes(t *testing.T) {
	// PointInTime nil + AOF — AOF 전체 replay (default 동작).
	v := &ValkeyRestoreCustomValidator{}
	obj := validRestoreTargetRefSource()
	obj.Spec.RestoreType = cachev1alpha1.RestoreTypeAOF

	if _, err := v.ValidateCreate(context.Background(), obj); err != nil {
		t.Errorf("AOF without PointInTime should pass: %v", err)
	}
}
