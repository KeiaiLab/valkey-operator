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

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// STSParams — Valkey / ValkeyCluster 양쪽이 공유하는 STS 빌드 파라미터.
type STSParams struct {
	CRName       string
	Namespace    string
	Replicas     int32
	Image        string
	PullPolicy   corev1.PullPolicy
	Resources    corev1.ResourceRequirements
	StorageClass string
	StorageSize  resource.Quantity
	PasswordRef  *corev1.SecretKeySelector
	ClusterMode  bool
	ExporterImg  string // 비어 있으면 sidecar 없음
	Pod          *cachev1alpha1.PodSpec
	// TLSSecretName — 비어 있지 않으면 해당 Secret (kubernetes.io/tls + ca.crt) 을
	// `/tls` 에 readOnly 마운트. configmap 의 tls-* 디렉티브 가 이 경로 를 참조.
	TLSSecretName string
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
			Env:       envFromPassword,
			Resources: p.Resources,
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
			Name:  "metrics",
			Image: p.ExporterImg,
			Env: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: "redis://127.0.0.1:6379"},
				{Name: "REDIS_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: p.PasswordRef}},
			},
			Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: PortMetrics, Protocol: corev1.ProtocolTCP}},
		})
	}

	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(p.CRName)},
				},
			},
		},
	}
	volumes = append(volumes, tlsVolumes(p.TLSSecretName)...)

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
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
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
