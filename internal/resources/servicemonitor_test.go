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
	"strings"
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestBuildServiceMonitor_disabled_returnsNil(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	if got := BuildServiceMonitorForCluster(vc); got != nil {
		t.Errorf("monitoring nil → expected nil, got %+v", got)
	}

	vc.Spec.Monitoring = &cachev1alpha1.MonitoringSpec{Enabled: false}
	if got := BuildServiceMonitorForCluster(vc); got != nil {
		t.Errorf("monitoring disabled → expected nil")
	}
}

func TestBuildServiceMonitor_enabled_default(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.Monitoring = &cachev1alpha1.MonitoringSpec{Enabled: true}

	got := BuildServiceMonitorForCluster(vc)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.GetName() != "vk" {
		t.Errorf("name: %q", got.GetName())
	}
	if got.GetNamespace() != "ns" {
		t.Errorf("namespace: %q", got.GetNamespace())
	}
	gvk := got.GroupVersionKind()
	if gvk.Group != "monitoring.coreos.com" || gvk.Kind != "ServiceMonitor" {
		t.Errorf("GVK: %v", gvk)
	}

	spec, _ := got.Object["spec"].(map[string]any)
	endpoints, _ := spec["endpoints"].([]any)
	if len(endpoints) != 1 {
		t.Fatalf("endpoints len: %d", len(endpoints))
	}
	ep := endpoints[0].(map[string]any)
	if ep["interval"] != "30s" {
		t.Errorf("default interval: %v", ep["interval"])
	}
	if ep["port"] != "metrics" {
		t.Errorf("port: %v", ep["port"])
	}
}

func TestBuildServiceMonitor_customInterval_extraLabels(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.Monitoring = &cachev1alpha1.MonitoringSpec{
		Enabled: true,
		ServiceMonitor: &cachev1alpha1.ServiceMonitorSpec{
			Interval: "15s",
			Labels:   map[string]string{"team": "platform", "release": "prom"},
		},
	}

	got := BuildServiceMonitorForCluster(vc)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	spec, _ := got.Object["spec"].(map[string]any)
	endpoints, _ := spec["endpoints"].([]any)
	ep := endpoints[0].(map[string]any)
	if ep["interval"] != "15s" {
		t.Errorf("interval: %v", ep["interval"])
	}

	labels := got.GetLabels()
	if labels["team"] != "platform" || labels["release"] != "prom" {
		t.Errorf("extra labels missing: %v", labels)
	}
}

// AutoFailover 디렉티브 통합 — ConfigMap 렌더 결과 검증.
func TestConfigMap_autoFailoverFalse_setsDirective(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.NodeTimeoutMillis = 15000
	vc.Spec.AutoFailover = false

	cm, err := BuildConfigMapForValkeyCluster(vc, "pwd")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	conf := cm.Data[ConfigFileName]
	if !strings.Contains(conf, "cluster-replica-no-failover yes") {
		t.Errorf("expected cluster-replica-no-failover yes directive, got:\n%s", conf)
	}
}

func TestConfigMap_autoFailoverTrue_omitsDirective(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.NodeTimeoutMillis = 15000
	vc.Spec.AutoFailover = true

	cm, err := BuildConfigMapForValkeyCluster(vc, "pwd")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	conf := cm.Data[ConfigFileName]
	if strings.Contains(conf, "cluster-replica-no-failover") {
		t.Errorf("autoFailover=true should omit directive, got:\n%s", conf)
	}
}

// Spec.Persistence.Mode 별 ConfigMap 디렉티브 검증.
func TestConfigMap_persistenceMode(t *testing.T) {
	cases := []struct {
		mode        string
		mustContain []string
		mustOmit    []string
	}{
		{
			mode:        "RDB",
			mustContain: []string{"save 3600 1 300 100 60 10000", "appendonly no"},
			mustOmit:    []string{"appendonly yes"},
		},
		{
			mode:        "AOF",
			mustContain: []string{`save ""`, "appendonly yes", "appendfsync everysec"},
			mustOmit:    []string{"save 3600"},
		},
		{
			mode:        "Both",
			mustContain: []string{"save 3600 1 300 100 60 10000", "appendonly yes", "appendfsync everysec"},
			mustOmit:    []string{"appendonly no"},
		},
		{
			mode:        "None",
			mustContain: []string{`save ""`, "appendonly no"},
			mustOmit:    []string{"appendonly yes"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			vc := &cachev1alpha1.ValkeyCluster{}
			vc.Name = "vk"
			vc.Namespace = "ns"
			vc.Spec.Shards = 3
			vc.Spec.ReplicasPerShard = 1
			vc.Spec.Persistence = &cachev1alpha1.PersistencePolicy{Mode: tc.mode}

			cm, err := BuildConfigMapForValkeyCluster(vc, "pwd")
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			conf := cm.Data[ConfigFileName]
			for _, s := range tc.mustContain {
				if !strings.Contains(conf, s) {
					t.Errorf("mode=%s: missing %q", tc.mode, s)
				}
			}
			for _, s := range tc.mustOmit {
				if strings.Contains(conf, s) {
					t.Errorf("mode=%s: should not contain %q", tc.mode, s)
				}
			}
		})
	}
}

// Spec.AdditionalConfig — operator-default 와 사용자 덮어쓰기 우선순위.
func TestConfigMap_additionalConfig(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.AdditionalConfig = map[string]string{
		"maxclients":               "10000",
		"hash-max-ziplist-entries": "512",
	}

	cm, err := BuildConfigMapForValkeyCluster(vc, "pwd")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	conf := cm.Data[ConfigFileName]
	if !strings.Contains(conf, "maxclients 10000") {
		t.Errorf("missing additional config maxclients")
	}
	if !strings.Contains(conf, "hash-max-ziplist-entries 512") {
		t.Errorf("missing additional config hash-max-ziplist-entries")
	}
}

func TestBuildMetricsService_separateLabels(t *testing.T) {
	svc := BuildMetricsService("vk", "ns")
	if svc.Name != "vk-metrics" {
		t.Errorf("name: %s", svc.Name)
	}
	if svc.Labels[LabelComponent] != "valkey-metrics" {
		t.Errorf("component label: %s", svc.Labels[LabelComponent])
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != PortMetrics {
		t.Errorf("ports: %+v", svc.Spec.Ports)
	}
}
