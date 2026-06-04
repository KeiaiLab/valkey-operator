//go:build chaos
// +build chaos

/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
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
