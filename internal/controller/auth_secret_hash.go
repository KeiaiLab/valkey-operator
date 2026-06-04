/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashAuthSecret — password 의 SHA256 (hex) hash. STS PodTemplate annotation
// 으로 주입되어 *rotation 추적*. 사용자가 AuthSecret 의 password 값을 갱신하면
//
//   - 다음 reconcile 에서 ensureAuthSecret 가 새 값을 read
//   - 새 hash 가 STS Template 에 set
//   - STS controller 가 PodTemplate 변경 감지 → rolling update 시작
//   - 모든 pod 가 새 password 로 재시작
//
// 빈 password (Auth.Enabled=false 등) 시 빈 문자열 반환 → annotation 미설정.
//
// 보안: SHA256 의 *hash* 만 노출. 원본 password 는 K8s API 에 노출 안 됨.
// 다만 hash 가 같다면 password 가 같다는 정보는 leak — 운영상 위협 없음 (hash
// 가 다른 cluster 끼리 비교 의미 없음).
func hashAuthSecret(password string) string {
	if password == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}
