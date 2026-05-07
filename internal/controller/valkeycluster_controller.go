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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/observability"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

const finalizerValkeyCluster = cachev1alpha1.FinalizerValkeyCluster

// ValkeyClusterReconciler reconciles a ValkeyCluster object (Cluster mode, 16384 slot).
type ValkeyClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeyclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete

func (r *ValkeyClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := observability.StartReconcileSpan(ctx, "ValkeyCluster", req.Namespace, req.Name)
	defer span.End()

	logger := log.FromContext(ctx)
	MetricReconcileTotal.WithLabelValues(req.Namespace, req.Name).Inc()

	vc := &cachev1alpha1.ValkeyCluster{}
	if err := r.Get(ctx, req.NamespacedName, vc); err != nil {
		if errors.IsNotFound(err) {
			DeleteMetricsFor(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 1. Finalizer / deletion. STS owner-ref 가 자식 GC 를 처리하지만 cluster 멤버십은
	//    각 valkey 노드 내부 nodes.conf 에 잔존 — 다음 동명 cluster 생성 시 stale 멤버
	//    참조 가능. 따라서 best-effort CLUSTER FORGET 시퀀스 발행.
	if !vc.DeletionTimestamp.IsZero() {
		return handleFinalizerCleanup(ctx, r.Client, vc, finalizerValkeyCluster,
			func(fctx context.Context) error {
				return r.gracefulClusterTeardown(fctx, vc)
			})
	}
	if !controllerutil.ContainsFinalizer(vc, finalizerValkeyCluster) {
		controllerutil.AddFinalizer(vc, finalizerValkeyCluster)
		if err := r.Update(ctx, vc); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 1b. Paused — ValkeyRestore (ADR-0015) 가 STS 를 직접 patch 중일 때
	//     본 controller 의 reconcile 가 init container 를 제거하지 않도록.
	if isPaused(vc) {
		logger.V(1).Info("paused — skipping reconcile (cache.keiailab.io/paused=true)",
			"name", vc.Name)
		return ctrl.Result{RequeueAfter: requeueSteady}, nil
	}

	// 2. Defaulting (CRD default 미커버 영역).
	r.applyDefaults(vc)

	// 3. Auth Secret 보장.
	password, secretRef, err := r.ensureAuthSecret(ctx, vc)
	if err != nil {
		return applyErrorCondition(ctx, r.Client, vc, "AuthSecret", err, r.Recorder)
	}

	// 4. ConfigMap (cluster-enabled yes).
	cm, err := resources.BuildConfigMapForValkeyCluster(vc, password)
	if err != nil {
		return applyErrorCondition(ctx, r.Client, vc, "ConfigMap", err, r.Recorder)
	}
	if err := applyConfigMap(ctx, r.Client, r.Scheme, vc, cm); err != nil {
		return applyErrorCondition(ctx, r.Client, vc, "ConfigMap", err, r.Recorder)
	}

	// 5. Headless + Client Service. Headless 는 cluster-bus(16379) 포트 추가.
	hs := resources.BuildHeadlessService(vc.Name, vc.Namespace, true)
	if err := applyService(ctx, r.Client, r.Scheme, vc, hs); err != nil {
		return applyErrorCondition(ctx, r.Client, vc, "HeadlessService", err, r.Recorder)
	}
	cs := resources.BuildClientService(vc.Name, vc.Namespace)
	if err := applyService(ctx, r.Client, r.Scheme, vc, cs); err != nil {
		return applyErrorCondition(ctx, r.Client, vc, "ClientService", err, r.Recorder)
	}
	// Monitoring 활성 시 metrics Service + ServiceMonitor (Prometheus Operator CRD).
	if vc.Spec.Monitoring != nil && vc.Spec.Monitoring.Enabled {
		ms := resources.BuildMetricsService(vc.Name, vc.Namespace)
		if err := applyService(ctx, r.Client, r.Scheme, vc, ms); err != nil {
			return applyErrorCondition(ctx, r.Client, vc, "MetricsService", err, r.Recorder)
		}
		if sm := resources.BuildServiceMonitorForCluster(vc); sm != nil {
			if err := applyServiceMonitor(ctx, r.Client, r.Scheme, vc, sm); err != nil {
				return applyErrorCondition(ctx, r.Client, vc, "ServiceMonitor", err, r.Recorder)
			}
		}
	}

	// TLS + cert-manager 통합: CertManager 명시 시 Certificate CR 자동 생성.
	// cert-manager 가 secretName 의 Secret 을 만들어 ca.crt 를 채움 → tlsConfigForCluster
	// 가 자동으로 RootCAs 로 로드 (ADR-0010).
	if cert := resources.BuildCertificateForCluster(vc); cert != nil {
		if err := applyServiceMonitor(ctx, r.Client, r.Scheme, vc, cert); err != nil {
			// CRD 미설치 시 fail-soft (applyServiceMonitor 가 NoMatchError 흡수).
			return applyErrorCondition(ctx, r.Client, vc, "Certificate", err, r.Recorder)
		}
	}

	// 6. StatefulSet (replicas = shards*(1+replicasPerShard)).
	//    Scale 가드: Spec.ScalePolicy.Deliberate=false 일 때 *기존 STS 의 replicas 와
	//    desired 가 다르면 Status.PendingScale 에 기록하고 STS replicas 변경 금지*
	//    (preserveReplicas=true). 사용자가 Deliberate=true 로 변경하면 다음 reconcile
	//    에서 적용.
	totalReplicas := vc.Spec.TotalNodes()
	preserveReplicas, pendingScale, err := r.evaluateScalePolicy(ctx, vc, totalReplicas)
	if err != nil {
		return applyErrorCondition(ctx, r.Client, vc, "ScalePolicy", err, r.Recorder)
	}
	vc.Status.PendingScale = pendingScale

	stsParams := resources.STSParams{
		CRName:       vc.Name,
		Namespace:    vc.Namespace,
		Replicas:     totalReplicas,
		Image:        imageOrDefault(vc.Spec.Version),
		PullPolicy:   vc.Spec.Version.ImagePullPolicy,
		Resources:    buildResourceReq(vc.Spec.Resources),
		StorageClass: vc.Spec.Storage.StorageClassName,
		StorageSize:  vc.Spec.Storage.Size,
		PasswordRef:  secretRef,
		ClusterMode:  true,
		Pod:          vc.Spec.Pod,
	}
	if vc.Spec.TLS != nil && vc.Spec.TLS.Enabled {
		// CertManager 와 CustomCert 둘 다 동일 secret 마운트 — webhook 이 둘 중 하나만
		// 활성 보장 (validateClusterSpec). 우선순위: CustomCert > CertManager.
		switch {
		case vc.Spec.TLS.CustomCert != nil && vc.Spec.TLS.CustomCert.SecretName != "":
			stsParams.TLSSecretName = vc.Spec.TLS.CustomCert.SecretName
		case vc.Spec.TLS.CertManager != nil && vc.Spec.TLS.CertManager.IssuerRef.Name != "":
			stsParams.TLSSecretName = resources.CertificateSecretName(vc.Name)
		}
	}
	if vc.Spec.Monitoring != nil && vc.Spec.Monitoring.Enabled {
		stsParams.ExporterImg = exporterImage(vc.Spec.Monitoring)
	}
	sts := resources.BuildStatefulSet(stsParams)
	if err := applyStatefulSet(ctx, r.Client, r.Scheme, vc, sts, preserveReplicas); err != nil {
		return applyErrorCondition(ctx, r.Client, vc, "StatefulSet", err, r.Recorder)
	}

	// 7. PDB / NetworkPolicy (opt-in).
	if vc.Spec.PodDisruptionBudget != nil && vc.Spec.PodDisruptionBudget.Enabled {
		pdb := resources.BuildPDB(vc.Name, vc.Namespace, totalReplicas, vc.Spec.PodDisruptionBudget)
		if err := applyPDB(ctx, r.Client, r.Scheme, vc, pdb); err != nil {
			return applyErrorCondition(ctx, r.Client, vc, "PDB", err, r.Recorder)
		}
	}
	if vc.Spec.NetworkPolicy != nil && vc.Spec.NetworkPolicy.Enabled {
		np := resources.BuildNetworkPolicy(vc.Name, vc.Namespace, true, vc.Spec.NetworkPolicy)
		if err := applyNetworkPolicy(ctx, r.Client, r.Scheme, vc, np); err != nil {
			return applyErrorCondition(ctx, r.Client, vc, "NetworkPolicy", err, r.Recorder)
		}
	}

	// 8. STS 상태 폴링.
	stsKey := types.NamespacedName{Name: resources.StatefulSetName(vc.Name), Namespace: vc.Namespace}
	stsObj, err := r.fetchSTS(ctx, stsKey)
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	// 9. CLUSTER MEET + ADDSLOTS + REPLICATE — 모든 pod 가 Ready 되었을 때 1회 호출.
	//    멱등성: ClusterInitialized 플래그 + 사전 QueryClusterInfo 로 이중 가드.
	allReady := stsObj.readyReplicas == totalReplicas && totalReplicas > 0
	if allReady && !vc.Status.ClusterInitialized {
		if err := r.ensureClusterMeet(ctx, vc, password); err != nil {
			logger.Error(err, "Cluster bootstrap pending — will retry")
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		vc.Status.ClusterInitialized = true
	}

	// 10. Cluster 상태 폴링 (Ready 후에만 의미 있음).
	var info *vk.ClusterInfo
	var nodes []vk.NodeView
	if allReady {
		info, nodes, err = r.pollClusterState(ctx, vc, password)
		if err != nil {
			logger.Error(err, "Cluster info query failed across all nodes")
		}
	}

	// 11. Shard status 빌드 + metrics 갱신.
	//     ADR-0004 후속: NODES 응답이 있으면 *실제 토폴로지* 기반 — failover 정확.
	//     없으면 spec 기반 fallback (부트스트랩 직후 / NODES 조회 실패 시).
	ns, name := vc.Namespace, vc.Name
	MetricReadyReplicas.WithLabelValues(ns, name).Set(float64(stsObj.readyReplicas))
	if info != nil && info.State == "ok" {
		if len(nodes) > 0 {
			vc.Status.Shards = buildShardStatusFromNodes(nodes, r.buildPodAddrMap(ctx, vc))
		} else {
			vc.Status.Shards = buildShardStatus(vc)
		}
		vc.Status.AssignedSlots = info.SlotsAssigned
		vc.Status.ClusterState = info.State
		MetricClusterStateOK.WithLabelValues(ns, name).Set(1)
		MetricClusterAssignedSlots.WithLabelValues(ns, name).Set(float64(info.SlotsAssigned))
		MetricClusterShards.WithLabelValues(ns, name).Set(float64(info.Size))
	} else if info != nil {
		vc.Status.ClusterState = info.State
		MetricClusterStateOK.WithLabelValues(ns, name).Set(0)
		MetricClusterAssignedSlots.WithLabelValues(ns, name).Set(float64(info.SlotsAssigned))
	} else {
		MetricClusterStateOK.WithLabelValues(ns, name).Set(0)
	}

	// 12. Status (general).
	vc.Status.ObservedGeneration = vc.Generation
	vc.Status.ReadyReplicas = stsObj.readyReplicas
	vc.Status.Version = vc.Spec.Version.Version
	vc.Status.Endpoint = fmt.Sprintf("%s.%s.svc:%d",
		resources.ClientServiceName(vc.Name), vc.Namespace, resources.PortClient)

	// 13. Phase 결정.
	vc.Status.Phase = decidePhase(vc, stsObj.readyReplicas, totalReplicas, info)

	conds := vc.GetConditions()
	applyClusterConditions(conds, vc, info, totalReplicas, stsObj.readyReplicas)
	SetPhaseMetric(ns, name, string(vc.Status.Phase))

	// 14. Status update + requeue.
	if err := updateStatusWithRetry(ctx, r.Client, vc); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	switch vc.Status.Phase {
	case cachev1alpha1.ClusterPhaseRunning:
		return ctrl.Result{RequeueAfter: requeueSteady}, nil
	case cachev1alpha1.ClusterPhaseResharding:
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	default:
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
}

// evaluateScalePolicy — Shards/ReplicasPerShard 변경 vs 현재 STS replicas 비교.
//
// 반환:
//   - preserveReplicas: true 이면 STS replicas 변경 안 함 (현재 값 유지).
//   - pendingScale: nil 또는 변경 의도 기록 (사용자에게 deliberate=true 요청 메시지).
//
// 결정 규칙:
//   - STS 미존재 (초기 부트스트랩): preserve=false, pending=nil → 정상 desired 적용.
//   - 현재 == desired: preserve=false (변경 없음), pending=nil.
//   - 현재 != desired & ScalePolicy.Deliberate=true: preserve=false, pending=nil → 즉시 적용.
//   - 현재 != desired & Deliberate=false (기본): preserve=true, pending 기록 → 변경 보류.
//
// Deliberate=false 가 *기본값* — Valkey cluster topology 변경은 16384 slot 재분배
// 트래픽을 동반하므로 사용자 명시 동의가 안전 (ADR-0006 예정).
func (r *ValkeyClusterReconciler) evaluateScalePolicy(
	ctx context.Context, vc *cachev1alpha1.ValkeyCluster, desired int32,
) (preserveReplicas bool, pendingScale *cachev1alpha1.PendingScale, err error) {
	stsKey := types.NamespacedName{Name: resources.StatefulSetName(vc.Name), Namespace: vc.Namespace}
	sts := &appsv1.StatefulSet{}
	if getErr := r.Get(ctx, stsKey, sts); getErr != nil {
		if errors.IsNotFound(getErr) {
			return false, nil, nil // 초기 부트스트랩.
		}
		return false, nil, getErr
	}
	if sts.Spec.Replicas == nil {
		return false, nil, nil
	}
	current := *sts.Spec.Replicas
	if current == desired {
		return false, nil, nil
	}
	deliberate := vc.Spec.ScalePolicy != nil && vc.Spec.ScalePolicy.Deliberate
	if deliberate {
		return false, nil, nil
	}
	// 변경 의도 기록 + STS 보존.
	pending := &cachev1alpha1.PendingScale{
		CurrentReplicas: current,
		DesiredReplicas: desired,
		RequestedAt:     metav1.Now().Format("2006-01-02T15:04:05Z07:00"),
		Reason: fmt.Sprintf(
			"Cluster topology change deferred: set Spec.ScalePolicy.Deliberate=true to apply (%d → %d)",
			current, desired),
	}
	return true, pending, nil
}

// applyClusterConditions — Reconcile 결과로부터 5개 표준 condition 갱신.
//
// CondTypeReady (legacy 호환): Phase 미러 — Phase != Failed 면 True.
// CondTypeClusterReady: cluster_state=ok && slots=16384.
// CondTypeCertReady: TLS 비활성 → True / TLS 활성 + RootCAs 로드 성공 → True / 진행중 → Unknown.
// CondTypeScalePending: Status.PendingScale != nil 시 True (보류 중).
// CondTypeUpgradeInProgress: Phase=Upgrading 시 True.
func applyClusterConditions(
	conds *[]metav1.Condition,
	vc *cachev1alpha1.ValkeyCluster,
	info *vk.ClusterInfo,
	totalReplicas, readyReplicas int32,
) {
	// Ready (종합).
	readyStatus := metav1.ConditionTrue
	readyReason := string(vc.Status.Phase)
	if vc.Status.Phase == cachev1alpha1.ClusterPhaseFailed {
		readyStatus = metav1.ConditionFalse
	}
	setCondition(conds, metav1.Condition{
		Type:               CondTypeReady,
		Status:             readyStatus,
		Reason:             readyReason,
		ObservedGeneration: vc.Generation,
	})

	// ClusterReady — cluster_state=ok && 16384 slot.
	clusterReady := info != nil && info.State == "ok" && info.SlotsAssigned == 16384
	clusterReason := "ClusterStateOK"
	clusterMsg := ""
	if !clusterReady {
		clusterReason = "ClusterNotConverged"
		if info == nil {
			clusterMsg = "Cluster info not yet polled"
		} else {
			clusterMsg = fmt.Sprintf("state=%s slots_assigned=%d ready_replicas=%d/%d",
				info.State, info.SlotsAssigned, readyReplicas, totalReplicas)
		}
	}
	setCondition(conds, metav1.Condition{
		Type:               CondTypeClusterReady,
		Status:             boolToConditionStatus(clusterReady),
		Reason:             clusterReason,
		Message:            clusterMsg,
		ObservedGeneration: vc.Generation,
	})

	// ScalePending.
	scalePending := vc.Status.PendingScale != nil
	scaleReason := "NoChange"
	scaleMsg := ""
	if scalePending {
		scaleReason = "DeliberateRequired"
		scaleMsg = vc.Status.PendingScale.Reason
	}
	setCondition(conds, metav1.Condition{
		Type:               CondTypeScalePending,
		Status:             boolToConditionStatus(scalePending),
		Reason:             scaleReason,
		Message:            scaleMsg,
		ObservedGeneration: vc.Generation,
	})

	// UpgradeInProgress.
	upgrading := vc.Status.Phase == cachev1alpha1.ClusterPhaseUpgrading
	upgradeReason := "NoUpgrade"
	upgradeMsg := ""
	if upgrading {
		upgradeReason = "VersionTransition"
		upgradeMsg = fmt.Sprintf("Rolling upgrade %s → %s",
			vc.Status.Version, vc.Spec.Version.Version)
	}
	setCondition(conds, metav1.Condition{
		Type:               CondTypeUpgradeInProgress,
		Status:             boolToConditionStatus(upgrading),
		Reason:             upgradeReason,
		Message:            upgradeMsg,
		ObservedGeneration: vc.Generation,
	})

	// CertReady — TLS 비활성 시 자동 True.
	certReady := vc.Spec.TLS == nil || !vc.Spec.TLS.Enabled
	certReason := "TLSDisabled"
	certMsg := ""
	if !certReady {
		// TLS 활성 시 RootCAs 가 로드됐는지 확인 — Reconcile 가 tlsConfigForCluster 를
		// 호출해 InsecureSkipVerify 결과를 알 수 있지만 본 함수는 그 정보 없음. 따라서
		// 단순 시그널만: CertManager / CustomCert 명시 여부 + Status.Phase Running.
		hasCertManager := vc.Spec.TLS.CertManager != nil && vc.Spec.TLS.CertManager.IssuerRef.Name != ""
		hasCustom := vc.Spec.TLS.CustomCert != nil && vc.Spec.TLS.CustomCert.SecretName != ""
		if hasCertManager || hasCustom {
			certReady = true
			certReason = "CABundleConfigured"
			if hasCertManager {
				certMsg = "Issuer: " + vc.Spec.TLS.CertManager.IssuerRef.Name
			} else {
				certMsg = "CustomCert: " + vc.Spec.TLS.CustomCert.SecretName
			}
		} else {
			certReason = "FallbackInsecureSkipVerify"
			certMsg = "TLS enabled but no CA bundle configured (ADR-0003)"
		}
	}
	setCondition(conds, metav1.Condition{
		Type:               CondTypeCertReady,
		Status:             boolToConditionStatus(certReady),
		Reason:             certReason,
		Message:            certMsg,
		ObservedGeneration: vc.Generation,
	})
}

// decidePhase — STS / cluster info / spec.Version 비교로 phase 결정. 순수함수.
//
// 우선순위:
//   - Upgrading: Spec.Version != 기록된 Status.Version + 아직 모든 pod ready 아님 (rolling).
//   - Pending:   readyReplicas == 0 (초기 STS 부트스트랩 시점).
//   - Initializing: STS rolling 중 (일부 pod 만 ready) 또는 cluster_state != ok.
//   - Resharding: state=ok + slots != 16384 (slot 재분배 진행).
//   - Running:    모든 조건 충족.
func decidePhase(vc *cachev1alpha1.ValkeyCluster, readyReplicas, totalReplicas int32, info *vk.ClusterInfo) cachev1alpha1.ClusterPhase {
	allReady := readyReplicas == totalReplicas && totalReplicas > 0
	versionChanged := vc.Status.Version != "" && vc.Status.Version != vc.Spec.Version.Version

	switch {
	case versionChanged && !allReady:
		return cachev1alpha1.ClusterPhaseUpgrading
	case readyReplicas == 0:
		return cachev1alpha1.ClusterPhasePending
	case !allReady:
		return cachev1alpha1.ClusterPhaseInitializing
	case info == nil || info.State != "ok":
		return cachev1alpha1.ClusterPhaseInitializing
	case info.SlotsAssigned != 16384:
		return cachev1alpha1.ClusterPhaseResharding
	default:
		return cachev1alpha1.ClusterPhaseRunning
	}
}

// applyDefaults — operator 측 defaulting (CRD default 미커버 영역).
func (r *ValkeyClusterReconciler) applyDefaults(vc *cachev1alpha1.ValkeyCluster) {
	if vc.Spec.Shards == 0 {
		vc.Spec.Shards = 3
	}
	if vc.Spec.NodeTimeoutMillis == 0 {
		vc.Spec.NodeTimeoutMillis = 15000
	}
	if vc.Spec.Version.Version == "" {
		vc.Spec.Version.Version = cachev1alpha1.DefaultValkeyVersion
	}
	if vc.Spec.Version.Image == "" {
		vc.Spec.Version.Image = cachev1alpha1.DefaultValkeyImage
	}
}

// ensureAuthSecret — Valkey 컨트롤러와 동일 패턴: PasswordSecretRef 미지정 시 자동 생성.
func (r *ValkeyClusterReconciler) ensureAuthSecret(
	ctx context.Context, vc *cachev1alpha1.ValkeyCluster,
) (string, *corev1.SecretKeySelector, error) {
	if vc.Spec.Auth.PasswordSecretRef != nil {
		ref := vc.Spec.Auth.PasswordSecretRef
		s := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: vc.Namespace}, s); err != nil {
			return "", nil, fmt.Errorf("get user-provided secret: %w", err)
		}
		key := ref.Key
		if key == "" {
			key = resources.SecretPasswordKey
		}
		return string(s.Data[key]), ref, nil
	}

	secretName := resources.DefaultSecretName(vc.Name)
	existing := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: vc.Namespace}, existing); err == nil {
		return string(existing.Data[resources.SecretPasswordKey]),
			&corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  resources.SecretPasswordKey,
			},
			nil
	} else if !errors.IsNotFound(err) {
		return "", nil, err
	}

	password, err := resources.GeneratePassword()
	if err != nil {
		return "", nil, err
	}
	if err := reconcileSecretIfNotExists(ctx, r.Client, r.Scheme, vc, secretName, func() *corev1.Secret {
		return resources.BuildAuthSecret(vc.Name, vc.Namespace, password)
	}); err != nil {
		return "", nil, err
	}
	return password, &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
		Key:                  resources.SecretPasswordKey,
	}, nil
}

