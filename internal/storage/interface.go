/*
Copyright 2026 Keiailab.
*/

package storage

import "context"

// ObjectStorageClient — backup target type (S3/GCS/Azure) 추상화.
//
// backup/restore Job 생성자가 본 interface 만 사용 — 구체 구현 (s3_client / gcs_client
// / azure_client) 는 dispatch 로 선택. ADR-0016 + ADR-0040 §gap #2.
//
// 모든 메서드는 caller 가 ctx timeout 을 책임 (보통 30s 권장).
type ObjectStorageClient interface {
	// Reachable — bucket/container 도달 가능 + 자격증명 valid 검증.
	// true: bucket/container exists + 권한 OK.
	// false + nil err: 명시적 not-exists.
	// false + err: 네트워크 / 자격증명 / 권한 fail.
	Reachable(ctx context.Context) (bool, error)

	// FPut — 로컬 파일 → object 업로드. objectKey 는 prefix 와 결합.
	// 반환: 업로드된 object 크기 (bytes).
	FPut(ctx context.Context, objectKey, filePath string) (int64, error)

	// FGet — object → 로컬 파일 다운로드. objectKey 는 prefix 와 결합.
	FGet(ctx context.Context, objectKey, filePath string) error

	// EndpointHost — log/debug 용 host string.
	EndpointHost() string
}
