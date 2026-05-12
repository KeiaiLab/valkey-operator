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
*/

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

// ensureVolumeSnapshotSource — Source.VolumeSnapshot: cloned PVC 보장.
//
// PVC.spec.dataSource = VolumeSnapshot 으로 새 PVC 생성. CSI driver 가 binding
// 시 snapshot 데이터를 새 volume 으로 복사. PVC.status.phase=Bound 도달 시
// 다음 phase 진입 가능.
//
// 반환: ok=true → caller 진행, ok=false → caller 가 result/err 전파.
//
// Phase 1: Standalone target 만 의도 (cloned PVC 가 RWO). Replication / Cluster
// 는 phase 2 (multi-PVC 동시 clone + STS dataSource 일괄 swap 필요).
//
// 시그니처는 ensurePVCSource / ensureTargetRefSource 와 정합 — caller 가 동일
// switch 분기에서 사용. fatal err 는 phase=Failed 로 전파해야 하므로 시그니처
// 에 *유지* (현재는 reconcile 차단 안 하고 전부 RequeueAfter 처리).
//
//nolint:unparam // signature parity with ensurePVCSource/ensureTargetRefSource.
func (r *ValkeyRestoreReconciler) ensureVolumeSnapshotSource(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, bool, error) {
	if rest.Spec.Source.VolumeSnapshot == nil || rest.Spec.Source.VolumeSnapshot.Name == "" {
		return ctrl.Result{}, false, nil // handlePending 의 사전 검증으로 도달 안 함.
	}

	pvcName := resources.RestoredPVCName(rest.Name)
	existing := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: rest.Namespace}, existing)

	if apierrors.IsNotFound(err) {
		size := resource.MustParse(cachev1alpha1.DefaultStorageSize)
		desired := resources.BuildPVCFromVolumeSnapshot(
			rest.Name, rest.Namespace,
			rest.Spec.Source.VolumeSnapshot.Name,
			nil,
			size,
		)
		if err := controllerutil.SetControllerReference(rest, desired, r.Scheme); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
		}
		if err := r.Create(ctx, desired); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
		}
		// 새로 생성 — 다음 reconcile 에서 Bound 검사.
		return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
	}
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
	}

	if existing.Status.Phase != corev1.ClaimBound {
		// CSI driver 가 snapshot 복사 진행 중.
		return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
	}
	return ctrl.Result{}, true, nil
}
