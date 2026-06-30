/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	commonsservice "github.com/keiailab/keiailab-commons/pkg/service"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// 조립(ObjectMeta + Spec)은 keiailab-commons/pkg/service.Build 에 위임. 포트 구성
// (clientPorts) + ServiceSpec 옵션 병합 + 서비스 종류 선택은 valkey 도메인 잔류.

// BuildHeadlessService — pod-to-pod stable DNS (StatefulSet 필수).
//
// tlsEnabled=true 시 TLS-port (6380) 를 추가 expose — Valkey 의 port (6379 plain) /
// tls-port (6380 TLS) 분리 모델 정합.
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
	return commonsservice.Build(commonsservice.Params{
		Name:           HeadlessServiceName(crName),
		Namespace:      namespace,
		Labels:         labels,
		Annotations:    maps.Clone(opts.Annotations),
		Selector:       SelectorLabels(crName),
		Ports:          ports,
		Headless:       true, // ClusterIP None + PublishNotReadyAddresses (cluster init/replication)
		IPFamilyPolicy: opts.IPFamilyPolicy,
		IPFamilies:     opts.IPFamilies,
	})
}

// BuildMetricsService — exporter sidecar(:9121) 전용 ClusterIP Service.
func BuildMetricsService(crName, namespace string) *corev1.Service {
	return commonsservice.Build(commonsservice.Params{
		Name:      MetricsServiceName(crName),
		Namespace: namespace,
		Labels:    CommonLabels(crName, "valkey-metrics"),
		Selector:  SelectorLabels(crName),
		Ports: []corev1.ServicePort{
			{Name: "metrics", Port: PortMetrics, TargetPort: intstr.FromInt(PortMetrics), Protocol: corev1.ProtocolTCP},
		},
	})
}

// MetricsServiceLabels — ServiceMonitor 가 selector 로 사용할 라벨.
func MetricsServiceLabels(crName string) map[string]string {
	return CommonLabels(crName, "valkey-metrics")
}

// BuildClientService — 외부 클라이언트용 ClusterIP Service.
func BuildClientService(
	crName, namespace string,
	tlsEnabled bool,
	serviceSpec ...*cachev1alpha1.ServiceSpec,
) *corev1.Service {
	opts := firstServiceSpec(serviceSpec)
	labels := CommonLabels(crName, "valkey")
	maps.Copy(labels, opts.Labels)
	return commonsservice.Build(commonsservice.Params{
		Name:           ClientServiceName(crName),
		Namespace:      namespace,
		Labels:         labels,
		Annotations:    maps.Clone(opts.Annotations),
		Selector:       SelectorLabels(crName),
		Ports:          clientPorts(tlsEnabled),
		Type:           opts.Type, // "" → commons 가 ClusterIP 기본 적용
		IPFamilyPolicy: opts.IPFamilyPolicy,
		IPFamilies:     opts.IPFamilies,
	})
}

// BuildPrimaryService — Replication primary-only ClusterIP Service.
//
// selector 에 role=primary 라벨 포함 → 컨트롤러(reconcilePrimaryPodLabels)가
// Status.CurrentPrimary pod 에 부여한 LabelValkeyRole=primary 와 매칭. 쓰기 클라이언트가
// 안정 이름(<cr>-primary)으로 접속하면 항상 현재 master 도달. failover 시 relabel →
// endpoints controller 가 즉시 추종.
func BuildPrimaryService(
	crName, namespace string,
	tlsEnabled bool,
	serviceSpec ...*cachev1alpha1.ServiceSpec,
) *corev1.Service {
	opts := firstServiceSpec(serviceSpec)
	labels := CommonLabels(crName, "valkey-primary")
	maps.Copy(labels, opts.Labels)
	return commonsservice.Build(commonsservice.Params{
		Name:           PrimaryServiceName(crName),
		Namespace:      namespace,
		Labels:         labels,
		Annotations:    maps.Clone(opts.Annotations),
		Selector:       PrimarySelectorLabels(crName),
		Ports:          clientPorts(tlsEnabled),
		Type:           corev1.ServiceTypeClusterIP,
		IPFamilyPolicy: opts.IPFamilyPolicy,
		IPFamilies:     opts.IPFamilies,
	})
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
