/*
Copyright 2026 Keiailab.
*/

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

// applyVolumeSnapshotForBackup — Type=VolumeSnapshot 시 snapshot.storage.k8s.io/v1
// VolumeSnapshot CR 생성. CRD 미설치 시 NoMatchError fail-soft 처리 — caller 가
// 그 경우 BackupPhase=Failed 로 전환 (cluster 가 본 backup type 미지원).
func applyVolumeSnapshotForBackup(ctx context.Context, c client.Client, b *cachev1alpha1.ValkeyBackup) error {
	desired := resources.BuildVolumeSnapshotForBackup(b, b.Spec.ClusterRef.Name)
	if desired == nil {
		return fmt.Errorf("VolumeSnapshot builder returned nil — Type=%q", b.Spec.Type)
	}
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(desired.GroupVersionKind())
	target.SetName(desired.GetName())
	target.SetNamespace(desired.GetNamespace())
	if err := c.Get(ctx, client.ObjectKeyFromObject(desired), target); err != nil {
		if apierrors.IsNotFound(err) {
			if createErr := c.Create(ctx, desired); createErr != nil {
				if meta.IsNoMatchError(createErr) {
					return fmt.Errorf("VolumeSnapshot CRD not installed: %w", createErr)
				}
				return fmt.Errorf("create VolumeSnapshot %s/%s: %w",
					desired.GetNamespace(), desired.GetName(), createErr)
			}
			return nil
		}
		if meta.IsNoMatchError(err) {
			return fmt.Errorf("VolumeSnapshot CRD not installed: %w", err)
		}
		return fmt.Errorf("get VolumeSnapshot: %w", err)
	}
	// 이미 존재 — VolumeSnapshot 은 immutable (snapshot 시점 고정). update 안 함.
	return nil
}

// pollVolumeSnapshotReady — VolumeSnapshot.status.readyToUse 조회.
//
// CRD 의 status 형식:
//
//	status:
//	  readyToUse: bool
//	  creationTime: time
//	  restoreSize: quantity
//	  error: { time, message }   # 실패 시
//
// 반환:
//
//	ready=true  → 사용 가능 (caller 가 Completed phase 전환)
//	ready=false + err=nil → 진행 중 (caller 가 RequeueAfter)
//	err != nil → fatal (caller 가 Failed phase 전환)
func pollVolumeSnapshotReady(ctx context.Context, c client.Client, b *cachev1alpha1.ValkeyBackup) (ready bool, err error) {
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(resources.VolumeSnapshotGVK)
	key := types.NamespacedName{
		Namespace: b.Namespace,
		Name:      resources.VolumeSnapshotName(b.Name),
	}
	if err := c.Get(ctx, key, target); err != nil {
		if apierrors.IsNotFound(err) {
			return false, fmt.Errorf("VolumeSnapshot %s not found", key.Name)
		}
		return false, fmt.Errorf("get VolumeSnapshot: %w", err)
	}
	status, found, err := unstructured.NestedMap(target.Object, "status")
	if err != nil || !found {
		return false, nil // status 미생성 — driver 가 아직 처리 시작 안 함.
	}
	// status.error 우선 검사 — driver 가 fail report 시 즉시 fatal.
	if errMap, _, _ := unstructured.NestedMap(status, "error"); len(errMap) > 0 {
		msg, _, _ := unstructured.NestedString(errMap, "message")
		if msg == "" {
			msg = "VolumeSnapshot driver reported error (no message)"
		}
		return false, fmt.Errorf("VolumeSnapshot error: %s", msg)
	}
	readyToUse, _, _ := unstructured.NestedBool(status, "readyToUse")
	return readyToUse, nil
}
