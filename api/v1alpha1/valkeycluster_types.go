/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package v1alpha1

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

	// shard 당 replica 수. 총 노드 = shards*(1+replicasPerShard).
	//
	// *pointer* 인 이유 (defect ④): non-pointer int32 + CRD `+kubebuilder:default=1` 은
	// apiserver/admission 이 "필드 미지정" 과 "명시 0" 을 구별하지 못해 명시 0
	// (masters-only 토폴로지) 을 default 1 로 덮어쓴다. pointer 로 두면 nil(미지정)
	// 과 명시 0 이 구별된다. CRD default 는 제거하고 nil→1 defaulting 은 코드
	// (GetReplicasPerShard 접근자 + mutating webhook) 에서 처리한다.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	// +optional
	ReplicasPerShard *int32 `json:"replicasPerShard,omitempty"`

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

	// Modules — Valkey 공식 BSD module(valkey-search/json/bloom) 또는 BYO module 로딩.
	// 외부 Redis Stack(RediSearch/RedisJSON, RSALv2/SSPL)은 라이선스 비호환으로 미지원
	// (ADR-0032). Name 만 지정 시 공식 preset 자동 resolve, Image 지정 시 BYO. Valkey CR
	// (ValkeySpec.Modules) 와 동일 타입/검증 — cluster 샤드 pod 전체에 동일 적용.
	// +optional
	Modules []ModuleSpec `json:"modules,omitempty"`

	// +optional
	SlowLog *SlowLogSpec `json:"slowLog,omitempty"`

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
	// `kubectl get vc -o wide` 의 priority=1 printcolumn 으로 노출.
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vkc
// +kubebuilder:storageversion
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

// GetReplicasPerShard — nil(미지정) → 1, 명시값(0 포함) → 그대로.
//
// defect ④: 명시 0 은 masters-only 토폴로지이므로 보존하고, 미지정만 1 로
// defaulting 한다. ReplicasPerShard 를 읽는 모든 곳은 이 접근자를 거친다.
func (s *ValkeyClusterSpec) GetReplicasPerShard() int32 {
	if s.ReplicasPerShard == nil {
		return 1
	}
	return *s.ReplicasPerShard
}

func (s *ValkeyClusterSpec) TotalNodes() int32 { return s.Shards * (1 + s.GetReplicasPerShard()) }
