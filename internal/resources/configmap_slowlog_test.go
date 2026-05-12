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
*/

package resources

import (
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestMergeSlowLog_nil_returns_extra_unchanged(t *testing.T) {
	extra := map[string]string{"maxmemory": "1gb"}
	out := mergeSlowLog(extra, nil)
	if len(out) != 1 || out["maxmemory"] != "1gb" {
		t.Errorf("unexpected: %v", out)
	}
}

func TestMergeSlowLog_threshold_only(t *testing.T) {
	out := mergeSlowLog(nil, &cachev1alpha1.SlowLogSpec{ThresholdMicros: 5000})
	if out["slowlog-log-slower-than"] != "5000" {
		t.Errorf("threshold: %q", out["slowlog-log-slower-than"])
	}
	if _, ok := out["slowlog-max-len"]; ok {
		t.Errorf("max-len should not be set: %v", out)
	}
}

func TestMergeSlowLog_max_entries_only(t *testing.T) {
	out := mergeSlowLog(nil, &cachev1alpha1.SlowLogSpec{MaxEntries: 256})
	if out["slowlog-max-len"] != "256" {
		t.Errorf("max-len: %q", out["slowlog-max-len"])
	}
}

func TestMergeSlowLog_user_extra_overrides(t *testing.T) {
	// 사용자가 AdditionalConfig 로 직접 명시 시 우선.
	out := mergeSlowLog(
		map[string]string{"slowlog-log-slower-than": "100000"},
		&cachev1alpha1.SlowLogSpec{ThresholdMicros: 5000},
	)
	if out["slowlog-log-slower-than"] != "100000" {
		t.Errorf("user override: %q", out["slowlog-log-slower-than"])
	}
}

func TestMergeSlowLog_zero_values_skipped(t *testing.T) {
	// Threshold=0 (비활성) + MaxEntries=0 → 명시적 directive 미생성.
	out := mergeSlowLog(map[string]string{"a": "b"}, &cachev1alpha1.SlowLogSpec{})
	if _, ok := out["slowlog-log-slower-than"]; ok {
		t.Errorf("zero threshold should not set directive: %v", out)
	}
	if out["a"] != "b" {
		t.Errorf("extra preserved: %v", out)
	}
}
