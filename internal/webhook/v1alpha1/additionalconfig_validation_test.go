/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// validateAdditionalConfig 회귀 보호 — spec.additionalConfig 는 valkey.conf
// 템플릿의 .Extra range 로 `{{ $k }} {{ $v }}` 형태로 *escape 없이* 렌더된다
// (internal/assets/valkey.conf.tmpl L70-72). 따라서 key/value 에 개행이 들어가면
// 임의 directive 주입이 가능하고, operator 가 관리하는 보안/토폴로지 directive
// (requirepass / tls-* / cluster-enabled 등) 를 덮어쓰면 silent 보안 우회가 된다.
// 본 테스트는 webhook 이 이 주입/override 를 admission 단계에서 차단함을 보증한다.

package v1alpha1

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestValidateAdditionalConfig(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec", "additionalConfig")

	t.Run("nil map → ok", func(t *testing.T) {
		t.Parallel()
		if errs := validateAdditionalConfig(path, nil); len(errs) > 0 {
			t.Errorf("nil additionalConfig → expected no error, got %v", errs)
		}
	})

	t.Run("정상 directive → ok", func(t *testing.T) {
		t.Parallel()
		cfg := map[string]string{
			"maxmemory":        "1gb",
			"maxmemory-policy": "allkeys-lru",
		}
		if errs := validateAdditionalConfig(path, cfg); len(errs) > 0 {
			t.Errorf("정상 directive → expected no error, got %v", errs)
		}
	})

	t.Run("빈 key → reject", func(t *testing.T) {
		t.Parallel()
		cfg := map[string]string{"": "1gb"}
		errs := validateAdditionalConfig(path, cfg)
		if len(errs) == 0 {
			t.Fatal("빈 key → expected error")
		}
		if !strings.Contains(errs[0].Error(), "non-empty") {
			t.Errorf("error message: %v", errs[0])
		}
	})

	t.Run("key 에 공백 포함 → reject (directive 토큰 위반)", func(t *testing.T) {
		t.Parallel()
		cfg := map[string]string{"max memory": "1gb"}
		errs := validateAdditionalConfig(path, cfg)
		if len(errs) == 0 {
			t.Fatal("공백 포함 key → expected error")
		}
	})

	t.Run("key 에 개행 포함 → reject (directive 주입)", func(t *testing.T) {
		t.Parallel()
		cfg := map[string]string{"maxmemory\nrequirepass hacked": "1gb"}
		errs := validateAdditionalConfig(path, cfg)
		if len(errs) == 0 {
			t.Fatal("개행 포함 key → expected error")
		}
	})

	t.Run("value 에 개행 포함 → reject (directive 주입)", func(t *testing.T) {
		t.Parallel()
		// value 개행 → 한 줄에 추가 directive 주입 (예: requirepass 무력화).
		cfg := map[string]string{"maxmemory": "1gb\nrequirepass \"\""}
		errs := validateAdditionalConfig(path, cfg)
		if len(errs) == 0 {
			t.Fatal("개행 포함 value → expected error")
		}
		if !strings.Contains(errs[0].Error(), "newline") {
			t.Errorf("error message: %v", errs[0])
		}
	})

	t.Run("value 에 캐리지리턴 포함 → reject", func(t *testing.T) {
		t.Parallel()
		cfg := map[string]string{"maxmemory": "1gb\rrequirepass x"}
		errs := validateAdditionalConfig(path, cfg)
		if len(errs) == 0 {
			t.Fatal("CR 포함 value → expected error")
		}
	})

	t.Run("reserved directive override → reject (대소문자 무관)", func(t *testing.T) {
		t.Parallel()
		// operator 가 관리하는 보안 critical directive 덮어쓰기 = silent 우회.
		// valkey directive 는 대소문자 구분 안 하므로 "RequirePass" 도 차단.
		for _, key := range []string{
			"requirepass", "masterauth", "tls-port", "tls-cert-file",
			"bind", "port", "protected-mode", "cluster-enabled", "dir",
			"replicaof", "RequirePass", "TLS-Cert-File",
		} {
			cfg := map[string]string{key: "x"}
			errs := validateAdditionalConfig(path, cfg)
			if len(errs) == 0 {
				t.Errorf("reserved key %q → expected error", key)
				continue
			}
			if !strings.Contains(errs[0].Error(), "operator-managed") {
				t.Errorf("reserved key %q error message: %v", key, errs[0])
			}
		}
	})

	t.Run("reserved 아닌 인접 directive → ok", func(t *testing.T) {
		t.Parallel()
		// "maxmemory" 는 reserved 아님 (operator default 가 noeviction 만 강제,
		// maxmemory 자체는 사용자 조정 허용). false-positive 방지.
		cfg := map[string]string{"maxmemory": "2gb", "appendfsync": "always"}
		if errs := validateAdditionalConfig(path, cfg); len(errs) > 0 {
			t.Errorf("non-reserved directive → expected no error, got %v", errs)
		}
	})
}

// TestValidateValkeySpec_AdditionalConfig — validateValkeySpec 진입점이
// additionalConfig 검증을 실제로 호출하는지 (wiring) 보증.
func TestValidateValkeySpec_AdditionalConfig(t *testing.T) {
	t.Parallel()
	v := &cachev1alpha1.Valkey{}
	v.Spec.Mode = cachev1alpha1.ModeStandalone
	v.Spec.Replicas = 1
	v.Spec.AdditionalConfig = map[string]string{"requirepass": "pwned"}
	errs := validateValkeySpec(v)
	if len(errs) == 0 {
		t.Fatal("Valkey spec.additionalConfig reserved override → expected error via validateValkeySpec")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "operator-managed") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected operator-managed error in %v", errs)
	}
}

// TestValidateClusterSpec_AdditionalConfig — validateClusterSpec 진입점도 동일
// 검증을 호출하는지 (ValkeyCluster CR 도 동일 .Extra 렌더 경로 공유).
func TestValidateClusterSpec_AdditionalConfig(t *testing.T) {
	t.Parallel()
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = ptr.To[int32](1)
	vc.Spec.AdditionalConfig = map[string]string{"tls-port": "9999"}
	errs := validateClusterSpec(vc)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "operator-managed") {
			found = true
		}
	}
	if !found {
		t.Errorf("ValkeyCluster spec.additionalConfig reserved override → expected operator-managed error, got %v", errs)
	}
}
