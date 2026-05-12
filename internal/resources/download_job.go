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

/*
Copyright 2026 Keiailab.

Download Job 빌더 — operator image 의 `download` sub-command 호출하여
외부 저장 (S3) 의 RDB 를 PVC 로 다운로드. ADR-0023 의 reverse 패턴.

ValkeyRestore Mounting phase 가 Source.TargetRef 시 본 빌더로 Job 생성.
*/

package resources

import (
	"k8s.io/apimachinery/pkg/api/resource"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RestoreSourcePVCName — ValkeyRestore 가 생성하는 임시 source PVC 이름.
// 외부 다운로드의 destination + Init Container source.
func RestoreSourcePVCName(restoreName string) string { return restoreName + "-source" }

// DownloadJobName — Download Job 이름.
func DownloadJobName(restoreName string) string { return restoreName + "-download" }

// RestoreLabels — Restore 의 모든 보조 리소스 (PVC, Job) 공통 label.
func RestoreLabels(restoreName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "valkey-restore",
		"app.kubernetes.io/instance":   restoreName,
		"app.kubernetes.io/managed-by": "valkey-operator",
		"app.kubernetes.io/component":  "restore",
	}
}

// BuildRestoreSourcePVC — ValkeyRestore 가 외부에서 다운로드한 RDB 를 보존
// 할 임시 PVC.
//
// Standalone: ReadWriteOnce. Replication / Cluster (multi-pod 동시 mount):
// ReadOnlyMany 필수 — storage class 가 ROX 지원 해야.
//
// 8Gi 고정 크기. Spec.SourcePVCSize 옵션 은 추후.
func BuildRestoreSourcePVC(
	restoreName, namespace string,
	accessMode corev1.PersistentVolumeAccessMode,
) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RestoreSourcePVCName(restoreName),
			Namespace: namespace,
			Labels:    RestoreLabels(restoreName),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("8Gi"),
				},
			},
		},
	}
}

// DownloadJobParams — Download Job spec 빌드 파라미터.
type DownloadJobParams struct {
	RestoreName string
	Namespace   string

	// OperatorImage — `valkey-operator` binary 이미지.
	OperatorImage string

	// PVCName — 다운로드 destination PVC.
	PVCName string

	// FilePath — Pod 안의 다운로드 destination 파일 경로.
	// 보통 BackupVolumeMountPath + "/" + BackupRDBFileName.
	FilePath string

	// S3 source.
	Endpoint       string
	Region         string
	Bucket         string
	ObjectKey      string
	ForcePathStyle bool

	CredentialsSecretName    string
	AccessKeyIDSecretKey     string
	SecretAccessKeySecretKey string

	// PITRCutoff — RFC3339 시각. 비어있지 않으면 download Job 의 cli 가 본 시각
	// 까지 AOF in-place truncate (PR #70 PITR phase 2 reconciler dispatch).
	// AOF restore type 만 의미. RDB 는 무시.
	PITRCutoff string
}

// BuildDownloadJob — operator image 의 `download` sub-command 호출.
//
// PodSpec 패턴 BuildUploadJob 와 동일. 차이: PVC ReadOnly=false (write 필요)
// + sub-command="download".
func BuildDownloadJob(p DownloadJobParams) *batchv1.Job {
	args := []string{
		"download",
		"--bucket=" + p.Bucket,
		"--object=" + p.ObjectKey,
		"--file=" + p.FilePath,
	}
	if p.PITRCutoff != "" {
		args = append(args, "--pitr-cutoff="+p.PITRCutoff)
	}
	env := s3EnvForJob(p.Endpoint, p.Region, p.ForcePathStyle,
		p.CredentialsSecretName, p.AccessKeyIDSecretKey, p.SecretAccessKeySecretKey)

	backoff := int32(2)
	ttl := int32(86400)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DownloadJobName(p.RestoreName),
			Namespace: p.Namespace,
			Labels:    RestoreLabels(p.RestoreName),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: RestoreLabels(p.RestoreName)},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Name:  "download",
						Image: p.OperatorImage,
						Args:  args,
						Env:   env,
						VolumeMounts: []corev1.VolumeMount{
							// download 는 write 필요 — ReadOnly=false.
							{Name: "backup", MountPath: BackupVolumeMountPath},
						},
						SecurityContext: buildRestrictedContainerSecurityContext(),
					}},
					Volumes: []corev1.Volume{
						{Name: "backup", VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: p.PVCName,
							},
						}},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptrBool(true),
						RunAsUser:    ptrInt64(65532),
						FSGroup:      ptrInt64(65532),
					},
				},
			},
		},
	}
}

// s3EnvForJob — Upload/Download Job 공통 env 빌더 (DRY).
func s3EnvForJob(endpoint, region string, forcePathStyle bool,
	secretName, akKey, skKey string,
) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "VALKEY_S3_ENDPOINT", Value: endpoint},
		{Name: "VALKEY_S3_REGION", Value: region},
		{Name: "VALKEY_S3_FORCE_PATH_STYLE", Value: boolStr(forcePathStyle)},
		{
			Name: "VALKEY_S3_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  akKey,
				},
			},
		},
		{
			Name: "VALKEY_S3_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  skKey,
				},
			},
		},
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
