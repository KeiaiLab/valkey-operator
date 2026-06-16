/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// TestResolveModule_공식_search_번들이미지_so경로 — 공식 preset 은 valkey-bundle
// 이미지 + allow-list .so 경로로 resolve.
func TestResolveModule_공식_search_번들이미지_so경로(t *testing.T) {
	rm := resolveModule(cachev1alpha1.ModuleSpec{Name: "valkey-search"}, "9.0.4")

	if rm.initImage != "docker.io/valkey/valkey-bundle:9.0.4" {
		t.Errorf("initImage = %q, want valkey-bundle:9.0.4", rm.initImage)
	}
	if rm.srcPath != "/usr/lib/valkey/libsearch.so" {
		t.Errorf("srcPath = %q, want /usr/lib/valkey/libsearch.so", rm.srcPath)
	}
	if rm.destPath() != "/modules/valkey-search.so" {
		t.Errorf("destPath = %q, want /modules/valkey-search.so", rm.destPath())
	}
}

// TestResolveModule_BYO_사용자이미지 — Image 명시 시 사용자 이미지 + 컨벤션 경로.
func TestResolveModule_BYO_사용자이미지(t *testing.T) {
	rm := resolveModule(cachev1alpha1.ModuleSpec{
		Name:           "my-mod",
		Image:          "example.com/mod:1",
		LoadModuleArgs: []string{"--threads", "4"},
	}, "9.0.4")

	if rm.initImage != "example.com/mod:1" {
		t.Errorf("initImage = %q, want example.com/mod:1 (BYO)", rm.initImage)
	}
	if rm.srcPath != "/modules/my-mod.so" {
		t.Errorf("srcPath = %q, want /modules/my-mod.so (BYO 컨벤션)", rm.srcPath)
	}
	if rm.destPath() != "/modules/my-mod.so" {
		t.Errorf("destPath = %q, want /modules/my-mod.so", rm.destPath())
	}
}

// TestModuleLoadArgs_순서_flat — --loadmodule <so> <args...> 시퀀스가 순서대로.
func TestModuleLoadArgs_순서_flat(t *testing.T) {
	got := moduleLoadArgs([]cachev1alpha1.ModuleSpec{
		{Name: "valkey-search"},
		{Name: "my-mod", Image: "x", LoadModuleArgs: []string{"--foo", "bar"}},
	})
	want := []string{
		"--loadmodule", "/modules/valkey-search.so",
		"--loadmodule", "/modules/my-mod.so", "--foo", "bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("moduleLoadArgs = %v, want %v", got, want)
	}
}

// TestModuleLoadArgs_빈입력_nil — 빈 modules 면 nil (no-op 회귀 가드).
func TestModuleLoadArgs_빈입력_nil(t *testing.T) {
	if got := moduleLoadArgs(nil); got != nil {
		t.Errorf("moduleLoadArgs(nil) = %v, want nil", got)
	}
}

// TestBuildModuleInitContainers_혼합_restrictedSC — official+BYO 각 1 init
// container, restricted SecurityContext + stage mount.
func TestBuildModuleInitContainers_혼합_restrictedSC(t *testing.T) {
	cs := buildModuleInitContainers([]cachev1alpha1.ModuleSpec{
		{Name: "valkey-search"},
		{Name: "byo", Image: "example.com/byo:1"},
	}, "9.0.4", corev1.PullIfNotPresent)

	if len(cs) != 2 {
		t.Fatalf("init containers = %d, want 2", len(cs))
	}
	// 첫 컨테이너 = 공식 search, restricted SC.
	sc := cs[0].SecurityContext
	if sc == nil || sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Errorf("init[0] RunAsNonRoot != true (PSS restricted, ADR-0036)")
	}
	if sc == nil || sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Errorf("init[0] ReadOnlyRootFilesystem != true")
	}
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Errorf("init[0] AllowPrivilegeEscalation != false")
	}
	// stage dir mount 존재.
	var mounted bool
	for _, vm := range cs[0].VolumeMounts {
		if vm.Name == "modules" && vm.MountPath == "/staged-modules" {
			mounted = true
		}
	}
	if !mounted {
		t.Errorf("init[0] 에 modules→/staged-modules mount 부재: %+v", cs[0].VolumeMounts)
	}
	if cs[1].Image != "example.com/byo:1" {
		t.Errorf("init[1].Image = %q, want example.com/byo:1", cs[1].Image)
	}
}

