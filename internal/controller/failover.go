/*
Copyright 2026 Keiailab.

Replication mode failover helpers (ADR-0017).
*/

package controller

import "sort"

// selectFailoverCandidate — replica ordinal → master_repl_offset/slave_repl_offset
// 맵에서 *가장 큰 offset* replica ordinal 선출. tie 시 ordinal 작은 것.
//
// ADR-0017: 가장 latest replica 가 primary 의 마지막 commit 시점에 가장
// 가까움 → 데이터 손실 최소화.
//
// 빈 맵 → ok=false. 모든 offset 0 일 시 ordinal 가장 작은 replica 반환.
func selectFailoverCandidate(offsets map[int]int64) (ordinal int, ok bool) {
	if len(offsets) == 0 {
		return 0, false
	}
	keys := make([]int, 0, len(offsets))
	for k := range offsets {
		keys = append(keys, k)
	}
	sort.Ints(keys) // tie-break: ordinal 작은 것 우선.

	bestIdx := keys[0]
	bestOffset := offsets[bestIdx]
	for _, k := range keys[1:] {
		if offsets[k] > bestOffset {
			bestOffset = offsets[k]
			bestIdx = k
		}
	}
	return bestIdx, true
}
