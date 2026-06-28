/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"fmt"
	"maps"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/keiailab/keiailab-commons/pkg/probes"
	"github.com/keiailab/keiailab-commons/pkg/security"
	"github.com/keiailab/keiailab-commons/pkg/storageclass"
	commonstopology "github.com/keiailab/keiailab-commons/pkg/topology"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// STSParams — Valkey / ValkeyCluster 양쪽이 공유하는 STS 빌드 파라미터.
type STSParams struct {
	CRName               string
	Namespace            string
	Replicas             int32
	Image                string
	PullPolicy           corev1.PullPolicy
	Resources            corev1.ResourceRequirements
	StorageClass         string
	StorageSize          resource.Quantity
	Storage              cachev1alpha1.StorageSpec
	PasswordRef          *corev1.SecretKeySelector
	ClusterMode          bool
	ExporterImg          string                      // 비어 있으면 sidecar 없음
	ExporterResources    corev1.ResourceRequirements // metrics sidecar 의 resources (cycle 21 stop hook 15차 — IaC drift 0 진정 도달)
	Pod                  *cachev1alpha1.PodSpec
	RevisionHistoryLimit *int32
	// TLSSecretName — 비어 있지 않으면 해당 Secret (kubernetes.io/tls + ca.crt) 을
	// `/tls` 에 readOnly 마운트. configmap 의 tls-* 디렉티브 가 이 경로 를 참조.
	TLSSecretName string

	// AuthSecretHash — AuthSecret data 의 SHA256 (hex) hash. PodTemplate 의
	// annotation `cache.keiailab.io/auth-secret-hash` 로 주입되어, hash 변경 시
	// STS rolling update 가 자동 트리거 (pod 들이 새 password 로 재시작).
	// 빈 문자열이면 annotation 미설정 (rotation 추적 비활성).
	AuthSecretHash string

	// TLSCertHash — TLS Secret data (tls.crt + tls.key + ca.crt) 의 SHA256 (hex)
	// hash. PodTemplate 의 annotation `cache.keiailab.io/tls-cert-hash` 로 주입되어,
	// cert-manager 가 cert/CA 를 재발급(rotation)해 Secret data 가 바뀌면 hash 변경
	// → STS rolling update 가 자동 트리거 → pod 들이 새 cert 를 마운트한 채 재시작.
	// valkey-server 가 디스크 cert 를 시작 시점에만 read 하는 결함을 우회한다.
	// 빈 문자열이면 annotation 미설정 (TLS 미활성 또는 Secret 미준비).
	TLSCertHash string

	// Modules — Valkey module 목록. 비어 있지 않으면 BuildModuleInitContainers 가
	// init-container(.so 를 공유 emptyDir 로 cp) + volume + --loadmodule args 를 생성한다.
	Modules []cachev1alpha1.ModuleSpec
}

