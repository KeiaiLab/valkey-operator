/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Hub marker — sigs.k8s.io/controller-runtime/pkg/conversion 의 Hub
// interface 구현. v1alpha2 가 *conversion hub* 임을 명시.
//
// PR-A2.2.1 (본 파일): Hub marker 5 type 만 추가. 실제 conversion
// webhook 활성 + v1alpha1 ↔ v1alpha2 ConvertTo/ConvertFrom 구현은
// 후속:
//   - PR-A2.2.2: api/v1alpha1/conversion.go 신규 (ConvertTo/ConvertFrom
//     5 type 본문).
//   - PR-A2.2.3: cmd/main.go SchemeBuilder + conversion webhook 활성 +
//     CRD `spec.conversion.strategy=Webhook` + cert-manager Certificate.
//   - PR-A2.2.4: internal/controller/* 의 v1alpha1 → v1alpha2 import
//     변경 + ensureAuthSecret Required 분기.
//   - PR-A2.2.5: controller-gen 재실행 (zz_generated.deepcopy.go 의
//     Required *bool / RotationPolicy / Modules / AutoCreate / PSS 갱신).
//
// ADR-0026 (Conversion Webhook deferred until v1alpha1 stable) 의
// *부분 회복* 시작 — Plan §2 D1 사용자 결정 4 (v1alpha2 + conversion
// webhook) 정합.

package v1alpha2

// Hub marks Valkey CRD as the conversion hub for the cache.keiailab.io
// API group.
func (*Valkey) Hub() {}

// Hub marks ValkeyCluster CRD as the conversion hub.
func (*ValkeyCluster) Hub() {}

// Hub marks ValkeyBackup CRD as the conversion hub.
func (*ValkeyBackup) Hub() {}

// Hub marks ValkeyBackupTarget CRD as the conversion hub.
func (*ValkeyBackupTarget) Hub() {}

// Hub marks ValkeyRestore CRD as the conversion hub.
func (*ValkeyRestore) Hub() {}
