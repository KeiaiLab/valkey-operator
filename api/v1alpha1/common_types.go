/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	commonsversion "github.com/keiailab/keiailab-commons/pkg/version"
)

// DefaultValkeyVersion / DefaultValkeyImage — defaulting webhook 이 spec.version
// 통째 누락 케이스에서 채워 넣는 값. CRD 의 kubebuilder default 와 동기화 유지.
const (
	DefaultValkeyVersion = "9.1.0"
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
// 9.1.0 = current stable, 9.0.4 = previous stable, 8.1.7/8.0.9 =
// milestone patch baseline. 8.1.6 은 기존 설치 호환 위해 유지.
var supportedValkeyList = commonsversion.MustList("8.0.9", "8.1.6", "8.1.7", "9.0.4", "9.1.0")

// SupportedValkeyVersions — 외부 노출 슬라이스 (chart values / docs / 기존 호환).
var SupportedValkeyVersions = supportedValkeyList.Strings()

// IsSupportedValkeyVersion — webhook validation 호출용 헬퍼. commons 위임.
func IsSupportedValkeyVersion(v string) bool {
	return supportedValkeyList.IsSupported(v)
}

// ValkeyVersion 은 Valkey 컨테이너 이미지 / 버전 지정.
type ValkeyVersion struct {
	// +kubebuilder:validation:Pattern=`^\d+\.\d+(\.\d+)?$`
	// +kubebuilder:default="9.1.0"
	Version string `json:"version"`

	// +kubebuilder:default="docker.io/valkey/valkey"
	// +optional
	Image string `json:"image,omitempty"`

	// ImageRef 는 registry/repository:tag@sha256:digest 전체 image reference.
	// 명시 시 Image + Version 조합보다 우선한다. 외부 chart 의 digest 고정
	// image 패턴을 운영에서 그대로 사용할 때 쓴다.
	// +optional
	ImageRef string `json:"imageRef,omitempty"`

	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// StorageSpec — Valkey 데이터 디렉터리(/data) 마운트용 PVC.
type StorageSpec struct {
	// Ephemeral — true 면 PVC 대신 emptyDir 를 사용한다. 외부 chart 의
	// persistence.enabled=false 패턴과 동등한 dev/test 모드. 운영 데이터
	// 보존이 필요하면 false 유지.
	// +kubebuilder:default=false
	// +optional
	Ephemeral bool `json:"ephemeral,omitempty"`

	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// +kubebuilder:default="8Gi"
	Size resource.Quantity `json:"size,omitempty"`

	// +kubebuilder:default="/data"
	DataDirPath string `json:"dataDirPath,omitempty"`

	// ExistingClaim — 사전 생성된 PVC 를 data volume 으로 사용한다. operator 는
	// 해당 PVC lifecycle 을 소유하지 않는다.
	// +optional
	ExistingClaim string `json:"existingClaim,omitempty"`

	// AccessModes — 신규 PVC volumeClaimTemplates accessModes. 미지정 시
	// ReadWriteOnce.
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// EncryptionRequired — true 시 reconciler 가 StorageClass parameters 를
	// 검사해 encryption 표시자 (encrypted=true / type=Premium_LRS 등) 가 없으면
	// Warning event 발행 (compliance audit). 강제 reject 는 아님 — 운영자가
	// 의도적 평문 SC 사용 시 차단되지 않도록.
	// +kubebuilder:default=false
	// +optional
	EncryptionRequired bool `json:"encryptionRequired,omitempty"`

	// EncryptionEnforce — EncryptionRequired=true + 본 필드 true 시 *audit 외에
	// 강제 reject*. StorageClass 가 encryption 미표시 → reconcile 실패 → STS 미생성
	// (data plane noncompliant 위험 사전 차단).
	//
	// 권장:
	//   dev/test: EncryptionRequired=false
	//   staging: EncryptionRequired=true (audit Warning only)
	//   production compliance: EncryptionRequired=true + EncryptionEnforce=true
	//
	// +kubebuilder:default=false
	// +optional
	EncryptionEnforce bool `json:"encryptionEnforce,omitempty"`
}

// SlowLogSpec — Valkey SLOWLOG 임계값 + 보존 entry 수.
//
// Threshold 보다 오래 걸린 명령은 SLOWLOG 에 기록 — `valkey-cli SLOWLOG GET` 로 조회.
// redis_exporter sidecar 가 자동으로 redis_slowlog_length metric 으로 노출.
type SlowLogSpec struct {
	// 단위: microseconds. 0 = SLOWLOG 비활성. -1 = 모든 명령 기록 (debug 만).
	// +kubebuilder:default=10000
	// +optional
	ThresholdMicros int64 `json:"thresholdMicros,omitempty"`

	// 보존 entry 수 — FIFO. 초과 시 가장 오래된 entry 폐기.
	// +kubebuilder:default=128
	// +optional
	MaxEntries int32 `json:"maxEntries,omitempty"`
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

	// RotationInterval — operator-managed 비밀번호 자동 로테이션 주기 ("720h" 형식,
	// time.ParseDuration). 빈 값=비활성. 첫 reconcile 은 baseline 만 기록(로테이션 X).
	// operator 가 *직접 회전 수행* — 자체 재설계(ESO 위임 대체), 외부 회전 반영과 별개.
	// +optional
	RotationInterval string `json:"rotationInterval,omitempty"`
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
	// +optional — AutoSelfSigned=true 일 때는 빈 값 허용 (operator 가 자동 생성).
	IssuerRef CertIssuerRef `json:"issuerRef,omitempty"`

	// AutoSelfSigned — true 시 operator 가 namespace-scope SelfSigned Issuer
	// (`<crName>-selfsigned`) 를 자동 생성하고 Certificate 가 그 Issuer 를
	// 참조한다. 외부 chart 의 `tls.autoGenerated` 패턴과 동등 — 사용자가
	// cert-manager 만 설치하면 IssuerRef 명시 불필요.
	//
	// 제약:
	//   - cert-manager CRD (Issuer + Certificate) 가 cluster 에 사전 설치 필요.
	//     미설치 시 unstructured apply 가 NoMatchError fail-soft (ADR-0010 패턴).
	//   - IssuerRef.Name 과 동시 명시 시 webhook reject (mutually exclusive).
	//   - 신뢰 root 가 *해당 namespace 의 SelfSigned* — 외부 CA chain 부재.
	//     production 외부 노출용은 IssuerRef 로 외부 Issuer (Let's Encrypt 등) 사용.
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
type PodSpec struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
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
	// Type 은 client Service 에만 적용된다. headless Service 는 항상 ClusterIP=None.
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

// ScalePolicy — 인스턴스 수 변경 가드 (ADR-0006).
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

// AutoscalingSpec — operator-managed HorizontalPodAutoscaler v2.
//
// ADR-0027 (Replication mode 만 — Cluster mode 는 slot 재분배 위험으로 미지원).
// Enabled=true 시 operator 가 ValkeyName 기반 HPA CRD 자동 생성/갱신/삭제.
//
// 제약:
//   - Mode=Replication 만 — Standalone (replicas=1) / Cluster mode 는 webhook reject.
//   - ScalePolicy.Deliberate 는 무시 (HPA 가 즉시 scale).
//   - Spec.Replicas 는 *default 값* — HPA 활성 시 reconciler 가 STS replicas 보존.
//   - PodDisruptionBudget.MinAvailable 은 MinReplicas 와 일관 권장 (webhook warn 대상).
type AutoscalingSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// HPA minReplicas — Replication mode 의 최소 replica 수 (primary 포함).
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=15
	// +optional
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// HPA maxReplicas — 최대 replica 수.
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=15
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// 평균 CPU 사용률 (resources.requests.cpu 대비 %).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=70
	// +optional
	TargetCPUUtilizationPercentage int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// 평균 메모리 사용률 (resources.requests.memory 대비 %). 0 = 미설정.
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
