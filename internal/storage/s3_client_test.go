/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// S3Client builder + parseEndpoint 단위 테스트. ADR-0022.
package storage

import (
	"strings"
	"testing"

	"github.com/minio/minio-go/v7/pkg/encrypt"

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

// parseServerSideEncryption — 각 분기 (빈값 / AES256 / aws:kms / aws:kms:<id> /
// 잘못된 값) 테이블 테스트. 실 S3 호출 없이 파싱 + encrypt.ServerSide 구성만 검증.
func TestParseServerSideEncryption(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantNil  bool         // nil ServerSide 기대 (= 암호화 미설정)
		wantType encrypt.Type // wantNil=false 시 기대 Type
		wantErr  bool
	}{
		{name: "빈값은 nil (기존 동작 보존)", in: "", wantNil: true},
		{name: "AES256은 SSE-S3", in: "AES256", wantType: encrypt.S3},
		{name: "aws:kms는 기본키 SSE-KMS", in: "aws:kms", wantType: encrypt.KMS},
		{name: "aws:kms:<id>는 지정키 SSE-KMS", in: "aws:kms:arn:aws:kms:ap-northeast-2:111122223333:key/abcd-1234", wantType: encrypt.KMS},
		{name: "aws:kms: 빈 keyID는 에러", in: "aws:kms:", wantErr: true},
		{name: "알 수 없는 값은 에러", in: "rot13", wantErr: true},
		{name: "소문자 aes256은 에러 (대소문자 구분)", in: "aes256", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseServerSideEncryption(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil (sse=%v)", tc.in, got)
				}
				if got != nil {
					t.Fatalf("expected nil ServerSide on error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil ServerSide for %q, got %v", tc.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil ServerSide for %q, got nil", tc.in)
			}
			if got.Type() != tc.wantType {
				t.Fatalf("for %q: got Type=%q, want %q", tc.in, got.Type(), tc.wantType)
			}
		})
	}
}

// BuildS3Client 가 ServerSideEncryption 을 파싱해 S3Client.sse 에 저장하는지 검증.
func TestBuildS3Client_storesSSE(t *testing.T) {
	t.Run("AES256 저장", func(t *testing.T) {
		s3 := &cachev1alpha1.S3Spec{
			Endpoint:             "https://s3.amazonaws.com",
			Region:               "ap-northeast-2",
			Bucket:               "b",
			ServerSideEncryption: "AES256",
		}
		c, err := BuildS3Client(s3, "AKIA", "secret")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if c.sse == nil || c.sse.Type() != encrypt.S3 {
			t.Fatalf("expected SSE-S3 stored, got %v", c.sse)
		}
	})

	t.Run("빈값은 sse nil (기존 동작)", func(t *testing.T) {
		s3 := &cachev1alpha1.S3Spec{
			Endpoint: "https://s3.amazonaws.com",
			Region:   "us-east-1",
			Bucket:   "b",
		}
		c, err := BuildS3Client(s3, "AKIA", "secret")
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if c.sse != nil {
			t.Fatalf("expected nil sse for empty ServerSideEncryption, got %v", c.sse)
		}
	})

	t.Run("잘못된 값은 BuildS3Client 에러", func(t *testing.T) {
		s3 := &cachev1alpha1.S3Spec{
			Endpoint:             "https://s3.amazonaws.com",
			Region:               "us-east-1",
			Bucket:               "b",
			ServerSideEncryption: "bogus",
		}
		if _, err := BuildS3Client(s3, "AKIA", "secret"); err == nil {
			t.Fatalf("expected error on invalid ServerSideEncryption")
		}
	})
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
