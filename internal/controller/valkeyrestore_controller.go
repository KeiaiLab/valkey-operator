/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/observability"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

const (
	finalizerValkeyRestore = cachev1alpha1.FinalizerValkeyRestore
)

// sourcePVCName — Source.PVC 시 그대로, Source.TargetRef 시 임시 PVC 이름.
// Restoring phase 의 init container 가 mount 하는 PVC.
func sourcePVCName(rest *cachev1alpha1.ValkeyRestore) string {
	if rest.Spec.Source.PVC != nil && rest.Spec.Source.PVC.Name != "" {
		return rest.Spec.Source.PVC.Name
	}
	return resources.RestoreSourcePVCName(rest.Name)
}

// sourceRDBPath — Source PVC 안의 RDB 파일 상대 경로.
func sourceRDBPath(rest *cachev1alpha1.ValkeyRestore) string {
	if rest.Spec.Source.PVC != nil && rest.Spec.Source.PVC.Path != "" {
		return rest.Spec.Source.PVC.Path
	}
	return resources.BackupRDBFileName // "dump.rdb"
}

// ValkeyRestoreReconciler — Source.PVC 에서 RDB 를 cluster 로 복원 (ADR-0015).
//
// 본 commit 의 범위 (Track A AI-002):
//   - Source: PVC 만 (TargetRef = 외부 저장 은 별개 commit, ADR-0016 통합).
//   - 대상: ClusterRef.Kind="Valkey" + Mode=Standalone (replicas=1) 만.
//     Replication / ValkeyCluster 는 ReadOnlyMany source PVC 가 필요 →
//     별개 commit.
//
// Phase 전이 ("" → Pending → Mounting → Restoring → Verifying → Completed):
//   - "" → Pending: status 초기화 + Conditions["Ready"]=False/Pending.
//   - Pending → Mounting: ClusterRef + Source.PVC 검증 + Standalone 검증.
//   - Mounting → Restoring: paused annotation set + STS 에 init container
//     inject + Update.
//   - Restoring → Verifying: STS 의 모든 pod Ready 확인 (rolling 완료).
//   - Verifying → Completed: STS 원복 (init container 제거) + paused 제거.
//
// Finalizer (cache.keiailab.io/valkeyrestore-finalizer): CR 삭제 시 STS
// 원복 + paused 정리. 정상 Completed 흐름에서는 이미 정리되어 no-op.
type ValkeyRestoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyrestores/finalizers,verbs=update

func (r *ValkeyRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := observability.StartReconcileSpan(ctx, "ValkeyRestore", req.Namespace, req.Name)
	defer span.End()

	logger := logf.FromContext(ctx)

	rest := &cachev1alpha1.ValkeyRestore{}
	if err := r.Get(ctx, req.NamespacedName, rest); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Finalizer / deletion.
	if !rest.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, rest)
	}
	if !controllerutil.ContainsFinalizer(rest, finalizerValkeyRestore) {
		controllerutil.AddFinalizer(rest, finalizerValkeyRestore)
		if err := r.Update(ctx, rest); err != nil {
			return ctrl.Result{}, err
		}
		// finalizer 추가는 객체 ResourceVersion 갱신 → 다음 reconcile 에서 phase 진입.
		return ctrl.Result{Requeue: true}, nil
	}

	if rest.IsTerminal() {
		return ctrl.Result{}, nil
	}

	// Phase 전이.
	switch rest.Status.Phase {
	case "":
		return r.transitionToPending(ctx, rest)
	case cachev1alpha1.RestorePhasePending:
		return r.handlePending(ctx, rest)
	case cachev1alpha1.RestorePhaseMounting:
		return r.handleMounting(ctx, rest)
	case cachev1alpha1.RestorePhaseRestoring:
		return r.handleRestoring(ctx, rest)
	case cachev1alpha1.RestorePhaseVerifying:
		return r.handleVerifying(ctx, rest)
	}

	logger.V(1).Info("unknown phase — no-op", "phase", rest.Status.Phase)
	return ctrl.Result{}, nil
}

