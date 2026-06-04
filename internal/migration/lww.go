/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package migration — online schema-less migration 의 순수 충돌 해소 로직 (ROADMAP 2.x).
// RDB diff 를 source→target 으로 merge 할 때 동일 key 충돌을 LWW(Last-Write-Wins)로
// 결정론적 해소한다. controller/RDB diff 도구가 이 결정을 사용.
package migration

// VersionedValue — 한 key 의 값 + 충돌 해소 메타데이터.
type VersionedValue struct {
	Value     string // key 의 값(RDB 직렬화된 payload 또는 참조)
	Timestamp int64  // 최종 쓰기 시각(unix nanos) — LWW 비교 1순위
	Source    string // 출처 식별자 — timestamp 동률 시 결정론적 tie-break
}

// ResolveLWW — 두 후보 중 승자를 결정론적으로 고른다(Last-Write-Wins).
//
//	1순위: Timestamp 큰 쪽(최신 쓰기)
//	2순위(동률): Source 사전순 작은 쪽
//	3순위(동률): Value 사전순 작은 쪽
//
// 3-단계 tie-break 로 교환법칙(ResolveLWW(a,b)==ResolveLWW(b,a))을 보장 —
// 모든 노드가 동일 입력에 동일 승자로 수렴(split-brain 회피).
func ResolveLWW(a, b VersionedValue) VersionedValue {
	if a.Timestamp != b.Timestamp {
		if a.Timestamp > b.Timestamp {
			return a
		}
		return b
	}
	if a.Source != b.Source {
		if a.Source < b.Source {
			return a
		}
		return b
	}
	if a.Value <= b.Value {
		return a
	}
	return b
}
