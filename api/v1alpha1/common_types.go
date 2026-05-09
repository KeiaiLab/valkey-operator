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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	commonsversion "github.com/keiailab/operator-commons/pkg/version"
)

// DefaultValkeyVersion / DefaultValkeyImage — defaulting webhook 이 spec.version
// 통째 누락 케이스에서 채워 넣는 값. CRD 의 kubebuilder default 와 동기화 유지.
const (
	DefaultValkeyVersion = "9.0.4"
	DefaultValkeyImage   = "docker.io/valkey/valkey"
)

// ClusterRef.Kind 표준 식별자 — RFC-0017 audit 후 4-repo cross-cut goconst
// 추출 (valkeybackup_controller / valkeybackuptarget_controller 가 9 + 6 occurrences
// 사용).
const (
	KindValkey        = "Valkey"
	KindValkeyCluster = "ValkeyCluster"
)

// DefaultStorageSize — Spec.StorageSize / Backup.StorageSize 의 fallback 기본값.
// CRD 의 +kubebuilder:default="8Gi" 와 정합 유지.
const DefaultStorageSize = "8Gi"

// supportedValkeyList — webhook validation 화이트리스트 (commons 위임).
// 신규 추가 시 호환성 검증 (RDB format, replication wire protocol) 후 추가.
// 9.0.4 = current stable, 8.1.7/8.0.9 = milestone patch baseline. 8.1.6 은
// 기존 설치 호환 위해 유지.
var supportedValkeyList = commonsversion.MustList("8.0.9", "8.1.6", "8.1.7", "9.0.4")

// SupportedValkeyVersions — 외부 노출 슬라이스 (chart values / docs / 기존 호환).
var SupportedValkeyVersions = supportedValkeyList.Strings()

// IsSupportedValkeyVersion — webhook validation 호출용 헬퍼. commons 위임.
func IsSupportedValkeyVersion(v string) bool {
	return supportedValkeyList.IsSupported(v)
}

// ValkeyVersion 은 Valkey 컨테이너 이미지 / 버전 지정.
type ValkeyVersion struct {
	// +kubebuilder:validation:Pattern=`^\d+\.\d+(\.\d+)?$`
	// +kubebuilder:default="9.0.4"
	Version string `json:"version"`

	// +kubebuilder:default="docker.io/valkey/valkey"
	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// StorageSpec — Valkey 데이터 디렉터리(/data) 마운트용 PVC.
type StorageSpec struct {
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// +kubebuilder:default="8Gi"
	Size resource.Quantity `json:"size,omitempty"`

	// +kubebuilder:default="/data"
	DataDirPath string `json:"dataDirPath,omitempty"`
}

// ResourcesSpec — pod resource requests/limits.
type ResourcesSpec struct {
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
}

// AuthSpec — Valkey 인증 (password + 선택적 ACL).
//
// Valkey 는 단순 password (requirepass) 와 ACL v2 (users) 두 모드 지원.
// PasswordSecretRef 의 Secret 에 key "password" 로 저장된 값을 requirepass 에 주입.
// PasswordSecretRef 미지정 시 controller 가 32 byte random Secret 자동 생성.
type AuthSpec struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// +optional
	PasswordSecretRef *corev1.SecretKeySelector `json:"passwordSecretRef,omitempty"`

	// +optional
	Users []ValkeyUser `json:"users,omitempty"`
}

// ValkeyUser — ACL v2 사용자 정의 (M5 단계에서 활성).
type ValkeyUser struct {
	Name              string                   `json:"name"`
	PasswordSecretRef corev1.SecretKeySelector `json:"passwordSecretRef"`
	// 예: "+@read", "~cache:*", "&pubsub:notify"
	Rules []string `json:"rules,omitempty"`
}

