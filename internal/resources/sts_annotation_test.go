/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"testing"

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