// BuildStatefulSet — Valkey 데이터 STS.
func BuildStatefulSet(p STSParams) *appsv1.StatefulSet {
	labels := CommonLabels(p.CRName, "valkey")
	selector := SelectorLabels(p.CRName)

	storageSize := p.StorageSize
	if !p.Storage.Size.IsZero() {
		storageSize = p.Storage.Size
	}
	if storageSize.IsZero() {
		storageSize = resource.MustParse("8Gi")
	}

	accessModes := p.Storage.AccessModes
	if len(accessModes) == 0 {
		accessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}
	dataPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "data",
			Labels:      maps.Clone(p.Storage.Labels),
			Annotations: maps.Clone(p.Storage.Annotations),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: storageSize},
			},
		},
	}
	storageClass := p.StorageClass
	if p.Storage.StorageClassName != "" {
		storageClass = p.Storage.StorageClassName
	}
	// commons storageclass.Normalize — 빈 값 = nil (cluster default StorageClass).
	dataPVC.Spec.StorageClassName = storageclass.Normalize(storageClass)

	// POD_IP — downward API 로 pod 의 실제 IP 주입. cluster mode 에서
	// cluster-announce-ip 로 사용 (Defect ②: pod 재시작 후 새 IP 를 gossip 에
	// 광고하지 못해 멤버십이 깨지는 결함 방지). 비-cluster 모드에서도 무해.
	podEnv := []corev1.EnvVar{{
		Name:      "POD_IP",
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}},
	}}
	if p.PasswordRef != nil {
		podEnv = append(podEnv, corev1.EnvVar{
			Name:      "VALKEY_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: p.PasswordRef},
		})
	}

	pingArgs := []string{"-c", `valkey-cli -h 127.0.0.1 -p 6379 ${VALKEY_PASSWORD:+-a "$VALKEY_PASSWORD"} ping | grep -q PONG`}

	containers := []corev1.Container{
		{
			Name:            "valkey",
			Image:           p.Image,
			ImagePullPolicy: p.PullPolicy,
			Command:         []string{"valkey-server"},
			Args:            []string{fmt.Sprintf("%s/%s", ConfigMapMountPath, ConfigFileName)},
			Ports: []corev1.ContainerPort{
				{Name: "client", ContainerPort: PortClient, Protocol: corev1.ProtocolTCP},
			},
			Env:             podEnv,
			Resources:       p.Resources,
			SecurityContext: buildRestrictedContainerSecurityContext(),
			VolumeMounts: append([]corev1.VolumeMount{
				{Name: "data", MountPath: DataDir},
				{Name: "config", MountPath: ConfigMapMountPath},
			}, tlsVolumeMounts(p.TLSSecretName)...),
			// commons probes.Builder — Exec ping probe. FailureThreshold 는 builder
			// 기본값 3 동일. SuccessThreshold 는 builder 가 명시 1 (이전 인라인은
			// 미설정 0 — API server 가 동일 값 1 로 default 하므로 라이브 무변경).
			LivenessProbe: probes.New().
				Exec(append([]string{"sh"}, pingArgs...)...).
				InitialDelay(20 * time.Second).
				Period(10 * time.Second).
				Timeout(5 * time.Second).
				Build(),
			ReadinessProbe: probes.New().
				Exec(append([]string{"sh"}, pingArgs...)...).
				InitialDelay(5 * time.Second).
				Period(5 * time.Second).
				Timeout(3 * time.Second).
				Build(),
		},
	}
	if p.Pod != nil && len(p.Pod.ExtraEnv) > 0 {
		containers[0].Env = append(containers[0].Env, p.Pod.ExtraEnv...)
	}
	if p.ClusterMode {
		containers[0].Ports = append(containers[0].Ports, corev1.ContainerPort{
			Name: "cluster-bus", ContainerPort: PortClusterBus, Protocol: corev1.ProtocolTCP,
		})
	}

	if p.ExporterImg != "" {
		containers = append(containers, corev1.Container{
			Name:            "metrics",
			Image:           p.ExporterImg,
			SecurityContext: buildRestrictedContainerSecurityContext(),
			// cycle 21 stop hook 15차: ExporterResources 를 sidecar 에 명시. 빈
			// ResourceRequirements 면 K8s default (Burstable QoS) — 이전 동작과 동등.
			Resources: p.ExporterResources,
			Env: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: "redis://127.0.0.1:6379"},
				{Name: "REDIS_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: p.PasswordRef}},
			},
			Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: PortMetrics, Protocol: corev1.ProtocolTCP}},
		})
	}

	tlsVols := tlsVolumes(p.TLSSecretName)
	volumes := make([]corev1.Volume, 0, 1+len(tlsVols))
	volumes = append(volumes, corev1.Volume{
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(p.CRName)},
			},
		},
	})
	if p.Storage.Ephemeral {
		volumes = append(volumes, corev1.Volume{
			Name:         "data",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		})
	} else if p.Storage.ExistingClaim != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: p.Storage.ExistingClaim,
				},
			},
		})
	}
	volumes = append(volumes, tlsVols...)

	// 모듈 init-container + emptyDir + --loadmodule (ADR-0032, BuildModuleInitContainers).
	// init-container 가 .so 를 공유 emptyDir 로 cp 하고, valkey container 가 적재한다.
	moduleInits, moduleVol, moduleLoadArgs := BuildModuleInitContainers(p.Modules)
	if len(moduleInits) > 0 {
		for _, la := range moduleLoadArgs {
			containers[0].Args = append(containers[0].Args, "--loadmodule")
			containers[0].Args = append(containers[0].Args, strings.Fields(la)...)
		}
		containers[0].VolumeMounts = append(containers[0].VolumeMounts,
			corev1.VolumeMount{Name: ModuleVolumeName, MountPath: moduleMountPath})
		volumes = append(volumes, moduleVol)
	}

	// Cluster mode: cluster-announce-ip 를 pod 의 실제 IP 로 광고 (Defect ②).
	// cluster-announce-ip 는 startup 시점 literal 이어야 하므로 ConfigMap 이 아니라
	// 컨테이너 command 에서 $POD_IP 를 셸 확장한다. CLI flag 가 valkey.conf 보다
	// 우선하므로 ConfigMap 의 기타 directive 는 그대로 유효.
	// announce-port=client(6379/tls 6380 무관 — 6379 는 평문/내부 dial 기준),
	// announce-bus-port=tls-cluster 시 tls-port+10000(16380) / 아니면 port+10000(16379)
	// — 실제 bus listen 포트와 일치해야 gossip 성립 (clusterAnnounceCommand 참조).
	if p.ClusterMode {
		containers[0].Command, containers[0].Args = clusterAnnounceCommand(containers[0].Args, p.TLSSecretName != "")
	}

	podSpec := corev1.PodSpec{
		Containers:     containers,
		InitContainers: moduleInits,
		Volumes:        volumes,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: new(true),
			RunAsUser:    new(int64(999)),
			FSGroup:      new(int64(999)),
		},
		TerminationGracePeriodSeconds: new(int64(30)),
	}
	if p.Pod != nil {
		if p.Pod.SecurityContext != nil {
			podSpec.SecurityContext = p.Pod.SecurityContext
		}
		podSpec.Affinity = p.Pod.Affinity
		podSpec.Tolerations = p.Pod.Tolerations
		podSpec.NodeSelector = p.Pod.NodeSelector
		podSpec.PriorityClassName = p.Pod.PriorityClassName
		podSpec.ServiceAccountName = p.Pod.ServiceAccountName
		podSpec.TopologySpreadConstraints = p.Pod.TopologySpreadConstraints
		podSpec.ImagePullSecrets = p.Pod.ImagePullSecrets
		podSpec.HostAliases = p.Pod.HostAliases
		if p.Pod.TerminationGracePeriodSeconds != nil {
			podSpec.TerminationGracePeriodSeconds = p.Pod.TerminationGracePeriodSeconds
		}
		if p.Pod.ContainerSecurityContext != nil {
			for i := range podSpec.Containers {
				podSpec.Containers[i].SecurityContext = p.Pod.ContainerSecurityContext
			}
		}
		if p.Pod.LivenessProbe != nil {
			podSpec.Containers[0].LivenessProbe = p.Pod.LivenessProbe
		}
		if p.Pod.ReadinessProbe != nil {
			podSpec.Containers[0].ReadinessProbe = p.Pod.ReadinessProbe
		}
		if p.Pod.StartupProbe != nil {
			podSpec.Containers[0].StartupProbe = p.Pod.StartupProbe
		}
	}
	// HA out-of-box default: Spec.Pod.TopologySpreadConstraints 미명시 + replicas >= 2
	// 시 zone + node 2-축 spread 자동 주입. 단일 노드/zone 장애에 의한 동시 장애 방지.
	// MaxSkew=1, ScheduleAnyway (강제 not-Schedulable 회피). commons pkg/topology
	// 의 Defaulted 가 본 로직을 보존 (default WithMinReplicas(2)).
	podSpec.TopologySpreadConstraints = commonstopology.Defaulted(
		podSpec.TopologySpreadConstraints, p.Replicas, selector,
	)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName(p.CRName),
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:            new(p.Replicas),
			ServiceName:         HeadlessServiceName(p.CRName),
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector:            &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podTemplateLabels(labels, p.Pod),
					Annotations: podTemplateAnnotations(p),
				},
				Spec: podSpec,
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: new(int32(0)),
				},
			},
		},
	}
	if p.RevisionHistoryLimit != nil {
		sts.Spec.RevisionHistoryLimit = p.RevisionHistoryLimit
	}
	if !p.Storage.Ephemeral && p.Storage.ExistingClaim == "" {
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{dataPVC}
	}
	return sts
}

