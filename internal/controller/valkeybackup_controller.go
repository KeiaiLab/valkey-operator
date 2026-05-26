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

package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	commonsfinalizer "github.com/keiailab/operator-commons/pkg/finalizer"
	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/observability"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

const (
	// operatorImageEnv — Upload Job 의 image 결정 env. Deployment manifest 에서 주입.
	operatorImageEnv     = "OPERATOR_IMAGE"
	defaultOperatorImage = "controller:latest"

	// finalizerValkeyBackup — backup CR 삭제 시 PVC + Job cleanup.
	finalizerValkeyBackup = cachev1alpha1.FinalizerValkeyBackup
)

// ValkeyBackupReconciler — RDB / AOF backup 트리거 + 상태 추적.
//
// 본 iter (M2) 의 책임:
//  1. Spec.ClusterRef 가 가리키는 Valkey / ValkeyCluster 존재 검증.
//  2. Phase 전이 (Pending → InProgress → Completed | Failed).
//  3. Status.StartedAt / CompletedAt / Conditions 기록.
//
// 미구현 (M3 후속):
//   - 실제 BGSAVE / BGREWRITEAOF 명령 발행 + LASTSAVE 폴링.
//   - 결과 PVC 동적 프로비저닝 + RDB 파일 복사 (Job 기반).
//   - TTL 기반 자동 삭제.
type ValkeyBackupReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeys;valkeyclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackuptargets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *ValkeyBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := observability.StartReconcileSpan(ctx, "ValkeyBackup", req.Namespace, req.Name)
	defer span.End()

	logger := logf.FromContext(ctx)

	b := &cachev1alpha1.ValkeyBackup{}
	if err := r.Get(ctx, req.NamespacedName, b); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Finalizer / deletion.
	if !b.DeletionTimestamp.IsZero() {
		return r.handleBackupDeletion(ctx, b)
	}
	if !commonsfinalizer.Has(b, finalizerValkeyBackup) {
		commonsfinalizer.Add(b, finalizerValkeyBackup)
		if err := r.Update(ctx, b); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Terminal phase: TTL 처리 (만료 시 self-delete) + RequeueAfter scheduling.
	if b.IsTerminal() {
		return r.handleBackupTerminal(ctx, b)
	}

	// 1. ClusterRef 검증 — 대상 CR 존재 확인.
	if err := r.validateClusterRef(ctx, b); err != nil {
		return r.markFailed(ctx, b, "TargetNotFound", err.Error())
	}

	// 2. Phase 전이.
	switch b.Status.Phase {
	case "":
		// 신규 — Pending 으로 시작.
		b.Status.Phase = cachev1alpha1.BackupPhasePending
		b.Status.ObservedGeneration = b.Generation
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Pending",
			Message:            "Backup queued",
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil

	case cachev1alpha1.BackupPhasePending:
		// VolumeSnapshot path — k8s native snapshot, valkey-cli 미사용.
		// CRD 미설치 시 fail-soft (markFailed). 정상 시 InProgress 진입.
		if b.Spec.Type == cachev1alpha1.BackupTypeVolumeSnapshot {
			if err := applyVolumeSnapshotForBackup(ctx, r.Client, b); err != nil {
				return r.markFailed(ctx, b, "VolumeSnapshotApplyFailed", err.Error())
			}
			now := metav1.Now()
			b.Status.Phase = cachev1alpha1.BackupPhaseInProgress
			b.Status.StartedAt = &now
			b.Status.Message = "VolumeSnapshot CR applied — polling readyToUse"
			setCondition(b.GetConditions(), metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "InProgress",
				Message:            "VolumeSnapshot in progress",
				ObservedGeneration: b.Generation,
			})
			if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
				return ctrl.Result{RequeueAfter: requeueProgress}, nil
			}
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		// BGSAVE / BGREWRITEAOF 발행 + LASTSAVE 기준 시각 기록 → InProgress.
		preLastSave, err := r.triggerBackup(ctx, b)
		if err != nil {
			return r.markFailed(ctx, b, "BackupTriggerFailed", err.Error())
		}
		now := metav1.Now()
		b.Status.Phase = cachev1alpha1.BackupPhaseInProgress
		b.Status.StartedAt = &now
		// preLastSave 를 message 에 인코딩 — 다음 phase 에서 비교용. 별도 status 필드를
		// 추가하지 않기 위한 간단한 prologue. (대안: annotation 사용, 더 깔끔 — 후속.)
		b.Status.Message = fmt.Sprintf("preLastSave=%d", preLastSave.Unix())
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "InProgress",
			Message:            fmt.Sprintf("Backup %s issued for %s/%s", b.Spec.Type, b.Spec.ClusterRef.Kind, b.Spec.ClusterRef.Name),
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		logger.Info("Backup BGSAVE/BGREWRITEAOF issued",
			"name", b.Name, "type", b.Spec.Type, "target", b.Spec.ClusterRef.Name,
			"preLastSave", preLastSave)
		return ctrl.Result{RequeueAfter: requeueProgress}, nil

	case cachev1alpha1.BackupPhaseInProgress:
		// VolumeSnapshot path — readyToUse polling.
		if b.Spec.Type == cachev1alpha1.BackupTypeVolumeSnapshot {
			ready, err := pollVolumeSnapshotReady(ctx, r.Client, b)
			if err != nil {
				return r.markFailed(ctx, b, "VolumeSnapshotFailed", err.Error())
			}
			// 30분 timeout — 대용량 dataset 도 일반적으로 충분.
			if b.Status.StartedAt != nil && time.Since(b.Status.StartedAt.Time) > 30*time.Minute {
				return r.markFailed(ctx, b, "VolumeSnapshotTimeout",
					"VolumeSnapshot did not reach readyToUse within 30m")
			}
			if !ready {
				return ctrl.Result{RequeueAfter: requeueProgress}, nil
			}
			// 완료. RDB/AOF 와 달리 별도 Copy/Upload 단계 없음 (storage 가 in-cluster snapshot).
			now := metav1.Now()
			b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
			b.Status.CompletedAt = &now
			b.Status.Message = "VolumeSnapshot ready"
			setCondition(b.GetConditions(), metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Completed",
				Message:            "VolumeSnapshot.status.readyToUse=true",
				ObservedGeneration: b.Generation,
			})
			MetricBackupTotal.WithLabelValues(b.Namespace, b.Name, "Completed").Inc()
			if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
				return ctrl.Result{RequeueAfter: requeueProgress}, nil
			}
			return ctrl.Result{RequeueAfter: requeueSteady}, nil
		}
		// LASTSAVE 가 preLastSave 보다 커지면 RDB 스냅샷 완료.
		var preLastSaveUnix int64
		_, _ = fmt.Sscanf(b.Status.Message, "preLastSave=%d", &preLastSaveUnix)
		curLastSave, err := r.queryLastSave(ctx, b)
		if err != nil {
			logger.Info("LASTSAVE poll failed — will retry", "error", err.Error())
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		// 30 분 timeout — RDB 가 매우 큰 dataset 가 아닌 한 충분.
		if b.Status.StartedAt != nil && time.Since(b.Status.StartedAt.Time) > 30*time.Minute {
			return r.markFailed(ctx, b, "BackupTimeout",
				fmt.Sprintf("LASTSAVE did not advance within 30m (pre=%d cur=%d)",
					preLastSaveUnix, curLastSave.Unix()))
		}
		if curLastSave.Unix() <= preLastSaveUnix {
			// 아직 진행 중.
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		// LASTSAVE advanced — RDB 가 노드 에 생성됨. 다음 단계: PVC 복사 Job spawn.
		b.Status.Phase = cachev1alpha1.BackupPhaseCopying
		b.Status.Message = fmt.Sprintf("RDB snapshot at %s — copying to PVC",
			curLastSave.Format(time.RFC3339))
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Copying",
			Message:            b.Status.Message,
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		logger.Info("Backup LASTSAVE advanced — transitioning to Copying",
			"name", b.Name, "lastSave", curLastSave)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil

	case cachev1alpha1.BackupPhaseCopying:
		return r.reconcileCopyingPhase(ctx, b)

	case cachev1alpha1.BackupPhaseUploading:
		return r.reconcileUploadingPhase(ctx, b)
	}

	return ctrl.Result{}, nil
}

