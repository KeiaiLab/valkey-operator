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

Package cli — operator binary 의 sub-command 분기 (ADR-0023).
flag 표준 라이브러리만 사용. cobra 등 외부 의존성 없음.
*/

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Dispatch — args[0] 이 sub-command 이름. 나머지 args 가 sub-command 인자.
//
// 알려진 sub-command:
//   - upload   : S3 에 파일 업로드
//   - download : S3 에서 파일 다운로드
//
// 알 수 없는 sub-command 는 ErrUnknown 반환 — caller 가 controller manager
// 로 fall through.
func Dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return ErrUnknown
	}
	switch args[0] {
	case "upload":
		return runUpload(ctx, args[1:], stdout, stderr)
	case "download":
		return runDownload(ctx, args[1:], stdout, stderr)
	default:
		return ErrUnknown
	}
}

// ErrUnknown — args[0] 이 등록된 sub-command 가 아님. caller 가 다른
// dispatch 또는 controller manager 진입 결정.
var ErrUnknown = fmt.Errorf("unknown subcommand")

// IsKnownSubcommand — main.go 가 sub-command 진입 vs controller manager
// 진입 결정에 사용.
func IsKnownSubcommand(s string) bool {
	switch s {
	case "upload", "download":
		return true
	}
	return false
}

// envOr — 환경변수 미설정 시 fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBool — "true" / "1" → true. 그 외 false.
func envBool(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1"
}
