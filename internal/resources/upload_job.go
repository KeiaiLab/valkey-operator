/*
Copyright 2026 Keiailab.

Upload Job 빌더 — operator image 의 `upload` sub-command 호출하여 PVC 의
RDB 를 외부 저장 (S3) 으로 업로드. ADR-0023.
*/

package resources

import (
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UploadJobName — 업로드 Job 이름. ValkeyBackup CR 이름 + "-upload".
func UploadJobName(backupName string) string { return backupName + "-upload" }

// UploadJobParams — Upload Job spec 빌드 파라미터.
//
// Endpoint / Region / Bucket / ObjectKey / ForcePathStyle 은 ValkeyBackupTarget
// + ValkeyBackup.Spec.Destination.TargetRef.Path 결합 결과 — Reconciler 가
// 결정. Sub-command 는 prefix 인지 안 함 (이미 결합된 ObjectKey 받음).
type UploadJobParams struct {
	BackupName string
	Namespace  string

	// OperatorImage — `valkey-operator` binary 가 들어 있는 image.
	// Deployment manifest 의 OPERATOR_IMAGE env 에서 주입.
	OperatorImage string

	// PVCName — RDB 파일 이 들어 있는 backup PVC.
	PVCName string

	// FilePath — Pod 안 의 RDB 파일 경로 (PVC mount 후).
	// 보통 BackupVolumeMountPath + "/" + BackupRDBFileName.
	FilePath string

	// S3 destination.
	Endpoint       string
	Region         string
	Bucket         string
	ObjectKey      string
	ForcePathStyle bool

	// 자격증명 Secret reference. envFrom 패턴 — Job 이 spawn 시점 snapshot.
	CredentialsSecretName    string
	AccessKeyIDSecretKey     string
	SecretAccessKeySecretKey string
}

// BuildUploadJob — operator image 의 `upload` sub-command 를 호출하는 Job.
//
// PodSpec:
//   - command: nil (Dockerfile ENTRYPOINT /manager 그대로)
//   - args: ["upload", "--bucket=X", "--object=Y", "--file=Z"]
//   - env: VALKEY_S3_* (endpoint/region/style + access/secret keys)
//   - volumeMount: backup PVC at /backup (ReadOnly — RDB 파일만 읽음)
//
// SecurityContext: runAsNonRoot, drop ALL caps. 1G RWO PVC 의 read-only mount.
//
// BackoffLimit 2, TTL 24h (성공 후 자동 정리).
func BuildUploadJob(p UploadJobParams) *batchv1.Job {
	args := []string{
		"upload",
		"--bucket=" + p.Bucket,
		"--object=" + p.ObjectKey,
		"--file=" + p.FilePath,
	}

	env := []corev1.EnvVar{
		{Name: "VALKEY_S3_ENDPOINT", Value: p.Endpoint},
		{Name: "VALKEY_S3_REGION", Value: p.Region},
		{Name: "VALKEY_S3_FORCE_PATH_STYLE", Value: strconv.FormatBool(p.ForcePathStyle)},
		{
			Name: "VALKEY_S3_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: p.CredentialsSecretName},
					Key:                  p.AccessKeyIDSecretKey,
				},
			},
		},
		{
			Name: "VALKEY_S3_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: p.CredentialsSecretName},
					Key:                  p.SecretAccessKeySecretKey,
				},
			},
		},
	}

	backoff := int32(2)
	ttl := int32(86400) // 24h
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      UploadJobName(p.BackupName),
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
						Name:  "upload",
						Image: p.OperatorImage,
						Args:  args,
						Env:   env,
						VolumeMounts: []corev1.VolumeMount{
							{Name: "backup", MountPath: BackupVolumeMountPath, ReadOnly: true},
						},
						SecurityContext: buildRestrictedContainerSecurityContext(),
					}},
					Volumes: []corev1.Volume{
						{Name: "backup", VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: p.PVCName,
								ReadOnly:  true,
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
