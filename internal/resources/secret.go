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
