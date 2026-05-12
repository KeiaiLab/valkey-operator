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

ValkeyRestore Reconcile + phase 전이 단위 테스트 — fake client.
ADR-0015. Source.PVC + Standalone Valkey 만 첫 commit 범위.
*/

package controller

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
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

func fakeRestoreReconciler(rest *cachev1alpha1.ValkeyRestore, others ...client.Object) *ValkeyRestoreReconciler {
	scheme := runtime.NewScheme()
	_ = cachev1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	objs := make([]client.Object, 0, 1+len(others))
	objs = append(objs, rest)
	objs = append(objs, others...)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&cachev1alpha1.ValkeyRestore{}).
		Build()
	return &ValkeyRestoreReconciler{Client: c, Scheme: scheme}
}

// freshRestoreCR — finalizer 포함 (Phase 가 이미 진행된 CR 은 finalizer 도 있음).
func freshRestoreCR(name, ns, target string) *cachev1alpha1.ValkeyRestore {
	return &cachev1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns, Generation: 1,
			Finalizers: []string{finalizerValkeyRestore},
		},
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: target},
			Source: cachev1alpha1.RestoreSource{
				PVC: &cachev1alpha1.RestoreSourcePVC{Name: "backup-pvc"},
			},
			RestoreType: cachev1alpha1.RestoreTypeRDB,
		},
	}
}

// bareRestoreCR — finalizer 없음. 신규 CR 시뮬레이션.
func bareRestoreCR(name, ns, target string) *cachev1alpha1.ValkeyRestore {
	r := freshRestoreCR(name, ns, target)
	r.Finalizers = nil
	return r
}

func standaloneValkey(name, ns string) *cachev1alpha1.Valkey {
	return &cachev1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: cachev1alpha1.ValkeySpec{
			Mode:     cachev1alpha1.ModeStandalone,
			Replicas: 1,
		},
	}
}

func sourcePVC(name, ns string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
	}
}

func standaloneSTS(name, ns string, ready int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes:    []corev1.Volume{{Name: "data"}},
					Containers: []corev1.Container{{Name: "valkey"}},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      1,
			ReadyReplicas: ready,
		},
	}
}

func reqFor(name, ns string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: ns}}
}

func reloadRestore(t *testing.T, r *ValkeyRestoreReconciler, name, ns string) *cachev1alpha1.ValkeyRestore {
	t.Helper()
	got := &cachev1alpha1.ValkeyRestore{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, got); err != nil {
		t.Fatalf("get restore: %v", err)
	}
	return got
}

// 1. "" → Pending — finalizer 추가 + status 초기화.
func TestRestore_transitionPending(t *testing.T) {
	rest := bareRestoreCR("r1", "ns", "vk")
	r := fakeRestoreReconciler(rest)

	// 1차 reconcile — finalizer 추가 후 명시적 requeue.
	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if !containsStr(got.Finalizers, finalizerValkeyRestore) {
		t.Fatalf("finalizer not added")
	}

	// 2차 reconcile — Phase=Pending.
	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	got = reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhasePending {
		t.Fatalf("expected Phase=Pending, got %s", got.Status.Phase)
	}
}

// 2. Pending → Mounting (정상 흐름).
func TestRestore_pendingToMounting(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseMounting {
		t.Fatalf("expected Mounting, got %s", got.Status.Phase)
	}
}

// 3. Pending: ClusterRef Valkey not found → Failed.
func TestRestore_pendingTargetNotFound(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk-missing")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	r := fakeRestoreReconciler(rest)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
}

// 4. Pending: Replication mode + Source.PVC RWO → Failed (SourcePVCNotROX).
func TestRestore_pendingReplicationRequiresROXSource(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	v := standaloneValkey("vk", "ns")
	v.Spec.Mode = cachev1alpha1.ModeReplication
	v.Spec.Replicas = 3
	rwoSourcePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-pvc", Namespace: "ns"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}
	r := fakeRestoreReconciler(rest, v, rwoSourcePVC)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed (RWO not allowed for Replication), got %s", got.Status.Phase)
	}
}

// 4b. Pending: Replication mode + Source.PVC ROX → Mounting OK.
func TestRestore_pendingReplicationROXSource_OK(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	v := standaloneValkey("vk", "ns")
	v.Spec.Mode = cachev1alpha1.ModeReplication
	v.Spec.Replicas = 3
	roxSourcePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-pvc", Namespace: "ns"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
		},
	}
	r := fakeRestoreReconciler(rest, v, roxSourcePVC)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseMounting {
		t.Fatalf("expected Mounting (ROX OK), got %s", got.Status.Phase)
	}
}

