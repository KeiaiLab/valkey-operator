/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// Valkey 공식 module preset 적재 — ADR-0032 (init container mount + 공식 preset
// only) + ADR-0062 (v1alpha1 미러). 두 모드:
//   - 공식 preset (ModuleSpec.Image 빈): valkey-bundle 이미지에서 .so 를 init
//     container 가 공유 emptyDir 로 cp → 메인 valkey 가 --loadmodule 로 적재.
//   - bring-your-own (ModuleSpec.Image 명시): 사용자 이미지의 /modules/<name>.so
//     를 동일 경로로 cp.
//
// init container 는 restore.go 의 BuildRestoreInitContainer 패턴 미러 — busybox
// 류 cp + buildRestrictedContainerSecurityContext (PSS Restricted, ADR-0036).

const (
	// ModulesDir — 메인 valkey 컨테이너가 module .so 를 읽는 경로 (공유 emptyDir,
	// readOnly). valkey 는 --loadmodule /modules/<name>.so 로 적재.
	ModulesDir = "/modules"

	// moduleStageDir — init container 가 .so 를 쓰는 공유 emptyDir mount path.
	// BYO 이미지의 /modules 경로와 shadow 회피 위해 메인과 다른 mount path 사용.
	moduleStageDir = "/staged-modules"

	// moduleVolumeName — 공유 emptyDir 볼륨 이름.
	moduleVolumeName = "modules"

	// DefaultValkeyBundleRepo — 공식 preset init container 이미지 repo. tag 는
	// valkey 버전과 동일 pin (ADR-0062 D4 — .so ABI 를 valkey-server 버전과 정합).
	DefaultValkeyBundleRepo = "docker.io/valkey/valkey-bundle"
)

// officialModuleSOMap — 공식 preset name → valkey-bundle 이미지 내 .so 절대경로.
// 값은 e2e MODULE LIST 로 실증 봉인 (ADR-0062 D3). 미등록 name 은 webhook reject
// (ADR-0032 — RSALv2/SSPL 비호환 서드파티 차단).
var officialModuleSOMap = map[string]string{
	"valkey-search": "/usr/lib/valkey/libsearch.so",
	"valkey-json":   "/usr/lib/valkey/libjson.so",
	"valkey-bloom":  "/usr/lib/valkey/libvalkey_bloom.so",
}

// IsOfficialModule — name 이 공식 preset allow-list 에 있나 (webhook 공유).
func IsOfficialModule(name string) bool {
	_, ok := officialModuleSOMap[name]
	return ok
}

// BundleTagOrDefault — 공식 preset valkey-bundle image tag. 빈 버전이면
// DefaultValkeyVersion (controller 가 spec.version.version 전달, .so ABI 정합).
func BundleTagOrDefault(version string) string {
	v := version
	if v == "" {
		v = cachev1alpha1.DefaultValkeyVersion
	}
	// valkey-bundle 은 major.minor 태그만 안정 발행 (patch 태그는 valkey-server 와
	// 어긋남 — 예: valkey 9.0.4 존재하나 bundle 은 9.0.3/9.0 까지). module .so ABI
	// 는 minor 단위 호환 → major.minor 로 resolve (실측: hub.docker.com/valkey/valkey-bundle).
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

// OfficialModuleNames — allow-list name 목록 (webhook 에러 메시지용, 정렬됨).
func OfficialModuleNames() []string {
	return []string{"valkey-bloom", "valkey-json", "valkey-search"}
}

// resolvedModule — 한 모듈의 적재 정보.
type resolvedModule struct {
	name      string
	initImage string   // init container 이미지
	srcPath   string   // init image 내 .so 경로
	args      []string // LoadModuleArgs
}

// destPath — 메인 컨테이너가 로드하는 경로 /modules/<name>.so.
func (r resolvedModule) destPath() string { return ModulesDir + "/" + r.name + ".so" }

// stagePath — init container 가 쓰는 공유 경로 /staged-modules/<name>.so.
func (r resolvedModule) stagePath() string { return moduleStageDir + "/" + r.name + ".so" }

// resolveModule — official preset / BYO 분기. bundleTag = valkey 버전 (official 만 사용).
func resolveModule(m cachev1alpha1.ModuleSpec, bundleTag string) resolvedModule {
	rm := resolvedModule{name: m.Name, args: m.LoadModuleArgs}
	if m.Image != "" {
		// BYO: 사용자 이미지의 /modules/<name>.so 컨벤션 (ADR-0032).
		rm.initImage = m.Image
		rm.srcPath = ModulesDir + "/" + m.Name + ".so"
		return rm
	}
	// 공식 preset: valkey-bundle:<버전> + allow-list .so 경로.
	// 미등록 name 은 srcPath="" — webhook 가 사전 reject (방어적: e2e 가 검출).
	rm.initImage = DefaultValkeyBundleRepo + ":" + bundleTag
	rm.srcPath = officialModuleSOMap[m.Name]
	return rm
}

// buildModuleInitContainers — 모듈당 init container (restore.go 패턴 미러).
// 각 init 은 .so 를 공유 emptyDir(/staged-modules)로 cp. restricted SC 라 root
// 불필요 (busybox/valkey 이미지의 cp 가 fsGroup ownership 따름).
func buildModuleInitContainers(modules []cachev1alpha1.ModuleSpec, bundleTag string, pullPolicy corev1.PullPolicy) []corev1.Container {
	if len(modules) == 0 {
		return nil
	}
	cs := make([]corev1.Container, 0, len(modules))
	for _, m := range modules {
		rm := resolveModule(m, bundleTag)
		cs = append(cs, corev1.Container{
			Name:            "module-" + rm.name,
			Image:           rm.initImage,
			ImagePullPolicy: pullPolicy,
			Command: []string{"sh", "-c", fmt.Sprintf(
				"set -eu; cp -f %q %q && ls -l %q", rm.srcPath, rm.stagePath(), rm.stagePath())},
			SecurityContext: buildRestrictedContainerSecurityContext(),
			VolumeMounts:    []corev1.VolumeMount{{Name: moduleVolumeName, MountPath: moduleStageDir}},
		})
	}
	return cs
}

// moduleLoadArgs — 메인 valkey 컨테이너 Args 에 append 할 --loadmodule 시퀀스.
// valkey-server <config-file> --loadmodule /modules/<name>.so <args...> 형식.
func moduleLoadArgs(modules []cachev1alpha1.ModuleSpec) []string {
	if len(modules) == 0 {
		return nil
	}
	var args []string
	for _, m := range modules {
		rm := resolveModule(m, "") // destPath 는 bundleTag 무관.
		args = append(args, "--loadmodule", rm.destPath())
		args = append(args, rm.args...)
	}
	return args
}

// moduleVolume — 공유 emptyDir 볼륨.
func moduleVolume() corev1.Volume {
	return corev1.Volume{
		Name:         moduleVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
}