// AnnotationAuthSecretHash — pod template annotation 키. AuthSecret rotation
// 추적용. 값 변경 시 STS RollingUpdate 가 자동 발동되어 모든 pod 가 새 password
// 로 재시작.
const AnnotationAuthSecretHash = "cache.keiailab.io/auth-secret-hash"

// AnnotationTLSCertHash — pod template annotation 키. TLS Secret(cert/CA)
// rotation 추적용. cert-manager 가 cert 를 재발급해 hash 가 바뀌면 STS
// RollingUpdate 가 자동 발동되어 모든 pod 가 새 cert 로 재시작.
const AnnotationTLSCertHash = "cache.keiailab.io/tls-cert-hash"

func podTemplateAnnotations(p STSParams) map[string]string {
	annotations := map[string]string{}
	if p.Pod != nil {
		maps.Copy(annotations, p.Pod.Annotations)
	}
	if p.AuthSecretHash != "" {
		annotations[AnnotationAuthSecretHash] = p.AuthSecretHash
	}
	if p.TLSCertHash != "" {
		annotations[AnnotationTLSCertHash] = p.TLSCertHash
	}
	if len(annotations) == 0 {
		return nil
	}
	return annotations
}

func podTemplateLabels(base map[string]string, pod *cachev1alpha1.PodSpec) map[string]string {
	labels := maps.Clone(base)
	if pod == nil {
		return labels
	}
	maps.Copy(labels, pod.Labels)
	return labels
}