// operatorImage — Upload Job 의 image. OPERATOR_IMAGE env 미설정 시 default.
func (r *ValkeyBackupReconciler) operatorImage() string {
	if v := os.Getenv(operatorImageEnv); v != "" {
		return v
	}
	return defaultOperatorImage
}

// hasExternalDestination — Spec.Destination 이 TargetRef 인지.
func hasExternalDestination(b *cachev1alpha1.ValkeyBackup) bool {
	return b.Spec.Destination != nil &&
		b.Spec.Destination.Type == cachev1alpha1.BackupDestTargetRef
}

// validateClusterRef — Spec.ClusterRef 가 가리키는 Valkey / ValkeyCluster 존재 확인.
func (r *ValkeyBackupReconciler) validateClusterRef(ctx context.Context, b *cachev1alpha1.ValkeyBackup) error {
	key := types.NamespacedName{Name: b.Spec.ClusterRef.Name, Namespace: b.Namespace}
	switch b.Spec.ClusterRef.Kind {
	case cachev1alpha1.KindValkeyCluster:
		obj := &cachev1alpha1.ValkeyCluster{}
		if err := r.Get(ctx, key, obj); err != nil {
			return fmt.Errorf("get ValkeyCluster %s: %w", key, err)
		}
	case cachev1alpha1.KindValkey:
		obj := &cachev1alpha1.Valkey{}
		if err := r.Get(ctx, key, obj); err != nil {
			return fmt.Errorf("get Valkey %s: %w", key, err)
		}
	default:
		return fmt.Errorf("unsupported ClusterRef.Kind: %q", b.Spec.ClusterRef.Kind)
	}
	return nil
}