// TLSSpec — Valkey TLS (cert-manager 연동 옵션).
type TLSSpec struct {
	Enabled bool `json:"enabled"`

	// +optional
	CertManager *CertManagerSpec `json:"certManager,omitempty"`

	// +optional
	CustomCert *CustomCertSpec `json:"customCert,omitempty"`

	// ClientAuth 는 client certificate 검증 정책. valkey 의 tls-auth-clients
	// 옵션과 1:1 매핑.
	//   - required (default): tls-auth-clients=yes — mTLS 강제. 모든 client 가
	//     valid client cert 제시 의무. operator 자체도 cert load.
	//   - optional: tls-auth-clients=optional — client cert 제시하면 검증, 없으면
	//     password-only auth 허용.
	//   - disabled: tls-auth-clients=no — server-only TLS (cert 검증 안 함).
	//     plaintext password 보호 + transit encryption 만 활성. 외부 client
	//     마이그레이션 path (mTLS infrastructure 부재 시).
	// +kubebuilder:validation:Enum=required;optional;disabled
	// +kubebuilder:default="required"
	// +optional
	ClientAuth string `json:"clientAuth,omitempty"`
}

type CertManagerSpec struct {
	IssuerRef CertIssuerRef `json:"issuerRef"`

	// +kubebuilder:default="2160h"
	Duration string `json:"duration,omitempty"`

	// +kubebuilder:default="360h"
	RenewBefore string `json:"renewBefore,omitempty"`
}

type CertIssuerRef struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=Issuer;ClusterIssuer
	// +kubebuilder:default="ClusterIssuer"
	Kind string `json:"kind"`
}

type CustomCertSpec struct {
	SecretName string `json:"secretName"`
}

// MonitoringSpec — Prometheus exporter sidecar + ServiceMonitor.
type MonitoringSpec struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`

	// +optional
	Exporter *ExporterSpec `json:"exporter,omitempty"`
}

type ServiceMonitorSpec struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +kubebuilder:default="30s"
	Interval string `json:"interval,omitempty"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ExporterSpec — prometheus-valkey-exporter sidecar.
type ExporterSpec struct {
	// +kubebuilder:default="oliver006/redis_exporter:latest"
	Image string `json:"image,omitempty"`
	// +optional
	Resources ResourcesSpec `json:"resources,omitempty"`
}

// PodSpec — pod 레벨 스케줄/보안 옵션.
type PodSpec struct {
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// NetworkPolicySpec — opt-in NetworkPolicy. 기본 deny + 같은 인스턴스 pod 간 6379/16379 허용.
type NetworkPolicySpec struct {
	// +kubebuilder:default=false
	Enabled               bool                `json:"enabled"`
	AdditionalIngressFrom []NetworkPolicyPeer `json:"additionalIngressFrom,omitempty"`
}

type NetworkPolicyPeer struct {
	// +optional
	PodSelector *map[string]string `json:"podSelector,omitempty"`
	// +optional
	NamespaceSelector *map[string]string `json:"namespaceSelector,omitempty"`
}

// PodDisruptionBudgetSpec — opt-in PDB. minAvailable 기본값 = replicas-1.
type PodDisruptionBudgetSpec struct {
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// ScalePolicy — 인스턴스 수 변경 가드. mongodb-operator ADR 0008 패턴 차용.
//
// Valkey replication / cluster 토폴로지 변경은 무시할 수 없는 부작용을 동반:
// - Replication: full SYNC 트래픽 폭주
// - Cluster: 16384 slot 재분배 (수십분~수시간), 그 동안 일부 키 일시적 MOVED 응답
// Deliberate=true 가 명시되어야만 즉시 적용. 그 외에는 Status.PendingScale 에 기록.
type ScalePolicy struct {
	// +optional
	Deliberate bool `json:"deliberate,omitempty"`
}

type PendingScale struct {
	CurrentReplicas int32  `json:"currentReplicas"`
	DesiredReplicas int32  `json:"desiredReplicas"`
	RequestedAt     string `json:"requestedAt,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// PersistencePolicy — RDB / AOF 정책.
type PersistencePolicy struct {
	// +kubebuilder:validation:Enum=RDB;AOF;Both;None
	// +kubebuilder:default="RDB"
	Mode string `json:"mode,omitempty"`

	// RDB save 지시문 (예: "3600 1 300 100 60 10000"). 비어있으면 valkey 기본값.
	// +optional
	RDBSaveSchedule string `json:"rdbSaveSchedule,omitempty"`

	// AOF appendfsync 정책.
	// +kubebuilder:validation:Enum=always;everysec;no
	// +kubebuilder:default="everysec"
	AOFAppendFsync string `json:"aofAppendFsync,omitempty"`
}