// 4c. Pending: Replication + Source.TargetRef + SourcePVCAccessMode 미명시 → Failed.
func TestRestore_pendingReplicationTargetRef_requiresROXOption(t *testing.T) {
	rest := freshTargetRefRestore("r1", "ns", "vk", "tgt", "dump.rdb")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	v := standaloneValkey("vk", "ns")
	v.Spec.Mode = cachev1alpha1.ModeReplication
	v.Spec.Replicas = 3
	r := fakeRestoreReconciler(rest, v)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
}

// 4d. Pending: Replication + Source.TargetRef + SourcePVCAccessMode=ROX → Mounting.
func TestRestore_pendingReplicationTargetRefROX_OK(t *testing.T) {
	rest := freshTargetRefRestore("r1", "ns", "vk", "tgt", "dump.rdb")
	rest.Spec.SourcePVCAccessMode = cachev1alpha1.SourcePVCAccessModeROX
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	v := standaloneValkey("vk", "ns")
	v.Spec.Mode = cachev1alpha1.ModeReplication
	v.Spec.Replicas = 3
	r := fakeRestoreReconciler(rest, v)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseMounting {
		t.Fatalf("expected Mounting, got %s", got.Status.Phase)
	}
}

// 5. Pending: Source.PVC nil → Failed.
func TestRestore_pendingMissingSource(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	rest.Spec.Source.PVC = nil
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
}

// 6. Mounting → Restoring + paused annotation set.
func TestRestore_mountingToRestoring_setsPaused(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhaseMounting
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"), sourcePVC("backup-pvc", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseRestoring {
		t.Fatalf("expected Restoring, got %s", got.Status.Phase)
	}
	v := &cachev1alpha1.Valkey{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vk", Namespace: "ns"}, v); err != nil {
		t.Fatalf("get valkey: %v", err)
	}
	if v.Annotations[PausedAnnotation] != "true" {
		t.Fatalf("expected paused annotation set, got %v", v.Annotations)
	}
}

func TestRestore_mountingValkeyCluster_setsPaused(t *testing.T) {
	rest := freshClusterRestoreCR("r1", "ns", "vc")
	rest.Status.Phase = cachev1alpha1.RestorePhaseMounting
	vc := standaloneCluster("vc", "ns", 3, 1)
	r := fakeRestoreReconciler(rest, vc, sourcePVC("backup-pvc", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseRestoring {
		t.Fatalf("expected Restoring, got %s", got.Status.Phase)
	}
	gotVC := &cachev1alpha1.ValkeyCluster{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vc", Namespace: "ns"}, gotVC); err != nil {
		t.Fatalf("get valkeycluster: %v", err)
	}
	if gotVC.Annotations[PausedAnnotation] != "true" {
		t.Fatalf("expected paused annotation set, got %v", gotVC.Annotations)
	}
}

// 7. Mounting: source PVC 부재 → Failed.
func TestRestore_mountingPVCNotFound(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhaseMounting
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
}

// 8. Restoring: STS 에 init container inject + re-queue (Phase 그대로 유지).
func TestRestore_restoring_injectsInitContainer(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhaseRestoring
	sts := standaloneSTS("vk", "ns", 0)
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"), sourcePVC("backup-pvc", "ns"), sts)

	res, err := r.Reconcile(context.Background(), reqFor("r1", "ns"))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatalf("expected requeue while STS rolling")
	}
	got := &appsv1.StatefulSet{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vk", Namespace: "ns"}, got); err != nil {
		t.Fatalf("get sts: %v", err)
	}
	hasInit := false
	for _, c := range got.Spec.Template.Spec.InitContainers {
		if c.Name == resources.RestoreInitContainerName {
			hasInit = true
		}
	}
	if !hasInit {
		t.Fatalf("expected restore init container injected")
	}
}

// 9. Restoring: STS pods all ready → Verifying.
func TestRestore_restoring_allReady_toVerifying(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhaseRestoring
	// STS 가 *이미* init container patch 됐다고 가정 (직접 injection).
	sts := standaloneSTS("vk", "ns", 1) // ready=1
	resources.InjectRestoreIntoPodSpec(&sts.Spec.Template.Spec, "dump.rdb", "backup-pvc")
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"), sourcePVC("backup-pvc", "ns"), sts)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseVerifying {
		t.Fatalf("expected Verifying, got %s", got.Status.Phase)
	}
}

