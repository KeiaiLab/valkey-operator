/*
Copyright 2026 Keiailab.
*/

package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/keiailab/operator-commons/pkg/security"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// STSParams — Valkey / ValkeyCluster 양쪽이 공유하는 STS 빌드 파라미터.
type STSParams struct {
	CRName            string
	Namespace         string
	Replicas          int32
	Image             string
	PullPolicy        corev1.PullPolicy
	Resources         corev1.ResourceRequirements
	StorageClass      string
	StorageSize       resource.Quantity
	PasswordRef       *corev1.SecretKeySelector
	ClusterMode       bool
	ExporterImg       string                      // 비어 있으면 sidecar 없음
	ExporterResources corev1.ResourceRequirements // metrics sidecar 의 resources (cycle 21 stop hook 15차 — IaC drift 0 진정 도달)
	Pod               *cachev1alpha1.PodSpec
	// TLSSecretName — 비어 있지 않으면 해당 Secret (kubernetes.io/tls + ca.crt) 을
	// `/tls` 에 readOnly 마운트. configmap 의 tls-* 디렉티브 가 이 경로 를 참조.
	TLSSecretName string

	// AuthSecretHash — AuthSecret data 의 SHA256 (hex) hash. PodTemplate 의
	// annotation `cache.keiailab.io/auth-secret-hash` 로 주입되어, hash 변경 시
	// STS rolling update 가 자동 트리거 (pod 들이 새 password 로 재시작).
	// 빈 문자열이면 annotation 미설정 (rotation 추적 비활성).
	AuthSecretHash string
}

// BuildStatefulSet — Valkey 데이터 STS.
func BuildStatefulSet(p STSParams) *appsv1.StatefulSet {
	labels := CommonLabels(p.CRName, "valkey")
	selector := SelectorLabels(p.CRName)

	storageSize := p.StorageSize
	if storageSize.IsZero() {
		storageSize = resource.MustParse("8Gi")
	}

	dataPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: storageSize},
			},
		},
	}
	if p.StorageClass != "" {
		dataPVC.Spec.StorageClassName = &p.StorageClass
	}

	envFromPassword := []corev1.EnvVar{}
	if p.PasswordRef != nil {
		envFromPassword = append(envFromPassword, corev1.EnvVar{
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
			Env:             envFromPassword,
			Resources:       p.Resources,
			SecurityContext: buildRestrictedContainerSecurityContext(),
			VolumeMounts: append([]corev1.VolumeMount{
				{Name: "data", MountPath: DataDir},
				{Name: "config", MountPath: ConfigMapMountPath},
			}, tlsVolumeMounts(p.TLSSecretName)...),
			LivenessProbe: &corev1.Probe{
				ProbeHandler:        corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: append([]string{"sh"}, pingArgs...)}},
				InitialDelaySeconds: 20,
				PeriodSeconds:       10,
				TimeoutSeconds:      5,
				FailureThreshold:    3,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler:        corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: append([]string{"sh"}, pingArgs...)}},
				InitialDelaySeconds: 5,
				PeriodSeconds:       5,
				TimeoutSeconds:      3,
				FailureThreshold:    3,
			},
		},
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
	volumes = append(volumes, tlsVols...)

	podSpec := corev1.PodSpec{
		Containers: containers,
		Volumes:    volumes,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: ptrBool(true),
			RunAsUser:    ptrInt64(999),
			FSGroup:      ptrInt64(999),
		},
		TerminationGracePeriodSeconds: ptrInt64(30),
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
		if p.Pod.ContainerSecurityContext != nil {
			for i := range podSpec.Containers {
				podSpec.Containers[i].SecurityContext = p.Pod.ContainerSecurityContext
			}
		}
	}
	// HA out-of-box default: Spec.Pod.TopologySpreadConstraints 미명시 + replicas >= 2
	// 시 zone + node 2-축 spread 자동 주입. 단일 노드/zone 장애에 의한 동시 장애 방지.
	// MaxSkew=1, ScheduleAnyway (강제 not-Schedulable 회피).
	if len(podSpec.TopologySpreadConstraints) == 0 && p.Replicas >= 2 {
		podSpec.TopologySpreadConstraints = defaultTopologySpread(selector)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName(p.CRName),
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:            ptrInt32(p.Replicas),
			ServiceName:         HeadlessServiceName(p.CRName),
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector:            &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podTemplateAnnotations(p),
				},
				Spec: podSpec,
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{dataPVC},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: ptrInt32(0),
				},
			},
		},
	}
	return sts
}

func ptrBool(b bool) *bool    { return &b }
func ptrInt32(i int32) *int32 { return &i }
func ptrInt64(i int64) *int64 { return &i }

// defaultTopologySpread — Spec.Pod.TopologySpreadConstraints 미명시 + replicas >= 2
// 시 자동 주입되는 zone + node 2-축 spread. ScheduleAnyway 로 강제 unschedulable
// 회피 (single-zone cluster 환경 호환).
//
// MaxSkew=1: 동일 zone/node 에 +1 이상 몰리는 것 방지.
// 사용자가 Spec.Pod.TopologySpreadConstraints 명시하면 그쪽이 우선 (override).
func defaultTopologySpread(selector map[string]string) []corev1.TopologySpreadConstraint {
	labelSelector := &metav1.LabelSelector{MatchLabels: selector}
	return []corev1.TopologySpreadConstraint{
		{
			MaxSkew:           1,
			TopologyKey:       "topology.kubernetes.io/zone",
			WhenUnsatisfiable: corev1.ScheduleAnyway,
			LabelSelector:     labelSelector,
		},
		{
			MaxSkew:           1,
			TopologyKey:       "kubernetes.io/hostname",
			WhenUnsatisfiable: corev1.ScheduleAnyway,
			LabelSelector:     labelSelector,
		},
	}
}

// AnnotationAuthSecretHash — pod template annotation 키. AuthSecret rotation
// 추적용. 값 변경 시 STS RollingUpdate 가 자동 발동되어 모든 pod 가 새 password
// 로 재시작.
const AnnotationAuthSecretHash = "cache.keiailab.io/auth-secret-hash"

func podTemplateAnnotations(p STSParams) map[string]string {
	if p.AuthSecretHash == "" {
		return nil
	}
	return map[string]string{AnnotationAuthSecretHash: p.AuthSecretHash}
}

// PodSecurity "restricted" 정책을 만족하는 컨테이너 SecurityContext.
// restore init container, upload/download job container 등 4 곳에서 동일 정의가
// 인라인 중복되던 것을 commons 단일 진실원으로 위임. iteration 8: operator-
// commons/pkg/security 채택 — 3 operator (mongodb / valkey / postgres) 동일 패턴.
//
// 회귀 가드: PodSecurity Admission "restricted" 네임스페이스에서 pod 거부 시
// 데이터 가용성 사고 가능 — commons 패키지 100% line coverage 단위 test 보장.
func buildRestrictedContainerSecurityContext() *corev1.SecurityContext {
	// argos cycle 21 stop hook 34차: ReadOnlyRootFilesystem 활성 (modern security
	// baseline 마지막 layer). valkey-server 는 /data (PVC) + /etc/valkey
	// (configmap) + /tmp (emptyDir, 별 cycle) 외 rootfs 쓰기 부재.
	return security.RestrictedContainer(
		security.WithRunAsUser(999),
		security.WithReadOnlyRootFilesystem(true),
	)
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
				DefaultMode: ptrInt32(0o400),
			},
		},
	}}
}
