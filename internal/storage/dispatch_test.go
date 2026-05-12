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
*/

package storage

import (
	"context"
	"strings"
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestNewClient_S3_with_spec_passes(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		Type: cachev1alpha1.BackupTargetTypeS3,
		S3: &cachev1alpha1.S3Spec{
			Endpoint: "https://s3.amazonaws.com",
			Region:   "us-east-1",
			Bucket:   "test-bucket",
		},
	}
	c, err := NewClient(context.Background(), spec, Credentials{
		S3AccessKey: "AKIA...",
		S3SecretKey: "secret",
	})
	if err != nil {
		t.Fatalf("S3 build: %v", err)
	}
	if _, ok := c.(*S3Client); !ok {
		t.Errorf("expected *S3Client, got %T", c)
	}
}

func TestNewClient_S3_default_when_type_empty(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		// Type 미명시 — CRD default 적용 전 케이스 (controller 첫 read).
		S3: &cachev1alpha1.S3Spec{
			Endpoint: "https://s3.amazonaws.com",
			Region:   "us-east-1",
			Bucket:   "test-bucket",
		},
	}
	c, err := NewClient(context.Background(), spec, Credentials{S3AccessKey: "k", S3SecretKey: "s"})
	if err != nil {
		t.Fatalf("S3 default build: %v", err)
	}
	if _, ok := c.(*S3Client); !ok {
		t.Errorf("expected *S3Client (default), got %T", c)
	}
}

func TestNewClient_S3_without_subspec_rejects(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		Type: cachev1alpha1.BackupTargetTypeS3,
		// S3 nil — webhook 가 사전 reject 해야 하지만 dispatch 도 가드.
	}
	_, err := NewClient(context.Background(), spec, Credentials{})
	if err == nil {
		t.Fatal("expected error for type=S3 without spec.s3")
	}
	if !strings.Contains(err.Error(), "spec.s3") {
		t.Errorf("error message: %v", err)
	}
}

func TestNewClient_GCS_with_spec_passes(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		Type: cachev1alpha1.BackupTargetTypeGCS,
		GCS: &cachev1alpha1.GCSSpec{
			Bucket: "test-bucket",
		},
	}
	// 최소 service account JSON (parse 가능한 stub) — 실제 GCS 호출 안 함.
	saJSON := []byte(`{"type":"service_account","project_id":"p","private_key_id":"k",` +
		`"private_key":"-----BEGIN PRIVATE KEY-----\nFAKE\n-----END PRIVATE KEY-----\n",` +
		`"client_email":"x@y","client_id":"1","auth_uri":"https://accounts.google.com/o/oauth2/auth",` +
		`"token_uri":"https://oauth2.googleapis.com/token"}`)
	c, err := NewClient(context.Background(), spec, Credentials{GCSServiceAccountJSON: saJSON})
	if err != nil {
		t.Fatalf("GCS build: %v", err)
	}
	if _, ok := c.(*GCSClient); !ok {
		t.Errorf("expected *GCSClient, got %T", c)
	}
	defer func() { _ = c.(*GCSClient).Close() }()
	if c.EndpointHost() != "storage.googleapis.com" {
		t.Errorf("endpoint host: %q", c.EndpointHost())
	}
}

func TestNewClient_GCS_without_subspec_rejects(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		Type: cachev1alpha1.BackupTargetTypeGCS,
	}
	_, err := NewClient(context.Background(), spec, Credentials{})
	if err == nil {
		t.Fatal("expected error for type=GCS without spec.gcs")
	}
	if !strings.Contains(err.Error(), "spec.gcs") {
		t.Errorf("error message: %v", err)
	}
}

func TestNewClient_Azure_with_spec_passes(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		Type: cachev1alpha1.BackupTargetTypeAzure,
		Azure: &cachev1alpha1.AzureSpec{
			AccountName: "mystorage",
			Container:   "backups",
		},
	}
	// account key 는 base64 — Azure SDK 가 디코드 시도. FAKE 라도 NewSharedKeyCredential 통과.
	c, err := NewClient(context.Background(), spec, Credentials{AzureAccountKey: "YWJjZGVmZ2hpams="})
	if err != nil {
		t.Fatalf("Azure build: %v", err)
	}
	if _, ok := c.(*AzureClient); !ok {
		t.Errorf("expected *AzureClient, got %T", c)
	}
	if !strings.HasPrefix(c.EndpointHost(), "https://mystorage.blob.core.windows.net") {
		t.Errorf("Azure default URL: %q", c.EndpointHost())
	}
}

func TestNewClient_Azure_with_serviceURL_override(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		Type: cachev1alpha1.BackupTargetTypeAzure,
		Azure: &cachev1alpha1.AzureSpec{
			AccountName: "devstoreaccount1",
			Container:   "backups",
			ServiceURL:  "http://127.0.0.1:10000/devstoreaccount1/",
		},
	}
	c, err := NewClient(context.Background(), spec,
		Credentials{AzureAccountKey: "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="})
	if err != nil {
		t.Fatalf("Azure with override: %v", err)
	}
	if c.EndpointHost() != "http://127.0.0.1:10000/devstoreaccount1/" {
		t.Errorf("override endpoint: %q", c.EndpointHost())
	}
}

func TestNewClient_Azure_without_subspec_rejects(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		Type: cachev1alpha1.BackupTargetTypeAzure,
	}
	_, err := NewClient(context.Background(), spec, Credentials{})
	if err == nil {
		t.Fatal("expected error for type=Azure without spec.azure")
	}
	if !strings.Contains(err.Error(), "spec.azure") {
		t.Errorf("error message: %v", err)
	}
}

func TestNewClient_unknown_type_rejects(t *testing.T) {
	spec := &cachev1alpha1.ValkeyBackupTargetSpec{
		Type: "Unknown",
	}
	_, err := NewClient(context.Background(), spec, Credentials{})
	if err == nil {
		t.Fatal("expected error for unknown Type")
	}
}

func TestNewClient_nil_spec_rejects(t *testing.T) {
	_, err := NewClient(context.Background(), nil, Credentials{})
	if err == nil {
		t.Fatal("expected error for nil spec")
	}
}