// 9b. Restoring: RDB load 실패 등으로 valkey 컨테이너가 CrashLoopBackOff 이면 Failed.
func TestRestore_restoring_valkeyCrashLoop_marksFailed(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhaseRestoring
	sts := standaloneSTS("vk", "ns", 0)
	resources.InjectRestoreIntoPodSpec(&sts.Spec.Template.Spec, "dump.rdb", "backup-pvc")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vk-0",
			Namespace: "ns",
			Labels:    resources.SelectorLabels("vk"),
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "valkey",
				RestartCount: 3,
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "CrashLoopBackOff",
						Message: "back-off restarting failed container valkey",
					},
				},
			}},
		},
	}
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"), sourcePVC("backup-pvc", "ns"), sts, pod)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
	if !strings.Contains(got.Status.Message, "CrashLoopBackOff") {
		t.Fatalf("expected CrashLoopBackOff message, got %q", got.Status.Message)
	}
}

// 10. Verifying: init container 제거 + paused 해제 + Completed.
func TestRestore_verifying_revertsAndCompletes(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhaseVerifying
	sts := standaloneSTS("vk", "ns", 1)
	resources.InjectRestoreIntoPodSpec(&sts.Spec.Template.Spec, "dump.rdb", "backup-pvc")
	v := standaloneValkey("vk", "ns")
	v.Annotations = map[string]string{PausedAnnotation: "true"}
	r := fakeRestoreReconciler(rest, v, sts)

	// 1차 — STS 의 init container 제거 + re-queue.
	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	gotSTS := &appsv1.StatefulSet{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vk", Namespace: "ns"}, gotSTS); err != nil {
		t.Fatalf("get sts: %v", err)
	}
	for _, c := range gotSTS.Spec.Template.Spec.InitContainers {
		if c.Name == resources.RestoreInitContainerName {
			t.Fatalf("init container 가 제거되지 않음: %v", gotSTS.Spec.Template.Spec.InitContainers)
		}
	}

	// 2차 — paused 해제 + Phase=Completed.
	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseCompleted {
		t.Fatalf("expected Completed, got %s", got.Status.Phase)
	}
	gotV := &cachev1alpha1.Valkey{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vk", Namespace: "ns"}, gotV); err != nil {
		t.Fatalf("get valkey: %v", err)
	}
	if gotV.Annotations[PausedAnnotation] == "true" {
		t.Fatalf("expected paused annotation removed")
	}
}

func TestRestore_verifyingValkeyCluster_revertsAndCompletes(t *testing.T) {
	rest := freshClusterRestoreCR("r1", "ns", "vc")
	rest.Status.Phase = cachev1alpha1.RestorePhaseVerifying
	sts := standaloneSTS("vc", "ns", 1)
	resources.InjectRestoreIntoPodSpecForCluster(&sts.Spec.Template.Spec, 3, nil, "backup-pvc")
	vc := standaloneCluster("vc", "ns", 3, 1)
	vc.Annotations = map[string]string{PausedAnnotation: "true"}
	r := fakeRestoreReconciler(rest, vc, sts)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseCompleted {
		t.Fatalf("expected Completed, got %s", got.Status.Phase)
	}
	gotVC := &cachev1alpha1.ValkeyCluster{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vc", Namespace: "ns"}, gotVC); err != nil {
		t.Fatalf("get valkeycluster: %v", err)
	}
	if gotVC.Annotations[PausedAnnotation] == "true" {
		t.Fatalf("expected paused annotation removed")
	}
}

// === Source.TargetRef 시나리오 ===

func validBackupTarget(name, ns string) *cachev1alpha1.ValkeyBackupTarget {
	return &cachev1alpha1.ValkeyBackupTarget{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: cachev1alpha1.ValkeyBackupTargetSpec{
			Type: cachev1alpha1.BackupTargetTypeS3,
			S3: &cachev1alpha1.S3Spec{
				Endpoint: "https://s3.fake",
				Region:   "us-east-1",
				Bucket:   "b",
				CredentialsSecretRef: cachev1alpha1.S3CredentialsSecretRef{
					Name: "creds",
				},
			},
		},
		Status: cachev1alpha1.ValkeyBackupTargetStatus{
			Phase: cachev1alpha1.BackupTargetPhaseReachable,
		},
	}
}

func freshTargetRefRestore(name, ns, target, refTarget, path string) *cachev1alpha1.ValkeyRestore {
	r := freshRestoreCR(name, ns, target)
	r.Spec.Source = cachev1alpha1.RestoreSource{
		TargetRef: &cachev1alpha1.RestoreSourceTargetRef{Name: refTarget, Path: path},
	}
	return r
}