// markFailed — 백업을 Failed phase 로 전이 + 에러 condition.
//
//nolint:unparam // controller-runtime 표준 (ctrl.Result, error) 시그니처 보존.
func (r *ValkeyBackupReconciler) markFailed(ctx context.Context, b *cachev1alpha1.ValkeyBackup, reason, msg string) (ctrl.Result, error) {
	MetricBackupTotal.WithLabelValues(b.Namespace, b.Name, "Failed").Inc()
	if r.Recorder != nil {
		r.Recorder.Eventf(b, nil, "Warning", reason, reason, "%s", msg)
	}
	b.Status.Phase = cachev1alpha1.BackupPhaseFailed
	b.Status.Message = msg
	now := metav1.Now()
	b.Status.CompletedAt = &now
	setCondition(b.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: b.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	return ctrl.Result{}, nil
}

// triggerBackup — 대상 인스턴스의 primary (Valkey) 또는 임의 노드 (ValkeyCluster)
// 에 BGSAVE / BGREWRITEAOF 발행. preLastSave timestamp 반환 (완료 감지용).
func (r *ValkeyBackupReconciler) triggerBackup(ctx context.Context, b *cachev1alpha1.ValkeyBackup) (time.Time, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyBackup/TriggerBGSAVE")
	defer span.End()

	c, err := r.dialBackupTarget(ctx, b)
	if err != nil {
		span.RecordError(err)
		return time.Time{}, err
	}
	defer func() { _ = c.Close() }()

	preLastSave, err := vk.LastSaveTime(ctx, c)
	if err != nil {
		return time.Time{}, err
	}
	switch b.Spec.Type {
	case cachev1alpha1.BackupTypeAOF:
		if err := vk.BgRewriteAOF(ctx, c); err != nil {
			return time.Time{}, err
		}
	default: // RDB or unset.
		if err := vk.BgSave(ctx, c); err != nil {
			return time.Time{}, err
		}
	}
	return preLastSave, nil
}

// queryLastSave — primary (Valkey) 또는 임의 노드 (ValkeyCluster) 의 LASTSAVE 조회.
func (r *ValkeyBackupReconciler) queryLastSave(ctx context.Context, b *cachev1alpha1.ValkeyBackup) (time.Time, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyBackup/LASTSAVE")
	defer span.End()

	c, err := r.dialBackupTarget(ctx, b)
	if err != nil {
		span.RecordError(err)
		return time.Time{}, err
	}
	defer func() { _ = c.Close() }()
	return vk.LastSaveTime(ctx, c)
}

// dialBackupTarget — dial_helpers.go 의 dialClusterRefTarget thin wrapper.
func (r *ValkeyBackupReconciler) dialBackupTarget(ctx context.Context, b *cachev1alpha1.ValkeyBackup) (*redis.Client, error) {
	return dialClusterRefTarget(ctx, r.Client, b.Spec.ClusterRef, b.Namespace)
}

// reconcileCopyingPhase — Copying phase 의 reconcile loop.
//
// 처리 흐름:
//  1. backup PVC 보장 (Spec.TargetPVC 미명시 시 동적 생성).
//  2. backup Job 보장 (`valkey-cli --rdb /backup/dump.rdb`).
//  3. Job 상태 폴링: Succeeded → Completed, Failed → Failed, 진행 중 → requeue.
//
// TargetPVC 명시 시 PVC 생성 skip — 사용자 가 미리 만든 PVC 사용.
func (r *ValkeyBackupReconciler) reconcileCopyingPhase(ctx context.Context, b *cachev1alpha1.ValkeyBackup) (ctrl.Result, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyBackup/Copying")
	defer span.End()

	logger := logf.FromContext(ctx)

	// 1. PVC 보장.
	pvcName := resources.BackupPVCName(b.Name)
	if b.Spec.TargetPVC != nil && b.Spec.TargetPVC.Name != "" {
		pvcName = b.Spec.TargetPVC.Name
	} else {
		pvc := resources.BuildBackupPVC(b)
		if err := controllerutil.SetControllerReference(b, pvc, r.Scheme); err != nil {
			return r.markFailed(ctx, b, "PVCOwnerRef", err.Error())
		}
		existing := &corev1.PersistentVolumeClaim{}
		if err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, existing); err != nil {
			if errors.IsNotFound(err) {
				if err := r.Create(ctx, pvc); err != nil {
					return r.markFailed(ctx, b, "PVCCreateFailed", err.Error())
				}
			} else {
				return ctrl.Result{RequeueAfter: requeueProgress}, nil
			}
		}
	}

	// 2. Job 보장 (controllerutil.CreateOrUpdate — postgres / mongodb it42 패턴 차용).
	//
	// iteration 43 (mongodb it42 aa56f48 차용): 이전 *수동 Get + Create + IsAlreadyExists
	// guard* (it40 ac1421f) → controllerutil.CreateOrUpdate. controller-runtime 이
	// AlreadyExists 자동 retry + mutate fn 호출. mutate fn 이 *owner ref 만* 설정 →
	// 첫 reconcile 시 Create, 후속 reconcile 시 owner ref diff 없음 → no-op (Job spec
	// immutable field 안전).
	job, err := r.buildBackupJob(ctx, b, pvcName)
	if err != nil {
		return r.markFailed(ctx, b, "JobBuildFailed", err.Error())
	}
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, job, func() error {
		return controllerutil.SetControllerReference(b, job, r.Scheme)
	})
	if err != nil {
		return r.markFailed(ctx, b, "JobCreateFailed", err.Error())
	}
	if op == controllerutil.OperationResultCreated {
		logger.Info("Backup copy Job created", "name", job.Name)
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	existingJob := job

	// 3. Job 상태 폴링.
	if existingJob.Status.Succeeded > 0 {
		// Destination=TargetRef 시 → Uploading, 그 외 → Completed (M3.5 호환).
		if hasExternalDestination(b) {
			b.Status.Phase = cachev1alpha1.BackupPhaseUploading
			b.Status.PVCName = pvcName
			b.Status.Message = fmt.Sprintf("RDB on PVC %s — uploading to external target", pvcName)
			setCondition(b.GetConditions(), metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "Uploading",
				Message:            b.Status.Message,
				ObservedGeneration: b.Generation,
			})
			if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
				return ctrl.Result{RequeueAfter: requeueProgress}, nil
			}
			logger.Info("Backup copy Job succeeded — transitioning to Uploading", "pvc", pvcName)
			return ctrl.Result{Requeue: true}, nil
		}

		MetricBackupTotal.WithLabelValues(b.Namespace, b.Name, "Completed").Inc()
		if r.Recorder != nil {
			r.Recorder.Eventf(b, nil, "Normal", "Completed", "Completed", "ValkeyBackup completed")
		}
		now := metav1.Now()
		b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
		b.Status.CompletedAt = &now
		b.Status.PVCName = pvcName
		b.Status.Message = fmt.Sprintf("RDB copied to PVC %s by Job %s", pvcName, job.Name)
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Completed",
			Message:            b.Status.Message,
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		logger.Info("Backup copy Job succeeded — Completed", "pvc", pvcName)
		return ctrl.Result{}, nil
	}
	if existingJob.Status.Failed > 0 {
		return r.markFailed(ctx, b, "CopyJobFailed",
			fmt.Sprintf("Job %s failed (failed=%d)", job.Name, existingJob.Status.Failed))
	}
	// 진행 중.
	return ctrl.Result{RequeueAfter: requeueProgress}, nil
}

