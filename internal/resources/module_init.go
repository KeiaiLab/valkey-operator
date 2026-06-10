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
	"github.com/keiailab/valkey-operator/internal/modules"
)

// ModuleVolumeName — 모듈 .so 를 담는 공유 emptyDir. init-container 가 채우고
// valkey 컨테이너가 읽는다.
const ModuleVolumeName = "valkey-modules"

// moduleMountPath — valkey 컨테이너 + init-container 공통 mount 경로.
const moduleMountPath = "/modules"

// BuildModuleInitContainers — ModuleSpec 목록 → (init-container 들, 공유 emptyDir volume,
// valkey 컨테이너에 추가할 --loadmodule args).
//
// 각 module 은 출처 이미지의 .so 를 공유 emptyDir(/modules/<name>.so)로 cp 하는
// init-container 1개를 만든다:
//   - Name 만:  ResolveModulePreset 로 공식 BSD preset image+SOPath resolve
//   - Image 지정: BYO. image 의 /modules/<name>.so 를 cp
//   - 둘 다 아님(외부 Redis Stack 등): resolve 불가 → skip (admission webhook 이 거부)
//
// 빈 목록이면 init-container/args 는 비고, volume 은 항상 반환(StatefulSet 가 무조건 선언).
func BuildModuleInitContainers(mods []cachev1alpha1.ModuleSpec) ([]corev1.Container, corev1.Volume, []string) {
	volume := corev1.Volume{
		Name:         ModuleVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
	initContainers := make([]corev1.Container, 0, len(mods))
	loadArgs := make([]string, 0, len(mods))

	for _, m := range mods {
		var image, soPath string
		switch {
		case m.Image != "":
			image = m.Image
			soPath = fmt.Sprintf("%s/%s.so", moduleMountPath, m.Name) // BYO 컨벤션
		default:
			preset, ok := modules.ResolveModulePreset(m.Name)
			if !ok {
				continue // 외부 Redis Stack 등 — resolve 불가, webhook 이 거부
			}
			image, soPath = preset.Image, preset.SOPath
		}

		dest := fmt.Sprintf("%s/%s.so", moduleMountPath, m.Name)
		initContainers = append(initContainers, corev1.Container{
			Name:            "module-" + m.Name,
			Image:           image,
			Command:         []string{"sh", "-c", fmt.Sprintf("cp %s %s", soPath, dest)},
			VolumeMounts:    []corev1.VolumeMount{{Name: ModuleVolumeName, MountPath: moduleMountPath}},
			SecurityContext: buildRestrictedContainerSecurityContext(),
		})

		arg := dest
		if len(m.LoadModuleArgs) > 0 {
			arg = dest + " " + strings.Join(m.LoadModuleArgs, " ")
		}
		loadArgs = append(loadArgs, arg)
	}
	return initContainers, volume, loadArgs
}