// 12. Pending: Source.PVC + Source.TargetRef 동시 → AmbiguousSource.
func TestRestore_pending_ambiguousSource(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	rest.Spec.Source.TargetRef = &cachev1alpha1.RestoreSourceTargetRef{
		Name: "tgt", Path: "p",
	}
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
}

// 13. Pending: Source 둘 다 nil → MissingSource.
func TestRestore_pending_missingSource(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	rest.Spec.Source = cachev1alpha1.RestoreSource{}
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
}

// 14. Pending: Source.TargetRef + missing Path → MissingTargetRefPath.
func TestRestore_pending_missingTargetRefPath(t *testing.T) {
	rest := freshTargetRefRestore("r1", "ns", "vk", "tgt", "")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
}

// 15. Pending: Source.TargetRef 정상 → Mounting.
func TestRestore_pendingTargetRef_toMounting(t *testing.T) {
	rest := freshTargetRefRestore("r1", "ns", "vk", "tgt", "dump.rdb")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"))

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseMounting {
		t.Fatalf("expected Mounting, got %s", got.Status.Phase)
	}
}

// 16. Mounting: ValkeyBackupTarget not Reachable → 15s requeue (Phase 그대로).
func TestRestore_mountingTargetRef_targetNotReachable(t *testing.T) {
	rest := freshTargetRefRestore("r1", "ns", "vk", "tgt", "dump.rdb")
	rest.Status.Phase = cachev1alpha1.RestorePhaseMounting
	tgt := validBackupTarget("tgt", "ns")
	tgt.Status.Phase = cachev1alpha1.BackupTargetPhasePending
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"), tgt)

	res, err := r.Reconcile(context.Background(), reqFor("r1", "ns"))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter != 15*time.Second {
		t.Fatalf("expected 15s requeue (target not reachable), got %v", res)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseMounting {
		t.Fatalf("expected Phase=Mounting (still waiting), got %s", got.Status.Phase)
	}
}

// 17. Mounting: TargetRef + Reachable → 임시 PVC + Download Job 생성.
func TestRestore_mountingTargetRef_createsPVCAndJob(t *testing.T) {
	rest := freshTargetRefRestore("r1", "ns", "vk", "tgt", "dump.rdb")
	rest.Status.Phase = cachev1alpha1.RestorePhaseMounting
	tgt := validBackupTarget("tgt", "ns")
	r := fakeRestoreReconciler(rest, standaloneValkey("vk", "ns"), tgt)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// 임시 PVC 생성 확인.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(context.Background(),
		types.NamespacedName{Name: "r1-source", Namespace: "ns"}, pvc); err != nil {
		t.Fatalf("expected r1-source PVC created, got: %v", err)
	}
}

// === ValkeyCluster mode 시나리오 ===

func standaloneCluster(name, ns string, shards, repsPerShard int32) *cachev1alpha1.ValkeyCluster {
	return &cachev1alpha1.ValkeyCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: cachev1alpha1.ValkeyClusterSpec{
			Shards:           shards,
			ReplicasPerShard: repsPerShard,
		},
	}
}

// freshClusterRestoreCR — Kind=ValkeyCluster CR (Source.PVC + ShardLayout).
func freshClusterRestoreCR(name, ns, target string) *cachev1alpha1.ValkeyRestore {
	r := freshRestoreCR(name, ns, target)
	r.Spec.ClusterRef.Kind = "ValkeyCluster"
	return r
}

// 18. Pending: Kind=ValkeyCluster + Source.PVC ROX → Mounting.
func TestRestore_pendingValkeyClusterROXSource_OK(t *testing.T) {
	rest := freshClusterRestoreCR("r1", "ns", "vc")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	vc := standaloneCluster("vc", "ns", 3, 1)
	roxPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-pvc", Namespace: "ns"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
		},
	}
	r := fakeRestoreReconciler(rest, vc, roxPVC)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseMounting {
		t.Fatalf("expected Mounting, got %s", got.Status.Phase)
	}
}

func TestRestore_pendingValkeyClusterRWXSource_OK(t *testing.T) {
	rest := freshClusterRestoreCR("r1", "ns", "vc")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	vc := standaloneCluster("vc", "ns", 3, 1)
	rwxPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-pvc", Namespace: "ns"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
		},
	}
	r := fakeRestoreReconciler(rest, vc, rwxPVC)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseMounting {
		t.Fatalf("expected Mounting, got %s", got.Status.Phase)
	}
}

