/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// retention wiring 회귀 보호 — ValkeyBackupTarget.Spec.Retention 정책으로 이 target
// 을 참조하는 완료된 ValkeyBackup 을 internal/backuplifecycle.SelectExpired 로 만료.

package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func bkp(name, target string, phase cachev1alpha1.BackupPhase, completedAtUnix int64) cachev1alpha1.ValkeyBackup {
	b := cachev1alpha1.ValkeyBackup{}
	b.Name = name
	b.Spec.Destination = &cachev1alpha1.BackupDestination{
		Type:      cachev1alpha1.BackupDestTargetRef,
		TargetRef: &cachev1alpha1.BackupDestinationTargetRef{Name: target},
	}
	b.Status.Phase = phase
	if completedAtUnix > 0 {
		t := metav1.NewTime(time.Unix(completedAtUnix, 0).UTC())
		b.Status.CompletedAt = &t
	}
	return b
}

func TestSelectExpiredBackupsForTarget(t *testing.T) {
	t.Parallel()
	now := int64(1_000_000)
	day := int64(86400)

	t.Run("maxCount 초과분만 만료 (오래된 순)", func(t *testing.T) {
		t.Parallel()
		backups := []cachev1alpha1.ValkeyBackup{
			bkp("b1", "t1", cachev1alpha1.BackupPhaseCompleted, now-3*day),
			bkp("b2", "t1", cachev1alpha1.BackupPhaseCompleted, now-2*day),
			bkp("b3", "t1", cachev1alpha1.BackupPhaseCompleted, now-1*day),
		}
		ret := &cachev1alpha1.RetentionSpec{MaxCount: 2}
		expired := selectExpiredBackupsForTarget(backups, "t1", ret, now)
		if len(expired) != 1 || expired[0] != "b1" {
			t.Fatalf("maxCount=2 → 가장 오래된 b1 만 만료 기대, got %v", expired)
		}
	})

	t.Run("maxAgeDays 초과 만료", func(t *testing.T) {
		t.Parallel()
		backups := []cachev1alpha1.ValkeyBackup{
			bkp("old", "t1", cachev1alpha1.BackupPhaseCompleted, now-10*day),
			bkp("fresh", "t1", cachev1alpha1.BackupPhaseCompleted, now-1*day),
		}
		ret := &cachev1alpha1.RetentionSpec{MaxAgeDays: 7}
		expired := selectExpiredBackupsForTarget(backups, "t1", ret, now)
		if len(expired) != 1 || expired[0] != "old" {
			t.Fatalf("maxAgeDays=7 → old(10일) 만 만료 기대, got %v", expired)
		}
	})

	t.Run("다른 target 참조 backup 은 제외", func(t *testing.T) {
		t.Parallel()
		backups := []cachev1alpha1.ValkeyBackup{
			bkp("t1-a", "t1", cachev1alpha1.BackupPhaseCompleted, now-3*day),
			bkp("t1-b", "t1", cachev1alpha1.BackupPhaseCompleted, now-2*day),
			bkp("t2-a", "t2", cachev1alpha1.BackupPhaseCompleted, now-9*day), // 다른 target
		}
		ret := &cachev1alpha1.RetentionSpec{MaxCount: 1}
		expired := selectExpiredBackupsForTarget(backups, "t1", ret, now)
		if len(expired) != 1 || expired[0] != "t1-a" {
			t.Fatalf("t1 만 대상 → t1-a 만료 기대(t2-a 제외), got %v", expired)
		}
	})

	t.Run("미완료(Pending/Failed) backup 은 만료 대상 아님", func(t *testing.T) {
		t.Parallel()
		backups := []cachev1alpha1.ValkeyBackup{
			bkp("done1", "t1", cachev1alpha1.BackupPhaseCompleted, now-5*day),
			bkp("done2", "t1", cachev1alpha1.BackupPhaseCompleted, now-4*day),
			bkp("running", "t1", cachev1alpha1.BackupPhasePending, 0),
			bkp("failed", "t1", cachev1alpha1.BackupPhaseFailed, now-9*day),
		}
		ret := &cachev1alpha1.RetentionSpec{MaxCount: 1}
		expired := selectExpiredBackupsForTarget(backups, "t1", ret, now)
		// 완료된 done1/done2 만 후보 → maxCount=1 → done1 만료. running/failed 제외.
		if len(expired) != 1 || expired[0] != "done1" {
			t.Fatalf("완료 backup 만 대상 → done1 만료 기대, got %v", expired)
		}
	})

	t.Run("PVC 대상(TargetRef 아님) backup 제외", func(t *testing.T) {
		t.Parallel()
		pvc := cachev1alpha1.ValkeyBackup{}
		pvc.Name = "pvc-backup"
		pvc.Spec.Destination = &cachev1alpha1.BackupDestination{Type: cachev1alpha1.BackupDestPVC} // TargetRef nil
		pvc.Status.Phase = cachev1alpha1.BackupPhaseCompleted
		ct := metav1.NewTime(time.Unix(now-9*day, 0).UTC())
		pvc.Status.CompletedAt = &ct
		backups := []cachev1alpha1.ValkeyBackup{
			bkp("t1-a", "t1", cachev1alpha1.BackupPhaseCompleted, now-1*day),
			pvc,
		}
		ret := &cachev1alpha1.RetentionSpec{MaxCount: 1}
		expired := selectExpiredBackupsForTarget(backups, "t1", ret, now)
		if len(expired) != 0 {
			t.Fatalf("t1 참조 1개뿐(maxCount=1) → 만료 0 기대(PVC 제외), got %v", expired)
		}
	})

	t.Run("retention nil → 만료 없음", func(t *testing.T) {
		t.Parallel()
		backups := []cachev1alpha1.ValkeyBackup{
			bkp("a", "t1", cachev1alpha1.BackupPhaseCompleted, now-9*day),
		}
		if expired := selectExpiredBackupsForTarget(backups, "t1", nil, now); len(expired) != 0 {
			t.Fatalf("retention nil → 만료 0 기대, got %v", expired)
		}
	})
}

