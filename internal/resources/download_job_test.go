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

Download Job + Restore Source PVC 빌더 단위 테스트.
*/

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestBuildRestoreSourcePVC_RWO(t *testing.T) {
	pvc := BuildRestoreSourcePVC("vkr", "ns", corev1.ReadWriteOnce)
	if pvc.Name != "vkr-source" {
		t.Fatalf("expected name=vkr-source, got %s", pvc.Name)
	}
	if pvc.Namespace != "ns" {
		t.Fatalf("ns=%s", pvc.Namespace)
	}
	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Fatalf("expected RWO, got %v", pvc.Spec.AccessModes)
	}
	got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if got.String() != "8Gi" {
		t.Fatalf("size=%s, want 8Gi", got.String())
	}
}

func TestBuildRestoreSourcePVC_ROX(t *testing.T) {
	pvc := BuildRestoreSourcePVC("vkr", "ns", corev1.ReadOnlyMany)
	if pvc.Spec.AccessModes[0] != corev1.ReadOnlyMany {
		t.Fatalf("expected ROX, got %v", pvc.Spec.AccessModes)
	}
}

func TestBuildDownloadJob_args(t *testing.T) {
	j := BuildDownloadJob(DownloadJobParams{
		RestoreName:              "vkr",
		Namespace:                "ns",
		OperatorImage:            "img",
		PVCName:                  "vkr-source",
		FilePath:                 "/backup/dump.rdb",
		Endpoint:                 "https://s3.fake",
		Region:                   "us-east-1",
		Bucket:                   "valkey-backups",
		ObjectKey:                "cluster-A/dump.rdb",
		ForcePathStyle:           true,
		CredentialsSecretName:    "creds",
		AccessKeyIDSecretKey:     "AWS_ACCESS_KEY_ID",
		SecretAccessKeySecretKey: "AWS_SECRET_ACCESS_KEY",
	})
	if j.Name != "vkr-download" {
		t.Fatalf("name=%s", j.Name)
	}
	c := j.Spec.Template.Spec.Containers[0]
	wantArgs := []string{"download", "--bucket=valkey-backups",
		"--object=cluster-A/dump.rdb", "--file=/backup/dump.rdb"}
	if len(c.Args) != len(wantArgs) {
		t.Fatalf("args=%v want=%v", c.Args, wantArgs)
	}
	for i, w := range wantArgs {
		if c.Args[i] != w {
			t.Fatalf("args[%d]=%s want=%s", i, c.Args[i], w)
		}
	}
}

func TestBuildDownloadJob_pvcMountWritable(t *testing.T) {
	j := BuildDownloadJob(DownloadJobParams{
		RestoreName: "vkr", Namespace: "ns",
		OperatorImage: "img", PVCName: "vkr-source",
		FilePath: "/backup/dump.rdb",
		Endpoint: "https://s3.fake", Region: "us-east-1",
		Bucket: "b", ObjectKey: "k",
		CredentialsSecretName:    "creds",
		AccessKeyIDSecretKey:     "AWS_ACCESS_KEY_ID",
		SecretAccessKeySecretKey: "AWS_SECRET_ACCESS_KEY",
	})
	pod := j.Spec.Template.Spec
	// PVC volume RW (download 는 write 필요).
	if pod.Volumes[0].PersistentVolumeClaim.ReadOnly {
		t.Fatalf("download PVC volume should be writable (not ReadOnly)")
	}
	c := pod.Containers[0]
	if c.VolumeMounts[0].ReadOnly {
		t.Fatalf("download VolumeMount should be writable")
	}
}

func TestBuildDownloadJob_envFromSecret(t *testing.T) {
	j := BuildDownloadJob(DownloadJobParams{
		RestoreName: "vkr", Namespace: "ns",
		OperatorImage: "img", PVCName: "vkr-source",
		FilePath: "/backup/dump.rdb",
		Endpoint: "https://s3.fake", Region: "us-east-1",
		Bucket: "b", ObjectKey: "k", ForcePathStyle: false,
		CredentialsSecretName:    "creds",
		AccessKeyIDSecretKey:     "AWS_ACCESS_KEY_ID",
		SecretAccessKeySecretKey: "AWS_SECRET_ACCESS_KEY",
	})
	envByName := map[string]corev1.EnvVar{}
	for _, e := range j.Spec.Template.Spec.Containers[0].Env {
		envByName[e.Name] = e
	}
	if envByName["VALKEY_S3_ENDPOINT"].Value != "https://s3.fake" {
		t.Fatalf("endpoint env wrong")
	}
	if envByName["VALKEY_S3_FORCE_PATH_STYLE"].Value != "false" {
		t.Fatalf("force-path-style false")
	}
	ak := envByName["VALKEY_S3_ACCESS_KEY_ID"]
	if ak.ValueFrom == nil || ak.ValueFrom.SecretKeyRef == nil ||
		ak.ValueFrom.SecretKeyRef.Name != "creds" {
		t.Fatalf("access key SecretKeyRef wrong: %+v", ak)
	}
}

func TestRestoreLabels_keys(t *testing.T) {
	l := RestoreLabels("vkr")
	wantKeys := []string{
		"app.kubernetes.io/name",
		"app.kubernetes.io/instance",
		"app.kubernetes.io/managed-by",
		"app.kubernetes.io/component",
	}
	for _, k := range wantKeys {
		if _, ok := l[k]; !ok {
			t.Fatalf("missing label key %s", k)
		}
	}
	if l["app.kubernetes.io/instance"] != "vkr" {
		t.Fatalf("instance=%s", l["app.kubernetes.io/instance"])
	}
}
