/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"time"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/autoupdate"
)

// applyAutoUpdate — AutoUpdate 정책을 spec.Version 에 in-memory 주입한다.
//
// 비활성이면 no-op. 활성 + MaintenanceWindow 안 + channel 제약 내 상위 버전 존재 시
// spec.Version.Version 을 effective version 으로 덮어쓴다. 주입된 version 은
// imageOrDefault → StatefulSet 이미지 + Status.Version 으로 자동 전파된다.
// applied=true 면 호출 측이 Event/log 를 남긴다.
//
// catalog 는 안전 버전 화이트리스트(cachev1alpha1.SupportedValkeyVersions),
// now 는 비교 기준 시각(UTC). major 상승은 internal/autoupdate 가 거른다.
func applyAutoUpdate(spec *cachev1alpha1.ValkeySpec, catalog []string, now time.Time) (applied bool) {
	if !spec.IsAutoUpdateEnabled() {
		return false
	}
	eff, ok := autoupdate.ResolveVersion(
		spec.Version.Version,
		spec.AutoUpdateChannel(),
		spec.AutoUpdate.MaintenanceWindow,
		catalog,
		now,
	)
	if !ok {
		return false
	}
	spec.Version.Version = eff
	return true
}

// applyAutoUpdateCluster — applyAutoUpdate 의 ValkeyCluster 판. 동일 정책을
// ValkeyClusterSpec.Version 에 in-memory 주입한다(샤드 전체 동일 버전).
func applyAutoUpdateCluster(spec *cachev1alpha1.ValkeyClusterSpec, catalog []string, now time.Time) (applied bool) {
	if !spec.IsAutoUpdateEnabled() {
		return false
	}
	eff, ok := autoupdate.ResolveVersion(
		spec.Version.Version,
		spec.AutoUpdateChannel(),
		spec.AutoUpdate.MaintenanceWindow,
		catalog,
		now,
	)
	if !ok {
		return false
	}
	spec.Version.Version = eff
	return true
}
