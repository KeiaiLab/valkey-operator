/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 수동 major 버전 상승 차단 회귀 보호 — AutoUpdate 가 patch/minor 만 자동화하므로
// 운영자의 수동 major 상승은 admission 단계에서 거부한다(명시적 마이그레이션 필요).

package v1alpha1

import (
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestValkeyNoManualMajorUpgrade(t *testing.T) {
	t.Parallel()
	mk := func(ver string) *cachev1alpha1.Valkey {
		v := &cachev1alpha1.Valkey{}
		v.Spec.Version.Version = ver
		return v
	}
	t.Run("major 상승 9→10 → reject", func(t *testing.T) {
		t.Parallel()
		if errs := validateValkeyImmutable(mk("9.0.4"), mk("10.0.0")); len(errs) == 0 {
			t.Error("수동 major 상승 → expected reject")
		}
	})
	t.Run("patch 상승 9.0.4→9.0.5 → ok", func(t *testing.T) {
		t.Parallel()
		if errs := validateValkeyImmutable(mk("9.0.4"), mk("9.0.5")); len(errs) != 0 {
			t.Errorf("patch 상승 → expected ok, got %v", errs)
		}
	})
}

func TestValkeyClusterNoManualMajorUpgrade(t *testing.T) {
	t.Parallel()
	mk := func(ver string) *cachev1alpha1.ValkeyCluster {
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Spec.Version.Version = ver
		return vc
	}
	t.Run("major 상승 9→10 → reject", func(t *testing.T) {
		t.Parallel()
		if errs := validateClusterImmutable(mk("9.0.4"), mk("10.0.0")); len(errs) == 0 {
			t.Error("수동 major 상승 → expected reject")
		}
	})
	t.Run("minor 상승 9.0.4→9.1.0 → ok", func(t *testing.T) {
		t.Parallel()
		if errs := validateClusterImmutable(mk("9.0.4"), mk("9.1.0")); len(errs) != 0 {
			t.Errorf("minor 상승 → expected ok, got %v", errs)
		}
	})
}