// fetchSTS — STS 의 readyReplicas / replicas 조회. ValkeyReconciler.fetchSTS 와 동일 동작.
func (r *ValkeyClusterReconciler) fetchSTS(ctx context.Context, key types.NamespacedName) (*stsStatus, error) {
	obj := &appsv1StatefulSet{}
	if err := r.Get(ctx, key, obj.Inner()); err != nil {
		if errors.IsNotFound(err) {
			return &stsStatus{}, nil
		}
		return nil, err
	}
	return &stsStatus{
		readyReplicas: obj.s.Status.ReadyReplicas,
		totalReplicas: obj.s.Status.Replicas,
	}, nil
}

// ensureClusterMeet — 모든 pod 가 Ready 일 때 CLUSTER MEET + ADDSLOTS + REPLICATE 1회 호출.
//
// pod ordinal 매핑: 0..shards-1 = primary, shards..total-1 = replica.
// CreateCluster 의 round-robin 배치와 일치하도록 addresses 를 같은 순서로 구성한다.
//
// 멱등성: 사전 QueryClusterInfo (multi-node fallback) 로 state=ok && slots=16384 인 경우
// skip — operator 재기동 / pod-0 일시 다운 시에도 정확히 동작.
func (r *ValkeyClusterReconciler) ensureClusterMeet(
	ctx context.Context, vc *cachev1alpha1.ValkeyCluster, password string,
) error {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyCluster/EnsureClusterMeet")
	defer span.End()

	addresses := podAddresses(vc)

	if info, _, err := r.queryAnyNode(ctx, vc, password); err == nil &&
		info != nil && info.State == "ok" && info.SlotsAssigned == 16384 {
		return nil
	}

	tlsCfg, err := r.tlsConfigForCluster(ctx, vc)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("tls config: %w", err)
	}
	dial := func(addr string) *redis.Client { return dialPod(addr, password, tlsCfg) }
	createCtx, createSpan := observability.StartCallSpan(ctx, "ValkeyCluster/CreateCluster")
	defer createSpan.End()
	if err := vk.CreateCluster(createCtx, dial, addresses, int(vc.Spec.Shards), int(vc.Spec.ReplicasPerShard)); err != nil {
		createSpan.RecordError(err)
		return err
	}
	return nil
}

