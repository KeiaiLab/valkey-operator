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

// Package resources — Valkey / ValkeyCluster 가 사용하는 K8s 리소스 빌더.
// mongodb-operator/internal/resources/builder.go 의 함수형 builder 패턴 차용.
package resources

import (
	"fmt"

	commonslabels "github.com/keiailab/operator-commons/pkg/labels"
)

const (
	LabelAppName      = "app.kubernetes.io/name"
	LabelInstanceName = "app.kubernetes.io/instance"
	LabelComponent    = "app.kubernetes.io/component"
	LabelManagedBy    = "app.kubernetes.io/managed-by"
	LabelPartOf       = "app.kubernetes.io/part-of"
	LabelValkeyRole   = "valkey.keiailab.io/role" // primary | replica | cluster-node
	LabelValkeyShard  = "valkey.keiailab.io/shard"

	ManagedByValue = "valkey-operator"
	PartOfValue    = "valkey"

	PortClient     = 6379
	PortClusterBus = 16379
	PortTLS        = 6380
	PortMetrics    = 9121

	ConfigMapMountPath = "/etc/valkey"
	ConfigFileName     = "valkey.conf"
	DataDir            = "/data"
	TLSDir             = "/tls"
)

// CommonLabels — Valkey 인스턴스의 공통 라벨.
//
// iteration 29 (2026-05-07): operator-commons/pkg/labels v0.3.0 위임. 5-key
// app.kubernetes.io/* convention (mongodb / postgres 와 통일). PartOfValue 명시
// 하여 commons Set 의 PartOf 자동 omit 회피.
func CommonLabels(instanceName, component string) map[string]string {
	return commonslabels.Set{
		Name:      "valkey",
		Instance:  instanceName,
		Component: component,
		ManagedBy: ManagedByValue,
		PartOf:    PartOfValue,
	}.All()
}

// SelectorLabels — Service / PodDisruptionBudget 의 selector 용 (안정 라벨만).
// commons.Set.Selector() 패턴 차용 — version 제외 (k8s immutable selector field).
// 단 valkey 는 component 도 selector 에서 제외 (cluster mode 의 다중 component
// 가 같은 service 매칭하기 위함) — Set.Selector() 보다 더 좁은 set.
func SelectorLabels(instanceName string) map[string]string {
	return map[string]string{
		LabelAppName:      "valkey",
		LabelInstanceName: instanceName,
	}
}

// StatefulSetName — Valkey CR name → STS name.
func StatefulSetName(crName string) string     { return crName }
func HeadlessServiceName(crName string) string { return crName + "-headless" }
func ClientServiceName(crName string) string   { return crName }
func MetricsServiceName(crName string) string  { return crName + "-metrics" }
func ConfigMapName(crName string) string       { return crName + "-config" }
func DefaultSecretName(crName string) string   { return crName + "-auth" }
func PDBName(crName string) string             { return crName }
func NetworkPolicyName(crName string) string   { return crName }
func ServiceMonitorName(crName string) string  { return crName }
func PodFQDN(crName string, ordinal int, namespace string) string {
	return fmt.Sprintf("%s-%d.%s.%s.svc", crName, ordinal, HeadlessServiceName(crName), namespace)
}
