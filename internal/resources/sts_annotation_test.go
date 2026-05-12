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
