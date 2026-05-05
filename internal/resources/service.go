/*
Copyright 2026 Keiailab.
*/

package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BuildHeadlessService — pod-to-pod stable DNS (StatefulSet 필수).
func BuildHeadlessService(crName, namespace string, clusterMode bool) *corev1.Service {
	ports := []corev1.ServicePort{
		{Name: "client", Port: PortClient, TargetPort: intstr.FromInt(PortClient), Protocol: corev1.ProtocolTCP},
	}
	if clusterMode {
		ports = append(ports, corev1.ServicePort{
			Name: "cluster-bus", Port: PortClusterBus,
			TargetPort: intstr.FromInt(PortClusterBus), Protocol: corev1.ProtocolTCP,
		})
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HeadlessServiceName(crName),
			Namespace: namespace,
			Labels:    CommonLabels(crName, "valkey"),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                "None",
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
func BuildClientService(crName, namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClientServiceName(crName),
			Namespace: namespace,
			Labels:    CommonLabels(crName, "valkey"),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: SelectorLabels(crName),
			Ports: []corev1.ServicePort{
				{Name: "client", Port: PortClient, TargetPort: intstr.FromInt(PortClient), Protocol: corev1.ProtocolTCP},
			},
		},
	}
}
