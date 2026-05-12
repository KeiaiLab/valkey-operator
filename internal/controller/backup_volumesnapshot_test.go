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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

func volumeSnapshotScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := cachev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	return scheme
}

func backupOfTypeVS(name string) *cachev1alpha1.ValkeyBackup {
	return &cachev1alpha1.ValkeyBackup{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Spec: cachev1alpha1.ValkeyBackupSpec{
			ClusterRef: cachev1alpha1.ClusterReference{Kind: "Valkey", Name: "vk"},
			Type:       cachev1alpha1.BackupTypeVolumeSnapshot,
		},
	}
}

func snapshotCR(backupName, namespace string, status map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(resources.VolumeSnapshotGVK)
	u.SetName(resources.VolumeSnapshotName(backupName))
	u.SetNamespace(namespace)
	if status != nil {
		u.Object["status"] = status
	}
	return u
}

func TestPollVolumeSnapshotReady_no_status_returns_not_ready(t *testing.T) {
	b := backupOfTypeVS("vk-snap")
	c := fake.NewClientBuilder().WithScheme(volumeSnapshotScheme(t)).
		WithObjects(snapshotCR("vk-snap", "ns", nil)).Build()

	ready, err := pollVolumeSnapshotReady(testCtx(), c, b)
	if err != nil {
		t.Fatalf("no status should not be err: %v", err)
	}
	if ready {
		t.Error("no status should not be ready")
	}
}

func TestPollVolumeSnapshotReady_readyToUse_true(t *testing.T) {
	b := backupOfTypeVS("vk-snap")
	c := fake.NewClientBuilder().WithScheme(volumeSnapshotScheme(t)).
		WithObjects(snapshotCR("vk-snap", "ns", map[string]any{
			"readyToUse": true,
		})).Build()

	ready, err := pollVolumeSnapshotReady(testCtx(), c, b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ready {
		t.Error("readyToUse=true should be ready")
	}
}

func TestPollVolumeSnapshotReady_readyToUse_false(t *testing.T) {
	b := backupOfTypeVS("vk-snap")
	c := fake.NewClientBuilder().WithScheme(volumeSnapshotScheme(t)).
		WithObjects(snapshotCR("vk-snap", "ns", map[string]any{
			"readyToUse": false,
		})).Build()

	ready, err := pollVolumeSnapshotReady(testCtx(), c, b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ready {
		t.Error("readyToUse=false should not be ready")
	}
}

func TestPollVolumeSnapshotReady_error_status_propagates(t *testing.T) {
	b := backupOfTypeVS("vk-snap")
	c := fake.NewClientBuilder().WithScheme(volumeSnapshotScheme(t)).
		WithObjects(snapshotCR("vk-snap", "ns", map[string]any{
			"readyToUse": false,
			"error": map[string]any{
				"message": "CSI driver internal error: snap creation failed",
			},
		})).Build()

	_, err := pollVolumeSnapshotReady(testCtx(), c, b)
	if err == nil {
		t.Fatal("expected fatal error from status.error")
	}
}

func TestPollVolumeSnapshotReady_not_found_returns_err(t *testing.T) {
	b := backupOfTypeVS("vk-snap-missing")
	c := fake.NewClientBuilder().WithScheme(volumeSnapshotScheme(t)).Build()

	_, err := pollVolumeSnapshotReady(testCtx(), c, b)
	if err == nil {
		t.Fatal("expected error when VolumeSnapshot CR missing")
	}
}

func TestApplyVolumeSnapshotForBackup_creates_when_absent(t *testing.T) {
	b := backupOfTypeVS("vk-snap")
	c := fake.NewClientBuilder().WithScheme(volumeSnapshotScheme(t)).Build()

	if err := applyVolumeSnapshotForBackup(testCtx(), c, b); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// 검증: VolumeSnapshot CR 가 cluster 에 생성됐는지.
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(resources.VolumeSnapshotGVK)
	if err := c.Get(testCtx(), client.ObjectKey{Namespace: "ns", Name: "vk-snap-snap"}, got); err != nil {
		t.Fatalf("get after apply: %v", err)
	}
	if got.GetName() != "vk-snap-snap" {
		t.Errorf("created VolumeSnapshot name: %q", got.GetName())
	}
}

func TestApplyVolumeSnapshotForBackup_idempotent_when_exists(t *testing.T) {
	b := backupOfTypeVS("vk-snap")
	existing := snapshotCR("vk-snap", "ns", map[string]any{"readyToUse": true})
	c := fake.NewClientBuilder().WithScheme(volumeSnapshotScheme(t)).
		WithObjects(existing).Build()

	// VolumeSnapshot 은 immutable — apply 두 번 호출해도 에러 없어야.
	if err := applyVolumeSnapshotForBackup(testCtx(), c, b); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := applyVolumeSnapshotForBackup(testCtx(), c, b); err != nil {
		t.Fatalf("second apply (idempotent): %v", err)
	}
}
