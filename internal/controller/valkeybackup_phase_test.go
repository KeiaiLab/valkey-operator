/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// ValkeyBackup phase 전이 단위테스트 — Reconcile() 직접 호출.
package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// fakeBackupReconciler — fake client 에 backup 1건 + (선택) 대상 cluster.
func fakeBackupReconciler(b *cachev1alpha1.ValkeyBackup, target client.Object) *ValkeyBackupReconciler {
	scheme := runtime.NewScheme()
	_ = cachev1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	objs := []client.Object{b}
	if target != nil {
		objs = append(objs, target)
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&cachev1alpha1.ValkeyBackup{}).
		Build()
	return &ValkeyBackupReconciler{Client: c, Scheme: scheme}
}

// freshBackup — finalizer 포함. 신규 CR (finalizer 없음) 시뮬레이션은
// freshBackupBare.
func freshBackup(name, namespace, kind, target string) *cachev1alpha1.ValkeyBackup {
	return &cachev1alpha1.ValkeyBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: namespace, Generation: 1,
			Finalizers: []string{finalizerValkeyBackup},
		},
		Spec: cachev1alpha1.ValkeyBackupSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: kind, Name: target},
			Type:       cachev1alpha1.BackupTypeRDB,
		},
	}
}

func TestBackup_phase_transition_pending_inProgress_completed(t *testing.T) {
	cluster := &cachev1alpha1.ValkeyCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "vk", Namespace: "ns"},
	}
	b := freshBackup("b1", "ns", "ValkeyCluster", "vk")
	r := fakeBackupReconciler(b, cluster)
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "b1", Namespace: "ns"}}

	// 1차 reconcile: "" → Pending.
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	got := &cachev1alpha1.ValkeyBackup{}
	if err := r.Get(ctx, req.NamespacedName, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != cachev1alpha1.BackupPhasePending {
		t.Errorf("phase 1: %s", got.Status.Phase)
	}

	// 2차: Pending → BGSAVE 발행 시도 → 테스트 환경 (실 valkey 미연결) 에서 dial
	// 실패 → Failed phase. M3 구현 후 본 unit test 는 *실패 경로* 를 검증.
	// (정상 BGSAVE → InProgress → LASTSAVE 폴링 → Completed 흐름은 envtest /
	// integration 으로 검증 — 별도 PR.)
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	_ = r.Get(ctx, req.NamespacedName, got)
	if got.Status.Phase != cachev1alpha1.BackupPhaseFailed {
		t.Errorf("phase 2: want Failed (no valkey to dial), got %s", got.Status.Phase)
	}
	if got.Status.CompletedAt == nil {
		t.Error("CompletedAt should be set after Failed")
	}
	if !got.IsTerminal() {
		t.Error("IsTerminal should be true after Failed")
	}

	// 4차 (terminal): no-op.
	res, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile 4 (terminal): %v", err)
	}
	if res.RequeueAfter != 0 {
		t.Errorf("terminal phase should not requeue: %v", res)
	}
}

func TestBackup_targetNotFound_marksFailed(t *testing.T) {
	b := freshBackup("b2", "ns", "ValkeyCluster", "missing")
	r := fakeBackupReconciler(b, nil) // 대상 ValkeyCluster 미존재.
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "b2", Namespace: "ns"}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := &cachev1alpha1.ValkeyBackup{}
	if err := r.Get(ctx, req.NamespacedName, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != cachev1alpha1.BackupPhaseFailed {
		t.Errorf("phase: %s want Failed", got.Status.Phase)
	}
	c := findCondition(got.Status.Conditions, "Ready")
	if c == nil || c.Reason != "TargetNotFound" {
		t.Errorf("Ready condition: %+v", c)
	}
}

func TestBackup_unsupportedKind(t *testing.T) {
	b := freshBackup("b3", "ns", "WrongKind", "x")
	r := fakeBackupReconciler(b, nil)
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "b3", Namespace: "ns"}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := &cachev1alpha1.ValkeyBackup{}
	_ = r.Get(ctx, req.NamespacedName, got)
	if got.Status.Phase != cachev1alpha1.BackupPhaseFailed {
		t.Errorf("phase: %s want Failed", got.Status.Phase)
	}
}

func TestBackup_buildJob_ValkeyClusterUsesPerShardPrimaryHosts(t *testing.T) {
	cluster := &cachev1alpha1.ValkeyCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "vk", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyClusterSpec{
			Shards:           3,
			ReplicasPerShard: 1,
		},
		Status: cachev1alpha1.ValkeyClusterStatus{
			Shards: []cachev1alpha1.ShardStatus{
				{Index: 0, PrimaryPod: "vk-0"},
				{Index: 1, PrimaryPod: "vk-4"},
				{Index: 2, PrimaryPod: "vk-2"},
			},
		},
	}
	b := freshBackup("b4", "ns", "ValkeyCluster", "vk")
	r := fakeBackupReconciler(b, cluster)

	job, err := r.buildBackupJob(context.Background(), b, "backup-pvc")
	if err != nil {
		t.Fatalf("buildBackupJob: %v", err)
	}
	shCmd := job.Spec.Template.Spec.Containers[0].Command[2]
	for i, pod := range []string{"vk-0", "vk-4", "vk-2"} {
		host := fmt.Sprintf("%s.vk-headless.ns.svc", pod)
		if !strings.Contains(shCmd, host) {
			t.Fatalf("shard %d primary host 누락: %s", i, shCmd)
		}
		path := fmt.Sprintf("/backup/shard-%d/dump.rdb", i)
		if !strings.Contains(shCmd, path) {
			t.Fatalf("shard %d RDB path 누락: %s", i, shCmd)
		}
	}
}

// IsTerminal — Completed / Failed.
func TestBackup_IsTerminal(t *testing.T) {
	cases := map[cachev1alpha1.BackupPhase]bool{
		"":                                  false,
		cachev1alpha1.BackupPhasePending:    false,
		cachev1alpha1.BackupPhaseInProgress: false,
		cachev1alpha1.BackupPhaseCompleted:  true,
		cachev1alpha1.BackupPhaseFailed:     true,
	}
	for phase, want := range cases {
		b := &cachev1alpha1.ValkeyBackup{}
		b.Status.Phase = phase
		if got := b.IsTerminal(); got != want {
			t.Errorf("phase=%q IsTerminal=%v want %v", phase, got, want)
		}
	}
}
