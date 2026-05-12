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
package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// VolumeSnapshotGVK — snapshot.storage.k8s.io/v1 VolumeSnapshot.
//
// 본 패키지는 external-snapshotter 의존성을 추가하지 않고 unstructured 만 사용 —
// CRD 미설치 시 NoMatchError 자동 fail-soft (cert-manager / Prometheus Operator
// 와 동일 패턴, ADR-0010).
var VolumeSnapshotGVK = schema.GroupVersionKind{
	Group:   "snapshot.storage.k8s.io",
	Version: "v1",
	Kind:    "VolumeSnapshot",
}

// VolumeSnapshotName — Backup CR 의 VolumeSnapshot CR 이름.
func VolumeSnapshotName(backupName string) string { return backupName + "-snap" }

// BuildVolumeSnapshotForBackup — ValkeyBackup type=VolumeSnapshot 시 생성할
// snapshot.storage.k8s.io/v1 VolumeSnapshot CR (unstructured).
//
// source PVC: 대상 CR 의 첫 ordinal pod 의 data PVC (data-<crName>-0).
// VolumeSnapshotClassName: 미명시 시 spec field omit → cluster default class
// 사용.
//
// 참고: VolumeSnapshot 은 *cluster + storage* 단위 — 다중 replica/shard 의 PVC
// 각각 snapshot 받으려면 caller 가 ordinal 별로 본 함수 반복 호출 필요. 본
// PR 은 *single PVC snapshot* 만 (ordinal=0). 다중 PVC snapshot 은 phase 2.
func BuildVolumeSnapshotForBackup(b *cachev1alpha1.ValkeyBackup, sourceCRName string) *unstructured.Unstructured {
	if b.Spec.Type != cachev1alpha1.BackupTypeVolumeSnapshot {
		return nil
	}

	spec := map[string]any{
		"source": map[string]any{
			"persistentVolumeClaimName": dataPVCName(sourceCRName, 0),
		},
	}
	if b.Spec.VolumeSnapshotClassName != "" {
		spec["volumeSnapshotClassName"] = b.Spec.VolumeSnapshotClassName
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(VolumeSnapshotGVK)
	u.SetName(VolumeSnapshotName(b.Name))
	u.SetNamespace(b.Namespace)
	u.SetLabels(map[string]string{
		LabelAppName:      "valkey",
		LabelInstanceName: sourceCRName,
		LabelComponent:    "backup-snapshot",
		LabelManagedBy:    ManagedByValue,
		LabelPartOf:       PartOfValue,
	})
	u.Object["spec"] = spec
	return u
}

// dataPVCName — STS controller 가 명명하는 PVC 패턴: `data-<crName>-<ordinal>`.
// volumesnapshot.go 가 노출하는 helper (pvc_resize 의 prefix helper 와 동일 패턴).
func dataPVCName(crName string, ordinal int) string {
	return "data-" + crName + "-" + ordinalSuffix(ordinal)
}

func ordinalSuffix(n int) string {
	// 0~9 한 자릿수 — 10+ 도 필요 시 strconv.Itoa 로 일반화.
	digits := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	if n < 0 || n >= len(digits) {
		// fallback — 일반화 필요 시 strconv 도입 (현재 PVC ordinal 0 만 사용).
		return "0"
	}
	return digits[n]
}