// transitionToPending — "" → Pending. 단순 status 초기화.
func (r *ValkeyRestoreReconciler) transitionToPending(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	now := metav1.Now()
	rest.Status.Phase = cachev1alpha1.RestorePhasePending
	rest.Status.StartedAt = &now
	rest.Status.ObservedGeneration = rest.Generation
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Pending",
		Message:            "validating ClusterRef + Source",
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

// handlePending — ClusterRef + Source 검증 → Mounting.
//
// 지원: ClusterRef.Kind ∈ {Valkey, ValkeyCluster}. Source 는 PVC | TargetRef
// 둘 중 하나 (XOR). multi-pod 모드 (Replication / Cluster) 시 ROX source 강제.
func (r *ValkeyRestoreReconciler) handlePending(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	if rest.Spec.ClusterRef.Kind != "Valkey" && rest.Spec.ClusterRef.Kind != "ValkeyCluster" {
		return r.markFailed(ctx, rest, "UnsupportedClusterKind",
			fmt.Sprintf("kind=%s — Valkey 또는 ValkeyCluster 만 지원",
				rest.Spec.ClusterRef.Kind))
	}
	hasPVC := rest.Spec.Source.PVC != nil
	hasTargetRef := rest.Spec.Source.TargetRef != nil
	if !hasPVC && !hasTargetRef {
		return r.markFailed(ctx, rest, "MissingSource",
			"Source.PVC 또는 Source.TargetRef 중 하나 필요")
	}
	if hasPVC && hasTargetRef {
		return r.markFailed(ctx, rest, "AmbiguousSource",
			"Source.PVC + Source.TargetRef 동시 명시 — 하나만 명시")
	}
	if hasPVC && rest.Spec.Source.PVC.Name == "" {
		return r.markFailed(ctx, rest, "MissingSourcePVCName",
			"spec.source.pvc.name required")
	}
	if hasTargetRef {
		if rest.Spec.Source.TargetRef.Name == "" {
			return r.markFailed(ctx, rest, "MissingTargetRefName",
				"spec.source.targetRef.name required")
		}
		if rest.Spec.Source.TargetRef.Path == "" {
			return r.markFailed(ctx, rest, "MissingTargetRefPath",
				"spec.source.targetRef.path required")
		}
	}

	// 대상 CR 의 multi-pod 여부 결정.
	multiPod, err := r.isMultiPodTarget(ctx, rest)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.markFailed(ctx, rest, "TargetNotFound",
				fmt.Sprintf("%s/%s/%s not found",
					rest.Spec.ClusterRef.Kind, rest.Namespace, rest.Spec.ClusterRef.Name))
		}
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	// multi-pod (Replication replicas>1 또는 ValkeyCluster) 시 source PVC 가
	// ROX 인지 검증. RWO source 는 multi-pod 동시 mount 불가 — init container
	// 가 첫 pod 에서만 실행 후 attach 끊어지지 않아 다음 pod ContainerCreating
	// 무한 대기.
	if multiPod {
		if rest.Spec.Source.PVC != nil {
			// Source.PVC 시 사전 PVC 의 accessMode 검증.
			sourcePVC := &corev1.PersistentVolumeClaim{}
			if err := r.Get(ctx, types.NamespacedName{
				Name: rest.Spec.Source.PVC.Name, Namespace: rest.Namespace,
			}, sourcePVC); err == nil {
				shared := slices.ContainsFunc(sourcePVC.Spec.AccessModes, sourcePVCSupportsMultiPod)
				if !shared {
					return r.markFailed(ctx, rest, "SourcePVCNotShared",
						fmt.Sprintf("multi-pod target 에서 Source.PVC %s 가 ReadOnlyMany 또는 ReadWriteMany 필요 (RWO 는 multi-pod mount 불가)",
							rest.Spec.Source.PVC.Name))
				}
			}
			// Get 실패 시 (NotFound) 는 후속 handleMounting 에서 처리.
		}
		// Source.TargetRef 시: SourcePVCAccessMode 가 ROX 인지 검증.
		if rest.Spec.Source.TargetRef != nil {
			if rest.Spec.SourcePVCAccessMode != cachev1alpha1.SourcePVCAccessModeROX {
				return r.markFailed(ctx, rest, "SourcePVCAccessModeRequired",
					"multi-pod target + Source.TargetRef → Spec.SourcePVCAccessMode=ReadOnlyMany 명시 필수")
			}
		}
	}

	// → Mounting.
	rest.Status.Phase = cachev1alpha1.RestorePhaseMounting
	rest.Status.Message = "ClusterRef + Source validated — entering Mounting"
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Mounting",
		Message:            rest.Status.Message,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

// handleMounting — Source 확보 + paused annotation set → Restoring.
//
// Source.PVC: PVC 존재만 확인.
// Source.TargetRef: ValkeyBackupTarget Reachable 검증 + 임시 PVC 보장 +
//
//	Download Job spawn → 완료 시 진행.
func (r *ValkeyRestoreReconciler) handleMounting(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyRestore/Mounting")
	defer span.End()

	// Source 확보.
	if rest.Spec.Source.PVC != nil {
		if res, ok, err := r.ensurePVCSource(ctx, rest); !ok {
			return res, err
		}
	} else if rest.Spec.Source.TargetRef != nil {
		if res, ok, err := r.ensureTargetRefSource(ctx, rest); !ok {
			return res, err
		}
	}

	if err := r.pauseRestoreTarget(ctx, rest); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	// → Restoring.
	rest.Status.Phase = cachev1alpha1.RestorePhaseRestoring
	rest.Status.Message = fmt.Sprintf("Source PVC %s ready, target paused — STS patch pending",
		sourcePVCName(rest))
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Restoring",
		Message:            rest.Status.Message,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

// ensurePVCSource — Source.PVC: 사전 존재 확인 만. ok=false 시 caller 가
// 반환된 result/err 그대로 전파.
func (r *ValkeyRestoreReconciler) ensurePVCSource(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, bool, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.Source.PVC.Name,
		Namespace: rest.Namespace,
	}, pvc); err != nil {
		if errors.IsNotFound(err) {
			res, err := r.markFailed(ctx, rest, "SourcePVCNotFound",
				fmt.Sprintf("PVC/%s/%s not found", rest.Namespace, rest.Spec.Source.PVC.Name))
			return res, false, err
		}
		return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
	}
	return ctrl.Result{}, true, nil
}

