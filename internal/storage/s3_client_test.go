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

S3Client builder + parseEndpoint 단위 테스트. ADR-0022.
*/

package storage

import (
	"strings"
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestParseEndpoint_https(t *testing.T) {
	host, secure, err := parseEndpoint("https://s3.amazonaws.com")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if host != "s3.amazonaws.com" || !secure {
		t.Fatalf("got host=%s secure=%v", host, secure)
	}
}

func TestParseEndpoint_http(t *testing.T) {
	host, secure, err := parseEndpoint("http://minio.local:9000")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if host != "minio.local:9000" || secure {
		t.Fatalf("got host=%s secure=%v", host, secure)
	}
}

func TestParseEndpoint_noScheme_defaultsHTTPS(t *testing.T) {
	host, secure, err := parseEndpoint("s3.example.com")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if host != "s3.example.com" || !secure {
		t.Fatalf("expected default https, got host=%s secure=%v", host, secure)
	}
}

func TestParseEndpoint_empty(t *testing.T) {
	_, _, err := parseEndpoint("")
	if err == nil {
		t.Fatalf("expected error on empty endpoint")
	}
}

func TestParseEndpoint_invalidScheme(t *testing.T) {
	_, _, err := parseEndpoint("ftp://example.com")
	if err == nil || !strings.Contains(err.Error(), "unsupported scheme") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}

func TestBuildS3Client_nilSpec(t *testing.T) {
	_, err := BuildS3Client(nil, "ak", "sk")
	if err == nil {
		t.Fatalf("expected error on nil spec")
	}
}

func TestBuildS3Client_invalidEndpoint(t *testing.T) {
	s3 := &cachev1alpha1.S3Spec{
		Endpoint: "ftp://bad",
		Region:   "us-east-1",
		Bucket:   "b",
	}
	_, err := BuildS3Client(s3, "ak", "sk")
	if err == nil {
		t.Fatalf("expected error on invalid endpoint")
	}
}

func TestBuildS3Client_valid(t *testing.T) {
	s3 := &cachev1alpha1.S3Spec{
		Endpoint:       "https://s3.amazonaws.com",
		Region:         "ap-northeast-2",
		Bucket:         "valkey-backups",
		Prefix:         "cluster-A/",
		ForcePathStyle: false,
	}
	c, err := BuildS3Client(s3, "AKIA", "secret")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.bucket != "valkey-backups" {
		t.Fatalf("bucket=%s", c.bucket)
	}
	if c.prefix != "cluster-A/" {
		t.Fatalf("prefix=%s", c.prefix)
	}
	if got := c.EndpointHost(); got != "s3.amazonaws.com" {
		t.Fatalf("endpoint host=%s", got)
	}
}

func TestBuildS3Client_pathStyleMinIO(t *testing.T) {
	s3 := &cachev1alpha1.S3Spec{
		Endpoint:       "http://minio.local:9000",
		Region:         "us-east-1",
		Bucket:         "test",
		ForcePathStyle: true,
	}
	c, err := BuildS3Client(s3, "minio", "minio123")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.EndpointHost() != "minio.local:9000" {
		t.Fatalf("endpoint host=%s", c.EndpointHost())
	}
}

// Reachable / FPut / FGet 의 직접 호출은 실제 S3/MinIO endpoint 의존이므로
// 본 단위 테스트에서는 *nil receiver* 가드만 검증. 실제 동작 검증은 e2e
// 시나리오 (kind cluster + MinIO 컨테이너) 별개 commit.

func TestS3Client_Reachable_nil(t *testing.T) {
	var c *S3Client
	if _, err := c.Reachable(t.Context()); err == nil {
		t.Fatalf("expected error on nil client")
	}
}

func TestS3Client_FPut_nil(t *testing.T) {
	var c *S3Client
	if _, err := c.FPut(t.Context(), "obj", "/tmp/x"); err == nil {
		t.Fatalf("expected error on nil client")
	}
}

func TestS3Client_FGet_nil(t *testing.T) {
	var c *S3Client
	if err := c.FGet(t.Context(), "obj", "/tmp/x"); err == nil {
		t.Fatalf("expected error on nil client")
	}
}
