/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func minimalSTSParams() STSParams {
	return STSParams{
		CRName:      "vk",
		Namespace:   "ns",
		Replicas:    1,
		Image:       "valkey:9.0.4",
		StorageSize: resource.MustParse("8Gi"),
	}
}

func TestPodTemplateAnnotations_empty_hash_returns_nil(t *testing.T) {
	p := minimalSTSParams()
	a := podTemplateAnnotations(p)
	if a != nil {
		t.Errorf("empty hash should produce no annotations, got %v", a)
	}
}

func TestPodTemplateAnnotations_with_hash_set(t *testing.T) {
	p := minimalSTSParams()
	p.AuthSecretHash = "abcdef1234"
	a := podTemplateAnnotations(p)
	if a[AnnotationAuthSecretHash] != "abcdef1234" {
		t.Errorf("annotation: %v", a)
	}
}

func TestBuildStatefulSet_annotation_propagates_to_PodTemplate(t *testing.T) {
	p := minimalSTSParams()
	p.AuthSecretHash = "deadbeef"
	sts := BuildStatefulSet(p)
	got := sts.Spec.Template.Annotations[AnnotationAuthSecretHash]
	if got != "deadbeef" {
		t.Errorf("Pod template annotation = %q, want deadbeef", got)
	}
}

func TestBuildStatefulSet_no_annotation_when_hash_empty(t *testing.T) {
	p := minimalSTSParams()
	sts := BuildStatefulSet(p)
	if _, ok := sts.Spec.Template.Annotations[AnnotationAuthSecretHash]; ok {
		t.Errorf("annotation should not be set when hash empty: %v",
			sts.Spec.Template.Annotations)
	}
}

// Defect ①: TLS cert hash annotation. cert-manager rotation 시 hash 변경 →
// PodTemplate 변경 → STS rolling update.
func TestPodTemplateAnnotations_tls_hash_set(t *testing.T) {
	p := minimalSTSParams()
	p.TLSCertHash = "tlshash123"
	a := podTemplateAnnotations(p)
	if a[AnnotationTLSCertHash] != "tlshash123" {
		t.Errorf("tls annotation: %v", a)
	}
}

func TestPodTemplateAnnotations_both_hashes_independent(t *testing.T) {
	p := minimalSTSParams()
	p.AuthSecretHash = "auth1"
	p.TLSCertHash = "tls1"
	a := podTemplateAnnotations(p)
	if a[AnnotationAuthSecretHash] != "auth1" || a[AnnotationTLSCertHash] != "tls1" {
		t.Errorf("both hashes should be set independently: %v", a)
	}
}

func TestBuildStatefulSet_tls_hash_propagates_and_changes(t *testing.T) {
	p := minimalSTSParams()
	p.TLSCertHash = "hashA"
	sts1 := BuildStatefulSet(p)
	if got := sts1.Spec.Template.Annotations[AnnotationTLSCertHash]; got != "hashA" {
		t.Fatalf("tls hash annotation = %q, want hashA", got)
	}
	// Secret content 변경 → hash 변경 → PodTemplate annotation 변경 (rolling update 트리거).
	p.TLSCertHash = "hashB"
	sts2 := BuildStatefulSet(p)
	if got := sts2.Spec.Template.Annotations[AnnotationTLSCertHash]; got != "hashB" {
		t.Fatalf("tls hash annotation after change = %q, want hashB", got)
	}
	if sts1.Spec.Template.Annotations[AnnotationTLSCertHash] == sts2.Spec.Template.Annotations[AnnotationTLSCertHash] {
		t.Error("PodTemplate annotation must differ when TLS secret content changes")
	}
}

func TestBuildStatefulSet_no_tls_annotation_when_hash_empty(t *testing.T) {
	p := minimalSTSParams()
	sts := BuildStatefulSet(p)
	if _, ok := sts.Spec.Template.Annotations[AnnotationTLSCertHash]; ok {
		t.Errorf("tls annotation should not be set when hash empty: %v",
			sts.Spec.Template.Annotations)
	}
}

// Defect ②: cluster-announce-ip. POD_IP downward API env + cluster mode command
// 에서 $POD_IP 셸 확장.
func TestBuildStatefulSet_pod_ip_env_via_downward_api(t *testing.T) {
	p := minimalSTSParams()
	sts := BuildStatefulSet(p)
	c := sts.Spec.Template.Spec.Containers[0]
	var podIPEnv *corev1.EnvVar
	for i := range c.Env {
		if c.Env[i].Name == "POD_IP" {
			podIPEnv = &c.Env[i]
		}
	}
	if podIPEnv == nil {
		t.Fatalf("POD_IP env missing: %v", c.Env)
	}
	if podIPEnv.ValueFrom == nil || podIPEnv.ValueFrom.FieldRef == nil ||
		podIPEnv.ValueFrom.FieldRef.FieldPath != "status.podIP" {
		t.Errorf("POD_IP must come from downward API status.podIP, got %+v", podIPEnv.ValueFrom)
	}
}

func TestBuildStatefulSet_cluster_announce_command(t *testing.T) {
	p := minimalSTSParams()
	p.ClusterMode = true
	sts := BuildStatefulSet(p)
	c := sts.Spec.Template.Spec.Containers[0]
	if len(c.Command) != 3 || c.Command[0] != "sh" || c.Command[1] != "-c" {
		t.Fatalf("cluster mode command should be sh -c ..., got %v", c.Command)
	}
	shCmd := c.Command[2]
	if !strings.Contains(shCmd, "valkey-server") {
		t.Errorf("command must exec valkey-server: %s", shCmd)
	}
	if !strings.Contains(shCmd, `--cluster-announce-ip "$POD_IP"`) {
		t.Errorf("cluster-announce-ip $POD_IP missing: %s", shCmd)
	}
	if !strings.Contains(shCmd, "--cluster-announce-port 6379") {
		t.Errorf("cluster-announce-port missing: %s", shCmd)
	}
	if !strings.Contains(shCmd, "--cluster-announce-bus-port 16379") {
		t.Errorf("cluster-announce-bus-port missing: %s", shCmd)
	}
	// config path 가 보존되어야 함.
	if !strings.Contains(shCmd, ConfigMapMountPath+"/"+ConfigFileName) {
		t.Errorf("config path lost in cluster command: %s", shCmd)
	}
}

func TestBuildStatefulSet_standalone_keeps_plain_command(t *testing.T) {
	p := minimalSTSParams()
	sts := BuildStatefulSet(p)
	c := sts.Spec.Template.Spec.Containers[0]
	if len(c.Command) != 1 || c.Command[0] != "valkey-server" {
		t.Errorf("standalone command should stay valkey-server, got %v", c.Command)
	}
}
