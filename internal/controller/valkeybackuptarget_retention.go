/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/backuplifecycle"
)

const secondsPerDay int64 = 86400

// selectExpiredBackupsForTarget — backups 중 targetName 을 참조하는 *완료된* backup 만
// 골라 retention 정책(internal/backuplifecycle.SelectExpired)으로 만료 대상 이름을
// 반환한다. 순수 함수 (client 무관) — TDD 단위 검증 가능.
//
// 제외: 다른 target 참조 / PVC 대상(TargetRef nil) / 미완료(Phase != Completed) /
// CompletedAt 미설정. retention nil 또는 정책 0 이면 빈 결과.
func selectExpiredBackupsForTarget(
	backups []cachev1alpha1.ValkeyBackup,
	targetName string,
	retention *cachev1alpha1.RetentionSpec,
	now int64,
) []string {
	if retention == nil {
		return nil
	}
	var infos []backuplifecycle.BackupInfo
	for i := range backups {
		b := &backups[i]
		dst := b.Spec.Destination
		if dst == nil || dst.Type != cachev1alpha1.BackupDestTargetRef || dst.TargetRef == nil || dst.TargetRef.Name != targetName {
			continue
		}
		if b.Status.Phase != cachev1alpha1.BackupPhaseCompleted || b.Status.CompletedAt == nil {
			continue
		}
		infos = append(infos, backuplifecycle.BackupInfo{
			Name:      b.Name,
			CreatedAt: b.Status.CompletedAt.Unix(),
		})
	}
	maxAgeSec := int64(retention.MaxAgeDays) * secondsPerDay
	return backuplifecycle.SelectExpired(infos, retention.MaxCount, maxAgeSec, now)
}

// applyRetention — target 의 retention 정책을 적용한다. 같은 namespace 의 ValkeyBackup
// 을 List → 만료 대상 선정 → delete. 만료된 backup 개수를 반환.
//
// backup 삭제는 ValkeyBackup finalizer 가 외부 저장 object 정리를 처리한다(ADR-0016).
func (r *ValkeyBackupTargetReconciler) applyRetention(
	ctx context.Context,
	target *cachev1alpha1.ValkeyBackupTarget,
	now int64,
) (int, error) {
	if target.Spec.Retention == nil {
		return 0, nil
	}
	log := logf.FromContext(ctx)

	var backupList cachev1alpha1.ValkeyBackupList
	if err := r.List(ctx, &backupList, client.InNamespace(target.Namespace)); err != nil {
		return 0, fmt.Errorf("list backups for retention: %w", err)
	}

	expired := selectExpiredBackupsForTarget(backupList.Items, target.Name, target.Spec.Retention, now)
	deleted := 0
	for _, name := range expired {
		b := &cachev1alpha1.ValkeyBackup{}
		b.Name = name
		b.Namespace = target.Namespace
		if err := r.Delete(ctx, b); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return deleted, fmt.Errorf("delete expired backup %q: %w", name, err)
		}
		deleted++
		log.Info("retention 만료 backup 삭제", "backup", name, "target", target.Name)
	}
	return deleted, nil
}