// pollClusterState — 임의의 응답 가능한 노드에서 cluster state + nodes 조회.
// pod-0 SPOF 제거: 모든 노드를 순회하며 첫 성공 응답 사용.
//
// 반환: (info, nodes, err). nodes 는 NODES 조회 추가 실패 시 nil.
func (r *ValkeyClusterReconciler) pollClusterState(
	ctx context.Context, vc *cachev1alpha1.ValkeyCluster, password string,
) (*vk.ClusterInfo, []vk.NodeView, error) {
	return r.queryAnyNode(ctx, vc, password)
}

// gracefulClusterTeardown — 삭제 시점에 best-effort 로 cluster 멤버십을 정리.
//
// 실패 원인 다양 (이미 STS replicas=0, NetworkPolicy 차단, password Secret 삭제 등) —
// 본 함수는 *어떤 에러도 반환하지 않는다*. 핵심 원칙: "삭제는 막지 않는다" (force-tenant
// CLAUDE.md 정책과 동일). nodes.conf cleanup 이 안 되어도 PVC retention=Delete 라면
// 자연 정리, retention=Retain 이라면 사용자 책임.
//
// 시퀀스:
//  1. password 조회 (Secret 이미 삭제됐으면 skip).
//  2. 모든 노드의 CLUSTER MYID 수집 (timeout 5s).
//  3. 각 노드에서 *다른* 모든 노드 ID 에 대해 CLUSTER FORGET (timeout 5s).
//
// 타임아웃: 전체 30s 안에 완료. 그 이상은 STS 삭제로 진행.
func (r *ValkeyClusterReconciler) gracefulClusterTeardown(ctx context.Context, vc *cachev1alpha1.ValkeyCluster) error {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyCluster/GracefulTeardown")
	defer span.End()

	logger := log.FromContext(ctx)
	teardownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	password, _, err := r.ensureAuthSecret(teardownCtx, vc)
	if err != nil {
		logger.Info("Skipping graceful teardown — auth secret unavailable", "error", err.Error())
		return nil
	}
	tlsCfg, tlsErr := r.tlsConfigForCluster(teardownCtx, vc)
	if tlsErr != nil {
		logger.Info("Skipping graceful teardown — TLS config unavailable", "error", tlsErr.Error())
		return nil
	}
	addresses := podAddresses(vc)
	if len(addresses) == 0 {
		return nil
	}

	// 1) 각 노드의 ID 수집 — 도달 가능한 노드만.
	type idAddr struct {
		id, addr string
	}
	collected := make([]idAddr, 0, len(addresses))
	for _, addr := range addresses {
		c := dialPod(addr, password, tlsCfg)
		idCtx, idCancel := context.WithTimeout(teardownCtx, 5*time.Second)
		id, err := c.ClusterMyID(idCtx).Result()
		idCancel()
		_ = c.Close()
		if err == nil && id != "" {
			collected = append(collected, idAddr{id: id, addr: addr})
		}
	}
	if len(collected) < 2 {
		// 1 노드 이하 도달 가능 → forget 의미 없음.
		return nil
	}

	// 2) 각 노드에서 다른 모든 노드 ID 에 대해 forget.
	var forgottenCount int
	for _, src := range collected {
		c := dialPod(src.addr, password, tlsCfg)
		for _, other := range collected {
			if other.id == src.id {
				continue
			}
			fCtx, fCancel := context.WithTimeout(teardownCtx, 5*time.Second)
			err := vk.ForgetNode(fCtx, c, other.id)
			fCancel()
			if err == nil {
				forgottenCount++
			}
		}
		_ = c.Close()
	}
	logger.Info("Cluster graceful teardown completed",
		"reachable_nodes", len(collected),
		"forget_calls_succeeded", forgottenCount)
	return nil
}

