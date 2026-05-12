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

// Package storage — GCS object storage wrapper (cloud.google.com/go/storage v1.62.1).
//
// ADR-0016 + ADR-0040 §gap #2. sonatype-guide 검증 (2026-05-10): Apache-2.0,
// vulnerabilities 0, malicious 0, not EOL.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"

	gcs "cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// GCSClient — ValkeyBackupTarget.Spec.GCS + service account JSON 으로 초기화.
type GCSClient struct {
	client *gcs.Client
	bucket string
	prefix string
}

// BuildGCSClient — Spec + service account JSON bytes 로 client 생성.
//
// 자격증명: GCP service account 의 JSON key (Secret 의 한 key 에 저장). caller
// 가 Secret 에서 읽어 saJSON 으로 전달. WithCredentialsJSON 으로 client 초기화.
func BuildGCSClient(ctx context.Context, spec *cachev1alpha1.GCSSpec, saJSON []byte) (*GCSClient, error) {
	if spec == nil {
		return nil, fmt.Errorf("GCSSpec nil")
	}
	if len(saJSON) == 0 {
		return nil, fmt.Errorf("service account JSON empty")
	}
	// google.CredentialsFromJSON 은 SA1019 deprecated 표시: 입력 검증 부재가 위험
	// 사유. 본 operator 는 k8s Secret 에서 *namespace + RBAC 통제하* 자격증명을
	// 읽으므로 외부 untrusted 소스 위험에 해당 안 함 — nolint 정당.
	creds, err := google.CredentialsFromJSON(ctx, saJSON, //nolint:staticcheck // SA1019: input is k8s Secret, namespace+RBAC controlled
		"https://www.googleapis.com/auth/devstorage.full_control",
	)
	if err != nil {
		return nil, fmt.Errorf("google.CredentialsFromJSON: %w", err)
	}
	c, err := gcs.NewClient(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("gcs.NewClient: %w", err)
	}
	return &GCSClient{client: c, bucket: spec.Bucket, prefix: spec.Prefix}, nil
}

// Reachable — bucket attrs 조회 → exists + 권한 검증.
func (c *GCSClient) Reachable(ctx context.Context) (bool, error) {
	if c == nil || c.client == nil {
		return false, fmt.Errorf("GCSClient not initialized")
	}
	_, err := c.client.Bucket(c.bucket).Attrs(ctx)
	if err != nil {
		// gcs.ErrBucketNotExist 는 명시적 not-exists 시그널.
		if err == gcs.ErrBucketNotExist {
			return false, nil
		}
		return false, fmt.Errorf("Bucket(%s).Attrs: %w", c.bucket, err)
	}
	return true, nil
}

// FPut — 로컬 파일 → GCS object.
func (c *GCSClient) FPut(ctx context.Context, objectKey, filePath string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("GCSClient not initialized")
	}
	full := c.prefix + objectKey
	f, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	w := c.client.Bucket(c.bucket).Object(full).NewWriter(ctx)
	w.ContentType = "application/octet-stream"
	written, err := io.Copy(w, f)
	if err != nil {
		_ = w.Close()
		return 0, fmt.Errorf("copy to gcs %s/%s: %w", c.bucket, full, err)
	}
	if err := w.Close(); err != nil {
		return 0, fmt.Errorf("writer close gcs %s/%s: %w", c.bucket, full, err)
	}
	return written, nil
}

// FGet — GCS object → 로컬 파일.
func (c *GCSClient) FGet(ctx context.Context, objectKey, filePath string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("GCSClient not initialized")
	}
	full := c.prefix + objectKey
	r, err := c.client.Bucket(c.bucket).Object(full).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("NewReader %s/%s: %w", c.bucket, full, err)
	}
	defer func() { _ = r.Close() }()

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("copy from gcs %s/%s: %w", c.bucket, full, err)
	}
	return nil
}

// EndpointHost — GCS standard endpoint.
func (c *GCSClient) EndpointHost() string { return "storage.googleapis.com" }

// Close — gcs Client 의 background goroutine cleanup.
func (c *GCSClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
