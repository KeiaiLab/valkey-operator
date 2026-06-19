/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// BuildHeadlessService — pod-to-pod stable DNS (StatefulSet 필수).
//
// tlsEnabled=true 시 TLS-port (6380) 를 추가 expose — Valkey 의 port (6379 plain) /
// tls-port (6380 TLS) 분리 모델 정합. 외부 client (또는 inter-pod TLS replication)
// 가 6380 으로 connect 가능. tls-auth-clients=yes 운영 시 client cert 필요.
func BuildHeadlessService(
	crName, namespace string,
	clusterMode, tlsEnabled bool,
	serviceSpec ...*cachev1alpha1.ServiceSpec,
) *corev1.Service {
	ports := clientPorts(tlsEnabled)
	if clusterMode {
		ports = append(ports, corev1.ServicePort{
			Name: "cluster-bus", Port: PortClusterBus,
			TargetPort: intstr.FromInt(PortClusterBus), Protocol: corev1.ProtocolTCP,
		})
	}
	opts := firstServiceSpec(serviceSpec)
	labels := CommonLabels(crName, "valkey")
	maps.Copy(labels, opts.Labels)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        HeadlessServiceName(crName),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: maps.Clone(opts.Annotations),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                "None",
			IPFamilyPolicy:           opts.IPFamilyPolicy,
			IPFamilies:               opts.IPFamilies,
			PublishNotReadyAddresses: true, // cluster init / replication 시 미준비 pod DNS 필요
			Selector:                 SelectorLabels(crName),
			Ports:                    ports,
		},
	}
}

// BuildMetricsService — exporter sidecar(:9121) 전용 ClusterIP Service.
//
// ServiceMonitor 가 selector 매칭으로 자동 스크랩. 별도 Service 로 분리한 이유:
// ServiceMonitor 의 endpoint discovery 는 *Service 의 ports* 를 보지만 client/headless
// Service 에 metrics 포트를 추가하면 client traffic 과 의미가 섞임 + 사용자 혼란.
func BuildMetricsService(crName, namespace string) *corev1.Service {
	labels := CommonLabels(crName, "valkey-metrics")
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetricsServiceName(crName),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: SelectorLabels(crName),
			Ports: []corev1.ServicePort{
				{Name: "metrics", Port: PortMetrics, TargetPort: intstr.FromInt(PortMetrics), Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

// MetricsServiceLabels — ServiceMonitor 가 selector 로 사용할 라벨.
func MetricsServiceLabels(crName string) map[string]string {
	return CommonLabels(crName, "valkey-metrics")
}

// BuildClientService — 외부 클라이언트용 ClusterIP Service.
//
// tlsEnabled=true 시 client-tls (6380) 도 expose — 외부 client 가 TLS connection
// 사용 가능 (rediss:// scheme + tls-auth-clients=yes 시 client cert 필요).
func BuildClientService(
	crName, namespace string,
	tlsEnabled bool,
	serviceSpec ...*cachev1alpha1.ServiceSpec,
) *corev1.Service {
	ports := clientPorts(tlsEnabled)
	opts := firstServiceSpec(serviceSpec)
	serviceType := opts.Type
	if serviceType == "" {
		serviceType = corev1.ServiceTypeClusterIP
	}
	labels := CommonLabels(crName, "valkey")
	maps.Copy(labels, opts.Labels)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ClientServiceName(crName),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: maps.Clone(opts.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:           serviceType,
			IPFamilyPolicy: opts.IPFamilyPolicy,
			IPFamilies:     opts.IPFamilies,
			Selector:       SelectorLabels(crName),
			Ports:          ports,
		},
	}
}

// BuildPrimaryService — Replication primary-only ClusterIP Service.
//
// selector 에 role=primary 라벨 포함 → 컨트롤러(reconcilePrimaryPodLabels)가
// Status.CurrentPrimary pod 에 부여한 LabelValkeyRole=primary 와 매칭. 쓰기 클라이언트
// (세션/락/queue)가 이 안정 이름(<cr>-primary)으로 접속하면 *항상 현재 master* 도달 →
// RR Client Service(master+replica 양쪽 endpoint)의 write-to-replica READONLY 갭
// (phpredis 무한 세션락 행 등) 해소. failover 시 컨트롤러가 relabel → kube endpoints
// controller 가 즉시 endpoints 갱신 (polling/수동 Endpoints 불요 — 자동 추종).
func BuildPrimaryService(
	crName, namespace string,
	tlsEnabled bool,
	serviceSpec ...*cachev1alpha1.ServiceSpec,
) *corev1.Service {
	ports := clientPorts(tlsEnabled)
	opts := firstServiceSpec(serviceSpec)
	labels := CommonLabels(crName, "valkey-primary")
	maps.Copy(labels, opts.Labels)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        PrimaryServiceName(crName),
			Namespace:   namespace,
			Labels:      labels,
			Annotations: maps.Clone(opts.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:           corev1.ServiceTypeClusterIP,
			IPFamilyPolicy: opts.IPFamilyPolicy,
			IPFamilies:     opts.IPFamilies,
			Selector:       PrimarySelectorLabels(crName),
			Ports:          ports,
		},
	}
}

func firstServiceSpec(specs []*cachev1alpha1.ServiceSpec) *cachev1alpha1.ServiceSpec {
	if len(specs) == 0 || specs[0] == nil {
		return &cachev1alpha1.ServiceSpec{}
	}
	return specs[0]
}

// clientPorts — client (+ TLS) 포트 리스트. Headless / Client Service 공용.
func clientPorts(tlsEnabled bool) []corev1.ServicePort {
	ports := []corev1.ServicePort{
		{Name: "client", Port: PortClient, TargetPort: intstr.FromInt(PortClient), Protocol: corev1.ProtocolTCP},
	}
	if tlsEnabled {
		ports = append(ports, corev1.ServicePort{
			Name: "client-tls", Port: PortTLS, TargetPort: intstr.FromInt(PortTLS), Protocol: corev1.ProtocolTCP,
		})
	}
	return ports
}
