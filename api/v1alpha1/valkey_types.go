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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValkeyMode — 단일 인스턴스인지 replication 인지.
// +kubebuilder:validation:Enum=Standalone;Replication
type ValkeyMode string

const (
	ModeStandalone  ValkeyMode = "Standalone"
	ModeReplication ValkeyMode = "Replication"
)

// ValkeyPhase — 라이프사이클 페이즈.
// +kubebuilder:validation:Enum=Pending;Initializing;Running;Failed;Upgrading
type ValkeyPhase string

const (
	PhasePending      ValkeyPhase = "Pending"
	PhaseInitializing ValkeyPhase = "Initializing"
	PhaseRunning      ValkeyPhase = "Running"
	PhaseFailed       ValkeyPhase = "Failed"
	PhaseUpgrading    ValkeyPhase = "Upgrading"
)

// ValkeySpec — Standalone 또는 Replication (primary + replicas) 토폴로지.
// Cluster mode 는 별도 ValkeyCluster CRD 사용.
type ValkeySpec struct {
	// +kubebuilder:default="Standalone"
	Mode ValkeyMode `json:"mode,omitempty"`

	// 토폴로지가 Replication 일 때 전체 인스턴스 수 (primary 1 + replicas N).
	// Standalone 에서는 무시되며 항상 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=15
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

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

	// valkey.conf 의 추가 directive (예: maxmemory: "1gb").
	// +optional
	AdditionalConfig map[string]string `json:"additionalConfig,omitempty"`

	// AutoFailover — Replication mode 시 primary pod NotReady 30s+ 감지 시
	// 자동 failover (replica with largest master_repl_offset 선출).
	// ADR-0017. Standalone (replicas=1) 에서는 N/A.
	//
	// +kubebuilder:default=true
	// +optional
	AutoFailover *bool `json:"autoFailover,omitempty"`

	// Autoscaling — operator-managed HorizontalPodAutoscaler (HPA v2).
	// ADR-0027 (Replication mode 만 — Cluster mode 는 slot 재분배 위험).
	// 활성 시 ScalePolicy.Deliberate 무시 + Spec.Replicas 는 default 값으로만 사용.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// SlowLog — Valkey SLOWLOG 임계값 + 보존 entry 수 설정.
	// nil 이면 valkey 기본값 (10ms / 128 entries) 사용.
	// +optional
	SlowLog *SlowLogSpec `json:"slowLog,omitempty"`

	// +optional
	ExternalReplica *ExternalReplicaSpec `json:"externalReplica,omitempty"`

	// RevisionHistoryLimit — StatefulSet rollout history 보존 개수.
	// +kubebuilder:validation:Minimum=0
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`
}

// IsAutoFailoverEnabled — Spec.AutoFailover 가 nil 또는 true 면 true (default
// enabled). false 명시 시만 disabled.
func (s *ValkeySpec) IsAutoFailoverEnabled() bool {
	if s.AutoFailover == nil {
		return true
	}
	return *s.AutoFailover
}

// ValkeyStatus — observed state.
type ValkeyStatus struct {
	Phase              ValkeyPhase        `json:"phase,omitempty"`
	ReadyReplicas      int32              `json:"readyReplicas,omitempty"`
	CurrentPrimary     string             `json:"currentPrimary,omitempty"`
	Endpoint           string             `json:"endpoint,omitempty"`
	Version            string             `json:"version,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	PendingScale *PendingScale `json:"pendingScale,omitempty"`

	// Capabilities — 본 CR 에서 *활성된 optional features* 의 ordered list.
	// `kubectl get vk -o wide` 의 printcolumn 으로 한눈에 확인 가능.
	// reconcile 마다 갱신. 가능 값 (PR #38-#60):
	//   "TLS"             — Spec.TLS.Enabled
	//   "TLS-AutoCA"      — Spec.TLS.CertManager.AutoSelfSigned (PR #40)
	//   "Auth"            — Spec.Auth.Enabled
	//   "Autoscaling"     — Spec.Autoscaling.Enabled (PR #44)
	//   "SlowLog"         — Spec.SlowLog 명시 (PR #45)
	//   "EncryptionAudit" — Spec.Storage.EncryptionRequired (PR #45)
	//   "EncryptionEnforce" — Spec.Storage.EncryptionEnforce (PR #55)
	//   "NetworkPolicy"   — Spec.NetworkPolicy.Enabled
	//   "Monitoring"      — Spec.Monitoring.Enabled
	//   "ExternalReplica" — Spec.ExternalReplica.Enabled
	//   "EphemeralStorage" — Spec.Storage.Ephemeral
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vk
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.mode"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version.version"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Capabilities",type="string",JSONPath=".status.capabilities",priority=1

// Valkey is the Schema for the valkeys API.
type Valkey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ValkeySpec   `json:"spec,omitempty"`
	Status ValkeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ValkeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Valkey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Valkey{}, &ValkeyList{})
}

func (v *Valkey) GetConditions() *[]metav1.Condition { return &v.Status.Conditions }
func (v *Valkey) SetPhase(phase string)              { v.Status.Phase = ValkeyPhase(phase) }