// ensureTargetRefSource — Source.TargetRef: ValkeyBackupTarget Reachable +
// 임시 PVC 생성 + Download Job spawn → Job Succeeded 까지 대기.
//
// ok=true 만 호출자가 다음 단계 (paused annotation set + Restoring 전이) 진입.
func (r *ValkeyRestoreReconciler) ensureTargetRefSource(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, bool, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyRestore/EnsureTargetRefSource")
	defer span.End()

	logger := logf.FromContext(ctx)

	// 1. ValkeyBackupTarget Get + Reachable 검증.
	tgt := &cachev1alpha1.ValkeyBackupTarget{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.Source.TargetRef.Name,
		Namespace: rest.Namespace,
	}, tgt); err != nil {
		if errors.IsNotFound(err) {
			res, err := r.markFailed(ctx, rest, "TargetRefNotFound",
				fmt.Sprintf("ValkeyBackupTarget %s/%s not found",
					rest.Namespace, rest.Spec.Source.TargetRef.Name))
			return res, false, err
		}
		return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
	}
	if tgt.Status.Phase != cachev1alpha1.BackupTargetPhaseReachable {
		return ctrl.Result{RequeueAfter: requeueDependencyUnavailable}, false, nil
	}
	if tgt.Spec.S3 == nil {
		res, err := r.markFailed(ctx, rest, "TargetMissingS3",
			"ValkeyBackupTarget.Spec.S3 미설정")
		return res, false, err
	}

	// 2. 임시 source PVC 보장. Replication mode 시 ROX 필요.
	accessMode := corev1.ReadWriteOnce
	if rest.Spec.SourcePVCAccessMode == cachev1alpha1.SourcePVCAccessModeROX {
		accessMode = corev1.ReadOnlyMany
	}
	pvc := resources.BuildRestoreSourcePVC(rest.Name, rest.Namespace, accessMode)
	if err := controllerutil.SetControllerReference(rest, pvc, r.Scheme); err != nil {
		res, err := r.markFailed(ctx, rest, "PVCOwnerRef", err.Error())
		return res, false, err
	}
	existingPVC := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, existingPVC); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, pvc); err != nil {
				res, err := r.markFailed(ctx, rest, "PVCCreateFailed", err.Error())
				return res, false, err
			}
			return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
		}
		return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
	}

	// 3. Download Job 보장.
	downloadJob := resources.BuildDownloadJob(resources.DownloadJobParams{
		RestoreName:              rest.Name,
		Namespace:                rest.Namespace,
		OperatorImage:            r.operatorImage(),
		PVCName:                  pvc.Name,
		FilePath:                 resources.BackupVolumeMountPath + "/" + resources.BackupRDBFileName,
		Endpoint:                 tgt.Spec.S3.Endpoint,
		Region:                   tgt.Spec.S3.Region,
		Bucket:                   tgt.Spec.S3.Bucket,
		ObjectKey:                tgt.Spec.S3.Prefix + rest.Spec.Source.TargetRef.Path,
		ForcePathStyle:           tgt.Spec.S3.ForcePathStyle,
		CredentialsSecretName:    tgt.Spec.S3.CredentialsSecretRef.Name,
		AccessKeyIDSecretKey:     keyOrDefault(tgt.Spec.S3.CredentialsSecretRef.AccessKeyIDKey, "AWS_ACCESS_KEY_ID"),
		SecretAccessKeySecretKey: keyOrDefault(tgt.Spec.S3.CredentialsSecretRef.SecretAccessKeyKey, "AWS_SECRET_ACCESS_KEY"),
	})
	if err := controllerutil.SetControllerReference(rest, downloadJob, r.Scheme); err != nil {
		res, err := r.markFailed(ctx, rest, "JobOwnerRef", err.Error())
		return res, false, err
	}
	existingJob := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: downloadJob.Name, Namespace: downloadJob.Namespace}, existingJob); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, downloadJob); err != nil {
				res, err := r.markFailed(ctx, rest, "DownloadJobCreateFailed", err.Error())
				return res, false, err
			}
			logger.Info("Download Job created", "name", downloadJob.Name)
			return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
		}
		return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
	}

	// 4. Job 상태 폴링.
	if existingJob.Status.Succeeded > 0 {
		return ctrl.Result{}, true, nil // 다음 단계 진입.
	}
	if existingJob.Status.Failed > 0 {
		res, err := r.markFailed(ctx, rest, "DownloadJobFailed",
			fmt.Sprintf("Download Job %s failed (failed=%d)", downloadJob.Name, existingJob.Status.Failed))
		return res, false, err
	}
	// 진행 중.
	return ctrl.Result{RequeueAfter: requeueProgress}, false, nil
}

