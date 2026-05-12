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
package v1alpha1

import (
	"context"
	"strings"
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func validS3Target() *cachev1alpha1.ValkeyBackupTarget {
	return &cachev1alpha1.ValkeyBackupTarget{
		Spec: cachev1alpha1.ValkeyBackupTargetSpec{
			Type: cachev1alpha1.BackupTargetTypeS3,
			S3: &cachev1alpha1.S3Spec{
				Endpoint:             "https://s3.amazonaws.com",
				Region:               "us-east-1",
				Bucket:               "backups",
				CredentialsSecretRef: cachev1alpha1.S3CredentialsSecretRef{Name: "s3-creds"},
			},
		},
	}
}

func validGCSTarget() *cachev1alpha1.ValkeyBackupTarget {
	return &cachev1alpha1.ValkeyBackupTarget{
		Spec: cachev1alpha1.ValkeyBackupTargetSpec{
			Type: cachev1alpha1.BackupTargetTypeGCS,
			GCS: &cachev1alpha1.GCSSpec{
				Bucket:               "backups",
				CredentialsSecretRef: cachev1alpha1.GCSCredentialsSecretRef{Name: "gcs-sa"},
			},
		},
	}
}

func validAzureTarget() *cachev1alpha1.ValkeyBackupTarget {
	return &cachev1alpha1.ValkeyBackupTarget{
		Spec: cachev1alpha1.ValkeyBackupTargetSpec{
			Type: cachev1alpha1.BackupTargetTypeAzure,
			Azure: &cachev1alpha1.AzureSpec{
				AccountName:          "mystorage",
				Container:            "backups",
				CredentialsSecretRef: cachev1alpha1.AzureCredentialsSecretRef{Name: "azure-key"},
			},
		},
	}
}

func TestBackupTargetValidate_S3_GCS_Azure_minimal_pass(t *testing.T) {
	v := &ValkeyBackupTargetCustomValidator{}
	for _, c := range []struct {
		name string
		obj  *cachev1alpha1.ValkeyBackupTarget
	}{
		{"S3", validS3Target()},
		{"GCS", validGCSTarget()},
		{"Azure", validAzureTarget()},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := v.ValidateCreate(context.Background(), c.obj); err != nil {
				t.Errorf("expected valid, got %v", err)
			}
		})
	}
}

func TestBackupTargetValidate_type_subspec_mismatch_rejected(t *testing.T) {
	v := &ValkeyBackupTargetCustomValidator{}

	t.Run("S3 type with GCS sub-spec rejected", func(t *testing.T) {
		obj := validS3Target()
		obj.Spec.GCS = &cachev1alpha1.GCSSpec{Bucket: "x"}
		_, err := v.ValidateCreate(context.Background(), obj)
		if err == nil || !strings.Contains(err.Error(), "spec.gcs must be omitted") {
			t.Errorf("expected GCS-omit reject, got %v", err)
		}
	})

	t.Run("GCS type without sub-spec rejected", func(t *testing.T) {
		obj := &cachev1alpha1.ValkeyBackupTarget{
			Spec: cachev1alpha1.ValkeyBackupTargetSpec{Type: cachev1alpha1.BackupTargetTypeGCS},
		}
		_, err := v.ValidateCreate(context.Background(), obj)
		if err == nil || !strings.Contains(err.Error(), "spec.gcs") {
			t.Errorf("expected gcs-required reject, got %v", err)
		}
	})

	t.Run("Azure type with S3 sub-spec rejected", func(t *testing.T) {
		obj := validAzureTarget()
		obj.Spec.S3 = &cachev1alpha1.S3Spec{Endpoint: "x", Bucket: "x"}
		_, err := v.ValidateCreate(context.Background(), obj)
		if err == nil || !strings.Contains(err.Error(), "spec.s3 must be omitted") {
			t.Errorf("expected s3-omit reject, got %v", err)
		}
	})
}

func TestBackupTargetValidate_required_fields(t *testing.T) {
	v := &ValkeyBackupTargetCustomValidator{}

	t.Run("S3 missing endpoint", func(t *testing.T) {
		obj := validS3Target()
		obj.Spec.S3.Endpoint = ""
		_, err := v.ValidateCreate(context.Background(), obj)
		if err == nil || !strings.Contains(err.Error(), "endpoint") {
			t.Errorf("expected endpoint-required, got %v", err)
		}
	})

	t.Run("S3 missing credentials secret name", func(t *testing.T) {
		obj := validS3Target()
		obj.Spec.S3.CredentialsSecretRef.Name = ""
		_, err := v.ValidateCreate(context.Background(), obj)
		if err == nil || !strings.Contains(err.Error(), "secret name required") {
			t.Errorf("expected secret-required, got %v", err)
		}
	})

	t.Run("GCS missing bucket", func(t *testing.T) {
		obj := validGCSTarget()
		obj.Spec.GCS.Bucket = ""
		_, err := v.ValidateCreate(context.Background(), obj)
		if err == nil || !strings.Contains(err.Error(), "bucket") {
			t.Errorf("expected bucket-required, got %v", err)
		}
	})

	t.Run("Azure missing accountName", func(t *testing.T) {
		obj := validAzureTarget()
		obj.Spec.Azure.AccountName = ""
		_, err := v.ValidateCreate(context.Background(), obj)
		if err == nil || !strings.Contains(err.Error(), "accountName") {
			t.Errorf("expected accountName-required, got %v", err)
		}
	})
}

func TestBackupTargetValidate_Update_type_immutable(t *testing.T) {
	v := &ValkeyBackupTargetCustomValidator{}
	old := validS3Target()
	new := validGCSTarget()

	_, err := v.ValidateUpdate(context.Background(), old, new)
	if err == nil || !strings.Contains(err.Error(), "spec.type is immutable") {
		t.Errorf("expected type-immutable reject, got %v", err)
	}
}

func TestBackupTargetValidate_unknown_type_rejected(t *testing.T) {
	v := &ValkeyBackupTargetCustomValidator{}
	obj := &cachev1alpha1.ValkeyBackupTarget{
		Spec: cachev1alpha1.ValkeyBackupTargetSpec{Type: "Unknown"},
	}
	_, err := v.ValidateCreate(context.Background(), obj)
	if err == nil {
		t.Fatal("expected unknown type reject")
	}
}

func TestBackupTargetDefaulter_empty_type_defaults_to_S3(t *testing.T) {
	d := &ValkeyBackupTargetCustomDefaulter{}
	obj := &cachev1alpha1.ValkeyBackupTarget{}
	if err := d.Default(context.Background(), obj); err != nil {
		t.Fatalf("default: %v", err)
	}
	if obj.Spec.Type != cachev1alpha1.BackupTargetTypeS3 {
		t.Errorf("default Type = %q, want S3", obj.Spec.Type)
	}
}

func TestBackupTargetDefaulter_explicit_type_preserved(t *testing.T) {
	d := &ValkeyBackupTargetCustomDefaulter{}
	obj := &cachev1alpha1.ValkeyBackupTarget{
		Spec: cachev1alpha1.ValkeyBackupTargetSpec{Type: cachev1alpha1.BackupTargetTypeAzure},
	}
	_ = d.Default(context.Background(), obj)
	if obj.Spec.Type != cachev1alpha1.BackupTargetTypeAzure {
		t.Errorf("explicit Type changed: %q", obj.Spec.Type)
	}
}
