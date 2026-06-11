/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package v1alpha2 contains API Schema definitions for the cache v1alpha2
// API group — *type definition module* 단계 (PR-A2.1).
//
// 본 패키지는 Plan §2 D1/D2/D3 (사용자 결정 1: Auth + NetworkPolicy +
// PSS optional, default=true 유지) 의 *type 추가* 만 보유한다.
// controller / webhook / cmd/main.go 의 import 변경, conversion webhook
// 활성, Hub 결정은 *PR-A2.2* 에서 별도 PR.
//
// PR-A2.1 시점 anatomy:
//   - api/v1alpha2/*.go = v1alpha1 의 cp + 패키지명 + GroupVersion.Version 갱신
//   - common_types.go AuthSpec 에 Required *bool 신규 (default=true, ADR-0034)
//   - controller / webhook / main 미수정 — v1alpha1 가 여전히 hub
//   - conversion webhook 미설정 — kubectl apply cache.keiailab.io/v1alpha2 는
//     아직 controller 처리 안 됨. PR-A2.2 에서 활성.
//
// PR-A2.1 의 검증 기준:
//   - `go build ./api/v1alpha2/...` 통과
//   - `go vet ./api/v1alpha2/...` 통과
//   - `go test ./api/v1alpha2/...` 통과 (types_helpers_test.go 포함)
//
// 후속 PR-A2.2 에서 본 doc.go 는 갱신되어 Hub marker + conversion 안내.
//
// +kubebuilder:object:generate=true
// +groupName=cache.keiailab.io
package v1alpha2