// reconcileUploadingPhase — backup PVC 의 RDB 를 외부 저장 (S3) 으로 업로드.
//
// 흐름:
//  1. Spec.Destination.TargetRef 검증 + ValkeyBackupTarget CR Get + Phase=Reachable 검증.
//  2. ObjectKey 결정 (Spec.Destination.TargetRef.Path 명시 시 그 값,
//     아니면 DefaultBackupObjectPath).
//  3. Upload Job 보장 (멱등) — operator image 의 `upload` sub-command 호출.
//  4. Job status 폴링: Succeeded → Completed, Failed → Failed.
func (r *ValkeyBackupReconciler) reconcileUploadingPhase(
	ctx context.Context, b *cachev1alpha1.ValkeyBackup,
) (ctrl.Result, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyBackup/Uploading")
	defer span.End()

	logger := logf.FromContext(ctx)

	if !hasExternalDestination(b) || b.Spec.Destination.TargetRef == nil {
		return r.markFailed(ctx, b, "InvalidDestination",
			"Phase=Uploading 이지만 Spec.Destination.TargetRef 미설정")
	}

	// 1. Target CR Get.
	tgt := &cachev1alpha1.ValkeyBackupTarget{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      b.Spec.Destination.TargetRef.Name,
		Namespace: b.Namespace,
	}, tgt); err != nil {
		if errors.IsNotFound(err) {
			return r.markFailed(ctx, b, "TargetNotFound",
				fmt.Sprintf("ValkeyBackupTarget %s/%s 미존재",
					b.Namespace, b.Spec.Destination.TargetRef.Name))
		}
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	if tgt.Status.Phase != cachev1alpha1.BackupTargetPhaseReachable {
		// Reachable 이 아니면 — 잠시 대기 후 재확인. 영구 차단 방지.
		return ctrl.Result{RequeueAfter: requeueDependencyUnavailable}, nil
	}
	if tgt.Spec.S3 == nil {
		return r.markFailed(ctx, b, "TargetMissingS3",
			"ValkeyBackupTarget.Spec.S3 미설정")
	}

	// 2. ObjectKey 결정.
	objectKey := b.Spec.Destination.TargetRef.Path
	if objectKey == "" {
		startedAt := ""
		if b.Status.StartedAt != nil {
			startedAt = b.Status.StartedAt.UTC().Format(time.RFC3339)
		}
		objectKey = cachev1alpha1.DefaultBackupObjectPath(b.Name, startedAt)
	}
	objectKey = tgt.Spec.S3.Prefix + objectKey

	// 3. Upload Job 보장.
	pvcName := b.Status.PVCName
	if pvcName == "" {
		pvcName = resources.BackupPVCName(b.Name)
	}
	uploadJob := resources.BuildUploadJob(resources.UploadJobParams{
		BackupName:               b.Name,
		Namespace:                b.Namespace,
		OperatorImage:            r.operatorImage(),
		PVCName:                  pvcName,
		FilePath:                 resources.BackupVolumeMountPath + "/" + resources.BackupRDBFileName,
		Endpoint:                 tgt.Spec.S3.Endpoint,
		Region:                   tgt.Spec.S3.Region,
		Bucket:                   tgt.Spec.S3.Bucket,
		ObjectKey:                objectKey,
		ForcePathStyle:           tgt.Spec.S3.ForcePathStyle,
		CredentialsSecretName:    tgt.Spec.S3.CredentialsSecretRef.Name,
		AccessKeyIDSecretKey:     keyOrDefault(tgt.Spec.S3.CredentialsSecretRef.AccessKeyIDKey, "AWS_ACCESS_KEY_ID"),
		SecretAccessKeySecretKey: keyOrDefault(tgt.Spec.S3.CredentialsSecretRef.SecretAccessKeyKey, "AWS_SECRET_ACCESS_KEY"),
	})
	// iteration 43: controllerutil.CreateOrUpdate 마이그레이션 (mongodb it42 aa56f48
	// + postgres 패턴 차용). owner ref 만 mutate — Job spec immutable 안전.
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, uploadJob, func() error {
		return controllerutil.SetControllerReference(b, uploadJob, r.Scheme)
	})
	if err != nil {
		return r.markFailed(ctx, b, "UploadJobCreateFailed", err.Error())
	}
	if op == controllerutil.OperationResultCreated {
		logger.Info("Upload Job created", "name", uploadJob.Name, "object", objectKey)
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	existing := uploadJob // CreateOrUpdate 가 obj 를 in-place modify (Get 결과로 채움).

	// 4. Job 상태 폴링.
	if existing.Status.Succeeded > 0 {
		MetricBackupTotal.WithLabelValues(b.Namespace, b.Name, "Completed").Inc()
		if r.Recorder != nil {
			r.Recorder.Eventf(b, nil, "Normal", "Completed", "Completed", "ValkeyBackup completed")
		}
		now := metav1.Now()
		b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
		b.Status.CompletedAt = &now
		b.Status.Message = fmt.Sprintf("Uploaded to %s/%s — Completed", tgt.Spec.S3.Bucket, objectKey)
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Completed",
			Message:            b.Status.Message,
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		logger.Info("Upload Job succeeded — Backup Completed",
			"object", objectKey, "bucket", tgt.Spec.S3.Bucket)
		return ctrl.Result{}, nil
	}
	if existing.Status.Failed > 0 {
		return r.markFailed(ctx, b, "UploadJobFailed",
			fmt.Sprintf("Upload Job %s failed (failed=%d)", uploadJob.Name, existing.Status.Failed))
	}
	// 진행 중.
	return ctrl.Result{RequeueAfter: requeueProgress}, nil
}

