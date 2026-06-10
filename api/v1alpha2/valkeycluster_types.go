/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterPhase — Cluster 라이프사이클.
// +kubebuilder:validation:Enum=Pending;Initializing;Running;Resharding;Failed;Upgrading
type ClusterPhase string

const (
	ClusterPhasePending      ClusterPhase = "Pending"
	ClusterPhaseInitializing ClusterPhase = "Initializing"
	ClusterPhaseRunning      ClusterPhase = "Running"
	ClusterPhaseResharding   ClusterPhase = "Resharding"
	ClusterPhaseFailed       ClusterPhase = "Failed"
	ClusterPhaseUpgrading    ClusterPhase = "Upgrading"
)

// SlotMigrationPolicy — slot 재분배 정책.
// +kubebuilder:validation:Enum=Auto;Manual
type SlotMigrationPolicy string

const (
	SlotMigrationAuto   SlotMigrationPolicy = "Auto"
	SlotMigrationManual SlotMigrationPolicy = "Manual"
)

// ValkeyClusterSpec — Cluster mode (16384 slot, primary + replicas).
type ValkeyClusterSpec struct {
	// shard (primary) 수. 기본 3. 최소 3 — Valkey Cluster 는 quorum 위해 3+ 필요.
	// +kubebuilder:validation:Minimum=3
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=3
	Shards int32 `json:"shards"`

	// shard 당 replica 수. 기본 1 → 총 노드 = shards*(1+replicasPerShard).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=1
	ReplicasPerShard int32 `json:"replicasPerShard"`

	// +kubebuilder:default=true
	AutoFailover bool `json:"autoFailover,omitempty"`

	// +kubebuilder:default="Auto"
	SlotMigration SlotMigrationPolicy `json:"slotMigration,omitempty"`

	// cluster-node-timeout (ms). 기본 15000.
	// +kubebuilder:default=15000
	NodeTimeoutMillis int32 `json:"nodeTimeoutMillis,omitempty"`

	Version ValkeyVersion `json:"version"`

	// +optional
	Storage StorageSpec `json:"storage,omitempty"`
	// +optional
	Resources ResourcesSpec `json:"resources,omitempty"`
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`
	// +optional
	Auth AuthSpec `json:"auth,omitempty"`
	// +optional
	Monitoring *MonitoringSpec `json:"monitoring,omitempty"`
	// +optional
	Pod *PodSpec `json:"pod,omitempty"`
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	// +optional
	NetworkPolicy *NetworkPolicySpec `json:"networkPolicy,omitempty"`
	// +optional
	ScalePolicy *ScalePolicy `json:"scalePolicy,omitempty"`
	// +optional
	Persistence *PersistencePolicy `json:"persistence,omitempty"`
	// +optional
	AdditionalConfig map[string]string `json:"additionalConfig,omitempty"`

	// Modules — Valkey 공식 module 활성화 (ADR-0032).
	//
	// 지원 preset:
	//   - Name 만 지정: Valkey 공식 module preset (예: "valkey-search",
	//     "valkey-json", "valkey-bloom") 을 operator 가 valkey-bundle 에서
	//     추출해 loadmodule 로 적재.
	//   - 사용자 커스텀 module 은 ModuleSpec.Image 로 *bring-your-own*
	//     이미지 지정 가능. 단 외부 Redis Stack module/image 는 라이선스
	//     비호환으로 admission 에서 거부.
	// Cluster mode 에서는 모든 shard pod 에 동일 module set 을 로딩한다.
	// +kubebuilder:validation:MaxItems=16
	// +optional
	Modules []ModuleSpec `json:"modules,omitempty"`

	// RevisionHistoryLimit — StatefulSet rollout history 보존 개수.
	// +kubebuilder:validation:Minimum=0
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// AutoUpdate — operator-managed 자동 버전 업데이트 정책. channel 제약 내
	// 안전 patch/minor 만 자동 적용하며 major 상승은 운영자 명시를 요구한다.
	// +optional
	AutoUpdate *AutoUpdateSpec `json:"autoUpdate,omitempty"`
}

type ShardStatus struct {
	Index         int32    `json:"index"`
	PrimaryPod    string   `json:"primaryPod,omitempty"`
	ReplicaPods   []string `json:"replicaPods,omitempty"`
	SlotRange     string   `json:"slotRange,omitempty"`
	AssignedSlots int32    `json:"assignedSlots"`
}

type ValkeyClusterStatus struct {
	Phase              ClusterPhase       `json:"phase,omitempty"`
	ClusterState       string             `json:"clusterState,omitempty"`
	AssignedSlots      int32              `json:"assignedSlots,omitempty"`
	ReadyReplicas      int32              `json:"readyReplicas,omitempty"`
	Endpoint           string             `json:"endpoint,omitempty"`
	Version            string             `json:"version,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Shards []ShardStatus `json:"shards,omitempty"`
	// +optional
	PendingScale *PendingScale `json:"pendingScale,omitempty"`

	ClusterInitialized bool `json:"clusterInitialized,omitempty"`

	// Capabilities — 활성 optional features. Valkey CR Status.Capabilities 와 동일 패턴.
	// 예: TLS, Auth, Monitoring, Modules.
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vkc
// +kubebuilder:printcolumn:name="Shards",type="integer",JSONPath=".spec.shards"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".status.clusterState"
// +kubebuilder:printcolumn:name="Slots",type="integer",JSONPath=".status.assignedSlots"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version.version"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Capabilities",type="string",JSONPath=".status.capabilities",priority=1

// ValkeyCluster is the Schema for the valkeyclusters API.
type ValkeyCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ValkeyClusterSpec   `json:"spec,omitempty"`
	Status ValkeyClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ValkeyClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ValkeyCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ValkeyCluster{}, &ValkeyClusterList{})
}

func (v *ValkeyCluster) GetConditions() *[]metav1.Condition { return &v.Status.Conditions }
func (v *ValkeyCluster) SetPhase(phase string)              { v.Status.Phase = ClusterPhase(phase) }

func (s *ValkeyClusterSpec) TotalNodes() int32 { return s.Shards * (1 + s.ReplicasPerShard) }