// applyRetention 통합 — fake client 로 실제 List + Delete 검증.
func TestApplyRetention(t *testing.T) {
	t.Parallel()
	now := int64(1_000_000)
	day := int64(86400)
	ns := "data"

	newBackup := func(name string, completedAtUnix int64) *cachev1alpha1.ValkeyBackup {
		b := bkp(name, "t1", cachev1alpha1.BackupPhaseCompleted, completedAtUnix)
		b.Namespace = ns
		return &b
	}
	target := &cachev1alpha1.ValkeyBackupTarget{}
	target.Name = "t1"
	target.Namespace = ns
	target.Spec.Retention = &cachev1alpha1.RetentionSpec{MaxCount: 2}

	scheme := runtime.NewScheme()
	_ = cachev1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			target,
			newBackup("b1", now-3*day),
			newBackup("b2", now-2*day),
			newBackup("b3", now-1*day),
		).
		Build()
	r := &ValkeyBackupTargetReconciler{Client: c, Scheme: scheme}

	deleted, err := r.applyRetention(context.Background(), target, now)
	if err != nil {
		t.Fatalf("applyRetention err: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("maxCount=2, 3 backup → 1 만료 기대, got %d", deleted)
	}

	// b1 만 삭제되고 b2/b3 는 남아있어야 한다.
	var remaining cachev1alpha1.ValkeyBackupList
	if err := c.List(context.Background(), &remaining, client.InNamespace(ns)); err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	names := map[string]bool{}
	for i := range remaining.Items {
		names[remaining.Items[i].Name] = true
	}
	if names["b1"] {
		t.Error("가장 오래된 b1 은 삭제되어야 함")
	}
	if !names["b2"] || !names["b3"] {
		t.Errorf("b2/b3 는 보존되어야 함, remaining=%v", names)
	}

	// idempotent — 재실행 시 추가 삭제 없음 (b2/b3 = maxCount 2 이내).
	deleted2, err := r.applyRetention(context.Background(), target, now)
	if err != nil {
		t.Fatalf("재실행 err: %v", err)
	}
	if deleted2 != 0 {
		t.Errorf("재실행 시 추가 만료 0 기대 (b2/b3 보존), got %d", deleted2)
	}
}
