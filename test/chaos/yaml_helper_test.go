//go:build chaos
// +build chaos

/*
Copyright 2026 Keiailab.

unstructured → YAML helper. chaos-mesh CRD 를 kubectl apply 로 적용하기 위한
serializer.
*/

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
