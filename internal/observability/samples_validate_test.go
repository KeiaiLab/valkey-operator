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

// config/samples/*.yaml 가 현재 api/v1alpha1 의 Go 타입에 strict unmarshal 가능한지 검증.
//
// 사고 패턴: API 의 spec 필드 rename 후 sample 갱신 누락 → 사용자가 README
// 따라 `kubectl apply -f config/samples/...` 실행 시 admission 에서 unknown
// field 거절 또는 silent ignore (kubectl 의 client-side validation 우회 시).
// 본 테스트는 *strict* unmarshal 로 unknown field 즉시 검출.
//
// 추가 검증: sample 의 apiVersion/kind 가 실제 GVK 와 일치 + name 비어있지 않음.

package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// 본 테스트의 SSOT: 알려진 sample 파일과 대응하는 Go 타입 factory.
// 신규 CRD 추가 시 본 매핑 + sample 파일 동시 추가.
var sampleTypeMapping = map[string]func() any{
	"cache_v1alpha1_valkey.yaml":             func() any { return &cachev1alpha1.Valkey{} },
	"cache_v1alpha1_valkeycluster.yaml":      func() any { return &cachev1alpha1.ValkeyCluster{} },
	"cache_v1alpha1_valkeybackup.yaml":       func() any { return &cachev1alpha1.ValkeyBackup{} },
	"cache_v1alpha1_valkeybackuptarget.yaml": func() any { return &cachev1alpha1.ValkeyBackupTarget{} },
	"cache_v1alpha1_valkeyrestore.yaml":      func() any { return &cachev1alpha1.ValkeyRestore{} },
}

func samplesDir(t *testing.T) string {
	t.Helper()
	candidates := []string{"config/samples", "../../config/samples", "../../../config/samples"}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	t.Fatalf("config/samples not found: %v", candidates)
	return ""
}

func TestSamplesStrictUnmarshal(t *testing.T) {
	dir := samplesDir(t)
	for fname, factory := range sampleTypeMapping {
		t.Run(fname, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, fname)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			obj := factory()
			// UnmarshalStrict: unknown 필드 거절.
			if err := yaml.UnmarshalStrict(raw, obj); err != nil {
				t.Errorf("strict unmarshal 실패 (api 타입과 sample drift) — %v", err)
			}
		})
	}
}

func TestSamplesDirHasAllExpected(t *testing.T) {
	dir := samplesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	got := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") && e.Name() != "kustomization.yaml" {
			got[e.Name()] = true
		}
	}
	for name := range sampleTypeMapping {
		if !got[name] {
			t.Errorf("expected sample %q 누락 (config/samples/ 에 없음)", name)
		}
	}
	for name := range got {
		if _, ok := sampleTypeMapping[name]; !ok {
			t.Errorf("orphan sample %q — sampleTypeMapping 에 등록되지 않음 (신규 CRD 추가 시 본 테스트에 매핑 추가 필요)", name)
		}
	}
}

// kind / apiVersion 일치 + metadata.name 비어있지 않음 검증.
type minimalK8sMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

func TestSamplesMetadataValid(t *testing.T) {
	dir := samplesDir(t)
	expected := map[string]struct{ kind string }{
		"cache_v1alpha1_valkey.yaml":             {kind: "Valkey"},
		"cache_v1alpha1_valkeycluster.yaml":      {kind: "ValkeyCluster"},
		"cache_v1alpha1_valkeybackup.yaml":       {kind: "ValkeyBackup"},
		"cache_v1alpha1_valkeybackuptarget.yaml": {kind: "ValkeyBackupTarget"},
		"cache_v1alpha1_valkeyrestore.yaml":      {kind: "ValkeyRestore"},
	}
	for fname, want := range expected {
		t.Run(fname, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join(dir, fname))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var m minimalK8sMeta
			if err := yaml.Unmarshal(raw, &m); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if m.APIVersion != "cache.keiailab.io/v1alpha1" {
				t.Errorf("apiVersion=%q (want cache.keiailab.io/v1alpha1)", m.APIVersion)
			}
			if m.Kind != want.kind {
				t.Errorf("kind=%q (want %q)", m.Kind, want.kind)
			}
			if m.Metadata.Name == "" {
				t.Error("metadata.name 비어 있음")
			}
		})
	}
}