// queryAnyNode — addresses 순회하며 첫 응답 노드의 ClusterInfo + NODES 반환.
// 모든 노드 실패 시 마지막 에러 반환.
//
// NODES 조회는 추가 시도 — info 만 받고 NODES 실패 시 (info, nil, nil) 반환.
func (r *ValkeyClusterReconciler) queryAnyNode(
	ctx context.Context, vc *cachev1alpha1.ValkeyCluster, password string,
) (*vk.ClusterInfo, []vk.NodeView, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyCluster/QueryAnyNode")
	defer span.End()

	addresses := podAddresses(vc)
	if len(addresses) == 0 {
		return nil, nil, fmt.Errorf("no pod addresses")
	}
	tlsCfg, tlsErr := r.tlsConfigForCluster(ctx, vc)
	if tlsErr != nil {
		span.RecordError(tlsErr)
		return nil, nil, fmt.Errorf("tls config: %w", tlsErr)
	}
	var lastErr error
	for _, addr := range addresses {
		c := dialPod(addr, password, tlsCfg)
		info, err := vk.QueryClusterInfo(ctx, c)
		if err != nil {
			_ = c.Close()
			lastErr = err
			continue
		}
		nodes, nErr := vk.QueryClusterNodes(ctx, c)
		_ = c.Close()
		if nErr != nil {
			return info, nil, nil
		}
		return info, nodes, nil
	}
	return nil, nil, fmt.Errorf("all %d nodes unreachable: %w", len(addresses), lastErr)
}