// keyOrDefault — empty 시 default 반환.
func keyOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// handleBackupTerminal — Phase=Completed/Failed 시 TTL 만료 검사.
//
// Spec.TTL 명시 + Status.CompletedAt + 현재 > deadline → r.Delete(ctx, b).
// finalizer 가 cleanup 진행. 미만료 시 RequeueAfter=time.Until(deadline).
//
// TTL 미명시 시 보존 (no-op).
//
//nolint:unparam // controller-runtime 표준 시그니처 보존.
func (r *ValkeyBackupReconciler) handleBackupTerminal(
	ctx context.Context, b *cachev1alpha1.ValkeyBackup,
) (ctrl.Result, error) {
	if b.Spec.TTL == "" || b.Status.CompletedAt == nil {
		return ctrl.Result{}, nil
	}
	ttl, err := time.ParseDuration(b.Spec.TTL)
	if err != nil {
		// TTL 파싱 실패 — 보존 (operator 가 자동 삭제 시도 안 함).
		return ctrl.Result{}, nil
	}
	deadline := b.Status.CompletedAt.Add(ttl)
	if time.Now().After(deadline) {
		// 만료 — self-delete. finalizer 가 cleanup.
		if err := r.Delete(ctx, b); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: time.Until(deadline)}, nil
}

