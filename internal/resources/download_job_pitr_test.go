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
	"strings"
	"testing"
)

func minimalDownloadParams() DownloadJobParams {
	return DownloadJobParams{
		RestoreName:              "rest-x",
		Namespace:                "ns",
		OperatorImage:            "ghcr.io/keiailab/valkey-operator:latest",
		PVCName:                  "src-pvc",
		FilePath:                 "/backup/dump.aof",
		Endpoint:                 "https://s3.amazonaws.com",
		Region:                   "us-east-1",
		Bucket:                   "backups",
		ObjectKey:                "vk/dump.aof",
		CredentialsSecretName:    "s3-creds",
		AccessKeyIDSecretKey:     "AWS_ACCESS_KEY_ID",
		SecretAccessKeySecretKey: "AWS_SECRET_ACCESS_KEY",
	}
}

func TestBuildDownloadJob_no_PITR_args_skipped(t *testing.T) {
	job := BuildDownloadJob(minimalDownloadParams())
	args := job.Spec.Template.Spec.Containers[0].Args
	for _, a := range args {
		if strings.HasPrefix(a, "--pitr-cutoff=") {
			t.Errorf("PITR cutoff 미명시 시 --pitr-cutoff arg 부재해야: %v", args)
		}
	}
}

func TestBuildDownloadJob_with_PITR_appends_cutoff_arg(t *testing.T) {
	p := minimalDownloadParams()
	p.PITRCutoff = "2026-05-10T14:30:00Z"
	job := BuildDownloadJob(p)
	args := job.Spec.Template.Spec.Containers[0].Args

	found := false
	for _, a := range args {
		if a == "--pitr-cutoff=2026-05-10T14:30:00Z" {
			found = true
		}
	}
	if !found {
		t.Errorf("PITRCutoff 명시 시 --pitr-cutoff arg 추가돼야: %v", args)
	}
}
