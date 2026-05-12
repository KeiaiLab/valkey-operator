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

ValkeyRestore Init container + Source volume + Inject/Remove 단위 테스트.
*/

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestBuildRestoreInitContainer_basic(t *testing.T) {
	c := BuildRestoreInitContainer("dump.rdb")
	if c.Name != RestoreInitContainerName {
		t.Fatalf("expected name=%s, got %s", RestoreInitContainerName, c.Name)
	}
	if c.Image != RestoreInitImage {
		t.Fatalf("expected image=%s, got %s", RestoreInitImage, c.Image)
	}
	if len(c.VolumeMounts) != 2 {
		t.Fatalf("expected 2 volumeMounts (data + source), got %d", len(c.VolumeMounts))
	}
	// /restore/dump.rdb → /data/dump.rdb 가 cp command 에 포함.
	cmd := c.Command[2]
	if !contains(cmd, "/restore/dump.rdb") || !contains(cmd, "/data/dump.rdb") {
		t.Fatalf("expected cmd to copy /restore/dump.rdb → /data/dump.rdb, got: %s", cmd)
	}
}

func TestBuildRestoreSourceVolume_readonly(t *testing.T) {
	v := BuildRestoreSourceVolume("backup-pvc")
	if v.Name != RestoreSourceVolumeName {
		t.Fatalf("expected name=%s", RestoreSourceVolumeName)
	}
	if v.PersistentVolumeClaim == nil {
		t.Fatalf("expected PVC volume source")
	}
	if v.PersistentVolumeClaim.ClaimName != "backup-pvc" {
		t.Fatalf("expected claimName=backup-pvc, got %s", v.PersistentVolumeClaim.ClaimName)
	}
	if !v.PersistentVolumeClaim.ReadOnly {
		t.Fatalf("expected source PVC mounted ReadOnly")
	}
}

func TestInjectRestoreIntoPodSpec_idempotent(t *testing.T) {
	pod := &corev1.PodSpec{}
	InjectRestoreIntoPodSpec(pod, "dump.rdb", "backup-pvc")
	if got := len(pod.InitContainers); got != 1 {
		t.Fatalf("first inject — expected 1 init container, got %d", got)
	}
	if got := len(pod.Volumes); got != 1 {
		t.Fatalf("first inject — expected 1 volume, got %d", got)
	}

	// 두 번째 inject — replace, append 아님.
	InjectRestoreIntoPodSpec(pod, "dump.rdb", "backup-pvc")
	if got := len(pod.InitContainers); got != 1 {
		t.Fatalf("re-inject — still 1 init container, got %d", got)
	}
	if got := len(pod.Volumes); got != 1 {
		t.Fatalf("re-inject — still 1 volume, got %d", got)
	}
}

func TestInjectRestoreIntoPodSpec_preservesExisting(t *testing.T) {
	pod := &corev1.PodSpec{
		InitContainers: []corev1.Container{{Name: "existing-init"}},
		Volumes:        []corev1.Volume{{Name: "data"}},
	}
	InjectRestoreIntoPodSpec(pod, "dump.rdb", "backup-pvc")
	if got := len(pod.InitContainers); got != 2 {
		t.Fatalf("expected 2 init containers (existing + restore), got %d", got)
	}
	if got := len(pod.Volumes); got != 2 {
		t.Fatalf("expected 2 volumes (data + source), got %d", got)
	}
	if pod.InitContainers[0].Name != "existing-init" {
		t.Fatalf("existing init container 가 손상됨: %v", pod.InitContainers)
	}
}

func TestRemoveRestoreFromPodSpec_basic(t *testing.T) {
	pod := &corev1.PodSpec{}
	InjectRestoreIntoPodSpec(pod, "dump.rdb", "backup-pvc")
	RemoveRestoreFromPodSpec(pod)
	if len(pod.InitContainers) != 0 {
		t.Fatalf("expected 0 init containers after remove, got %d", len(pod.InitContainers))
	}
	if len(pod.Volumes) != 0 {
		t.Fatalf("expected 0 volumes after remove, got %d", len(pod.Volumes))
	}
}

func TestRemoveRestoreFromPodSpec_preservesOthers(t *testing.T) {
	pod := &corev1.PodSpec{
		InitContainers: []corev1.Container{{Name: "existing-init"}},
		Volumes:        []corev1.Volume{{Name: "data"}},
	}
	InjectRestoreIntoPodSpec(pod, "dump.rdb", "backup-pvc")
	RemoveRestoreFromPodSpec(pod)
	if got := len(pod.InitContainers); got != 1 || pod.InitContainers[0].Name != "existing-init" {
		t.Fatalf("expected only existing-init left, got %v", pod.InitContainers)
	}
	if got := len(pod.Volumes); got != 1 || pod.Volumes[0].Name != "data" {
		t.Fatalf("expected only data volume left, got %v", pod.Volumes)
	}
}

