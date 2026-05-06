/*
Copyright 2026 Keiailab.
*/

package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// 본 파일 — ValkeyRestore (ADR-0015) 의 STS PodTemplate patch 헬퍼.
//
// 동작:
//  1. ValkeyRestoreReconciler 가 BuildRestoreInitContainer + BuildRestoreSourceVolume
//     을 STS PodTemplate 에 inject (paused annotation 으로 ValkeyController 충돌 차단).
//  2. STS template 변경 → PodTemplate hash 변경 → STS rolling restart.
//  3. Init container 가 backup PVC 의 RDB 를 /data/dump.rdb 로 복사.
//  4. Main valkey-server 가 시작 시 자동 RDB 로드 (Valkey 의 표준 매커니즘).
//  5. Restore 검증 후 RemoveRestoreInitContainer + RemoveRestoreSourceVolume 으로 원복.
//
// 첫 commit 의 의도적 한계 (별개 commit 보강):
//   - Source PVC RWO multi-attach 충돌 회피 위해 Standalone Valkey
//     (replicas=1) 만 지원. Replication (3+ replicas) + Cluster mode 는
//     ReadOnlyMany source PVC 패턴 또는 per-shard source 매핑 필요.

const (
	// RestoreSourceVolumeName — STS PodTemplate 에 inject 되는 source volume 이름.
	RestoreSourceVolumeName = "valkey-restore-source"

	// RestoreInitContainerName — STS PodTemplate 에 inject 되는 init container 이름.
	// 원복 시 본 이름으로 식별하여 제거.
	RestoreInitContainerName = "valkey-restore-init"

	// RestoreSourceMountPath — init container 안 의 source PVC mount path.
	RestoreSourceMountPath = "/restore"

	// RestoreInitImage — init container 이미지 (cp 만 필요).
	RestoreInitImage = "busybox:1.36"
)