// TestBuildStatefulSet_modules_주입 — Modules 지정 시 STS 에 init container +
// --loadmodule args + 공유 emptyDir 볼륨 + 메인 컨테이너 /modules mount.
func TestBuildStatefulSet_modules_주입(t *testing.T) {
	sts := BuildStatefulSet(STSParams{
		CRName:    "vk",
		Namespace: "default",
		Replicas:  1,
		Image:     "docker.io/valkey/valkey:9.0.4",
		BundleTag: "9.0.4",
		Modules:   []cachev1alpha1.ModuleSpec{{Name: "valkey-search"}},
	})
	ps := sts.Spec.Template.Spec

	if len(ps.InitContainers) != 1 {
		t.Fatalf("InitContainers = %d, want 1", len(ps.InitContainers))
	}
	// 메인 valkey 컨테이너 Args 에 --loadmodule.
	var hasLoad bool
	for i, a := range ps.Containers[0].Args {
		if a == "--loadmodule" && i+1 < len(ps.Containers[0].Args) &&
			ps.Containers[0].Args[i+1] == "/modules/valkey-search.so" {
			hasLoad = true
		}
	}
	if !hasLoad {
		t.Errorf("메인 컨테이너 Args 에 --loadmodule /modules/valkey-search.so 부재: %v", ps.Containers[0].Args)
	}
	// 공유 emptyDir 볼륨.
	var hasVol bool
	for _, v := range ps.Volumes {
		if v.Name == "modules" && v.EmptyDir != nil {
			hasVol = true
		}
	}
	if !hasVol {
		t.Errorf("modules emptyDir 볼륨 부재: %+v", ps.Volumes)
	}
	// 메인 컨테이너 /modules mount (readOnly).
	var hasMount bool
	for _, vm := range ps.Containers[0].VolumeMounts {
		if vm.Name == "modules" && vm.MountPath == "/modules" {
			hasMount = true
		}
	}
	if !hasMount {
		t.Errorf("메인 컨테이너 /modules mount 부재: %+v", ps.Containers[0].VolumeMounts)
	}
}

// TestBundleTagOrDefault — valkey-bundle 태그를 major.minor 로 resolve.
// valkey-server 는 patch 태그(9.0.4)를 발행하나 valkey-bundle 은 major.minor
// (9.0)까지만 안정 발행 → patch 그대로 쓰면 ImagePullBackOff. module .so ABI 는
// minor 단위 호환이라 major.minor 가 정합 (실측: hub.docker.com/valkey/valkey-bundle).
func TestBundleTagOrDefault(t *testing.T) {
	cases := map[string]string{
		"9.0.4": "9.0",
		"8.1.6": "8.1",
		"9.0":   "9.0",
		"9":     "9",
		"":      "9.0", // DefaultValkeyVersion 9.0.4 → 9.0
	}
	for in, want := range cases {
		if got := BundleTagOrDefault(in); got != want {
			t.Errorf("BundleTagOrDefault(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBuildStatefulSet_빈_modules_no_op — Modules 미지정 시 init container/볼륨/
// args 변화 0 (기존 동작 회귀 가드).
func TestBuildStatefulSet_빈_modules_no_op(t *testing.T) {
	sts := BuildStatefulSet(STSParams{
		CRName:    "vk",
		Namespace: "default",
		Replicas:  1,
		Image:     "docker.io/valkey/valkey:9.0.4",
	})
	ps := sts.Spec.Template.Spec

	if len(ps.InitContainers) != 0 {
		t.Errorf("빈 Modules 인데 InitContainers = %d, want 0", len(ps.InitContainers))
	}
	for _, v := range ps.Volumes {
		if v.Name == "modules" {
			t.Errorf("빈 Modules 인데 modules 볼륨 존재")
		}
	}
	for _, a := range ps.Containers[0].Args {
		if a == "--loadmodule" {
			t.Errorf("빈 Modules 인데 --loadmodule args 존재: %v", ps.Containers[0].Args)
		}
	}
}