// podAddresses — STS pod ordinal 순서대로 host:port 목록 (primaries 먼저, replicas 다음).
//
// CreateCluster 의 가정 — addresses[:shards] = primaries — 와 align 되도록
// pod ordinal 0..shards-1 을 primary 로 둔다 (round-robin 분배).
func podAddresses(vc *cachev1alpha1.ValkeyCluster) []string {
	total := int(vc.Spec.TotalNodes())
	port := resources.PortClient
	// TLS 활성 시 plain 6379 대신 tls-port 6380 사용 — operator 가 TLS handshake 를
	// plain port 에 시도하면 server 가 즉시 close 하고 timeout. 본 차이는
	// `port` 와 `tls-port` 가 별도라는 Valkey 의 모델 (6379 평문 / 6380 TLS) 에서 비롯.
	if vc.Spec.TLS != nil && vc.Spec.TLS.Enabled {
		port = resources.PortTLS
	}
	out := make([]string, 0, total)
	for i := 0; i < total; i++ {
		out = append(out, fmt.Sprintf("%s:%d",
			resources.PodFQDN(vc.Name, i, vc.Namespace), port))
	}
	return out
}

// dialPod — password + TLS 주입 redis client. tlsCfg 가 nil 이면 평문 접속.
func dialPod(addr, password string, tlsCfg *tls.Config) *redis.Client {
	opts := vk.DialOptions{Address: addr, Password: password}
	if tlsCfg != nil {
		opts.UseTLS = true
		opts.TLSConf = tlsCfg
	}
	return vk.NewSingleClient(opts)
}

