/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package secretrotation — operator-managed 자체 시크릿 로테이션 결정 로직.
//
// 기존 정책(ESO 위임)을 대체하는 자체 재설계: Valkey auth 비밀번호를 RotationInterval
// 주기로 operator 가 직접 회전한다. 결정(언제 회전)은 순수 함수로, 실제 회전(Secret 갱신
// + CONFIG SET requirepass 무중단 반영)은 controller 가 수행한다.
package secretrotation

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// ShouldRotate — lastRotation 이후 interval 이상 경과했으면 true.
//
//	interval <= 0:        로테이션 비활성
//	lastRotation.IsZero(): baseline 미설정 → 첫 reconcile 은 시각 기록만(로테이션 X, 안전)
//
// 경계(now-last == interval)는 회전 대상에 포함한다.
func ShouldRotate(lastRotation time.Time, interval time.Duration, now time.Time) bool {
	if interval <= 0 {
		return false
	}
	if lastRotation.IsZero() {
		return false
	}
	return now.Sub(lastRotation) >= interval
}

// GeneratePassword — 32바이트 암호학적 난수 → URL-safe base64(패딩 제거).
// auth_secret_hash 가 STS rollout 트리거를 위해 해시를 추적하므로, 회전 시 이 값으로
// Secret 을 갱신하면 reconcile 이 무중단 롤링을 수행한다.
func GeneratePassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
