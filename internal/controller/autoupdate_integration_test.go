/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// applyAutoUpdate — reconcile 진입점에서 spec.Version 에 effective version 을
// in-memory 주입하는 통합 헬퍼 회귀 보호. 주입된 version 은 imageOrDefault →
// STatefulSet 이미지 + Status.Version 으로 자동 전파된다.

package controller

import (
	"testing"
	"time"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestApplyAutoUpdate(t *testing.T) {
	t.Parallel()
	catalog := []string{"9.0.4", "9.0.7", "9.1.2"}
	inWindow, _ := time.Parse("15:04", "03:00")  // 02:00-04:00 안
	outWindow, _ := time.Parse("15:04", "05:00") // 02:00-04:00 밖
	mk := func(ver string, au *cachev1alpha1.AutoUpdateSpec) *cachev1alpha1.ValkeySpec {
		return &cachev1alpha1.ValkeySpec{
			Version:    cachev1alpha1.ValkeyVersion{Version: ver},
			AutoUpdate: au,
		}
	}
	cases := []struct {
		name        string
		spec        *cachev1alpha1.ValkeySpec
		now         time.Time
		wantVer     string
		wantApplied bool
	}{
		{"비활성 → version 불변", mk("9.0.4", nil), inWindow, "9.0.4", false},
		{"활성 patch + window 안 → 9.0.7 주입", mk("9.0.4", &cachev1alpha1.AutoUpdateSpec{Enabled: true, MaintenanceWindow: "02:00-04:00"}), inWindow, "9.0.7", true},
		{"활성 + window 밖 → 불변", mk("9.0.4", &cachev1alpha1.AutoUpdateSpec{Enabled: true, MaintenanceWindow: "02:00-04:00"}), outWindow, "9.0.4", false},
		{"활성 minor 상시 → 9.1.2 주입", mk("9.0.4", &cachev1alpha1.AutoUpdateSpec{Enabled: true, Channel: "minor"}), outWindow, "9.1.2", true},
		{"이미 최신 → 불변", mk("9.1.2", &cachev1alpha1.AutoUpdateSpec{Enabled: true, Channel: "minor"}), inWindow, "9.1.2", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			applied := applyAutoUpdate(c.spec, catalog, c.now)
			if applied != c.wantApplied {
				t.Fatalf("applied: got %v, want %v", applied, c.wantApplied)
			}
			if c.spec.Version.Version != c.wantVer {
				t.Fatalf("version: got %q, want %q", c.spec.Version.Version, c.wantVer)
			}
		})
	}
}