// tlsConfigForCluster — Spec.TLS.Enabled 시 다음 우선순위로 RootCAs 구성:
//
//  1. Spec.TLS.CustomCert.SecretName 의 ca.crt → x509 cert pool.
//  2. Spec.TLS.CertManager 의 issuer 가 만들어둔 Secret 의 ca.crt — 본 함수는 직접 추적
//     안 함 (cert-manager 가 만든 Secret 이름을 사용자가 CustomCert 로 명시해야 함 또는
//     ADR-0003 후속 PR 에서 Issuer status 추적).
//  3. 둘 다 미제공 → InsecureSkipVerify (fallback, 경고 로그).
//
// 미활성 시 nil 반환 → 평문 접속.
func (r *ValkeyClusterReconciler) tlsConfigForCluster(ctx context.Context, vc *cachev1alpha1.ValkeyCluster) (*tls.Config, error) {
	if vc.Spec.TLS == nil || !vc.Spec.TLS.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// cert-manager 가 발급하는 SAN 에 fully-qualified DNS 만 들어가므로 (예
		// `vc-tls-headless.default.svc`, `*.vc-tls-headless.default.svc`),
		// short name 만 ServerName 으로 넘기면 verification 실패.
		ServerName: resources.HeadlessServiceName(vc.Name) + "." + vc.Namespace + ".svc",
	}

	// CustomCert 가 명시되면 해당 Secret 의 ca.crt + tls.{crt,key} 로드 (mTLS).
	if vc.Spec.TLS.CustomCert != nil && vc.Spec.TLS.CustomCert.SecretName != "" {
		secretName := vc.Spec.TLS.CustomCert.SecretName
		pool, err := r.loadCABundle(ctx, vc.Namespace, secretName)
		if err != nil {
			return nil, fmt.Errorf("load ca bundle: %w", err)
		}
		if pool != nil {
			cfg.RootCAs = pool
			if cert, err := r.loadClientCert(ctx, vc.Namespace, secretName); err == nil && cert != nil {
				cfg.Certificates = []tls.Certificate{*cert}
			}
			return cfg, nil
		}
	}

	// CertManager 명시 시 우리가 만든 Certificate CR 의 secretName 추적 (ADR-0010).
	if vc.Spec.TLS.CertManager != nil && vc.Spec.TLS.CertManager.IssuerRef.Name != "" {
		secretName := resources.CertificateSecretName(vc.Name)
		pool, err := r.loadCABundle(ctx, vc.Namespace, secretName)
		if err != nil {
			return nil, fmt.Errorf("load cert-manager ca bundle: %w", err)
		}
		if pool != nil {
			cfg.RootCAs = pool
			if cert, err := r.loadClientCert(ctx, vc.Namespace, secretName); err == nil && cert != nil {
				cfg.Certificates = []tls.Certificate{*cert}
			}
			return cfg, nil
		}
		// Certificate 가 아직 ready 가 아닌 경우 (cert-manager 가 Secret 생성 중) →
		// fallback. 다음 reconcile 에서 자동 회복.
	}

	// CA bundle 미발견 → InsecureSkipVerify fallback + warning.
	cfg.InsecureSkipVerify = true //nolint:gosec // ADR-0003: CA bundle 미준비 시 fallback
	log.FromContext(ctx).Info(
		"TLS enabled without CA bundle — using InsecureSkipVerify fallback",
		"cluster", vc.Name)
	return cfg, nil
}