// BuildRestoreInitContainer — backup PVC 의 RDB 를 /data/dump.rdb 로 복사
// 하는 init container.
//
// srcRelPath: source PVC 안 의 RDB 파일 상대 경로 (e.g. "dump.rdb").
//
// 권한: valkey container 의 fsGroup=999 가 PVC 마운트 시 자동 chown — init
// container 는 root 권한 불필요. busybox 의 기본 cp 가 fsGroup ownership 따름.
func BuildRestoreInitContainer(srcRelPath string) corev1.Container {
	srcAbsPath := RestoreSourceMountPath + "/" + srcRelPath
	dstPath := "/data/dump.rdb"
	return corev1.Container{
		Name:  RestoreInitContainerName,
		Image: RestoreInitImage,
		Command: []string{
			"sh", "-c",
			fmt.Sprintf("set -eu; cp -f %q %q && ls -la %q && echo restore-init-ok",
				srcAbsPath, dstPath, dstPath),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
			{Name: RestoreSourceVolumeName, MountPath: RestoreSourceMountPath, ReadOnly: true},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: ptrBool(true),
			RunAsUser:    ptrInt64(999),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}
}

// BuildRestoreInitContainerForCluster — ValkeyCluster mode init container.
//
// 단일 STS 의 모든 pod 가 동일 init container 사용. shell 안에서 pod
// hostname 의 ordinal 추출 → shard index 결정 → 적절한 shard RDB cp.
//
// ValkeyCluster ordinal 매핑 (valkeycluster_controller.go):
//   - ordinal 0..shards-1: primary (shard index = ordinal)
//   - ordinal shards..total-1: replica (shard index = (ordinal - shards) % shards)
//
// ShardLayout: shard index → source PVC 내부 path 매핑. 미명시 시 default
// `shard-{index}/dump.rdb`. caller (controller) 가 default 채워서 전달.
//
// ROX source PVC 가정 (multi-pod 동시 mount).
func BuildRestoreInitContainerForCluster(shards int32, shardLayout map[int]string) corev1.Container {
	dstPath := "/data/dump.rdb"
	// shell case 분기 빌드 — shard index 별 source path.
	caseLines := ""
	for i := int32(0); i < shards; i++ {
		path, ok := shardLayout[int(i)]
		if !ok || path == "" {
			path = fmt.Sprintf("shard-%d/dump.rdb", i)
		}
		caseLines += fmt.Sprintf("    %d) SRC=%q ;;\n", i, path)
	}
	caseLines += "    *) echo \"unknown shard index $SHARD_IDX\"; exit 1 ;;\n"

	cmd := fmt.Sprintf(`set -eu
ORDINAL=${HOSTNAME##*-}
SHARDS=%d
if [ "$ORDINAL" -lt "$SHARDS" ]; then
  SHARD_IDX=$ORDINAL
else
  SHARD_IDX=$(( (ORDINAL - SHARDS) %% SHARDS ))
fi
case $SHARD_IDX in
%sesac
SRC_FULL=%s/$SRC
cp -f "$SRC_FULL" %q
ls -la %q
echo "restore-init-ok ordinal=$ORDINAL shard=$SHARD_IDX src=$SRC"
`, shards, caseLines, RestoreSourceMountPath, dstPath, dstPath)

	return corev1.Container{
		Name:    RestoreInitContainerName,
		Image:   RestoreInitImage,
		Command: []string{"sh", "-c", cmd},
		Env: []corev1.EnvVar{
			{
				Name: "HOSTNAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
			{Name: RestoreSourceVolumeName, MountPath: RestoreSourceMountPath, ReadOnly: true},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: ptrBool(true),
			RunAsUser:    ptrInt64(999),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}
}

// InjectRestoreIntoPodSpecForCluster — ValkeyCluster 의 STS PodTemplate 에
// init container + source volume 을 *멱등* 추가 (cluster 형태).
func InjectRestoreIntoPodSpecForCluster(
	pod *corev1.PodSpec,
	shards int32,
	shardLayout map[int]string,
	sourcePVC string,
) {
	initC := BuildRestoreInitContainerForCluster(shards, shardLayout)
	srcVol := BuildRestoreSourceVolume(sourcePVC)

	replaced := false
	for i, c := range pod.InitContainers {
		if c.Name == RestoreInitContainerName {
			pod.InitContainers[i] = initC
			replaced = true
			break
		}
	}
	if !replaced {
		pod.InitContainers = append(pod.InitContainers, initC)
	}

	replaced = false
	for i, v := range pod.Volumes {
		if v.Name == RestoreSourceVolumeName {
			pod.Volumes[i] = srcVol
			replaced = true
			break
		}
	}
	if !replaced {
		pod.Volumes = append(pod.Volumes, srcVol)
	}
}

// BuildRestoreSourceVolume — STS PodTemplate 에 추가할 source PVC volume.
//
// ReadOnly=true — source 는 변경 불가 (init container 가 cp 만).
func BuildRestoreSourceVolume(pvcName string) corev1.Volume {
	return corev1.Volume{
		Name: RestoreSourceVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
				ReadOnly:  true,
			},
		},
	}
}

// InjectRestoreIntoPodSpec — PodSpec 에 init container + source volume 을 *멱등* 추가.
// 이미 있으면 update (replace). 호출자는 STS update 후 status 모니터링.
func InjectRestoreIntoPodSpec(pod *corev1.PodSpec, srcRelPath, sourcePVC string) {
	initC := BuildRestoreInitContainer(srcRelPath)
	srcVol := BuildRestoreSourceVolume(sourcePVC)

	// init container — 이름 일치 시 replace, 아니면 append.
	replaced := false
	for i, c := range pod.InitContainers {
		if c.Name == RestoreInitContainerName {
			pod.InitContainers[i] = initC
			replaced = true
			break
		}
	}
	if !replaced {
		pod.InitContainers = append(pod.InitContainers, initC)
	}

	// volume — 이름 일치 시 replace, 아니면 append.
	replaced = false
	for i, v := range pod.Volumes {
		if v.Name == RestoreSourceVolumeName {
			pod.Volumes[i] = srcVol
			replaced = true
			break
		}
	}
	if !replaced {
		pod.Volumes = append(pod.Volumes, srcVol)
	}
}

// RemoveRestoreFromPodSpec — Inject 의 반대. 이름 기준으로 제거 (멱등).
// 원복 후 ValkeyController 가 다시 reconcile (paused annotation 제거 후).
func RemoveRestoreFromPodSpec(pod *corev1.PodSpec) {
	filteredInit := pod.InitContainers[:0]
	for _, c := range pod.InitContainers {
		if c.Name != RestoreInitContainerName {
			filteredInit = append(filteredInit, c)
		}
	}
	pod.InitContainers = filteredInit

	filteredVols := pod.Volumes[:0]
	for _, v := range pod.Volumes {
		if v.Name != RestoreSourceVolumeName {
			filteredVols = append(filteredVols, v)
		}
	}
	pod.Volumes = filteredVols
}
