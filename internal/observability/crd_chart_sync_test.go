/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// config/crd/bases/ ↔ charts/valkey-operator/crds/ 의 CRD 파일 byte-level 동기 검증.
//
// 사고 패턴 (실제 발견): config/crd/bases/cache.keiailab.io_valkeys.yaml 에
// autoFailover 필드가 추가되었지만 charts/valkey-operator/crds/ 의 사본이
// 갱신되지 않음 → kustomize 로 deploy 한 사용자는 autoFailover 사용 가능,
// Helm chart 으로 설치한 사용자는 *해당 필드가 admission 단계에서 거절* —
// 동일 operator 가 *deploy 방법에 따라 다른 기능* 을 제공하는 silent failure.
//
// 본 테스트가 byte-level 동기를 강제 — `make manifests` 후 chart 측 사본도
// 함께 갱신해야 머지 가능.

package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCRDBaseChartSync(t *testing.T) {
	candidates := []string{".", "..", "../.."}
	var root string
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "go.mod")); err == nil {
			root, _ = filepath.Abs(c)
			break
		}
	}
	if root == "" {
		t.Fatal("repo root not found")
	}

	baseDir := filepath.Join(root, "config/crd/bases")
	chartDir := filepath.Join(root, "charts/valkey-operator/crds")

	if _, err := os.Stat(baseDir); err != nil {
		t.Skipf("config/crd/bases 없음 — skip (%v)", err)
	}
	if _, err := os.Stat(chartDir); err != nil {
		t.Skipf("charts/valkey-operator/crds 없음 — skip (%v)", err)
	}

	baseEntries, err := os.ReadDir(baseDir)
	if err != nil {
		t.Fatalf("readdir base: %v", err)
	}
	chartEntries, err := os.ReadDir(chartDir)
	if err != nil {
		t.Fatalf("readdir chart: %v", err)
	}

	baseFiles := map[string]bool{}
	chartFiles := map[string]bool{}
	for _, e := range baseEntries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			baseFiles[e.Name()] = true
		}
	}
	for _, e := range chartEntries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			chartFiles[e.Name()] = true
		}
	}

	// 양방향 파일 존재 검증.
	for name := range baseFiles {
		if !chartFiles[name] {
			t.Errorf("config/crd/bases/%s 가 charts/valkey-operator/crds/ 에 없음 — chart 측 사본 누락", name)
		}
	}
	for name := range chartFiles {
		if !baseFiles[name] {
			t.Errorf("charts/valkey-operator/crds/%s 가 config/crd/bases/ 에 없음 — orphan 사본 (controller-gen 출력 외)", name)
		}
	}

	// 양쪽 다 있는 파일은 byte-level 동일성 검증.
	for name := range baseFiles {
		if !chartFiles[name] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			baseSum, err := fileSha256(filepath.Join(baseDir, name))
			if err != nil {
				t.Fatalf("hash base: %v", err)
			}
			chartSum, err := fileSha256(filepath.Join(chartDir, name))
			if err != nil {
				t.Fatalf("hash chart: %v", err)
			}
			if baseSum != chartSum {
				t.Errorf("CRD drift: %s\n  config/crd/bases sha256: %s\n  charts/valkey-operator/crds sha256: %s\n  → `cp config/crd/bases/%s charts/valkey-operator/crds/%s` 후 재커밋",
					name, baseSum, chartSum, name, name)
			}
		})
	}
}

func fileSha256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), nil
}
