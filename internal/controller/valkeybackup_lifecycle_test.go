/*
Copyright 2026 Keiailab.

ValkeyBackup TTL + finalizer + cleanup 단위 테스트.
*/

package controller

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

func backupReconcilerWithFakeClient(objs ...client.Object) *ValkeyBackupReconciler {
	scheme := runtime.NewScheme()
	_ = cachev1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&cachev1alpha1.ValkeyBackup{}).
		Build()
	return &ValkeyBackupReconciler{Client: c, Scheme: scheme}
}

// 1. Terminal + TTL 미만료 → RequeueAfter 설정.
func TestBackup_terminal_ttlPending_requeues(t *testing.T) {
	now := metav1.Now()
	b := freshBackup("b1", "ns", "Valkey", "vk")
	b.Spec.TTL = "1h"
	b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
	b.Status.CompletedAt = &now
	r := backupReconcilerWithFakeClient(b)

	res, err := r.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: "b1", Namespace: "ns"}})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// TTL=1h, completed=now → deadline in ~1h → RequeueAfter ~1h.
	if res.RequeueAfter < 50*time.Minute || res.RequeueAfter > 65*time.Minute {
		t.Fatalf("expected ~1h requeue, got %v", res.RequeueAfter)
	}
}

// 2. Terminal + TTL 만료 → self-delete (CR 가 fake client 에서 deleted).
func TestBackup_terminal_ttlExpired_selfDeletes(t *testing.T) {
	pastTime := metav1.NewTime(time.Now().Add(-2 * time.Hour))
	b := freshBackup("b1", "ns", "Valkey", "vk")
	b.Spec.TTL = "1h"
	b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
	b.Status.CompletedAt = &pastTime
	r := backupReconcilerWithFakeClient(b)

	if _, err := r.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: "b1", Namespace: "ns"}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// fake client 에서 delete 호출 후, CR 가 deletionTimestamp 가 set 되거나 finalizer 가 cleanup 후 사라짐.
	// finalizer 가 있으므로 deletionTimestamp 만 set + 다음 reconcile 에서 cleanup.
	got := &cachev1alpha1.ValkeyBackup{}
	if err := r.Get(context.Background(),
		types.NamespacedName{Name: "b1", Namespace: "ns"}, got); err == nil {
		// 존재한다면 deletionTimestamp 가 set 되어야 함.
		if got.DeletionTimestamp == nil {
			t.Fatalf("expected deletionTimestamp set after self-delete")
		}
	}
}

// 3. Terminal + TTL 미명시 → no-op (RequeueAfter=0).
func TestBackup_terminal_noTTL_noop(t *testing.T) {
	now := metav1.Now()
	b := freshBackup("b1", "ns", "Valkey", "vk")
	b.Spec.TTL = "" // 미명시
	b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
	b.Status.CompletedAt = &now
	r := backupReconcilerWithFakeClient(b)

	res, err := r.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: "b1", Namespace: "ns"}})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter != 0 {
		t.Fatalf("expected no requeue (TTL 미명시), got %v", res.RequeueAfter)
	}
}

// 4. Deletion: finalizer cleanup — backup Job + PVC 삭제 (RetainPVC=false).
func TestBackup_deletion_cleansUpPVCAndJobs(t *testing.T) {
	now := metav1.Now()
	b := freshBackup("b1", "ns", "Valkey", "vk")
	b.DeletionTimestamp = &now
	b.Spec.RetainPVC = false
	b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
	b.Status.PVCName = "b1-backup"

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "b1-backup", Namespace: "ns"},
	}
	backupJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: resources.BackupJobName("b1"), Namespace: "ns"},
	}
	uploadJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: resources.UploadJobName("b1"), Namespace: "ns"},
	}
	r := backupReconcilerWithFakeClient(b, pvc, backupJob, uploadJob)

	if _, err := r.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: "b1", Namespace: "ns"}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// PVC 삭제 확인.
	gotPVC := &corev1.PersistentVolumeClaim{}
	err := r.Get(context.Background(), types.NamespacedName{Name: "b1-backup", Namespace: "ns"}, gotPVC)
	if err == nil {
		t.Fatalf("PVC 삭제 안 됨")
	}

	// Job 들 삭제 확인.
	for _, jobName := range []string{resources.BackupJobName("b1"), resources.UploadJobName("b1")} {
		gotJob := &batchv1.Job{}
		err := r.Get(context.Background(), types.NamespacedName{Name: jobName, Namespace: "ns"}, gotJob)
		if err == nil {
			t.Fatalf("%s 삭제 안 됨", jobName)
		}
	}
}

// 5. Deletion: RetainPVC=true → PVC 보존, Job 만 삭제.
func TestBackup_deletion_retainsPVC(t *testing.T) {
	now := metav1.Now()
	b := freshBackup("b1", "ns", "Valkey", "vk")
	b.DeletionTimestamp = &now
	b.Spec.RetainPVC = true
	b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
	b.Status.PVCName = "b1-backup"

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "b1-backup", Namespace: "ns"},
	}
	r := backupReconcilerWithFakeClient(b, pvc)

	if _, err := r.Reconcile(context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: "b1", Namespace: "ns"}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// PVC 보존 확인.
	gotPVC := &corev1.PersistentVolumeClaim{}
	if err := r.Get(context.Background(),
		types.NamespacedName{Name: "b1-backup", Namespace: "ns"}, gotPVC); err != nil {
		t.Fatalf("RetainPVC=true 인데 PVC 가 삭제됨: %v", err)
	}
}
