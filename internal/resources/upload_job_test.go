/*
Copyright 2026 Keiailab.

Upload Job 빌더 단위 테스트.
*/

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestBuildUploadJob_basicFields(t *testing.T) {
	j := BuildUploadJob(UploadJobParams{
		BackupName:               "vkb",
		Namespace:                "ns",
		OperatorImage:            "ghcr.io/keiailab/valkey-operator:v0.1.0",
		PVCName:                  "vkb-backup",
		FilePath:                 "/backup/dump.rdb",
		Endpoint:                 "https://s3.amazonaws.com",
		Region:                   "ap-northeast-2",
		Bucket:                   "valkey-backups",
		ObjectKey:                "cluster-A/2026-05-06/dump.rdb",
		ForcePathStyle:           false,
		CredentialsSecretName:    "s3-creds",
		AccessKeyIDSecretKey:     "AWS_ACCESS_KEY_ID",
		SecretAccessKeySecretKey: "AWS_SECRET_ACCESS_KEY",
	})
	if j.Name != "vkb-upload" {
		t.Fatalf("expected name=vkb-upload, got %s", j.Name)
	}
	if j.Namespace != "ns" {
		t.Fatalf("expected ns=ns, got %s", j.Namespace)
	}
	c := j.Spec.Template.Spec.Containers[0]
	if c.Image != "ghcr.io/keiailab/valkey-operator:v0.1.0" {
		t.Fatalf("expected image, got %s", c.Image)
	}
	// args[0]=upload, --bucket=, --object=, --file=
	wantArgs := []string{"upload", "--bucket=valkey-backups",
		"--object=cluster-A/2026-05-06/dump.rdb", "--file=/backup/dump.rdb"}
	if len(c.Args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d, got %v", len(c.Args), len(wantArgs), c.Args)
	}
	for i, w := range wantArgs {
		if c.Args[i] != w {
			t.Fatalf("args[%d] = %s, want %s", i, c.Args[i], w)
		}
	}
}

func TestBuildUploadJob_envHasS3Vars(t *testing.T) {
	j := BuildUploadJob(UploadJobParams{
		BackupName: "vkb", Namespace: "ns",
		OperatorImage:            "img",
		PVCName:                  "p",
		FilePath:                 "/backup/dump.rdb",
		Endpoint:                 "https://s3.fake",
		Region:                   "us-east-1",
		Bucket:                   "b",
		ObjectKey:                "k",
		ForcePathStyle:           true,
		CredentialsSecretName:    "creds",
		AccessKeyIDSecretKey:     "AWS_ACCESS_KEY_ID",
		SecretAccessKeySecretKey: "AWS_SECRET_ACCESS_KEY",
	})
	c := j.Spec.Template.Spec.Containers[0]
	envByName := map[string]corev1.EnvVar{}
	for _, e := range c.Env {
		envByName[e.Name] = e
	}
	if got := envByName["VALKEY_S3_ENDPOINT"].Value; got != "https://s3.fake" {
		t.Fatalf("endpoint env = %s", got)
	}
	if got := envByName["VALKEY_S3_REGION"].Value; got != "us-east-1" {
		t.Fatalf("region env = %s", got)
	}
	if got := envByName["VALKEY_S3_FORCE_PATH_STYLE"].Value; got != "true" {
		t.Fatalf("force-path-style env = %s", got)
	}
	// access/secret 는 ValueFrom secretKeyRef.
	ak := envByName["VALKEY_S3_ACCESS_KEY_ID"]
	if ak.ValueFrom == nil || ak.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("access key should be SecretKeyRef")
	}
	if ak.ValueFrom.SecretKeyRef.Name != "creds" {
		t.Fatalf("creds secret name = %s", ak.ValueFrom.SecretKeyRef.Name)
	}
	sk := envByName["VALKEY_S3_SECRET_ACCESS_KEY"]
	if sk.ValueFrom == nil || sk.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("secret key should be SecretKeyRef")
	}
}

func TestBuildUploadJob_pvcMountReadOnly(t *testing.T) {
	j := BuildUploadJob(UploadJobParams{
		BackupName: "vkb", Namespace: "ns",
		OperatorImage:            "img",
		PVCName:                  "vkb-backup",
		FilePath:                 "/backup/dump.rdb",
		Endpoint:                 "https://s3.fake",
		Region:                   "us-east-1",
		Bucket:                   "b",
		ObjectKey:                "k",
		CredentialsSecretName:    "creds",
		AccessKeyIDSecretKey:     "AWS_ACCESS_KEY_ID",
		SecretAccessKeySecretKey: "AWS_SECRET_ACCESS_KEY",
	})
	pod := j.Spec.Template.Spec
	// PVC volume.
	if len(pod.Volumes) != 1 || pod.Volumes[0].PersistentVolumeClaim == nil {
		t.Fatalf("expected 1 PVC volume, got %v", pod.Volumes)
	}
	if !pod.Volumes[0].PersistentVolumeClaim.ReadOnly {
		t.Fatalf("PVC volume should be ReadOnly")
	}
	if pod.Volumes[0].PersistentVolumeClaim.ClaimName != "vkb-backup" {
		t.Fatalf("PVC claim name = %s", pod.Volumes[0].PersistentVolumeClaim.ClaimName)
	}
	// VolumeMount ReadOnly.
	c := pod.Containers[0]
	if len(c.VolumeMounts) != 1 || !c.VolumeMounts[0].ReadOnly {
		t.Fatalf("VolumeMount should be ReadOnly")
	}
}

func TestBuildUploadJob_securityContext(t *testing.T) {
	j := BuildUploadJob(UploadJobParams{
		BackupName: "vkb", Namespace: "ns",
		OperatorImage:            "img",
		PVCName:                  "p",
		FilePath:                 "/backup/dump.rdb",
		Endpoint:                 "https://s3.fake",
		Region:                   "us-east-1",
		Bucket:                   "b",
		ObjectKey:                "k",
		CredentialsSecretName:    "creds",
		AccessKeyIDSecretKey:     "AWS_ACCESS_KEY_ID",
		SecretAccessKeySecretKey: "AWS_SECRET_ACCESS_KEY",
	})
	pod := j.Spec.Template.Spec
	if pod.SecurityContext == nil || pod.SecurityContext.RunAsNonRoot == nil ||
		!*pod.SecurityContext.RunAsNonRoot {
		t.Fatalf("PodSecurityContext.RunAsNonRoot should be true")
	}
	c := pod.Containers[0]
	if c.SecurityContext == nil || c.SecurityContext.Capabilities == nil {
		t.Fatalf("Container.SecurityContext.Capabilities should drop ALL")
	}
	dropped := false
	for _, cap := range c.SecurityContext.Capabilities.Drop {
		if cap == "ALL" {
			dropped = true
		}
	}
	if !dropped {
		t.Fatalf("expected drop ALL caps")
	}
}