// loadCABundle — Secret 의 ca.crt 를 x509 cert pool 에 로드.
// Secret 미존재 시 (nil, nil), key 누락 시 (nil, nil), 파싱 실패 시 (nil, err).
func (r *ValkeyClusterReconciler) loadCABundle(ctx context.Context, namespace, secretName string) (*x509.CertPool, error) {
	s := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, s); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	caBytes, ok := s.Data["ca.crt"]
	if !ok || len(caBytes) == 0 {
		return nil, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("invalid PEM in %s/%s/ca.crt", namespace, secretName)
	}
	return pool, nil
}

// loadClientCert — Valkey 의 `tls-auth-clients yes` 가 mTLS 를 강제하므로 operator
// 는 동일 Secret 의 tls.crt + tls.key 를 client cert 로 제시해야 한다.
// (CustomCert / CertManager 가 만든 Secret 에 둘 다 있는 일반 형식 사용).
func (r *ValkeyClusterReconciler) loadClientCert(ctx context.Context, namespace, secretName string) (*tls.Certificate, error) {
	s := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, s); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	crt, hasCrt := s.Data["tls.crt"]
	key, hasKey := s.Data["tls.key"]
	if !hasCrt || !hasKey || len(crt) == 0 || len(key) == 0 {
		return nil, nil
	}
	cert, err := tls.X509KeyPair(crt, key)
	if err != nil {
		return nil, fmt.Errorf("invalid keypair in %s/%s: %w", namespace, secretName, err)
	}
	return &cert, nil
}

// buildShardStatusFromNodes — CLUSTER NODES 응답 기반 ShardStatus.
//
// ADR-0004 후속: 기존 spec 기반 `buildShardStatus` 는 failover 시점 거짓말을 했음.
// 본 함수는 *실제* cluster 토폴로지를 반영 — replica 가 primary 로 승격된 경우에도
// 정확히 보고한다.
//
// 매핑:
//   - 각 primary 노드 → ShardStatus 1건. Index 는 slot 범위 시작값으로 정렬한 순서.
//   - replica 노드의 MasterID 로 primary 매핑 → ReplicaPods 채움.
//   - SlotRange 는 NodeView.Slots 의 [Start, End] 를 "low-high" 형식으로 직렬화.
//
// addrToPod: "10.0.0.1:6379" → "vk-0" 매핑 함수. nil 일 때는 IP:Port 그대로 사용.
// Reconcile 에서 K8s Pod list 로 채움.
func buildShardStatusFromNodes(nodes []vk.NodeView, addrToPod func(string) string) []cachev1alpha1.ShardStatus {
	primaries := make([]vk.NodeView, 0)
	replicas := make([]vk.NodeView, 0)
	for _, n := range nodes {
		switch {
		case n.IsPrimary():
			primaries = append(primaries, n)
		case n.IsReplica():
			replicas = append(replicas, n)
		}
	}
	if len(primaries) == 0 {
		return nil
	}

	// primary 를 slot 시작점 기준 정렬 (안정적 Index 부여).
	sortPrimariesBySlotStart(primaries)

	idToReplicas := make(map[string][]string, len(primaries))
	for _, r := range replicas {
		idToReplicas[r.MasterID] = append(idToReplicas[r.MasterID], r.Addr)
	}

	out := make([]cachev1alpha1.ShardStatus, 0, len(primaries))
	for i, p := range primaries {
		var assigned int32
		ranges := make([]string, 0, len(p.Slots))
		for _, r := range p.Slots {
			// valkey 슬롯 범위는 [0, 16383] 도메인 — 차이가 int32 범위를 초과할
			// 수 없음. gosec G115 (int → int32) 회피를 위해 bound check 명시.
			if d := r.End - r.Start + 1; d > 0 && d <= 16384 {
				assigned += int32(d)
			}
			ranges = append(ranges, fmt.Sprintf("%d-%d", r.Start, r.End))
		}
		primaryPod := p.Addr
		if addrToPod != nil {
			if mapped := addrToPod(p.Addr); mapped != "" {
				primaryPod = mapped
			}
		}
		replicaPods := idToReplicas[p.ID]
		if addrToPod != nil {
			mapped := make([]string, 0, len(replicaPods))
			for _, rp := range replicaPods {
				if name := addrToPod(rp); name != "" {
					mapped = append(mapped, name)
				} else {
					mapped = append(mapped, rp)
				}
			}
			replicaPods = mapped
		}
		out = append(out, cachev1alpha1.ShardStatus{
			Index:         int32(i),
			PrimaryPod:    primaryPod,
			ReplicaPods:   replicaPods,
			SlotRange:     joinRanges(ranges),
			AssignedSlots: assigned,
		})
	}
	return out
}

