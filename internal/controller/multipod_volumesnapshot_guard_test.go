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
package controller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// 본 unit 은 isMultiPodTarget 의 fake-client driven 분기 가 아닌, handlePending 진입
// 직전 단계 의 *Source.VolumeSnapshot + multi-pod target* 조합 분기 검증.
//
// PR #67 후: Replication mode 는 phase 1.5 통과 (handleMounting 에서 N PVC 생성),
// ValkeyCluster (sharded) 는 여전히 reject.
//
// envtest 통합 회귀는 별도 (test/e2e 의 cluster_recovery_test 패턴).
func TestHandlePending_VolumeSnapshot_with_ValkeyCluster_target_rejected(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1: %v", err)
	}
	if err := cachev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("cachev1alpha1: %v", err)
	}

	// ValkeyCluster (sharded) — Source.VolumeSnapshot 미지원 (phase 1).
	target := &cachev1alpha1.ValkeyCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "vc-shard", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyClusterSpec{
			Shards:           3,
			ReplicasPerShard: 1,
			Version:          cachev1alpha1.ValkeyVersion{Version: "8.1.6"},
		},
	}
	rest := &cachev1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{Name: "rest-vs-cluster", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "ValkeyCluster", Name: "vc-shard"},
			Source: cachev1alpha1.RestoreSource{
				VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap"},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(target, rest).
		WithStatusSubresource(rest).
		Build()

	r := &ValkeyRestoreReconciler{
		Client:   c,
		Scheme:   scheme,
		Recorder: events.NewFakeRecorder(10),
	}
	_, _ = r.handlePending(testCtx(), rest)

	// rest 의 status 가 Failed phase 로 전이됐는지 검증.
	updated := &cachev1alpha1.ValkeyRestore{}
	if err := c.Get(testCtx(), client.ObjectKeyFromObject(rest), updated); err != nil {
		t.Fatalf("get rest: %v", err)
	}
	if updated.Status.Phase != cachev1alpha1.RestorePhaseFailed {
		t.Errorf("expected Failed phase, got %q", updated.Status.Phase)
	}
	if !strings.Contains(updated.Status.Message, "ValkeyCluster") &&
		!strings.Contains(updated.Status.Message, "shard") {
		t.Errorf("Status.Message should explain ValkeyCluster limitation, got: %q",
			updated.Status.Message)
	}
}

func TestHandlePending_VolumeSnapshot_with_Standalone_target_passes(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1: %v", err)
	}
	if err := cachev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("cachev1alpha1: %v", err)
	}

	target := &cachev1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: "vk-stand", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeySpec{
			Mode:     cachev1alpha1.ModeStandalone,
			Replicas: 1,
		},
	}
	rest := &cachev1alpha1.ValkeyRestore{
		ObjectMeta: metav1.ObjectMeta{Name: "rest-vs-stand", Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyRestoreSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk-stand"},
			Source: cachev1alpha1.RestoreSource{
				VolumeSnapshot: &cachev1alpha1.RestoreSourceVolumeSnapshot{Name: "snap"},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(target, rest).
		WithStatusSubresource(rest).
		Build()

	r := &ValkeyRestoreReconciler{
		Client:   c,
		Scheme:   scheme,
		Recorder: events.NewFakeRecorder(10),
	}
	_, _ = r.handlePending(testCtx(), rest)

	updated := &cachev1alpha1.ValkeyRestore{}
	if err := c.Get(testCtx(), client.ObjectKeyFromObject(rest), updated); err != nil {
		t.Fatalf("get rest: %v", err)
	}
	// Standalone + VolumeSnapshot → Mounting 으로 전이 (Failed 아님).
	if updated.Status.Phase == cachev1alpha1.RestorePhaseFailed {
		t.Errorf("Standalone + VolumeSnapshot should not Failed, got Failed: %q",
			updated.Status.Message)
	}
}
