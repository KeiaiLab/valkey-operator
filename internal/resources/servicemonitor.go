/*
Copyright 2026 Keiailab.
*/

package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// ServiceMonitorGVK — Prometheus Operator CRD 의 ServiceMonitor.
//
// 본 패키지는 prometheus-operator 의존성을 추가하지 않고 unstructured 로 다룬다 —
// 의존성 폭발 회피 + ServiceMonitor CRD 가 클러스터에 미설치 시 생성 시도가 NotFound
// 로 자연스럽게 fail-soft.
var ServiceMonitorGVK = schema.GroupVersionKind{
	Group:   "monitoring.coreos.com",
	Version: "v1",
	Kind:    "ServiceMonitor",
}

// BuildServiceMonitorForCluster — Prometheus 가 metrics 를 스크랩하도록 ServiceMonitor
// 생성. metrics endpoint 는 client Service 의 :9121/metrics (redis_exporter sidecar).
func BuildServiceMonitorForCluster(vc *cachev1alpha1.ValkeyCluster) *unstructured.Unstructured {
	if vc.Spec.Monitoring == nil || !vc.Spec.Monitoring.Enabled {
		return nil
	}
	interval := "30s"
	extraLabels := map[string]string{}
	if sm := vc.Spec.Monitoring.ServiceMonitor; sm != nil {
		if sm.Interval != "" {
			interval = sm.Interval
		}
		for k, v := range sm.Labels {
			extraLabels[k] = v
		}
	}

	labels := CommonLabels(vc.Name, "valkey-cluster")
	for k, v := range extraLabels {
		labels[k] = v
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(ServiceMonitorGVK)
	u.SetName(ServiceMonitorName(vc.Name))
	u.SetNamespace(vc.Namespace)
	u.SetLabels(labels)

	// Service selector — metrics Service 만 매칭 (`...-metrics`, component=valkey-metrics).
	spec := map[string]any{
		"selector": map[string]any{
			"matchLabels": stringMap(MetricsServiceLabels(vc.Name)),
		},
		"namespaceSelector": map[string]any{
			"matchNames": []any{vc.Namespace},
		},
		"endpoints": []any{
			map[string]any{
				"port":          "metrics",
				"interval":      interval,
				"scheme":        "http",
				"path":          "/metrics",
				"scrapeTimeout": "10s",
			},
		},
	}
	u.Object["spec"] = spec
	return u
}

func stringMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