// handleBackupDeletion — finalizer cleanup. Backup PVC (RetainPVC=false 시) +
// Backup Job + Upload Job 정리. Owner-ref GC 가 RetainPVC=false 인 경우
// PVC 도 처리하지만, RetainPVC=true 시 owner-ref 미설정 패턴 (별개 commit
// 보강). 본 commit 은 *명시적 cleanup* 으로 안전 보장.
//
//nolint:unparam // controller-runtime 표준 시그니처 보존.
func (r *ValkeyBackupReconciler) handleBackupDeletion(
	ctx context.Context, b *cachev1alpha1.ValkeyBackup,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Backup Job + Upload Job 명시적 삭제 (best-effort).
	for _, jobName := range []string{
		resources.BackupJobName(b.Name),
		resources.UploadJobName(b.Name),
	} {
		job := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: b.Namespace}, job); err == nil {
			// PropagationPolicy=Background — Job 의 Pod 도 함께 삭제.
			policy := metav1.DeletePropagationBackground
			_ = r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &policy})
		}
	}

	// PVC 정리 — RetainPVC=false 시만.
	if !b.Spec.RetainPVC {
		pvcName := b.Status.PVCName
		if pvcName == "" {
			pvcName = resources.BackupPVCName(b.Name)
		}
		pvc := &corev1.PersistentVolumeClaim{}
		if err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: b.Namespace}, pvc); err == nil {
			_ = r.Delete(ctx, pvc)
		}
	}

	commonsfinalizer.Remove(b, finalizerValkeyBackup)
	if err := r.Update(ctx, b); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	logger.Info("ValkeyBackup deleted — Job/PVC cleanup", "name", b.Name, "retainPVC", b.Spec.RetainPVC)
	return ctrl.Result{}, nil
}