// buildPodAddrMap — K8s Pod list 를 조회해 IP:Port → Pod 이름 매핑 생성.
// Pod IP 가 비어 있거나 namespace 매칭 실패 시 nil 반환 (caller 가 fallback 처리).
func (r *ValkeyClusterReconciler) buildPodAddrMap(ctx context.Context, vc *cachev1alpha1.ValkeyCluster) func(string) string {
	pods := &corev1.PodList{}
	selector := client.MatchingLabels(resources.SelectorLabels(vc.Name))
	if err := r.List(ctx, pods, client.InNamespace(vc.Namespace), selector); err != nil {
		return nil
	}
	addrToName := make(map[string]string, len(pods.Items))
	for _, p := range pods.Items {
		if p.Status.PodIP == "" {
			continue
		}
		addrToName[fmt.Sprintf("%s:%d", p.Status.PodIP, resources.PortClient)] = p.Name
	}
	return func(addr string) string { return addrToName[addr] }
}

// sortPrimariesBySlotStart — primary 의 첫 slot 범위 시작값 기준 오름차순 정렬.
// slot 미보유 primary 는 끝으로 (아직 할당 안 된 상태).
func sortPrimariesBySlotStart(p []vk.NodeView) {
	for i := 0; i < len(p); i++ {
		for j := i + 1; j < len(p); j++ {
			ai, aj := primaryFirstSlot(p[i]), primaryFirstSlot(p[j])
			if ai > aj {
				p[i], p[j] = p[j], p[i]
			}
		}
	}
}

func primaryFirstSlot(n vk.NodeView) int {
	if len(n.Slots) == 0 {
		return 1 << 30 // 미할당 → 마지막으로 정렬.
	}
	return n.Slots[0].Start
}

func joinRanges(rs []string) string {
	if len(rs) == 0 {
		return ""
	}
	if len(rs) == 1 {
		return rs[0]
	}
	out := rs[0]
	for _, r := range rs[1:] {
		out = out + "," + r
	}
	return out
}

// buildShardStatus — pod ordinal 기준 shard 매핑 (legacy / 부트스트랩 직후 fallback).
//
// CreateCluster 의 배치 규칙과 동일하게:
//   - primary i (pod ordinal i, i ∈ [0, shards))
//   - replica j (pod ordinal shards+j) 는 primary (j % shards) 의 replica
//
// slot range 도 CreateCluster 와 동일한 균등 분배 (마지막 shard 가 잔여 흡수).
func buildShardStatus(vc *cachev1alpha1.ValkeyCluster) []cachev1alpha1.ShardStatus {
	shards := int(vc.Spec.Shards)
	rps := int(vc.Spec.ReplicasPerShard)
	if shards == 0 {
		return nil
	}

	const totalSlots = 16384
	per := totalSlots / shards

	out := make([]cachev1alpha1.ShardStatus, shards)
	idx := 0
	for i := 0; i < shards; i++ {
		end := idx + per - 1
		if i == shards-1 {
			end = totalSlots - 1
		}
		st := cachev1alpha1.ShardStatus{
			Index:         int32(i),
			PrimaryPod:    fmt.Sprintf("%s-%d", vc.Name, i),
			SlotRange:     fmt.Sprintf("%d-%d", idx, end),
			AssignedSlots: int32(end - idx + 1),
		}
		// replica j (j=0..rps-1) 가 primary (j*shards % shards == 0... primary i) 와 매핑되도록
		// CreateCluster 는 replicas[i] 를 primaries[i % shards] 에 붙임.
		// 즉 replica ordinal = shards + (k*shards + i) (k=0..rps-1).
		for k := 0; k < rps; k++ {
			repOrd := shards + k*shards + i
			st.ReplicaPods = append(st.ReplicaPods,
				fmt.Sprintf("%s-%d", vc.Name, repOrd))
		}
		out[i] = st
		idx = end + 1
	}
	return out
}

func (r *ValkeyClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// nolint:staticcheck // 새 events API 마이그레이션은 ADR-0002 (예정). sibling Valkey
	// 컨트롤러와 일관성 유지 — helpers.go:applyErrorCondition 시그니처 변경 동반 필요.
	r.Recorder = mgr.GetEventRecorderFor("valkeycluster-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.ValkeyCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Named("valkeycluster").
		Complete(r)
}
