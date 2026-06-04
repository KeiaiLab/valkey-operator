/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// validateModules 회귀 보호 (ADR-0032) — 외부 Redis Stack 거부 + 공식 BSD preset 허용.
// 사용자 의도: 외부 Redis Stack 모듈이 아닌 *자체 재설계* BSD module 만 turnkey 허용.

package v1alpha1

import (
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func standaloneValkeyWithModules(mods ...cachev1alpha1.ModuleSpec) *cachev1alpha1.Valkey {
	v := &cachev1alpha1.Valkey{}
	v.Spec.Mode = cachev1alpha1.ModeStandalone
	v.Spec.Replicas = 1
	v.Spec.Modules = mods
	return v
}

func TestValidateModules(t *testing.T) {
	t.Parallel()

	t.Run("official preset (valkey-search) → ok", func(t *testing.T) {
		t.Parallel()
		v := standaloneValkeyWithModules(cachev1alpha1.ModuleSpec{Name: "valkey-search"})
		if errs := validateValkeySpec(v); len(errs) > 0 {
			t.Errorf("valkey-search preset → expected no error, got %v", errs)
		}
	})

	t.Run("external Redis Stack name (redisearch) → reject", func(t *testing.T) {
		t.Parallel()
		v := standaloneValkeyWithModules(cachev1alpha1.ModuleSpec{Name: "redisearch"})
		if errs := validateValkeySpec(v); len(errs) == 0 {
			t.Error("redisearch (external Redis Stack) → expected reject (ADR-0032)")
		}
	})

	t.Run("external Redis Stack name with BYO image → still reject", func(t *testing.T) {
		t.Parallel()
		// BYO image 로 라이선스를 회피하려는 우회 — 이름이 외부 Stack 이면 image 무관 거부.
		v := standaloneValkeyWithModules(cachev1alpha1.ModuleSpec{Name: "rejson", Image: "ghcr.io/x/y:1"})
		if errs := validateValkeySpec(v); len(errs) == 0 {
			t.Error("rejson 이름은 BYO image 동반이어도 거부 (라이선스 회피 차단)")
		}
	})

	t.Run("unknown preset without image → reject (allow-list)", func(t *testing.T) {
		t.Parallel()
		v := standaloneValkeyWithModules(cachev1alpha1.ModuleSpec{Name: "unknown-mod"})
		if errs := validateValkeySpec(v); len(errs) == 0 {
			t.Error("비공식 preset 이름(image 없음) → allow-list reject")
		}
	})

	t.Run("BYO external Redis Stack image → reject", func(t *testing.T) {
		t.Parallel()
		v := standaloneValkeyWithModules(cachev1alpha1.ModuleSpec{Name: "custom-mod", Image: "redislabs/redisearch:latest"})
		if errs := validateValkeySpec(v); len(errs) == 0 {
			t.Error("BYO redislabs image → reject (ADR-0032)")
		}
	})

	t.Run("BYO arbitrary in-house image → ok", func(t *testing.T) {
		t.Parallel()
		v := standaloneValkeyWithModules(cachev1alpha1.ModuleSpec{Name: "custom-mod", Image: "ghcr.io/keiailab/mymod:1.0"})
		if errs := validateValkeySpec(v); len(errs) > 0 {
			t.Errorf("임의 사내 BYO image → expected ok, got %v", errs)
		}
	})

	t.Run("no modules → ok", func(t *testing.T) {
		t.Parallel()
		v := standaloneValkeyWithModules()
		if errs := validateValkeySpec(v); len(errs) > 0 {
			t.Errorf("modules 없음 → expected ok, got %v", errs)
		}
	})
}
