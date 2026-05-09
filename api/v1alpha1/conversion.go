/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// ConvertTo / ConvertFrom — v1alpha1 (Spoke) ↔ v1alpha2 (Hub) 양방향 변환.
//
// Plan §2 D1 + 사용자 결정 4 (v1alpha2 + conversion webhook). PR-A2.2.2
// (본 파일): 5 type × 2 함수 = 10 함수, JSON byte-copy 공통 helper 패턴.
//
// 매핑 패턴 (JSON byte-copy):
//   - 동일 JSON tag + 동일 구조 필드: 자동 매핑 (encoding/json 가 처리).
//   - v1alpha2 신규 필드 (Required *bool, RotationPolicy, AutoCreate *bool,
//     PodSecurityRestricted *bool, Modules): v1alpha1 부재 → JSON unmarshal
//     시 nil/zero — controller 의 kubebuilder default 적용 (Required nil →
//     true, RotationPolicy "" → "Manual", etc.).
//   - v1alpha1 의 deprecated 예정 필드 (AuthSpec.Enabled): v1alpha2 도
//     동등 필드 보유 (호환 보존).
//
// 후속 (PR-A2.2.2.refactor): JSON byte-copy → *명시 매핑 함수* 로 refactor.
// production conversion 의 표준 패턴 + 명시적 type assertion.

package v1alpha1

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/keiailab/valkey-operator/api/v1alpha2"
)

// convertViaJSON — 5 type 의 ConvertTo / ConvertFrom 공통 helper.
//
// JSON marshal/unmarshal 으로 동일 JSON tag 필드 자동 매핑. 신규 v1alpha2
// 필드는 nil/zero — controller default 적용.
func convertViaJSON(src, dst any, typeName string) error {
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("conversion %s marshal: %w", typeName, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("conversion %s unmarshal: %w", typeName, err)
	}
	return nil
}

// ConvertTo converts this Valkey (v1alpha1) to the Hub version (v1alpha2).
func (src *Valkey) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.Valkey)
	return convertViaJSON(src, dst, "Valkey")
}

// ConvertFrom converts the Hub version (v1alpha2) to this Valkey (v1alpha1).
func (dst *Valkey) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.Valkey)
	return convertViaJSON(src, dst, "Valkey")
}

// ConvertTo / ConvertFrom — ValkeyCluster.
func (src *ValkeyCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.ValkeyCluster)
	return convertViaJSON(src, dst, "ValkeyCluster")
}

func (dst *ValkeyCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.ValkeyCluster)
	return convertViaJSON(src, dst, "ValkeyCluster")
}

// ConvertTo / ConvertFrom — ValkeyBackup.
func (src *ValkeyBackup) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.ValkeyBackup)
	return convertViaJSON(src, dst, "ValkeyBackup")
}

func (dst *ValkeyBackup) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.ValkeyBackup)
	return convertViaJSON(src, dst, "ValkeyBackup")
}

// ConvertTo / ConvertFrom — ValkeyBackupTarget.
func (src *ValkeyBackupTarget) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.ValkeyBackupTarget)
	return convertViaJSON(src, dst, "ValkeyBackupTarget")
}

func (dst *ValkeyBackupTarget) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.ValkeyBackupTarget)
	return convertViaJSON(src, dst, "ValkeyBackupTarget")
}

// ConvertTo / ConvertFrom — ValkeyRestore.
func (src *ValkeyRestore) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha2.ValkeyRestore)
	return convertViaJSON(src, dst, "ValkeyRestore")
}

func (dst *ValkeyRestore) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha2.ValkeyRestore)
	return convertViaJSON(src, dst, "ValkeyRestore")
}
