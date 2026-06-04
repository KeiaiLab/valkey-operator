/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 모듈 init-container 빌더 회귀 보호 (ADR-0032, PR-C6.2).
// 공식 BSD preset(Name only)은 ResolveModulePreset, BYO(Image)는 그대로. 외부 Redis
// Stack(preset 아님 + Image 없음)은 resolve 불가로 skip(webhook 이 거부).

package resources

import (
	"strings"
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestBuildModuleInitContainers(t *testing.T) {
	t.Parallel()

	t.Run("공식 preset valkey-search → init-container + emptyDir + loadArg", func(t *testing.T) {
		t.Parallel()
		ics, vol, args := BuildModuleInitContainers([]cachev1alpha1.ModuleSpec{{Name: "valkey-search"}})
		if len(ics) != 1 {
			t.Fatalf("init-container 1 expected, got %d", len(ics))
		}
		if vol.Name == "" || vol.EmptyDir == nil {
			t.Fatalf("emptyDir volume 필요: %+v", vol)
		}
		if len(args) != 1 || !strings.Contains(args[0], "valkey-search.so") {
			t.Fatalf("loadArg /modules/valkey-search.so 기대: %v", args)
		}
		if len(ics[0].VolumeMounts) == 0 {
			t.Fatalf("init-container 가 modules volume 을 mount 해야")
		}
	})

	t.Run("빈 modules → no-op", func(t *testing.T) {
		t.Parallel()
		ics, _, args := BuildModuleInitContainers(nil)
		if len(ics) != 0 || len(args) != 0 {
			t.Fatalf("빈 modules: ics=%d args=%d", len(ics), len(args))
		}
	})

	t.Run("custom Image BYO", func(t *testing.T) {
		t.Parallel()
		ics, _, _ := BuildModuleInitContainers([]cachev1alpha1.ModuleSpec{{Name: "mymod", Image: "reg/mymod:1"}})
		if len(ics) != 1 || ics[0].Image != "reg/mymod:1" {
			t.Fatalf("custom image init-container 기대: %+v", ics)
		}
	})

	t.Run("외부 Redis Stack(preset 아님 + Image 없음) → skip", func(t *testing.T) {
		t.Parallel()
		ics, _, args := BuildModuleInitContainers([]cachev1alpha1.ModuleSpec{{Name: "redisearch"}})
		if len(ics) != 0 || len(args) != 0 {
			t.Fatalf("redisearch 는 resolve 불가 → skip, got ics=%d args=%d", len(ics), len(args))
		}
	})

	t.Run("LoadModuleArgs 가 loadArg 에 부착", func(t *testing.T) {
		t.Parallel()
		_, _, args := BuildModuleInitContainers([]cachev1alpha1.ModuleSpec{
			{Name: "valkey-search", LoadModuleArgs: []string{"--reader-threads", "4"}},
		})
		if len(args) != 1 || !strings.Contains(args[0], "--reader-threads 4") {
			t.Fatalf("LoadModuleArgs 부착 기대: %v", args)
		}
	})
}
