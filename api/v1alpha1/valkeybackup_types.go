/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupType — RDB snapshot 또는 AOF rewrite.
// +kubebuilder:validation:Enum=RDB;AOF
type BackupType string

const (
	BackupTypeRDB BackupType = "RDB"
	BackupTypeAOF BackupType = "AOF"
)

// BackupPhase — 라이프사이클.
// +kubebuilder:validation:Enum=Pending;InProgress;Copying;Uploading;Completed;Failed
type BackupPhase string

const (
	BackupPhasePending    BackupPhase = "Pending"
	BackupPhaseInProgress BackupPhase = "InProgress"
	// BackupPhaseCopying — RDB 가 valkey 노드 에 생성된 후 외부 PVC 로 복사 중.
	// (M3.5: valkey-cli --rdb 를 실행하는 Job 이 backup PVC 에 RDB 스트림.)
	BackupPhaseCopying BackupPhase = "Copying"
	// BackupPhaseUploading — Destination.Type=TargetRef 시: PVC 의 RDB 를
	// 외부 저장 (S3 등) 에 업로드 중. ADR-0016 + ADR-0023.
	BackupPhaseUploading BackupPhase = "Uploading"
	BackupPhaseCompleted BackupPhase = "Completed"
	BackupPhaseFailed    BackupPhase = "Failed"
)

// BackupDestinationType — PVC 보존 (M3.5 호환) 또는 외부 저장 (ADR-0016).
//
// +kubebuilder:validation:Enum=PVC;TargetRef
type BackupDestinationType string

const (
	BackupDestPVC       BackupDestinationType = "PVC"
	BackupDestTargetRef BackupDestinationType = "TargetRef"
)

// BackupDestinationTargetRef — 외부 ValkeyBackupTarget 참조 (ADR-0016 대칭).
type BackupDestinationTargetRef struct {
	// 같은 namespace 의 ValkeyBackupTarget CR 이름.
	Name string `json:"name"`

	// target prefix + 본 path = 최종 object key.
	// 미명시 시 default "<backup-name>/<startedAt-RFC3339>/dump.rdb".
	//
	// +optional
	Path string `json:"path,omitempty"`
}

// BackupDestination — backup 결과물의 저장 위치.
//
// Type=PVC (default): ValkeyBackupSpec 의 기존 TargetPVC/StorageSize/RetainPVC
// 필드 그대로 활용 — M3.5 호환성 유지.
//
// Type=TargetRef: 외부 저장 (S3/GCS/Azure). PVC 는 *중간 staging* 으로 활용 후
// Uploading phase 가 외부로 전송, 성공 시 RetainPVC=false 면 PVC 삭제.
type BackupDestination struct {
	// +kubebuilder:default=PVC
	// +optional
	Type BackupDestinationType `json:"type,omitempty"`

	// Type=TargetRef 시 필수.
	//
	// +optional
	TargetRef *BackupDestinationTargetRef `json:"targetRef,omitempty"`
}

// ValkeyBackupSpec — backup 트리거 정의.
type ValkeyBackupSpec struct {
	// 대상 Valkey 또는 ValkeyCluster CR.
	ClusterRef ClusterReference `json:"clusterRef"`

	// +kubebuilder:default="RDB"
	Type BackupType `json:"type,omitempty"`

	// 결과 저장 PVC.
	//
	// 미명시 (nil) 시 operator 가 *동적 PVC 생성* — controllerOwnerRef 부착,
	// StorageSize 와 default storageClass 사용. 권장 default behavior.
	//
	// 명시 (Name 설정) 시 *사용자 사전 생성 PVC 사용* — operator 가 *PVC 생성 안 함*.
	// 사용자가 사전에 같은 namespace 에 PVC 생성 의무. 미생성 시 backup Job pod 가
	// FailedScheduling "persistentvolumeclaim X not found" 로 stuck (iteration 38 발견).
	//
	// 권장: TargetPVC=nil + StorageSize 만 명시 (operator 자동 관리).
	// +optional
	TargetPVC *corev1.LocalObjectReference `json:"targetPVC,omitempty"`

	// 결과 PVC 크기 (TargetPVC 미명시 시 적용). 기본 8Gi.
	// +kubebuilder:default="8Gi"
	// +optional
	StorageSize string `json:"storageSize,omitempty"`

	// 백업 후 PVC 보존. 기본 true (false 면 operator 가 PVC 삭제).
	// +kubebuilder:default=true
	RetainPVC bool `json:"retainPVC,omitempty"`

	// TTL — 본 백업 CR 의 자동 삭제 기한 (예: "168h" = 7일). 미명시 시 보존.
	// +optional
	TTL string `json:"ttl,omitempty"`

	// Destination — 결과물의 저장 위치. 미명시 시 type=PVC (기존 동작).
	//
	// +optional
	Destination *BackupDestination `json:"destination,omitempty"`
}

// DefaultBackupObjectPath — Spec.Destination.TargetRef.Path 미명시 시 자동
// 생성될 object key. <backup-name>/<startedAt-RFC3339>/dump.rdb.
//
// startedAt 은 Status.StartedAt 시점 — Reconcile 진입 시 결정. 본 함수는
// caller (controller) 가 startedAt 을 가지고 있을 때 호출.
func DefaultBackupObjectPath(backupName, startedAt string) string {
	return backupName + "/" + startedAt + "/dump.rdb"
}

// ClusterReference — 대상 CR 참조 (Valkey 또는 ValkeyCluster).
type ClusterReference struct {
	// +kubebuilder:validation:Enum=Valkey;ValkeyCluster
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// ValkeyBackupStatus — 진행 상황.
type ValkeyBackupStatus struct {
	Phase              BackupPhase  `json:"phase,omitempty"`
	StartedAt          *metav1.Time `json:"startedAt,omitempty"`
	CompletedAt        *metav1.Time `json:"completedAt,omitempty"`
	BackupSizeBytes    int64        `json:"backupSizeBytes,omitempty"`
	PVCName            string       `json:"pvcName,omitempty"`
	Message            string       `json:"message,omitempty"`
	ObservedGeneration int64        `json:"observedGeneration,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vkb
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Size",type="integer",JSONPath=".status.backupSizeBytes"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ValkeyBackup is the Schema for the valkeybackups API.
type ValkeyBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ValkeyBackupSpec   `json:"spec,omitempty"`
	Status ValkeyBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ValkeyBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ValkeyBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ValkeyBackup{}, &ValkeyBackupList{})
}

func (b *ValkeyBackup) GetConditions() *[]metav1.Condition { return &b.Status.Conditions }
func (b *ValkeyBackup) SetPhase(phase string)              { b.Status.Phase = BackupPhase(phase) }

// IsTerminal — Completed / Failed 면 true.
func (b *ValkeyBackup) IsTerminal() bool {
	return b.Status.Phase == BackupPhaseCompleted || b.Status.Phase == BackupPhaseFailed
}
