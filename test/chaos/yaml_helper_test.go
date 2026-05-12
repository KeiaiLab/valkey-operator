//go:build chaos
// +build chaos

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

// unstructured → YAML helper. chaos-mesh CRD 를 kubectl apply 로 적용하기 위한
// serializer.
package chaos

import (
	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func unstructuredToYAML(u *unstructured.Unstructured) (string, error) {
	b, err := yaml.Marshal(u.Object)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
