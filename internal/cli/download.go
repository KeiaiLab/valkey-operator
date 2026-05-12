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

// `valkey-operator download` sub-command — S3 에서 파일 다운로드 (ADR-0023).
// ValkeyRestore Mounting phase 의 외부 source 에 사용.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/aoftime"
	"github.com/keiailab/valkey-operator/internal/storage"
)

type downloadOptions struct {
	bucket   string
	object   string
	filePath string
	// pitrCutoff — RFC3339 시각. 비어있지 않으면 download 후 AOF 를 본 시각까지
	// truncate (in-place, dst=src). PITR phase 2 reconciler dispatch (PR #70).
	pitrCutoff string
	// pitrBackup — 비어있지 않으면 truncate 전에 *원본 AOF 를 본 경로 에 backup*
	// (PR #72 rollback foundation). reconciler 가 init container 부팅 실패 시
	// 본 backup 으로 복원 → 전체 AOF replay fallback.
	pitrBackup string
}

// downloader — runDownload 가 사용하는 단일 메서드.
type downloader interface {
	FGet(ctx context.Context, objectKey, filePath string) error
	EndpointHost() string
}

type downloadBuilder func(s3 *cachev1alpha1.S3Spec, ak, sk string) (downloader, error)

func defaultBuildDownloader(s3 *cachev1alpha1.S3Spec, ak, sk string) (downloader, error) {
	return storage.BuildS3Client(s3, ak, sk)
}

// runDownload — flag + env 로 S3Client 구성 후 FGet 호출.
//
// flags:
//
//	--bucket=<S3 bucket name>
//	--object=<key inside bucket>
//	--file=<로컬 파일 경로 — 다운로드 대상>
func runDownload(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bucket := fs.String("bucket", "", "S3 bucket name (required)")
	object := fs.String("object", "", "object key inside bucket (required)")
	filePath := fs.String("file", "", "local file path to download to (required)")
	pitrCutoff := fs.String("pitr-cutoff", "",
		"optional RFC3339 timestamp. If set, download AOF then truncate to cutoff (PITR phase 2)")
	pitrBackup := fs.String("pitr-backup", "",
		"optional file path. If set with --pitr-cutoff, original AOF preserved here before truncate "+
			"(reconciler 가 replay 실패 시 본 파일로 rollback, PR #72)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := downloadOptions{
		bucket: *bucket, object: *object, filePath: *filePath,
		pitrCutoff: *pitrCutoff, pitrBackup: *pitrBackup,
	}
	return runDownloadWithBuilder(ctx, opts, stdout, defaultBuildDownloader)
}

func runDownloadWithBuilder(
	ctx context.Context,
	opts downloadOptions,
	stdout io.Writer,
	build downloadBuilder,
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
	if err := c.FGet(ctx, opts.object, opts.filePath); err != nil {
		return fmt.Errorf("download s3://%s/%s → %s: %w",
			opts.bucket, opts.object, opts.filePath, err)
	}
	_, _ = fmt.Fprintf(stdout, "downloaded %s/%s → %s\n",
		c.EndpointHost(), opts.object, opts.filePath)

	// PITR phase 2: --pitr-cutoff 명시 시 AOF in-place truncate.
	if opts.pitrCutoff != "" {
		cutoff, err := time.Parse(time.RFC3339, opts.pitrCutoff)
		if err != nil {
			return fmt.Errorf("parse --pitr-cutoff %q (RFC3339 expected): %w", opts.pitrCutoff, err)
		}
		written, truncated, err := aoftime.TruncateAOFFileWithBackup(
			opts.filePath, opts.filePath, opts.pitrBackup, cutoff)
		if err != nil {
			return fmt.Errorf("PITR truncate: %w", err)
		}
		if opts.pitrBackup != "" {
			_, _ = fmt.Fprintf(stdout,
				"PITR rollback backup preserved at %s\n", opts.pitrBackup)
		}
		if !truncated {
			_, _ = fmt.Fprintf(stdout,
				"WARNING: AOF lacks #TS: timestamps — full file preserved (PITR ineffective). "+
					"Set 'aof-timestamp-enabled yes' on the source Valkey for next backup.\n")
		} else {
			_, _ = fmt.Fprintf(stdout,
				"PITR: truncated AOF to %s (%d bytes)\n", cutoff.UTC().Format(time.RFC3339), written)
		}
	}
	return nil
}
