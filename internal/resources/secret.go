/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"crypto/rand"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const SecretPasswordKey = "password"

// GeneratePassword — 32 byte random hex string (64 chars).
func GeneratePassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// BuildAuthSecret — 자동 생성 password Secret.
func BuildAuthSecret(crName, namespace, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultSecretName(crName),
			Namespace: namespace,
			Labels:    CommonLabels(crName, "auth"),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{SecretPasswordKey: []byte(password)},
	}
}
