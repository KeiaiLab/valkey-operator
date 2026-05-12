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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	commonsmonitoring "github.com/keiailab/operator-commons/pkg/monitoring"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// ServiceMonitorGVK — Prometheus Operator CRD 의 ServiceMonitor.
//
// 본 패키지는 prometheus-operator 의존성을 추가하지 않고 unstructured 로 다룬다 —
// 의존성 폭발 회피 + ServiceMonitor CRD 가 클러스터에 미설치 시 생성 시도가 NotFound
// 로 자연스럽게 fail-soft.
//
// iteration 23 (2026-05-07): operator-commons/pkg/monitoring 위임 — 3 operator
// 가 동일 builder 사용. 본 함수는 valkey 특화 옵션 (label / interval / nsScope)
// 만 결정 후 commons.NewServiceMonitor 호출.
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
		maps.Copy(extraLabels, sm.Labels)
	}

	labels := CommonLabels(vc.Name, "valkey-cluster")
	maps.Copy(labels, extraLabels)

	return commonsmonitoring.NewServiceMonitor(commonsmonitoring.ServiceMonitorParams{
		Name:              ServiceMonitorName(vc.Name),
		Namespace:         vc.Namespace,
		Labels:            labels,
		Selector:          MetricsServiceLabels(vc.Name),
		NamespaceSelector: []string{vc.Namespace},
		Port:              "metrics",
		Path:              "/metrics",
		Interval:          interval,
		ScrapeTimeout:     "10s",
		Scheme:            "http",
	})
}
