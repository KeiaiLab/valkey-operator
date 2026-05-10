/*
Copyright 2026 Keiailab.
*/

package controller

import (
	"context"
	"fmt"
	"strings"

	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// auditEncryptionAtRest — Spec.Storage.EncryptionRequired=true 일 때 StorageClass
// 의 parameters 를 검사해 *encryption 표시자* 가 없으면 Warning 반환.
//
// compliance audit — 강제 reject 가 아닌 *발견 사항* 만. 운영자가 의도적 평문 SC
// 사용 시 차단되지 않도록.
//
// 표시자 (provider 별):
//   - AWS EBS: "encrypted" = "true"
//   - Azure Disk: "kind" = "Managed" + "skuName" = "Premium_LRS" (SSE-D 자동) 또는
//     "diskEncryptionSetID" 명시
//   - GCE PD: "replication-type" 외에 "encryption-key" or "kms-key-name"
//   - Ceph RBD: "encrypted" = "true" (CSI driver 패턴)
//   - rook-ceph: "encrypted" = "true"
//
// 미발견 시 caller 가 Warning event 발행.
func auditEncryptionAtRest(ctx context.Context, c client.Client, scName string) (encrypted bool, hint string, err error) {
	logger := log.FromContext(ctx).WithName("encryption-audit")
	if scName == "" {
		// default StorageClass — provider 별 다양 → 검사 보류.
		return false, "no storageClassName specified — using cluster default (unable to audit)", nil
	}
	sc := &storagev1.StorageClass{}
	if err := c.Get(ctx, types.NamespacedName{Name: scName}, sc); err != nil {
		if apierrors.IsNotFound(err) {
			return false, fmt.Sprintf("StorageClass %q not found", scName), nil
		}
		return false, "", fmt.Errorf("get StorageClass %s: %w", scName, err)
	}

	enc, hint := isLikelyEncrypted(sc)
	if !enc {
		logger.Info("StorageClass does not appear to advertise encryption-at-rest",
			"storageClass", scName, "provisioner", sc.Provisioner, "hint", hint)
	}
	return enc, hint, nil
}

// encryptedTrue — StorageClass.Parameters[encrypted] 의 표준 값 "true". 다수 provider
// (AWS EBS / Ceph / rook-ceph) 가 동일 키 사용 — goconst 회피용 const.
const encryptedTrue = "true"

func isLikelyEncrypted(sc *storagev1.StorageClass) (bool, string) {
	p := sc.Parameters
	if p == nil {
		return false, "StorageClass has no parameters"
	}
	// 가장 일반적인 패턴: parameters.encrypted=true (AWS, Ceph, rook-ceph)
	if v, ok := p["encrypted"]; ok && strings.EqualFold(v, encryptedTrue) {
		return true, "parameters.encrypted=true"
	}
	// AWS gp3 + at-rest encryption.
	if strings.Contains(sc.Provisioner, "ebs.csi.aws.com") {
		if v, ok := p["encrypted"]; ok && strings.EqualFold(v, encryptedTrue) {
			return true, "AWS EBS encrypted=true"
		}
		return false, "AWS EBS without parameters.encrypted=true"
	}
	// Azure Disk: Premium_LRS 자동 암호화 (SSE-D platform-managed).
	if strings.Contains(sc.Provisioner, "disk.csi.azure.com") {
		if v, ok := p["skuName"]; ok && strings.HasPrefix(strings.ToLower(v), "premium_") {
			return true, "Azure Disk Premium SKU (SSE-D platform-managed)"
		}
		if _, ok := p["diskEncryptionSetID"]; ok {
			return true, "Azure Disk with diskEncryptionSetID"
		}
		return false, "Azure Disk without Premium SKU or diskEncryptionSetID"
	}
	// GCE Persistent Disk: kms-key-name 명시 시.
	if strings.Contains(sc.Provisioner, "pd.csi.storage.gke.io") {
		if _, ok := p["disk-encryption-kms-key"]; ok {
			return true, "GCE PD with disk-encryption-kms-key"
		}
		return false, "GCE PD without disk-encryption-kms-key (default Google-managed key may apply)"
	}
	return false, fmt.Sprintf("unknown provisioner %q — manual verification required", sc.Provisioner)
}
