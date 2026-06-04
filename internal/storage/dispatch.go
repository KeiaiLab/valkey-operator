/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package storage

import (
	"context"
	"fmt"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// Credentials — provider-별 자격증명 union (caller 가 Secret 에서 채워서 전달).
//
// 호출자는 BackupTargetType 에 맞는 필드만 채우면 됨 — dispatch 가 unused 무시.
type Credentials struct {
	// S3 / S3-compatible.
	S3AccessKey string
	S3SecretKey string

	// GCS service account JSON (Secret 의 `key.json` 등).
	GCSServiceAccountJSON []byte

	// Azure storage account key.
	AzureAccountKey string
}

// NewClient — ValkeyBackupTarget.Spec 의 Type 에 따라 적절한
// ObjectStorageClient 구현을 반환. ADR-0040 §gap #2 의 dispatch 진입점.
//
// 사전 조건: webhook 가 Spec.Type 과 sub-spec 의 일관성을 검증 — 본 함수는
// runtime 안전성만 보장 (sub-spec nil 시 에러).
func NewClient(ctx context.Context, spec *cachev1alpha1.ValkeyBackupTargetSpec, creds Credentials) (ObjectStorageClient, error) {
	if spec == nil {
		return nil, fmt.Errorf("ValkeyBackupTargetSpec nil")
	}
	switch spec.Type {
	case cachev1alpha1.BackupTargetTypeS3, "": // empty 는 default S3 (CRD default).
		if spec.S3 == nil {
			return nil, fmt.Errorf("type=S3 requires spec.s3")
		}
		return BuildS3Client(spec.S3, creds.S3AccessKey, creds.S3SecretKey)
	case cachev1alpha1.BackupTargetTypeGCS:
		if spec.GCS == nil {
			return nil, fmt.Errorf("type=GCS requires spec.gcs")
		}
		return BuildGCSClient(ctx, spec.GCS, creds.GCSServiceAccountJSON)
	case cachev1alpha1.BackupTargetTypeAzure:
		if spec.Azure == nil {
			return nil, fmt.Errorf("type=Azure requires spec.azure")
		}
		return BuildAzureClient(spec.Azure, creds.AzureAccountKey)
	default:
		return nil, fmt.Errorf("unsupported backup target type: %q", spec.Type)
	}
}
