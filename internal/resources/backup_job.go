/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package resources

import (
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/keiailab/operator-commons/pkg/security"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// BackupPVCName — 백업 결과 PVC 이름. ValkeyBackup CR 이름 + "-backup".
func BackupPVCName(backupName string) string { return backupName + "-backup" }

// BackupJobName — RDB 복사 Job 이름.
func BackupJobName(backupName string) string { return backupName + "-rdb-copy" }

// BackupVolumeMountPath — Job 컨테이너 안의 backup PVC 마운트 경로.
const BackupVolumeMountPath = "/backup"

// BackupRDBFileName — Job 이 생성하는 RDB 파일명 (PVC 안 의 상대 경로).
const BackupRDBFileName = "dump.rdb"

// BuildBackupPVC — TargetPVC 미명시 시 동적 PVC 생성.
//
// AccessMode: ReadWriteOnce (Job 단일 mount 가정 — 외부 도구 가 RWO 환경 에서도
// snapshot 복사 후 별도 분석 가능).
// StorageSize: Spec.StorageSize 기본 8Gi.
func BuildBackupPVC(b *cachev1alpha1.ValkeyBackup) *corev1.PersistentVolumeClaim {
	size := b.Spec.StorageSize
	if size == "" {
		size = cachev1alpha1.DefaultStorageSize
	}
	q := resource.MustParse(size)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BackupPVCName(b.Name),
			Namespace: b.Namespace,
			Labels:    BackupLabels(b.Name),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: q},
			},
		},
	}
}

// BackupLabels — 본 백업 의 모든 보조 리소스 (PVC, Job) 공통 label.
func BackupLabels(backupName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "valkey-backup",
		"app.kubernetes.io/instance":   backupName,
		"app.kubernetes.io/managed-by": "valkey-operator",
		"app.kubernetes.io/component":  "backup",
	}
}

// BackupJobParams — backup Job 빌드 파라미터.
type BackupJobParams struct {
	BackupName     string
	Namespace      string
	PVCName        string
	Image          string   // valkey 이미지 (valkey-cli 포함)
	TargetHost     string   // 대상 노드 FQDN
	TargetHosts    []string // ValkeyCluster 샤드별 대상 FQDN. shard index 순서로 저장.
	TargetPort     int32    // 6379 (plain) 또는 6380 (TLS)
	PasswordSecret *corev1.SecretKeySelector
	UseTLS         bool
	TLSSecretName  string // TLS 활성 시 cert mount 용 (ca.crt, tls.crt, tls.key)
}

// BuildBackupJob — `valkey-cli --rdb` 를 실행하는 Job. RDB 가 backup PVC 에 저장.
//
// `valkey-cli --rdb /backup/dump.rdb` 는 SYNC 프로토콜 로 fresh RDB 를 클라이언트
// 측에 다운로드 — 서버의 /data/dump.rdb 와 무관하게 일관된 스냅샷 보장.
// TLS 활성 시 --tls --cacert/cert/key 추가.
func BuildBackupJob(p BackupJobParams) *batchv1.Job {
	shellCmd := buildBackupShellCommand(p)
	cmd := []string{"sh", "-c", shellCmd}

	volumeMounts := []corev1.VolumeMount{
		{Name: "backup", MountPath: BackupVolumeMountPath},
	}
	volumes := []corev1.Volume{
		{Name: "backup", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: p.PVCName},
		}},
	}
	if p.UseTLS && p.TLSSecretName != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "tls", MountPath: "/tls", ReadOnly: true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "tls", VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  p.TLSSecretName,
					DefaultMode: ptrInt32(0o400),
				},
			},
		})
	}

	backoff := int32(2)
	ttl := int32(86400) // 24h — Job 자체 는 24시간 후 자동 정리, PVC 는 보존.
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BackupJobName(p.BackupName),
			Namespace: p.Namespace,
			Labels:    BackupLabels(p.BackupName),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: BackupLabels(p.BackupName)},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Name:    "rdb-copy",
						Image:   p.Image,
						Command: cmd,
						Env: []corev1.EnvVar{{
							Name: "VALKEY_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: p.PasswordSecret,
							},
						}},
						VolumeMounts: volumeMounts,
						// iteration 37 (cluster incident fix): PodSecurity restricted invariant.
						// data ns 의 enforce=restricted 가 admission 단계에서 capabilities.drop /
						// seccompProfile / allowPrivilegeEscalation 미설정 pod 거부 → backup
						// job-controller 가 매 5-15s 재시도하며 ValkeyBackup Phase=Copying stuck.
						// commons.RestrictedContainer 위임 — RunAsUser=999 (postgres-user 와 분리,
						// valkey 표준).
						SecurityContext: security.RestrictedContainer(security.WithRunAsUser(999)),
					}},
					Volumes: volumes,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptrBool(true),
						RunAsUser:    ptrInt64(999),
						FSGroup:      ptrInt64(999),
					},
				},
			},
		},
	}
}

func buildBackupShellCommand(p BackupJobParams) string {
	if len(p.TargetHosts) > 0 {
		dirs := make([]string, 0, len(p.TargetHosts))
		paths := make([]string, 0, len(p.TargetHosts))
		commands := []string{"set -eu"}
		for i, host := range p.TargetHosts {
			dir := fmt.Sprintf("%s/shard-%d", BackupVolumeMountPath, i)
			path := dir + "/" + BackupRDBFileName
			dirs = append(dirs, dir)
			paths = append(paths, path)
			args := buildBackupCLIArgs(p, host, path)
			commands = append(commands, fmt.Sprintf("valkey-cli %s", joinArgs(args)))
		}
		commands = append([]string{commands[0], "mkdir -p " + joinArgs(dirs)}, commands[1:]...)
		commands = append(commands, "ls -la "+joinArgs(paths))
		return strings.Join(commands, "; ")
	}

	path := BackupVolumeMountPath + "/" + BackupRDBFileName
	args := buildBackupCLIArgs(p, p.TargetHost, path)
	return fmt.Sprintf("set -eu; valkey-cli %s && ls -la %s",
		joinArgs(args), path)
}

func buildBackupCLIArgs(p BackupJobParams, host, outputPath string) []string {
	args := []string{
		"-h", host, "-p", fmt.Sprintf("%d", p.TargetPort),
		"-a", "$VALKEY_PASSWORD",
		"--rdb", outputPath,
	}
	if p.UseTLS {
		args = append(args,
			"--tls",
			"--cacert", "/tls/ca.crt",
			"--cert", "/tls/tls.crt",
			"--key", "/tls/tls.key",
			"--sni", host,
		)
	}
	return args
}

// joinArgs — `valkey-cli` 인자 를 shell-safe 하게 한 줄 로 결합. 본 함수는 매우 단순한
// 케이스 만 처리 (인자 에 공백 / 따옴표 없음 가정).
func joinArgs(args []string) string {
	var out strings.Builder
	for i, a := range args {
		if i > 0 {
			out.WriteString(" ")
		}
		out.WriteString(a)
	}
	return out.String()
}
