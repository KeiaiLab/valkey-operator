/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// validateModules 회귀 보호 (PR-C6.2, ADR-0032/0062) — 공식 preset allow-list +
// 중복 + BYO 분기.

package v1alpha1

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestValidateModules(t *testing.T) {
	t.Parallel()
	p := field.NewPath("spec", "modules")

	t.Run("미등록 official preset → reject", func(t *testing.T) {
		t.Parallel()
		errs := validateModules(p, []cachev1alpha1.ModuleSpec{{Name: "redisearch"}})
		if len(errs) == 0 {
			t.Fatal("미등록 공식 preset 인데 에러 0 (ADR-0032 allow-list)")
		}
		if !strings.Contains(strings.ToLower(errs[0].Error()), "supported") {
			t.Errorf("err message = %v, want NotSupported", errs[0])
		}
	})

	t.Run("등록 official preset → ok", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"valkey-search", "valkey-json", "valkey-bloom"} {
			errs := validateModules(p, []cachev1alpha1.ModuleSpec{{Name: name}})
			if len(errs) > 0 {
				t.Errorf("%s → expected ok, got %v", name, errs)
			}
		}
	})

	t.Run("중복 name → Duplicate", func(t *testing.T) {
		t.Parallel()
		errs := validateModules(p, []cachev1alpha1.ModuleSpec{
			{Name: "valkey-search"}, {Name: "valkey-search"},
		})
		if len(errs) == 0 {
			t.Fatal("중복 module name 인데 에러 0")
		}
	})

	t.Run("BYO custom image → ok (allow-list 무관)", func(t *testing.T) {
		t.Parallel()
		errs := validateModules(p, []cachev1alpha1.ModuleSpec{
			{Name: "my-mod", Image: "example.com/m:1"},
		})
		if len(errs) > 0 {
			t.Errorf("BYO → expected ok, got %v", errs)
		}
	})

	t.Run("빈 modules → ok", func(t *testing.T) {
		t.Parallel()
		if errs := validateModules(p, nil); len(errs) > 0 {
			t.Errorf("빈 modules → expected no error, got %v", errs)
		}
	})
}

// TestValidateValkeySpec_modules_wired — validateValkeySpec 가 validateModules 를
// 실제로 호출하는지 (wiring) 확인.
func TestValidateValkeySpec_modules_wired(t *testing.T) {
	v := &cachev1alpha1.Valkey{}
	v.Spec.Mode = cachev1alpha1.ModeStandalone
	v.Spec.Replicas = 1
	v.Spec.Modules = []cachev1alpha1.ModuleSpec{{Name: "not-an-official-preset"}}

	errs := validateValkeySpec(v)
	if len(errs) == 0 {
		t.Error("미등록 module 인데 validateValkeySpec 에러 0 — validateModules 미연결")
	}
}
