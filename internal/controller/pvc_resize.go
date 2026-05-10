/*
Copyright 2026 Keiailab.
*/

package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// dataPVCNamePrefix — STS VCT name "data" + STS name = `data-<crName>-`.
// k8s STS controller 가 각 replica 별로 `<vct-name>-<sts-name>-<ordinal>` 명명.
func dataPVCNamePrefix(crName string) string {
	return "data-" + crName + "-"
}

// expandDataPVCs — Spec.Storage.Size 가 증가했을 때 기존 PVC 의 storage request 를
// patch 해 K8s online volume expansion 을 트리거한다.
//
// StatefulSet.spec.volumeClaimTemplates 자체는 immutable 이므로 (k8s 1.31+ alpha 외),
// 표준 패턴: 라벨로 PVC 를 list → 각 PVC 의 spec.resources.requests.storage 를 직접
// patch. 새 replica 는 *desired STS VCT* 가 이미 새 size 로 렌더되므로 자동 반영.
//
// 사전 조건 (위반 시 noop + warn):
//   - StorageClass 의 AllowVolumeExpansion == true
//   - PVC 가 Bound 상태 (Pending / Lost 시 patch 무의미)
//   - desiredSize > currentSize (감소는 webhook 에서 reject)
//
// CSI driver 가 online resize 미지원 시 PVC.status.conditions 가 FileSystemResizePending
// 으로 남으며 다음 pod restart 시 완료. 본 함수는 *patch 만* 하고 완료 폴링은 하지 않음.
func expandDataPVCs(
	ctx context.Context,
	c client.Client,
	namespace, crName string,
	desiredSize resource.Quantity,
) error {
	logger := log.FromContext(ctx).WithName("pvc-resize").WithValues(
		"namespace", namespace, "cr", crName,
	)

	if desiredSize.IsZero() {
		return nil
	}

	// STS VCT 가 label 미상속하므로 *PVC name prefix* 로 필터.
	// k8s STS controller 가 `data-<crName>-<ordinal>` 형태로 PVC 명명.
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := c.List(ctx, pvcList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("list PVCs in %s: %w", namespace, err)
	}

	prefix := dataPVCNamePrefix(crName)
	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]
		if !strings.HasPrefix(pvc.Name, prefix) {
			continue
		}
		if err := expandSinglePVC(ctx, c, pvc, desiredSize); err != nil {
			// 한 PVC 실패가 다른 PVC patch 를 차단하지 않음 (best-effort).
			logger.Error(err, "PVC expansion failed", "pvc", pvc.Name)
		}
	}
	return nil
}

func expandSinglePVC(
	ctx context.Context,
	c client.Client,
	pvc *corev1.PersistentVolumeClaim,
	desiredSize resource.Quantity,
) error {
	logger := log.FromContext(ctx).WithName("pvc-resize")
	if pvc.Status.Phase != corev1.ClaimBound {
		logger.V(1).Info("skip non-Bound PVC", "pvc", pvc.Name, "phase", pvc.Status.Phase)
		return nil
	}

	currentSize, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if !ok {
		return fmt.Errorf("PVC %s missing spec.resources.requests.storage", pvc.Name)
	}
	if desiredSize.Cmp(currentSize) <= 0 {
		return nil // 이미 desired 이상.
	}

	// StorageClass 의 AllowVolumeExpansion 검증.
	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		sc := &storagev1.StorageClass{}
		if err := c.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, sc); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("skip PVC: StorageClass not found",
					"pvc", pvc.Name, "storageClass", *pvc.Spec.StorageClassName)
				return nil
			}
			return fmt.Errorf("get StorageClass %s: %w", *pvc.Spec.StorageClassName, err)
		}
		if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
			logger.Info("skip PVC: StorageClass does not allow volume expansion",
				"pvc", pvc.Name, "storageClass", sc.Name)
			return nil
		}
	}

	patched := pvc.DeepCopy()
	if patched.Spec.Resources.Requests == nil {
		patched.Spec.Resources.Requests = corev1.ResourceList{}
	}
	patched.Spec.Resources.Requests[corev1.ResourceStorage] = desiredSize
	if err := c.Patch(ctx, patched, client.MergeFrom(pvc)); err != nil {
		return fmt.Errorf("patch PVC %s: %w", pvc.Name, err)
	}
	logger.Info("PVC expansion patched",
		"pvc", pvc.Name, "from", currentSize.String(), "to", desiredSize.String())
	return nil
}
