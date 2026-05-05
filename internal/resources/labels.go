/*
Copyright 2026 Keiailab.
*/

// Package resources — Valkey / ValkeyCluster 가 사용하는 K8s 리소스 빌더.
// mongodb-operator/internal/resources/builder.go 의 함수형 builder 패턴 차용.
package resources

import "fmt"

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
func CommonLabels(instanceName, component string) map[string]string {
	return map[string]string{
		LabelAppName:      "valkey",
		LabelInstanceName: instanceName,
		LabelComponent:    component,
		LabelManagedBy:    ManagedByValue,
		LabelPartOf:       PartOfValue,
	}
}

// SelectorLabels — Service / PodDisruptionBudget 의 selector 용 (안정 라벨만).
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
