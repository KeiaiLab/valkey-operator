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
package resources

import (
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/keiailab/operator-commons/pkg/security"
	commonstopology "github.com/keiailab/operator-commons/pkg/topology"

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
			Labels:      copyStringMap(p.Storage.Labels),
			Annotations: copyStringMap(p.Storage.Annotations),
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
	if storageClass != "" {
		dataPVC.Spec.StorageClassName = &storageClass
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

	podSpec := corev1.PodSpec{
		Containers: containers,
		Volumes:    volumes,
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

func podTemplateAnnotations(p STSParams) map[string]string {
	annotations := map[string]string{}
	if p.Pod != nil {
		maps.Copy(annotations, p.Pod.Annotations)
	}
	if p.AuthSecretHash == "" {
		if len(annotations) == 0 {
			return nil
		}
		return annotations
	}
	annotations[AnnotationAuthSecretHash] = p.AuthSecretHash
	return annotations
}

func podTemplateLabels(base map[string]string, pod *cachev1alpha1.PodSpec) map[string]string {
	labels := copyStringMap(base)
	if pod == nil {
		return labels
	}
	maps.Copy(labels, pod.Labels)
	return labels
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
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
