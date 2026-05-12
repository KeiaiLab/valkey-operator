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

/*
Copyright 2026 Keiailab.
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
	ports := []corev1.ServicePort{
		{Name: "client", Port: PortClient, TargetPort: intstr.FromInt(PortClient), Protocol: corev1.ProtocolTCP},
	}
	if tlsEnabled {
		ports = append(ports, corev1.ServicePort{
			Name: "client-tls", Port: PortTLS, TargetPort: intstr.FromInt(PortTLS), Protocol: corev1.ProtocolTCP,
		})
	}
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
			Annotations: copyStringMap(opts.Annotations),
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
	ports := []corev1.ServicePort{
		{Name: "client", Port: PortClient, TargetPort: intstr.FromInt(PortClient), Protocol: corev1.ProtocolTCP},
	}
	if tlsEnabled {
		ports = append(ports, corev1.ServicePort{
			Name: "client-tls", Port: PortTLS, TargetPort: intstr.FromInt(PortTLS), Protocol: corev1.ProtocolTCP,
		})
	}
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
			Annotations: copyStringMap(opts.Annotations),
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

func firstServiceSpec(specs []*cachev1alpha1.ServiceSpec) *cachev1alpha1.ServiceSpec {
	if len(specs) == 0 || specs[0] == nil {
		return &cachev1alpha1.ServiceSpec{}
	}
	return specs[0]
}
