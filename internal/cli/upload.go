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

`valkey-operator upload` sub-command — S3 에 파일 업로드 (ADR-0023).
*/

package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/storage"
)

// uploadOptions — flag 파싱 결과.
type uploadOptions struct {
	bucket   string
	object   string
	filePath string
}

// runUpload — flag + env 로 S3Client 구성 후 FPut 호출.
//
// flags:
//
//	--bucket=<S3 bucket name>
//	--object=<key inside bucket, prefix 포함 전체 경로>
//	--file=<로컬 파일 경로>
//
// env:
//
//	VALKEY_S3_ENDPOINT, VALKEY_S3_REGION, VALKEY_S3_FORCE_PATH_STYLE
//	VALKEY_S3_ACCESS_KEY_ID, VALKEY_S3_SECRET_ACCESS_KEY
func runUpload(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("upload", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bucket := fs.String("bucket", "", "S3 bucket name (required)")
	object := fs.String("object", "", "object key inside bucket (required)")
	filePath := fs.String("file", "", "local file path to upload (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := uploadOptions{bucket: *bucket, object: *object, filePath: *filePath}
	return runUploadWithBuilder(ctx, opts, stdout, defaultBuildClient)
}

// clientBuilder — 테스트 주입용 indirection.
type clientBuilder func(s3 *cachev1alpha1.S3Spec, ak, sk string) (uploader, error)

// uploader — runUpload 가 사용하는 단일 메서드 (storage.S3Client 매핑).
type uploader interface {
	FPut(ctx context.Context, objectKey, filePath string) (int64, error)
	EndpointHost() string
}

func defaultBuildClient(s3 *cachev1alpha1.S3Spec, ak, sk string) (uploader, error) {
	return storage.BuildS3Client(s3, ak, sk)
}

// runUploadWithBuilder — 본 함수가 실제 로직. 테스트는 mock builder 주입.
func runUploadWithBuilder(
	ctx context.Context,
	opts uploadOptions,
	stdout io.Writer,
	build clientBuilder,
) error {
	if opts.bucket == "" || opts.object == "" || opts.filePath == "" {
		return fmt.Errorf("--bucket, --object, --file all required (got bucket=%q object=%q file=%q)",
			opts.bucket, opts.object, opts.filePath)
	}
	s3 := s3SpecFromEnv(opts.bucket)
	ak := envOr("VALKEY_S3_ACCESS_KEY_ID", "")
	sk := envOr("VALKEY_S3_SECRET_ACCESS_KEY", "")
	if ak == "" || sk == "" {
		return fmt.Errorf("VALKEY_S3_ACCESS_KEY_ID + VALKEY_S3_SECRET_ACCESS_KEY env required")
	}
	c, err := build(s3, ak, sk)
	if err != nil {
		return fmt.Errorf("build S3 client: %w", err)
	}
	size, err := c.FPut(ctx, opts.object, opts.filePath)
	if err != nil {
		return fmt.Errorf("upload %s → s3://%s/%s: %w",
			opts.filePath, opts.bucket, opts.object, err)
	}
	_, _ = fmt.Fprintf(stdout, "uploaded %s (%d bytes) → %s/%s\n",
		opts.filePath, size, c.EndpointHost(), opts.object)
	return nil
}

// s3SpecFromEnv — env 에서 S3Spec 구성. bucket 은 인자, 나머지는 env.
func s3SpecFromEnv(bucket string) *cachev1alpha1.S3Spec {
	return &cachev1alpha1.S3Spec{
		Endpoint:       envOr("VALKEY_S3_ENDPOINT", ""),
		Region:         envOr("VALKEY_S3_REGION", "us-east-1"),
		Bucket:         bucket,
		ForcePathStyle: envBool("VALKEY_S3_FORCE_PATH_STYLE"),
		// Prefix 는 caller (controller) 가 object key 에 이미 포함시켜 전달 — sub-command 는 prefix 인지 안 함.
	}
}