// buildBackupJob — 대상 CR 의 image / TLS 설정을 조회 한 후 Job spec 빌드.
func (r *ValkeyBackupReconciler) buildBackupJob(ctx context.Context, b *cachev1alpha1.ValkeyBackup, pvcName string) (*batchv1.Job, error) {
	var (
		image         = cachev1alpha1.DefaultValkeyImage + ":" + cachev1alpha1.DefaultValkeyVersion
		tlsEnabled    bool
		tlsSecretName string
		targetHosts   []string
		nsName        = types.NamespacedName{Name: b.Spec.ClusterRef.Name, Namespace: b.Namespace}
	)
	switch b.Spec.ClusterRef.Kind {
	case cachev1alpha1.KindValkey:
		obj := &cachev1alpha1.Valkey{}
		if err := r.Get(ctx, nsName, obj); err != nil {
			return nil, err
		}
		if obj.Spec.Version.Image != "" && obj.Spec.Version.Version != "" {
			image = fmt.Sprintf("%s:%s", obj.Spec.Version.Image, obj.Spec.Version.Version)
		}
		if obj.Spec.TLS != nil && obj.Spec.TLS.Enabled {
			tlsEnabled = true
			tlsSecretName = backupTargetTLSSecret(obj.Spec.TLS, obj.Name)
		}
	case cachev1alpha1.KindValkeyCluster:
		obj := &cachev1alpha1.ValkeyCluster{}
		if err := r.Get(ctx, nsName, obj); err != nil {
			return nil, err
		}
		if obj.Spec.Version.Image != "" && obj.Spec.Version.Version != "" {
			image = fmt.Sprintf("%s:%s", obj.Spec.Version.Image, obj.Spec.Version.Version)
		}
		if obj.Spec.TLS != nil && obj.Spec.TLS.Enabled {
			tlsEnabled = true
			tlsSecretName = backupTargetTLSSecret(obj.Spec.TLS, obj.Name)
		}
		targetHosts = valkeyClusterBackupHosts(obj, b.Namespace)
	}
	port := int32(resources.PortClient)
	if tlsEnabled {
		port = resources.PortTLS
	}
	return resources.BuildBackupJob(resources.BackupJobParams{
		BackupName:  b.Name,
		Namespace:   b.Namespace,
		PVCName:     pvcName,
		Image:       image,
		TargetHost:  resources.PodFQDN(b.Spec.ClusterRef.Name, 0, b.Namespace),
		TargetHosts: targetHosts,
		TargetPort:  port,
		PasswordSecret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: resources.DefaultSecretName(b.Spec.ClusterRef.Name),
			},
			Key: resources.SecretPasswordKey,
		},
		UseTLS:        tlsEnabled,
		TLSSecretName: tlsSecretName,
	}), nil
}

func valkeyClusterBackupHosts(vc *cachev1alpha1.ValkeyCluster, namespace string) []string {
	if vc.Spec.Shards <= 0 {
		return nil
	}
	hosts := make([]string, int(vc.Spec.Shards))
	for i := int32(0); i < vc.Spec.Shards; i++ {
		hosts[i] = resources.PodFQDN(vc.Name, int(i), namespace)
	}
	for _, shard := range vc.Status.Shards {
		if shard.Index < 0 || shard.Index >= vc.Spec.Shards || shard.PrimaryPod == "" {
			continue
		}
		hosts[shard.Index] = fmt.Sprintf("%s.%s.%s.svc",
			shard.PrimaryPod, resources.HeadlessServiceName(vc.Name), namespace)
	}
	return hosts
}

// backupTargetTLSSecret — TLS Spec 으로 부터 secret 이름 결정 (CustomCert 우선).
func backupTargetTLSSecret(t *cachev1alpha1.TLSSpec, crName string) string {
	if t.CustomCert != nil && t.CustomCert.SecretName != "" {
		return t.CustomCert.SecretName
	}
	if t.CertManager != nil && t.CertManager.IssuerRef.Name != "" {
		return resources.CertificateSecretName(crName)
	}
	return ""
}

// SetupWithManager sets up the controller with the Manager.
func (r *ValkeyBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// events API 마이그레이션 완료 (RFC-0023 Phase 2, 2026-05-11).
	r.Recorder = mgr.GetEventRecorder("valkeybackup-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.ValkeyBackup{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Named("valkeybackup").
		Complete(r)
}
