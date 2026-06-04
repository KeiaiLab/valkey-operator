/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

// ensureMultiPodVolumeSnapshotSources — Replication mode (replicas N>1) target
// 의 VolumeSnapshot restore. N 개 cloned PVC 생성 후 모두 Bound 도달까지 polling.
//
// PR #63 의 multi-pod reject 를 *완화* 한 path. Standalone 의
// ensureVolumeSnapshotSource 와 동일 패턴으로 ordinal 별 PVC 생성.
//
// Naming: data-{rest.name}-restored-{ordinal} (resources.MultiPodRestoredPVCName).
//
// Phase 1.5 (현재): N 개 PVC 생성 + Bound 대기. 실제 STS data PVC swap (data-{cr}-{ord}
// 를 cloned PVC 로 교체) 는 phase 2 별도 epic — 사용자가 Status.Message 의 안내에
// 따라 scale-down + swap 수동 수행.
//
// 시그니처 parity 위해 unparam nolint (다른 ensure* helper 와 동일 (ctrl.Result,
// bool, error)).
//
//nolint:unparam // signature parity with ensurePVCSource/ensureTargetRefSource/ensureVolumeSnapshotSource.
func (r *ValkeyRestoreReconciler) ensureMultiPodVolumeSnapshotSources(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore, replicaCount int32,
) (ctrl.Result, bool, error) {
	if rest.Spec.Source.VolumeSnapshot == nil || rest.Spec.Source.VolumeSnapshot.Name == "" {
		return ctrl.Result{}, false, nil
	}
	if replicaCount < 2 {
		// 도달 안 함 (caller 가 multi-pod 일 때만 호출). 방어적 가드.
		return ctrl.Result{}, false, nil
	}

	allBound := true
	for o := range int(replicaCount) {
		ord := int32(o)
		pvcName := resources.MultiPodRestoredPVCName(rest.Name, ord)
		existing := &corev1.PersistentVolumeClaim{}
		err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: rest.Namespace}, existing)

		if apierrors.IsNotFound(err) {
			size := resource.MustParse(cachev1alpha1.DefaultStorageSize)
			desired := resources.BuildPVCFromVolumeSnapshotForOrdinal(
				rest.Name, rest.Namespace,
				rest.Spec.Source.VolumeSnapshot.Name,
				ord,
				nil,
				size,
			)
			if err := controllerutil.SetControllerReference(rest, desired, r.Scheme); err != nil {
				return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
			}
			if err := r.Create(ctx, desired); err != nil {
				return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
			}
			allBound = false
			continue
		}
		if err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
		}
		if existing.Status.Phase != corev1.ClaimBound {
			allBound = false
		}
	}

	if !allBound {
		return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
	}

	// 모든 N PVC 가 Bound — Status.Message 로 phase 2 수동 절차 안내.
	// reconciler 는 *PVC 준비까지만* — STS swap 은 사용자 또는 phase 2 epic.
	rest.Status.Message = fmt.Sprintf(
		"Phase 1.5: %d cloned PVCs ready (data-%s-restored-{0..%d}). "+
			"Phase 2 (별도 epic): operator 가 Spec.ClusterRef target 의 STS data PVC "+
			"를 본 cloned PVCs 로 자동 swap. 임시 수동 절차: "+
			"(1) kubectl scale sts %s --replicas=0  "+
			"(2) per-ordinal: kubectl delete pvc data-%s-{ord} && rename cloned PVC  "+
			"(3) kubectl scale sts %s --replicas=%d",
		replicaCount, rest.Name, replicaCount-1,
		rest.Spec.ClusterRef.Name,
		rest.Spec.ClusterRef.Name,
		rest.Spec.ClusterRef.Name, replicaCount,
	)
	return ctrl.Result{}, true, nil
}
