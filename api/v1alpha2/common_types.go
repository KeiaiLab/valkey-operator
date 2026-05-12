/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha2

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

	// ImageRef 는 registry/repository:tag@sha256:digest 전체 image reference.
	// 명시 시 Image + Version 조합보다 우선한다.
	// +optional
	ImageRef string `json:"imageRef,omitempty"`

	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// StorageSpec — Valkey 데이터 디렉터리(/data) 마운트용 PVC.
type StorageSpec struct {
	// Ephemeral — true 면 PVC 대신 emptyDir 를 사용한다.
	// +kubebuilder:default=false
	// +optional
	Ephemeral bool `json:"ephemeral,omitempty"`

	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// +kubebuilder:default="8Gi"
	Size resource.Quantity `json:"size,omitempty"`

	// +kubebuilder:default="/data"
	DataDirPath string `json:"dataDirPath,omitempty"`

	// +optional
	ExistingClaim string `json:"existingClaim,omitempty"`

	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`
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
// v1alpha2 변경 (Plan §2 D1, ADR-0034 — supersedes ADR-0013):
// Required `*bool` 필드 신규로 Auth 강제 정책을 *토글 가능* 하게 한다.
// default=true 유지로 v1alpha1 보안 동작과 호환. 사용자가 외부 인증
// (Istio mTLS, sidecar auth) 사용 시 false 명시 가능.
//
// Valkey 는 단순 password (requirepass) 와 ACL v2 (users) 두 모드 지원.
// PasswordSecretRef 의 Secret 에 key "password" 로 저장된 값을 requirepass 에 주입.
// PasswordSecretRef 미지정 시 controller 가 32 byte random Secret 자동 생성.
type AuthSpec struct {
	// Enabled — legacy v1alpha1 호환 필드. v1alpha2 controller 는
	// Required 를 우선 평가; Required 가 nil 이면 Enabled 가 fallback.
	// PR-A2.2 controller migration 완료 후 Deprecated 표기 예정.
	//
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Required — Auth 강제 토글 (Plan §2 D1, ADR-0034 — supersedes ADR-0013).
	//
	//   - default=true: operator 가 random 32B password Secret 자동 생성 +
	//     requirepass 강제 (v1alpha1 ADR-0013 동등 동작, secure-by-default).
	//   - false: requirepass 미설정 + AUTH 명령 거부 + NOAUTH 응답.
	//     외부 인증 (Istio mTLS, sidecar proxy, network-isolated namespace)
	//     시나리오 지원.
	//   - nil (legacy 호환): Enabled fallback. 신규 v1alpha2 사용자는
	//     Required 명시 권장.
	//
	// +kubebuilder:default:=true
	// +optional
	Required *bool `json:"required,omitempty"`

	// +optional
	PasswordSecretRef *corev1.SecretKeySelector `json:"passwordSecretRef,omitempty"`

	// RotationPolicy — Password 회전 정책 (Plan §2 D6, ADR-0031).
	//
	//   - "Manual" (default): 사용자가 외부에서 PasswordSecretRef Secret
	//     변경 시 operator 무영향. 회전은 사용자 책임 (운영 매뉴얼).
	//   - "OnSecretChange": Secret resourceVersion 변경 감지 시 operator 가
	//     자동으로 valkey CONFIG SET requirepass 발행 (무중단 회전 path).
	//     replication mode 에서는 replica 먼저 reauth 후 primary 갱신
	//     (race 방지).
	//
	// 운영 규약: operator 자체는 *회전 수행* 안 함 (외부 ESO/OpenBao 위임).
	// 본 필드는 *외부 회전 반영* 만 — Plan §2 D6 의 "회전 수행 X, 반영 O" 정책.
	//
	// +kubebuilder:validation:Enum=Manual;OnSecretChange
	// +kubebuilder:default:=Manual
	// +optional
	RotationPolicy string `json:"rotationPolicy,omitempty"`

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
	// +optional — AutoSelfSigned=true 시 빈 값 허용.
	IssuerRef CertIssuerRef `json:"issuerRef,omitempty"`

	// AutoSelfSigned — true 시 operator 가 namespace-scope SelfSigned Issuer
	// (`<crName>-selfsigned`) 자동 생성 + Certificate 가 그 Issuer 참조.
	// Bitnami `tls.autoGenerated` 동등. IssuerRef.Name 과 상호 배타 (webhook reject).
	// +optional
	AutoSelfSigned bool `json:"autoSelfSigned,omitempty"`

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
//
// v1alpha2 변경 (Plan §2 D3, ADR-0036): PodSecurityRestricted 필드 신규.
// default=true 시 operator 가 Pod Security Admission "restricted"
// profile 강제 (v1alpha1 동작 동등 — capabilities.drop=ALL, runAsNonRoot,
// readOnlyRootFilesystem 등). false 시 SecurityContext /
// ContainerSecurityContext 의 사용자 정의 우선.
type PodSpec struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// PodSecurityRestricted — PSA "restricted" profile 강제 토글
	// (Plan §2 D3, ADR-0036). default=true 유지로 secure-by-default 보존.
	//
	// nil (legacy 호환): true 처리.
	// true (default): operator 가 restricted SecurityContext 강제 적용 —
	// SecurityContext / ContainerSecurityContext 사용자 정의는 *enforced
	// fields* 외 영역만 적용.
	// false: 사용자 정의 SecurityContext / ContainerSecurityContext 우선.
	// 외부 PSA policy (custom admission controller) 또는 K8s 1.25+ PSA
	// label namespace 분리 시나리오 지원.
	//
	// +kubebuilder:default:=true
	// +optional
	PodSecurityRestricted *bool `json:"podSecurityRestricted,omitempty"`

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
	// +optional
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
	// +optional
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// ServiceSpec — Valkey client/headless Service 커스터마이즈.
type ServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default="ClusterIP"
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`
	// IPFamilyPolicy — dual-stack / single-stack Service 정책. 미지정 시
	// Kubernetes 기본값 사용.
	// +optional
	IPFamilyPolicy *corev1.IPFamilyPolicy `json:"ipFamilyPolicy,omitempty"`
	// IPFamilies — Service IP family 순서. 예: ["IPv4"], ["IPv6"],
	// ["IPv4", "IPv6"].
	// +optional
	IPFamilies []corev1.IPFamily `json:"ipFamilies,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// ExternalReplicaSpec — 외부 Redis/Valkey primary 에서 단방향 복제.
type ExternalReplicaSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// +optional
	Host string `json:"host,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=6379
	// +optional
	Port int32 `json:"port,omitempty"`
	// +optional
	Auth *ExternalReplicaAuthSpec `json:"auth,omitempty"`
}

type ExternalReplicaAuthSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// +optional
	PasswordSecretRef *corev1.SecretKeySelector `json:"passwordSecretRef,omitempty"`
}

// NetworkPolicySpec — opt-in NetworkPolicy. 기본 deny + 같은 인스턴스 pod 간 6379/16379 허용.
//
// v1alpha2 변경 (Plan §2 D2, ADR-0035): AutoCreate 필드 신규 추가.
// Enabled 와 의미 분리:
//   - Enabled: NetworkPolicy *정책 사용 여부* (사용자 의도).
//   - AutoCreate: operator 가 *NP 리소스 자동 생성/관리* 책임을 가질지.
//
// AutoCreate=false + Enabled=true 시 사용자가 외부 NP 관리 책임
// (Calico GlobalNetworkPolicy / Cilium ClusterwideNetworkPolicy 등).
// secure-by-default 보존을 위해 default true.
type NetworkPolicySpec struct {
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// AutoCreate — operator 의 NP 리소스 자동 생성/관리 책임 토글
	// (Plan §2 D2, ADR-0035 — supersedes ADR-0057 가 정의한 자동 생성
	// 강제 동작). default=true 유지로 secure-by-default 보존.
	//
	// nil (legacy 호환): true 로 처리 — v1alpha1 동작 유지.
	// false: operator 가 NP 리소스 생성/갱신/삭제 안 함. 사용자가 외부
	// 관리 책임 (Calico / Cilium / Antrea 등 cluster-wide policy
	// engine 사용 시나리오 지원).
	//
	// +kubebuilder:default:=true
	// +optional
	AutoCreate *bool `json:"autoCreate,omitempty"`

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

// AutoscalingSpec — operator-managed HorizontalPodAutoscaler v2 (ADR-0027).
// v1alpha1 동일 구조 — type-only module.
type AutoscalingSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=15
	// +optional
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=15
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=70
	// +optional
	TargetCPUUtilizationPercentage int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetMemoryUtilizationPercentage int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
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
