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
// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
type BackupPhase string

const (
	BackupPhasePending    BackupPhase = "Pending"
	BackupPhaseInProgress BackupPhase = "InProgress"
	BackupPhaseCompleted  BackupPhase = "Completed"
	BackupPhaseFailed     BackupPhase = "Failed"
)

// ValkeyBackupSpec — backup 트리거 정의.
type ValkeyBackupSpec struct {
	// 대상 Valkey 또는 ValkeyCluster CR.
	ClusterRef ClusterReference `json:"clusterRef"`

	// +kubebuilder:default="RDB"
	Type BackupType `json:"type,omitempty"`

	// 결과 저장 PVC. 미명시 시 동적 PVC 생성 (operator 가 storageClass / size 결정).
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