// 19. Pending: Kind=ValkeyCluster + Source.PVC RWO → Failed (ROX 강제).
func TestRestore_pendingValkeyClusterRWO_Fails(t *testing.T) {
	rest := freshClusterRestoreCR("r1", "ns", "vc")
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	vc := standaloneCluster("vc", "ns", 3, 1)
	rwoPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-pvc", Namespace: "ns"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}
	r := fakeRestoreReconciler(rest, vc, rwoPVC)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed (RWO not allowed for ValkeyCluster), got %s", got.Status.Phase)
	}
}

// 20. Pending: Kind=Unknown → Failed.
func TestRestore_pendingUnknownKind(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vc")
	rest.Spec.ClusterRef.Kind = "UnknownKind"
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	r := fakeRestoreReconciler(rest)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := reloadRestore(t, r, "r1", "ns")
	if got.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Fatalf("expected Failed, got %s", got.Status.Phase)
	}
}

// 21. parseShardLayout: 다양한 key 형식 인식.
func TestParseShardLayout_keyFormats(t *testing.T) {
	in := map[string]string{
		"0":       "a/dump.rdb",
		"shard-1": "b/dump.rdb",
		"shard2":  "c/dump.rdb",
		"foo":     "ignored",
	}
	out := parseShardLayout(in)
	if got, want := len(out), 3; got != want {
		t.Fatalf("expected %d valid entries, got %d (out=%v)", want, got, out)
	}
	if out[0] != "a/dump.rdb" || out[1] != "b/dump.rdb" || out[2] != "c/dump.rdb" {
		t.Fatalf("parsed map mismatch: %v", out)
	}
}

// 11. Deletion: finalizer cleanup — STS 원복 + paused 제거 (best-effort).
func TestRestore_deletion_cleansUp(t *testing.T) {
	rest := freshRestoreCR("r1", "ns", "vk")
	now := metav1.Now()
	rest.DeletionTimestamp = &now
	rest.Finalizers = []string{finalizerValkeyRestore}
	rest.Status.Phase = cachev1alpha1.RestorePhaseRestoring
	sts := standaloneSTS("vk", "ns", 1)
	resources.InjectRestoreIntoPodSpec(&sts.Spec.Template.Spec, "dump.rdb", "backup-pvc")
	v := standaloneValkey("vk", "ns")
	v.Annotations = map[string]string{PausedAnnotation: "true"}
	r := fakeRestoreReconciler(rest, v, sts)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile deletion: %v", err)
	}
	gotSTS := &appsv1.StatefulSet{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vk", Namespace: "ns"}, gotSTS); err != nil {
		t.Fatalf("get sts: %v", err)
	}
	for _, c := range gotSTS.Spec.Template.Spec.InitContainers {
		if c.Name == resources.RestoreInitContainerName {
			t.Fatalf("STS 원복 안 됨")
		}
	}
	gotV := &cachev1alpha1.Valkey{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vk", Namespace: "ns"}, gotV); err != nil {
		t.Fatalf("get valkey: %v", err)
	}
	if gotV.Annotations[PausedAnnotation] == "true" {
		t.Fatalf("paused 해제 안 됨")
	}
}

func TestRestore_deletionValkeyCluster_cleansUp(t *testing.T) {
	rest := freshClusterRestoreCR("r1", "ns", "vc")
	now := metav1.Now()
	rest.DeletionTimestamp = &now
	rest.Finalizers = []string{finalizerValkeyRestore}
	rest.Status.Phase = cachev1alpha1.RestorePhaseRestoring
	sts := standaloneSTS("vc", "ns", 1)
	resources.InjectRestoreIntoPodSpecForCluster(&sts.Spec.Template.Spec, 3, nil, "backup-pvc")
	vc := standaloneCluster("vc", "ns", 3, 1)
	vc.Annotations = map[string]string{PausedAnnotation: "true"}
	r := fakeRestoreReconciler(rest, vc, sts)

	if _, err := r.Reconcile(context.Background(), reqFor("r1", "ns")); err != nil {
		t.Fatalf("reconcile deletion: %v", err)
	}
	gotVC := &cachev1alpha1.ValkeyCluster{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "vc", Namespace: "ns"}, gotVC); err != nil {
		t.Fatalf("get valkeycluster: %v", err)
	}
	if gotVC.Annotations[PausedAnnotation] == "true" {
		t.Fatalf("paused 해제 안 됨")
	}
}

// containsStr — 슬라이스 안에 s 가 있는지.
func containsStr(slice []string, s string) bool {
	return slices.Contains(slice, s)
}
