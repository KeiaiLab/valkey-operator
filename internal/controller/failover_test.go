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

Failover helpers 단위 테스트 (ADR-0017).
*/

package controller

import "testing"

func TestSelectFailoverCandidate_emptyMap(t *testing.T) {
	_, ok := selectFailoverCandidate(map[int]int64{})
	if ok {
		t.Fatalf("expected ok=false for empty map")
	}
}

func TestSelectFailoverCandidate_singleReplica(t *testing.T) {
	got, ok := selectFailoverCandidate(map[int]int64{2: 100})
	if !ok || got != 2 {
		t.Fatalf("expected ordinal=2 ok=true, got %d %v", got, ok)
	}
}

func TestSelectFailoverCandidate_largestOffset(t *testing.T) {
	got, ok := selectFailoverCandidate(map[int]int64{
		1: 100,
		2: 500,
		3: 250,
	})
	if !ok || got != 2 {
		t.Fatalf("expected ordinal=2 (offset 500), got %d ok=%v", got, ok)
	}
}

func TestSelectFailoverCandidate_tieBreakLowerOrdinal(t *testing.T) {
	got, ok := selectFailoverCandidate(map[int]int64{
		3: 100,
		1: 100,
		2: 100,
	})
	// tie — 모두 100. ordinal 가장 작은 것 (1) 선출.
	if !ok || got != 1 {
		t.Fatalf("expected tie-break ordinal=1, got %d ok=%v", got, ok)
	}
}

func TestSelectFailoverCandidate_zeroOffsetsAllReplicas(t *testing.T) {
	// 모든 replica offset 0 (예: 모두 lag 만큼 못 받아옴) — ordinal 작은 것.
	got, ok := selectFailoverCandidate(map[int]int64{
		1: 0,
		2: 0,
		3: 0,
	})
	if !ok || got != 1 {
		t.Fatalf("expected ordinal=1 (zero-tie), got %d ok=%v", got, ok)
	}
}

func TestSelectFailoverCandidate_excludingPrimary(t *testing.T) {
	// caller (reconcileFailover) 가 *primary 제외* 한 replicas 만 전달했다고 가정.
	// 본 함수 자체는 caller 책임. 검증: 주어진 키 들 만 비교.
	got, ok := selectFailoverCandidate(map[int]int64{
		1: 1000,
		2: 2000,
		// 3 (primary) 는 caller 가 제외함.
	})
	if !ok || got != 2 {
		t.Fatalf("expected ordinal=2, got %d", got)
	}
}