// operatorImage — Download/Upload Job image. valkeybackup_controller 와 동일.
func (r *ValkeyRestoreReconciler) operatorImage() string {
	if v := os.Getenv("OPERATOR_IMAGE"); v != "" {
		return v
	}
	return "controller:latest"
}

func sourcePVCSupportsMultiPod(mode corev1.PersistentVolumeAccessMode) bool {
	return mode == corev1.ReadOnlyMany || mode == corev1.ReadWriteMany
}

func (r *ValkeyRestoreReconciler) restoreTarget(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (client.Object, error) {
	key := types.NamespacedName{Name: rest.Spec.ClusterRef.Name, Namespace: rest.Namespace}
	switch rest.Spec.ClusterRef.Kind {
	case "Valkey":
		v := &cachev1alpha1.Valkey{}
		if err := r.Get(ctx, key, v); err != nil {
			return nil, err
		}
		return v, nil
	case "ValkeyCluster":
		vc := &cachev1alpha1.ValkeyCluster{}
		if err := r.Get(ctx, key, vc); err != nil {
			return nil, err
		}
		return vc, nil
	default:
		return nil, fmt.Errorf("unsupported kind %q", rest.Spec.ClusterRef.Kind)
	}
}

func (r *ValkeyRestoreReconciler) pauseRestoreTarget(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) error {
	target, err := r.restoreTarget(ctx, rest)
	if err != nil {
		return err
	}
	annotations := target.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	if annotations[PausedAnnotation] == "true" {
		return nil
	}
	annotations[PausedAnnotation] = "true"
	target.SetAnnotations(annotations)
	return r.Update(ctx, target)
}

func (r *ValkeyRestoreReconciler) unpauseRestoreTarget(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) error {
	target, err := r.restoreTarget(ctx, rest)
	if err != nil {
		return err
	}
	annotations := target.GetAnnotations()
	if annotations[PausedAnnotation] != "true" {
		return nil
	}
	delete(annotations, PausedAnnotation)
	target.SetAnnotations(annotations)
	return r.Update(ctx, target)
}

// isMultiPodTarget — 대상 CR 의 pod 수가 1 초과인지.
//   - Valkey: replicas > 1 (Mode=Replication)
//   - ValkeyCluster: 항상 multi-pod (shards × (1 + replicasPerShard) ≥ 3)
func (r *ValkeyRestoreReconciler) isMultiPodTarget(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (bool, error) {
	switch rest.Spec.ClusterRef.Kind {
	case "Valkey":
		v := &cachev1alpha1.Valkey{}
		if err := r.Get(ctx, types.NamespacedName{
			Name: rest.Spec.ClusterRef.Name, Namespace: rest.Namespace,
		}, v); err != nil {
			return false, err
		}
		return v.Spec.Replicas > 1, nil
	case "ValkeyCluster":
		vc := &cachev1alpha1.ValkeyCluster{}
		if err := r.Get(ctx, types.NamespacedName{
			Name: rest.Spec.ClusterRef.Name, Namespace: rest.Namespace,
		}, vc); err != nil {
			return false, err
		}
		_ = vc // existence 만 검증 — 항상 multi-pod.
		return true, nil
	}
	return false, fmt.Errorf("unsupported kind %q", rest.Spec.ClusterRef.Kind)
}

// shardCountForTarget — ValkeyCluster.Spec.Shards. Kind=Valkey 시 0 (cluster
// 모드 init container 미사용).
func (r *ValkeyRestoreReconciler) shardCountForTarget(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (int32, error) {
	if rest.Spec.ClusterRef.Kind != "ValkeyCluster" {
		return 0, nil
	}
	vc := &cachev1alpha1.ValkeyCluster{}
	if err := r.Get(ctx, types.NamespacedName{
		Name: rest.Spec.ClusterRef.Name, Namespace: rest.Namespace,
	}, vc); err != nil {
		return 0, err
	}
	return vc.Spec.Shards, nil
}

// parseShardLayout — Spec.Source.PVC.ShardLayout 의 string key → int 매핑.
// "0" / "shard-0" / "shard0" 모두 허용. 파싱 실패 line 은 skip.
func parseShardLayout(input map[string]string) map[int]string {
	out := map[int]string{}
	for k, v := range input {
		cleaned := strings.TrimPrefix(k, "shard-")
		cleaned = strings.TrimPrefix(cleaned, "shard")
		if i, err := strconv.Atoi(cleaned); err == nil {
			out[i] = v
		}
	}
	return out
}

// === 데이터 plane 검증 (Verifying phase 의 INFO keyspace) helpers ===
// 패턴은 valkeybackup_controller.go 의 dialBackupTarget / tlsConfigForBackupTarget
// / fetchBackupTargetPassword 와 동등. ClusterRef 만 ValkeyRestoreSpec 에서 가져옴.
//
// 추후 별개 commit 에서 *공통 helper* (receiver-less, 양 controller 활용)
// 로 추출 예정.

// dialValkey — dial_helpers.go 의 dialClusterRefTarget thin wrapper.
func (r *ValkeyRestoreReconciler) dialValkey(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (*redis.Client, error) {
	return dialClusterRefTarget(ctx, r.Client, rest.Spec.ClusterRef, rest.Namespace)
}

// verifyDataPlane — INFO keyspace 호출 (non-blocking). 실패는 warn log,
// restore 자체 성공 보장. Status.RestoredKeys 채움.
func (r *ValkeyRestoreReconciler) verifyDataPlane(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyRestore/VerifyDataPlane")
	defer span.End()

	logger := logf.FromContext(ctx)
	c, err := r.dialValkey(ctx, rest)
	if err != nil {
		logger.V(1).Info("dial valkey failed — non-blocking", "err", err)
		return
	}
	defer func() { _ = c.Close() }()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	keys, err := vk.CountKeyspaceKeys(pingCtx, c)
	if err != nil {
		logger.V(1).Info("INFO keyspace failed — non-blocking", "err", err)
		return
	}
	rest.Status.RestoredKeys = keys
	logger.Info("Restore data plane verified", "keys", keys, "name", rest.Name)
}

func (r *ValkeyRestoreReconciler) detectRestorePodFailure(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (string, bool, error) {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods,
		client.InNamespace(rest.Namespace),
		client.MatchingLabels(resources.SelectorLabels(rest.Spec.ClusterRef.Name)),
	); err != nil {
		return "", false, err
	}
	for _, pod := range pods.Items {
		if !pod.DeletionTimestamp.IsZero() {
			continue
		}
		if pod.Status.Phase == corev1.PodFailed {
			return fmt.Sprintf("Pod %s failed during restore", pod.Name), true, nil
		}
		if msg, ok := failedContainerStatus(pod.Name, pod.Status.InitContainerStatuses); ok {
			return msg, true, nil
		}
		if msg, ok := failedContainerStatus(pod.Name, pod.Status.ContainerStatuses); ok {
			return msg, true, nil
		}
	}
	return "", false, nil
}

func failedContainerStatus(podName string, statuses []corev1.ContainerStatus) (string, bool) {
	for _, status := range statuses {
		if status.State.Waiting != nil && restoreFailureWaitingReason(status.State.Waiting.Reason) {
			return fmt.Sprintf("Pod %s container %s waiting %s during restore: %s",
				podName, status.Name, status.State.Waiting.Reason, status.State.Waiting.Message), true
		}
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			return fmt.Sprintf("Pod %s container %s terminated with exitCode=%d during restore: %s",
				podName, status.Name, status.State.Terminated.ExitCode, status.State.Terminated.Message), true
		}
	}
	return "", false
}

func restoreFailureWaitingReason(reason string) bool {
	switch reason {
	case "CrashLoopBackOff",
		"CreateContainerConfigError",
		"CreateContainerError",
		"ErrImagePull",
		"ImagePullBackOff",
		"InvalidImageName",
		"RunContainerError":
		return true
	default:
		return false
	}
}

// handleRestoring — STS 에 init container inject + 모든 pod Ready 대기 → Verifying.
func (r *ValkeyRestoreReconciler) handleRestoring(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyRestore/Restoring")
	defer span.End()

	logger := logf.FromContext(ctx)

	// STS 이름 — Valkey controller 가 spec.clusterRef.name 그대로 STS 이름 사용.
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, sts); err != nil {
		if errors.IsNotFound(err) {
			return r.markFailed(ctx, rest, "STSNotFound",
				fmt.Sprintf("StatefulSet/%s/%s not found", rest.Namespace, rest.Spec.ClusterRef.Name))
		}
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	srcPath := sourceRDBPath(rest)
	srcPVC := sourcePVCName(rest)

	// 멱등 inject — Kind 별 분기.
	hadRestoreContainer := false
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		if c.Name == resources.RestoreInitContainerName {
			hadRestoreContainer = true
			break
		}
	}
	if rest.Spec.ClusterRef.Kind == "ValkeyCluster" {
		shards, err := r.shardCountForTarget(ctx, rest)
		if err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		var layout map[int]string
		if rest.Spec.Source.PVC != nil {
			layout = parseShardLayout(rest.Spec.Source.PVC.ShardLayout)
		}
		resources.InjectRestoreIntoPodSpecForCluster(
			&sts.Spec.Template.Spec, shards, layout, srcPVC)
	} else {
		resources.InjectRestoreIntoPodSpec(&sts.Spec.Template.Spec, srcPath, srcPVC)
	}
	if !hadRestoreContainer {
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		logger.Info("STS patched with restore init container", "sts", sts.Name)
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	// 모든 pod Ready 인가? — Restoring 중 rolling 진행 중일 수 있음.
	if sts.Status.ReadyReplicas < sts.Status.Replicas || sts.Status.Replicas == 0 {
		msg, failed, err := r.detectRestorePodFailure(ctx, rest)
		if err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		if failed {
			return r.markFailed(ctx, rest, "RestorePodFailed", msg)
		}
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	// → Verifying.
	rest.Status.Phase = cachev1alpha1.RestorePhaseVerifying
	rest.Status.Message = fmt.Sprintf("STS pods Ready (%d/%d) — verifying",
		sts.Status.ReadyReplicas, sts.Status.Replicas)
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Verifying",
		Message:            rest.Status.Message,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

// handleVerifying — STS 원복 + paused annotation 제거 → Completed.
//
// 본 commit 은 데이터 plane 검증 (PING + INFO keyspace) 미구현 — 단순
// "STS pods Ready" 통과 시 Completed. 데이터 plane 검증은 별개 commit.
func (r *ValkeyRestoreReconciler) handleVerifying(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyRestore/Verifying")
	defer span.End()

	logger := logf.FromContext(ctx)

	// STS 원복 (init container + source volume 제거).
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, sts); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	hadRestore := false
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		if c.Name == resources.RestoreInitContainerName {
			hadRestore = true
			break
		}
	}
	if hadRestore {
		resources.RemoveRestoreFromPodSpec(&sts.Spec.Template.Spec)
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		logger.Info("STS init container removed — second rolling triggered", "sts", sts.Name)
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	// 두 번째 rolling 도 완료 대기.
	if sts.Status.ReadyReplicas < sts.Status.Replicas || sts.Status.Replicas == 0 {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	if err := r.unpauseRestoreTarget(ctx, rest); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	// 데이터 plane 검증 — INFO keyspace 호출 (non-blocking).
	// 실패해도 restore 자체는 성공 — RestoredKeys 미채움 만.
	r.verifyDataPlane(ctx, rest)

	// → Completed.
	MetricRestoreTotal.WithLabelValues(rest.Namespace, rest.Name, "Completed").Inc()
	if r.Recorder != nil {
		r.Recorder.Event(rest, "Normal", "Completed", "ValkeyRestore completed")
	}
	now := metav1.Now()
	rest.Status.Phase = cachev1alpha1.RestorePhaseCompleted
	rest.Status.CompletedAt = &now
	rest.Status.Message = "Restore completed — STS reverted, paused removed"
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Completed",
		Message:            rest.Status.Message,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	logger.Info("Restore Completed", "name", rest.Name)
	return ctrl.Result{}, nil
}

// handleDeletion — finalizer cleanup. STS 원복 + paused annotation 제거.
// 정상 Completed 흐름에서는 이미 정리됨 — no-op.
func (r *ValkeyRestoreReconciler) handleDeletion(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// STS 원복 시도 (best-effort).
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      rest.Spec.ClusterRef.Name,
		Namespace: rest.Namespace,
	}, sts); err == nil {
		hadRestore := false
		for _, c := range sts.Spec.Template.Spec.InitContainers {
			if c.Name == resources.RestoreInitContainerName {
				hadRestore = true
				break
			}
		}
		if hadRestore {
			resources.RemoveRestoreFromPodSpec(&sts.Spec.Template.Spec)
			_ = r.Update(ctx, sts)
		}
	}

	// paused annotation 제거 (best-effort).
	_ = r.unpauseRestoreTarget(ctx, rest)

	controllerutil.RemoveFinalizer(rest, finalizerValkeyRestore)
	if err := r.Update(ctx, rest); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("ValkeyRestore deleted — STS reverted + paused removed (best-effort)", "name", rest.Name)
	return ctrl.Result{}, nil
}

// markFailed — Phase=Failed + Reason + Message 기록 후 종료.
// markFailed — Phase=Failed 전이 + metric 증가.
func (r *ValkeyRestoreReconciler) markFailed(
	ctx context.Context, rest *cachev1alpha1.ValkeyRestore,
	reason, msg string,
) (ctrl.Result, error) {
	MetricRestoreTotal.WithLabelValues(rest.Namespace, rest.Name, "Failed").Inc()
	if r.Recorder != nil {
		r.Recorder.Event(rest, "Warning", reason, msg)
	}
	now := metav1.Now()
	rest.Status.Phase = cachev1alpha1.RestorePhaseFailed
	rest.Status.CompletedAt = &now
	rest.Status.Message = msg
	setCondition(rest.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: rest.Generation,
	})
	if err := updateStatusWithRetry(ctx, r.Client, rest); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager — manager 에 등록.
func (r *ValkeyRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//nolint:staticcheck // SA1019: ADR-0002 — events API migration deferred until controller-runtime 의 GetEventRecorder API 가 stable.
	r.Recorder = mgr.GetEventRecorderFor("valkeyrestore-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.ValkeyRestore{}).
		Owns(&appsv1.StatefulSet{}).
		Named("valkeyrestore").
		Complete(r)
}
