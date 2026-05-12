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

// Package storage — Azure Blob Storage wrapper
// (github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.4).
//
// ADR-0016 + ADR-0040 §gap #2. SDK 는 Microsoft 공식. 라이선스 MIT. sonatype-guide
// PURL ecosystem 미수록 (verifier gap, Microsoft 공식 source 직접 검증).
package storage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// AzureClient — ValkeyBackupTarget.Spec.Azure + account key 로 초기화.
type AzureClient struct {
	client      *azblob.Client
	containerNm string
	prefix      string
	serviceURL  string
}

// BuildAzureClient — Spec + accountKey 로 client 생성.
//
// serviceURL 미명시 시 기본 https://<accountName>.blob.core.windows.net.
// Azurite (test) 또는 Azure China / Government 시 ServiceURL override.
func BuildAzureClient(spec *cachev1alpha1.AzureSpec, accountKey string) (*AzureClient, error) {
	if spec == nil {
		return nil, fmt.Errorf("AzureSpec nil")
	}
	if spec.AccountName == "" {
		return nil, fmt.Errorf("AzureSpec.AccountName empty")
	}
	if accountKey == "" {
		return nil, fmt.Errorf("azure account key empty")
	}
	cred, err := azblob.NewSharedKeyCredential(spec.AccountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("azblob.NewSharedKeyCredential: %w", err)
	}
	svcURL := spec.ServiceURL
	if svcURL == "" {
		svcURL = fmt.Sprintf("https://%s.blob.core.windows.net/", spec.AccountName)
	}
	c, err := azblob.NewClientWithSharedKeyCredential(svcURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azblob.NewClientWithSharedKeyCredential: %w", err)
	}
	return &AzureClient{
		client:      c,
		containerNm: spec.Container,
		prefix:      spec.Prefix,
		serviceURL:  svcURL,
	}, nil
}

// Reachable — container properties 조회. 404=not exists / 403=auth fail.
func (c *AzureClient) Reachable(ctx context.Context) (bool, error) {
	if c == nil || c.client == nil {
		return false, fmt.Errorf("AzureClient not initialized")
	}
	_, err := c.client.ServiceClient().NewContainerClient(c.containerNm).GetProperties(ctx, nil)
	if err == nil {
		return true, nil
	}
	// not exists 분류 — bloberror.ContainerNotFound.
	if isContainerNotFoundErr(err) {
		return false, nil
	}
	return false, fmt.Errorf("ContainerClient.GetProperties: %w", err)
}

// FPut — 로컬 파일 → Azure blob.
func (c *AzureClient) FPut(ctx context.Context, objectKey, filePath string) (int64, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("AzureClient not initialized")
	}
	full := c.prefix + objectKey
	f, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", filePath, err)
	}

	_, err = c.client.UploadFile(ctx, c.containerNm, full, f, &azblob.UploadFileOptions{})
	if err != nil {
		return 0, fmt.Errorf("UploadFile %s/%s: %w", c.containerNm, full, err)
	}
	return stat.Size(), nil
}

// FGet — Azure blob → 로컬 파일.
func (c *AzureClient) FGet(ctx context.Context, objectKey, filePath string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("AzureClient not initialized")
	}
	full := c.prefix + objectKey
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	_, err = c.client.DownloadFile(ctx, c.containerNm, full, f, &azblob.DownloadFileOptions{})
	if err != nil {
		return fmt.Errorf("DownloadFile %s/%s: %w", c.containerNm, full, err)
	}
	return nil
}

// EndpointHost — Azure service URL host.
func (c *AzureClient) EndpointHost() string {
	if c == nil {
		return ""
	}
	return c.serviceURL
}

func isContainerNotFoundErr(err error) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		if respErr.StatusCode == http.StatusNotFound {
			return true
		}
		if respErr.ErrorCode == string(bloberror.ContainerNotFound) {
			return true
		}
	}
	return false
}

// 컴파일 에러 방지: blob/container 패키지가 future evolution 에서 사용될 수 있음.
var (
	_ = blob.ClientOptions{}
	_ = container.ClientOptions{}
)
