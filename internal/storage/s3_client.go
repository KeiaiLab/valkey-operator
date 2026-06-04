/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package storage — S3-compatible object storage wrapper (minio-go v7).
// ADR-0022 채택. 본 패키지가 minio-go import 를 *유일하게* 가지며, 외부에는
// ADR-0016 의 ValkeyBackupTarget.Spec.S3 추상화를 노출.
package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// S3Client — ValkeyBackupTarget.Spec.S3 + 자격증명 으로 초기화된 minio Client wrapper.
type S3Client struct {
	mc     *minio.Client
	bucket string
	prefix string

	// sse — 업로드 시 적용할 서버측 암호화. nil 이면 암호화 미설정 (provider 기본값).
	// S3Spec.ServerSideEncryption 파싱 결과 (parseServerSideEncryption).
	sse encrypt.ServerSide
}

// BuildS3Client — Spec + 자격증명 으로 client 생성.
//
// endpoint 의 scheme (http:// vs https://) 으로 Secure 결정. 미지정 시 https://
// 강제 (보안 기본값).
//
// forcePathStyle=true 면 BucketLookupPath (MinIO/Ceph RGW 호환).
func BuildS3Client(s3 *cachev1alpha1.S3Spec, accessKey, secretKey string) (*S3Client, error) {
	if s3 == nil {
		return nil, fmt.Errorf("S3Spec nil")
	}
	endpoint, secure, err := parseEndpoint(s3.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint %q: %w", s3.Endpoint, err)
	}

	sse, err := parseServerSideEncryption(s3.ServerSideEncryption)
	if err != nil {
		return nil, fmt.Errorf("invalid serverSideEncryption %q: %w", s3.ServerSideEncryption, err)
	}

	bucketLookup := minio.BucketLookupAuto
	if s3.ForcePathStyle {
		bucketLookup = minio.BucketLookupPath
	}

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       secure,
		Region:       s3.Region,
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}
	return &S3Client{mc: mc, bucket: s3.Bucket, prefix: s3.Prefix, sse: sse}, nil
}

// Reachable — BucketExists 호출. true=권한 + reachability OK.
//
// 실패 사유 분류 (caller 가 Reason 매핑):
//   - DNS 해석 실패 / 네트워크 도달 불가 → err != nil, exists=false
//   - 권한 없음 (403) → err != nil with "AccessDenied"
//   - 자격증명 invalid (401) → err != nil with "InvalidAccessKeyId"
//   - bucket 부재 → exists=false, err=nil
//
// 첫 commit 은 단순 *exists OR error* 만 반환 — 실패 사유 분류 는 별개 commit.
func (c *S3Client) Reachable(ctx context.Context) (bool, error) {
	if c == nil || c.mc == nil {
		return false, fmt.Errorf("S3Client not initialized")
	}
	return c.mc.BucketExists(ctx, c.bucket)
}

// FPut — 로컬 파일 → S3 object 업로드. ContentType 기본 application/octet-stream.
// objectKey 가 prefix 와 결합되어 최종 key = c.prefix + objectKey.
//
// 본 wrapper 는 *binary RDB* 만 다루므로 multipart 자동 (minio-go default 64MB).
func (c *S3Client) FPut(ctx context.Context, objectKey, filePath string) (int64, error) {
	if c == nil || c.mc == nil {
		return 0, fmt.Errorf("S3Client not initialized")
	}
	full := c.prefix + objectKey
	info, err := c.mc.FPutObject(ctx, c.bucket, full, filePath, minio.PutObjectOptions{
		ContentType:          "application/octet-stream",
		ServerSideEncryption: c.sse, // nil 이면 minio-go 가 SSE 헤더 미설정 (기존 동작 보존).
	})
	if err != nil {
		return 0, fmt.Errorf("FPutObject %s/%s: %w", c.bucket, full, err)
	}
	return info.Size, nil
}

// FGet — S3 object → 로컬 파일 다운로드. objectKey 가 prefix 와 결합.
func (c *S3Client) FGet(ctx context.Context, objectKey, filePath string) error {
	if c == nil || c.mc == nil {
		return fmt.Errorf("S3Client not initialized")
	}
	full := c.prefix + objectKey
	if err := c.mc.FGetObject(ctx, c.bucket, full, filePath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("FGetObject %s/%s: %w", c.bucket, full, err)
	}
	return nil
}

// EndpointHost — endpoint URL 의 host 부분만 (디버그/로그용).
func (c *S3Client) EndpointHost() string {
	if c == nil || c.mc == nil {
		return ""
	}
	return c.mc.EndpointURL().Host
}

// parseEndpoint — URL 에서 host:port + Secure(https) 추출. scheme 미지정 시
// https:// 강제 (보안 기본값).
func parseEndpoint(raw string) (host string, secure bool, err error) {
	if raw == "" {
		return "", false, fmt.Errorf("endpoint empty")
	}
	// scheme 없으면 https:// 가정.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false, err
	}
	if u.Host == "" {
		return "", false, fmt.Errorf("missing host")
	}
	switch u.Scheme {
	case "https":
		return u.Host, true, nil
	case "http":
		return u.Host, false, nil
	default:
		return "", false, fmt.Errorf("unsupported scheme %q (want http/https)", u.Scheme)
	}
}

// parseServerSideEncryption — S3Spec.ServerSideEncryption 문자열 → minio encrypt.ServerSide.
//
// 지원 값 (AWS S3 / S3-compatible 표준):
//   - ""              → nil (암호화 미설정, provider 기본값 — 기존 동작 보존)
//   - "AES256"        → SSE-S3 (server-managed key, encrypt.NewSSE)
//   - "aws:kms"       → SSE-KMS, AWS-managed default key (keyID 미지정)
//   - "aws:kms:<id>"  → SSE-KMS, 지정 KMS keyID
//
// 알 수 없는 값은 명확한 error — 오타가 *silent 평문 업로드* 로 이어지지 않도록.
func parseServerSideEncryption(raw string) (encrypt.ServerSide, error) {
	switch {
	case raw == "":
		return nil, nil
	case raw == "AES256":
		return encrypt.NewSSE(), nil
	case raw == "aws:kms":
		// keyID 빈 문자열 = AWS 계정 기본 KMS key 사용 (AWS S3 표준 동작).
		return encrypt.NewSSEKMS("", nil)
	case strings.HasPrefix(raw, "aws:kms:"):
		keyID := strings.TrimPrefix(raw, "aws:kms:")
		if keyID == "" {
			return nil, fmt.Errorf("aws:kms key id empty (use %q for default key)", "aws:kms")
		}
		return encrypt.NewSSEKMS(keyID, nil)
	default:
		return nil, fmt.Errorf("unsupported value %q (want \"AES256\", \"aws:kms\", or \"aws:kms:<keyID>\")", raw)
	}
}
