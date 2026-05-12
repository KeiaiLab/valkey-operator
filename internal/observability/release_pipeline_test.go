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

package observability

import (
	"os"
	"strings"
	"testing"
)

// TestReleaseTargetInjectsBuildMetadataAndAmd64Only — CLAUDE.md §2 정합:
// release 컨테이너 이미지는 default builder 의 linux/amd64-only 빌드. 멀티아키
// 빌드 금지 (org-wide 정책). 이전 검증명 ...AndMultiArch 는 deprecated — 본
// 테스트가 multi-arch 빌드 유지를 *방지* 한다.
func TestReleaseTargetInjectsBuildMetadataAndAmd64Only(t *testing.T) {
	candidates := []string{"Makefile", "../../Makefile", "../../../Makefile"}
	var path string
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		t.Fatalf("Makefile not found: %v", candidates)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(raw)
	for _, want := range []string{
		"--platform linux/amd64",
		"--build-arg VERSION=\"$(VERSION)\"",
		"--build-arg COMMIT=\"$$COMMIT_VAL\"",
		"--build-arg BUILD_DATE=\"$$DATE_VAL\"",
	} {
		if !strings.Contains(makefile, want) {
			t.Fatalf("release target 누락: %s", want)
		}
	}
	// CLAUDE.md §2 정합 가드: 멀티아키 platform 명시는 실수 — 차단.
	if strings.Contains(makefile, "linux/amd64,linux/arm64") {
		t.Fatalf("멀티아키 빌드 금지 (CLAUDE.md §2): linux/amd64,linux/arm64 가 Makefile 에 잔존")
	}
}
