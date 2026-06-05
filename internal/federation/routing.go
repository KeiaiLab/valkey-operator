/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package federation — multi-cluster Valkey federation 의 순수 라우팅 결정 로직
// (ROADMAP 2.x). ValkeyFederation CRD/controller 가 이 결정을 사용해 primary
// member 선택 + topology-aware 라우팅을 수행한다. 인프라(kubeconfig/cross-cluster)
// 와 분리된 순수 함수로 유지해 단위 검증을 보장한다.
package federation

// Member — federation 구성원(다른 클러스터 또는 in-cluster 의 Valkey 인스턴스).
type Member struct {
	Name   string // federation 내 고유 식별자
	Weight int32  // 우선순위 — 높을수록 primary 선호
	Region string // topology-aware 라우팅 기준
}

// SelectPrimary — healthy member 중 최고 Weight 를 primary 로 선택한다.
// 동률이면 이름 사전순(결정론적 — 동일 입력 → 동일 출력, split-brain 회피).
// healthy 한 member 가 없으면 ok=false.
func SelectPrimary(members []Member, healthy map[string]bool) (primary string, ok bool) {
	var bestWeight int32 = -1
	for _, m := range members {
		if !healthy[m.Name] {
			continue
		}
		if m.Weight > bestWeight || (m.Weight == bestWeight && (primary == "" || m.Name < primary)) {
			primary, bestWeight = m.Name, m.Weight
		}
	}
	return primary, primary != ""
}
