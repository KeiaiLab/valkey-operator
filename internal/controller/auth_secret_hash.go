/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import corev1 "k8s.io/api/core/v1"

// authRotationMark — STS PodTemplate annotation(auth-secret-hash) 값. 인증 Secret
// 의 metadata.resourceVersion 을 그대로 쓴다. 비밀번호가 바뀌면 Secret 이 쓰이고
// resourceVersion 이 바뀐다 → PodTemplate 변경 → STS rolling update.
//
// 비밀번호에서 유도한 값(해시)을 쓰지 않는다 — annotation 은 Secret 보다 넓게
// 읽히고, 낮은 엔트로피 비밀번호의 해시는 오프라인 추측을 허용한다.
//
// 빈 password (Auth.Enabled=false 등) 시 빈 문자열 → annotation 미설정.
func authRotationMark(password, secretVersion string) string {
	if password == "" {
		return ""
	}
	return secretVersion
}

// createdVersion — SecretIfNotExists 가 방금 만든 Secret 의 resourceVersion.
// 이미 있어 build 가 불리지 않았으면 빈 문자열(다음 reconcile 이 채운다).
func createdVersion(built *corev1.Secret) string {
	if built == nil {
		return ""
	}
	return built.ResourceVersion
}