func TestRemoveRestoreFromPodSpec_noopWhenAbsent(t *testing.T) {
	pod := &corev1.PodSpec{
		InitContainers: []corev1.Container{{Name: "existing-init"}},
	}
	RemoveRestoreFromPodSpec(pod)
	if len(pod.InitContainers) != 1 {
		t.Fatalf("remove on absent — should be no-op, got %d", len(pod.InitContainers))
	}
}

// === ValkeyCluster init container 테스트 ===

func TestBuildRestoreInitContainerForCluster_defaultLayout(t *testing.T) {
	c := BuildRestoreInitContainerForCluster(3, nil)
	if c.Name != RestoreInitContainerName {
		t.Fatalf("name=%s", c.Name)
	}
	cmd := c.Command[2]
	// default shard layout: shard-0/dump.rdb, shard-1/dump.rdb, shard-2/dump.rdb
	for _, want := range []string{
		"SHARDS=3",
		"shard-0/dump.rdb",
		"shard-1/dump.rdb",
		"shard-2/dump.rdb",
		"ORDINAL=${HOSTNAME##*-}",
	} {
		if !contains(cmd, want) {
			t.Fatalf("expected %q in cmd, got: %s", want, cmd)
		}
	}
}

func TestBuildRestoreInitContainerForCluster_customLayout(t *testing.T) {
	layout := map[int]string{
		0: "primary-0/dump.rdb",
		1: "primary-1/dump.rdb",
	}
	c := BuildRestoreInitContainerForCluster(3, layout)
	cmd := c.Command[2]
	if !contains(cmd, "primary-0/dump.rdb") || !contains(cmd, "primary-1/dump.rdb") {
		t.Fatalf("custom layout not in cmd: %s", cmd)
	}
	// shard 2 는 layout 미명시 → default shard-2/dump.rdb.
	if !contains(cmd, "shard-2/dump.rdb") {
		t.Fatalf("default fallback for missing shard not applied: %s", cmd)
	}
}

func TestBuildRestoreInitContainerForCluster_hostnameDownwardAPI(t *testing.T) {
	c := BuildRestoreInitContainerForCluster(3, nil)
	if len(c.Env) != 1 || c.Env[0].Name != "HOSTNAME" {
		t.Fatalf("expected HOSTNAME env from downward API, got %v", c.Env)
	}
	if c.Env[0].ValueFrom == nil || c.Env[0].ValueFrom.FieldRef == nil {
		t.Fatalf("expected FieldRef for HOSTNAME")
	}
	if c.Env[0].ValueFrom.FieldRef.FieldPath != "metadata.name" {
		t.Fatalf("expected metadata.name FieldPath, got %s",
			c.Env[0].ValueFrom.FieldRef.FieldPath)
	}
}

func TestInjectRestoreIntoPodSpecForCluster_idempotent(t *testing.T) {
	pod := &corev1.PodSpec{}
	InjectRestoreIntoPodSpecForCluster(pod, 3, nil, "src-pvc")
	if got := len(pod.InitContainers); got != 1 {
		t.Fatalf("first inject — expected 1, got %d", got)
	}
	InjectRestoreIntoPodSpecForCluster(pod, 3, nil, "src-pvc")
	if got := len(pod.InitContainers); got != 1 {
		t.Fatalf("re-inject — still 1, got %d", got)
	}
	if got := len(pod.Volumes); got != 1 {
		t.Fatalf("volumes after re-inject — still 1, got %d", got)
	}
}

func TestInjectRestoreIntoPodSpecForCluster_RemoveCleansBoth(t *testing.T) {
	pod := &corev1.PodSpec{}
	InjectRestoreIntoPodSpecForCluster(pod, 3, nil, "src-pvc")
	RemoveRestoreFromPodSpec(pod)
	if len(pod.InitContainers) != 0 || len(pod.Volumes) != 0 {
		t.Fatalf("Remove 가 cluster 형태 inject 도 정리해야 — initContainers=%v volumes=%v",
			pod.InitContainers, pod.Volumes)
	}
}

// contains — strings.Contains 와 동등한 가벼운 헬퍼 (별도 import 회피).
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
