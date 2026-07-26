/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// authConfPathFor — password 가 있으면 valkey.conf 가 include 할 인증 조각
// 파일의 절대 경로, 없으면 "" (auth 미사용).
func authConfPathFor(password string) string {
	if password == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", AuthConfMountPath, AuthConfFileName)
}

// RenderAuthConf — 인증 directive 만 담은 valkey config 조각.
//
// ConfigMap(valkey.conf) 에서 분리하는 이유: 내장 `view` ClusterRole 은 secrets
// 를 제외하지만 configmaps 는 get/list/watch 를 허용한다. 따라서 requirepass /
// masterauth 를 ConfigMap 에 렌더하면 읽기전용 계정이 마스터 자격을 획득한다.
// 이 조각은 Secret 볼륨으로만 pod 에 노출된다.
//
// externalReplicaEnabled=true 이면 이 인스턴스는 외부 primary 의 replica 이므로
// masterauth 는 외부 primary 의 password 여야 한다 (자기 requirepass 아님).
// externalReplicaPassword 가 비어 있으면 masterauth 를 생략한다 (무인증 primary).
func RenderAuthConf(password string, externalReplicaEnabled bool, externalReplicaPassword string) string {
	if password == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("# operator 생성 — 인증 directive 전용 조각 (valkey.conf 가 include).\n")
	b.WriteString(fmt.Sprintf("requirepass %s\n", password))
	if externalReplicaEnabled {
		if externalReplicaPassword != "" {
			b.WriteString(fmt.Sprintf("masterauth %s\n", externalReplicaPassword))
		}
	} else {
		b.WriteString(fmt.Sprintf("masterauth %s\n", password))
	}
	return b.String()
}

// BuildAuthConfSecret — RenderAuthConf 결과를 담은 Secret.
//
// password 가 비어 있으면 nil 을 반환한다 (auth 미사용 → 볼륨/ include 둘 다 없음).
func BuildAuthConfSecret(
	crName, namespace, component, password string,
	externalReplicaEnabled bool,
	externalReplicaPassword string,
) *corev1.Secret {
	conf := RenderAuthConf(password, externalReplicaEnabled, externalReplicaPassword)
	if conf == "" {
		return nil
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthConfSecretName(crName),
			Namespace: namespace,
			Labels:    CommonLabels(crName, component),
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{AuthConfFileName: conf},
	}
}
