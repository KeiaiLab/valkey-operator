/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package v1alpha2

// AutoUpdateSpec — operator-managed 자동 버전 업데이트 정책.
//
// 안전 원칙: major 상승은 절대 자동화하지 않는다(데이터 호환성 — 운영자 명시 필요).
// 적용은 MaintenanceWindow 안에서만 일어나며, 적용 자체는 기존 버전 롤링 업데이트
// 경로를 그대로 탄다(추가 다운타임 없음). 결정 로직은 internal/autoupdate 공유.
type AutoUpdateSpec struct {
	// Enabled — 자동 버전 업데이트 활성화.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Channel — 자동 업데이트 허용 범위.
	//   "patch": 동일 major.minor 내 최신 patch (기본, 가장 보수적)
	//   "minor": 동일 major 내 최신 minor.patch
	// major 상승은 어떤 channel 에서도 자동화하지 않는다.
	// +kubebuilder:validation:Enum=patch;minor
	// +kubebuilder:default=patch
	// +optional
	Channel string `json:"channel,omitempty"`

	// MaintenanceWindow — 업데이트 적용 허용 시간대 "HH:MM-HH:MM"(UTC).
	// 빈 값이면 상시 허용. 자정 넘김(예: "22:00-02:00") 지원.
	// +optional
	MaintenanceWindow string `json:"maintenanceWindow,omitempty"`
}

// IsAutoUpdateEnabled — AutoUpdate 가 설정되고 Enabled 면 true.
func (s *ValkeySpec) IsAutoUpdateEnabled() bool {
	return s.AutoUpdate != nil && s.AutoUpdate.Enabled
}

// AutoUpdateChannel — 설정된 channel(미설정/빈 값이면 "patch").
func (s *ValkeySpec) AutoUpdateChannel() string {
	if s.AutoUpdate == nil || s.AutoUpdate.Channel == "" {
		return "patch"
	}
	return s.AutoUpdate.Channel
}

// IsAutoUpdateEnabled — AutoUpdate 가 설정되고 Enabled 면 true (ValkeyCluster).
func (s *ValkeyClusterSpec) IsAutoUpdateEnabled() bool {
	return s.AutoUpdate != nil && s.AutoUpdate.Enabled
}

// AutoUpdateChannel — 설정된 channel(미설정/빈 값이면 "patch") (ValkeyCluster).
func (s *ValkeyClusterSpec) AutoUpdateChannel() string {
	if s.AutoUpdate == nil || s.AutoUpdate.Channel == "" {
		return "patch"
	}
	return s.AutoUpdate.Channel
}
