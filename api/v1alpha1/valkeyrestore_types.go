/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RestoreType — RDB snapshot 또는 AOF (추후) 복원.
//
// +kubebuilder:validation:Enum=RDB;AOF
type RestoreType string

const (
	RestoreTypeRDB RestoreType = "RDB"
	RestoreTypeAOF RestoreType = "AOF"
)

// RestorePhase — 라이프사이클 (ADR-0015).
//
// +kubebuilder:validation:Enum=Pending;Mounting;Restoring;Verifying;Completed;Failed
type RestorePhase string

const (
	// RestorePhasePending — CR 생성 직후, ClusterRef 검증 전.
	RestorePhasePending RestorePhase = "Pending"
	// RestorePhaseMounting — Source 가 PVC 인 경우 PVC 존재 확인,
	// targetRef 인 경우 외부에서 다운로드 (별개 commit). Cluster paused 처리.
	RestorePhaseMounting RestorePhase = "Mounting"
	// RestorePhaseRestoring — STS template 에 init container 추가, rolling
	// restart 진행, valkey-server 가 RDB 자동 로드 중.
	RestorePhaseRestoring RestorePhase = "Restoring"
	// RestorePhaseVerifying — 모든 pod Ready, 데이터 plane 검증 (PING + 임의
	// key SET/GET).
	RestorePhaseVerifying RestorePhase = "Verifying"
	// RestorePhaseCompleted — STS 원복 (init container 제거), Cluster
	// unpaused. Restore CR 은 보존 (audit trail).
	RestorePhaseCompleted RestorePhase = "Completed"
	// RestorePhaseFailed — 임의 단계 실패. 사용자 개입 필요. STS 원복 시도.
	RestorePhaseFailed RestorePhase = "Failed"
)

// RestoreSourcePVC — Source.PVC: 기존 backup PVC 에서 RDB 직접 사용.
//
// 단일 valkey (Standalone/Replication 모드) 는 `<pvc>/dump.rdb` 경로.
// ValkeyCluster (sharded) 는 `<pvc>/<shardLayout[shardN]>` 매핑 — 추후 commit.
type RestoreSourcePVC struct {
	// 기존 PVC 이름 (같은 namespace).
	Name string `json:"name"`

	// PVC 안의 RDB 파일 경로. 미명시 시 "dump.rdb".
	//
	// +kubebuilder:default="dump.rdb"
	// +optional
	Path string `json:"path,omitempty"`

	// ValkeyCluster (sharded) 시 shard index → PVC 내부 path 매핑.
	// 예: { "shard-0": "shard-0/dump.rdb", ... }
	// 단일 Valkey (Standalone/Replication) 시 무시.
	//
	// +optional
	ShardLayout map[string]string `json:"shardLayout,omitempty"`
}

// RestoreSourceTargetRef — 외부 ValkeyBackupTarget 참조 (ADR-0016 대칭).
//
// 본 필드는 *별개 commit* 에서 활성화 — 첫 버전은 PVC 만.
type RestoreSourceTargetRef struct {
	// ValkeyBackupTarget CR 이름 (같은 namespace).
	Name string `json:"name"`

	// target 내부의 path/prefix. 예: "2026-05-05/dump.rdb".
	Path string `json:"path"`
}

// RestoreSource — PVC 또는 외부 target 중 하나.
//
// 두 필드가 동시에 명시되면 webhook validation 거절 (별개 commit).
type RestoreSource struct {
	// +optional
	PVC *RestoreSourcePVC `json:"pvc,omitempty"`

	// +optional
	TargetRef *RestoreSourceTargetRef `json:"targetRef,omitempty"`
}

// ValkeyRestoreSpec — restore 트리거 정의.
type ValkeyRestoreSpec struct {
	// 대상 Valkey 또는 ValkeyCluster CR (같은 namespace).
	ClusterRef ClusterReference `json:"clusterRef"`

	// 복원 source — PVC (즉시 사용 가능) 또는 TargetRef (S3, 별개 commit).
	Source RestoreSource `json:"source"`

	// +kubebuilder:default="RDB"
	// +optional
	RestoreType RestoreType `json:"restoreType,omitempty"`
}

// ValkeyRestoreStatus — 진행 상황.
type ValkeyRestoreStatus struct {
	Phase RestorePhase `json:"phase,omitempty"`

	StartedAt   *metav1.Time `json:"startedAt,omitempty"`
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// RDB 가 cluster 에 로드된 후 INFO 로 추정한 키 개수 (검증 단계 결과).
	RestoredKeys int64 `json:"restoredKeys,omitempty"`

	Message            string `json:"message,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vkr
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.restoreType"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Keys",type="integer",JSONPath=".status.restoredKeys"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ValkeyRestore — 기존 backup (PVC 또는 외부 target) 에서 RDB 를 복원.
// ADR-0015 (Init Container 패턴 + STS template patch + 원복).
type ValkeyRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ValkeyRestoreSpec   `json:"spec,omitempty"`
	Status ValkeyRestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ValkeyRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ValkeyRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ValkeyRestore{}, &ValkeyRestoreList{})
}

func (r *ValkeyRestore) GetConditions() *[]metav1.Condition { return &r.Status.Conditions }

// IsTerminal — Completed / Failed 면 true (이후 reconcile 무시).
func (r *ValkeyRestore) IsTerminal() bool {
	return r.Status.Phase == RestorePhaseCompleted || r.Status.Phase == RestorePhaseFailed
}