// PodSecurity "restricted" 정책을 만족하는 컨테이너 SecurityContext.
// restore init container, upload/download job container 등 4 곳에서 동일 정의가
// 인라인 중복되던 것을 commons 단일 진실원으로 위임. iteration 8: operator-
// commons/pkg/security 채택 — 3 operator (mongodb / valkey / postgres) 동일 패턴.
//
// 회귀 가드: PodSecurity Admission "restricted" 네임스페이스에서 pod 거부 시
// 데이터 가용성 사고 가능 — commons 패키지 100% line coverage 단위 test 보장.
func buildRestrictedContainerSecurityContext() *corev1.SecurityContext {
	// ReadOnlyRootFilesystem 활성 (modern security baseline 마지막 layer).
	// valkey-server 는 /data (PVC) + /etc/valkey (configmap) + /tmp
	// (emptyDir) 외 rootfs 쓰기 부재.
	return security.RestrictedContainer(
		security.WithRunAsUser(999),
		security.WithReadOnlyRootFilesystem(true),
	)
}

// clusterAnnounceCommand — cluster mode 컨테이너 command 를 셸 래핑하여
// cluster-announce-ip 를 $POD_IP(downward API)로 확장한다. 입력은 기존
// valkey-server args (config path + --loadmodule 등). 반환은 (command, args)
// 쌍: command 는 `sh -c '...'`, args 는 nil (셸 명령에 모두 인라인).
//
// cluster-announce-ip 는 valkey-server 시작 시점에 literal 이어야 하므로
// ConfigMap directive 로 표현할 수 없다 (pod IP 는 ConfigMap 렌더 시점 미확정).
// CLI flag 가 valkey.conf 보다 우선하므로 ConfigMap 의 cluster-* directive 는
// 유효하게 유지된다. `exec` 로 PID 1 을 valkey-server 로 교체해 signal/graceful
// shutdown 의미를 보존한다.
func clusterAnnounceCommand(serverArgs []string, tlsEnabled bool) ([]string, []string) {
	// cluster-bus 는 tls-cluster 시 tls-port(6380)+10000, 아니면 port(6379)+10000 에
	// listen 한다. announce-bus-port 가 실제 listen 포트와 어긋나면 peer 가 닫힌 포트로
	// gossip 을 시도해 모든 노드가 fail?/disconnected → cluster_state:fail, slots_ok:0.
	// (TLS 클러스터에서 PortClusterBus(16379) 를 그대로 announce 하던 회귀 사고.)
	busPort := PortClient + 10000
	if tlsEnabled {
		busPort = PortTLS + 10000
	}
	parts := append([]string{"exec", "valkey-server"}, serverArgs...)
	parts = append(parts,
		"--cluster-announce-ip", `"$POD_IP"`,
		"--cluster-announce-port", fmt.Sprintf("%d", PortClient),
		"--cluster-announce-bus-port", fmt.Sprintf("%d", busPort),
	)
	return []string{"sh", "-c", strings.Join(parts, " ")}, nil
}

// PortIntOrString — helper for Probe/Service ports.
func PortIntOrString(p int32) intstr.IntOrString { return intstr.FromInt(int(p)) }

// tlsVolumeMounts / tlsVolumes — TLSSecretName 이 비어 있지 않을 때만 활성화.
// /tls 에 readOnly 로 cert-manager 가 발급한 Secret 마운트.
func tlsVolumeMounts(secretName string) []corev1.VolumeMount {
	if secretName == "" {
		return nil
	}
	return []corev1.VolumeMount{{Name: "tls", MountPath: TLSDir, ReadOnly: true}}
}

func tlsVolumes(secretName string) []corev1.Volume {
	if secretName == "" {
		return nil
	}
	return []corev1.Volume{{
		Name: "tls",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretName,
				DefaultMode: new(int32(0o400)),
			},
		},
	}}
}
