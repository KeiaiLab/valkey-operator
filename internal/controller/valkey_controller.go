/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	commonsapply "github.com/keiailab/keiailab-commons/pkg/apply"
	commonsevents "github.com/keiailab/keiailab-commons/pkg/events"
	commonsfinalizer "github.com/keiailab/keiailab-commons/pkg/finalizer"
	commonspvc "github.com/keiailab/keiailab-commons/pkg/pvc"
	commonsreconcile "github.com/keiailab/keiailab-commons/pkg/reconcile"
	"github.com/keiailab/keiailab-commons/pkg/reconcilemetrics"
	commonsstatus "github.com/keiailab/keiailab-commons/pkg/status"
	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/observability"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

const (
	finalizerValkey = cachev1alpha1.FinalizerValkey
	// defaultImage — image:version 결합 형식 (image pull 폴백). image / version 분리는
	// cachev1alpha1.DefaultValkeyImage + DefaultValkeyVersion 참조.
	defaultImage = cachev1alpha1.DefaultValkeyImage + ":" + cachev1alpha1.DefaultValkeyVersion
)

// ValkeyReconciler reconciles a Valkey object (Standalone + Replication).
type ValkeyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeys/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile: 본 함수의 cyclomatic complexity 가 30을 초과하는 것은 의도적이다.
// reconcile 흐름은 *순차 단계* (fetch → finalizer → spec normalize → resource
// upsert × N → status aggregate) 로 구성되며, 각 단계는 *동일 함수 내 명시적
// switch* 가 가독성 좋다. 분해 시 호출 chain 추적 비용 증가 + envtest 회귀
// 위험. ADR-0030 AI-VK30-1 단계 3 정당화.
//
//nolint:gocyclo
func (r *ValkeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrlResult ctrl.Result, retErr error) {
	ctx, span := observability.StartReconcileSpan(ctx, "Valkey", req.Namespace, req.Name)
	defer span.End()

	// SLO histogram — wall-clock latency 관측 (commons ObserveReconcile).
	// result label: success|error.
	start := time.Now()
	crDeleted := false
	defer func() {
		if crDeleted {
			// CR 삭제 경로 — DeleteMetricsFor 이후 defer 가 시계열을 재생성하는
			// cardinality 누수 차단.
			return
		}
		reconMetrics.ObserveReconcile(req.Namespace, req.Name,
			reconcilemetrics.ResultFor(retErr), time.Since(start).Seconds())
	}()

	logger := log.FromContext(ctx)

	v := &cachev1alpha1.Valkey{}
	if err := r.Get(ctx, req.NamespacedName, v); err != nil {
		if errors.IsNotFound(err) {
			// CR 삭제 — reconcile trio + 도메인 시계열 제거 (누수 fix:
			// 기존엔 Valkey CR 삭제 시 어떤 시계열도 제거하지 않았다).
			crDeleted = true
			DeleteMetricsFor(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 1. Finalizer / deletion
	if !v.DeletionTimestamp.IsZero() {
		return commonsreconcile.HandleFinalizerCleanup(ctx, r.Client, v, finalizerValkey, nil)
	}
	if !commonsfinalizer.Has(v, finalizerValkey) {
		commonsfinalizer.Add(v, finalizerValkey)
		if err := r.Update(ctx, v); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 1b. Paused — ValkeyRestore (ADR-0015) 가 STS 를 직접 patch 중일 때
	//     본 controller 의 reconcile 가 init container 를 제거하지 않도록.
	if isPaused(v) {
		logger.V(1).Info("paused — skipping reconcile (cache.keiailab.io/paused=true)",
			"name", v.Name)
		return ctrl.Result{RequeueAfter: requeueSteady}, nil
	}

	// 2. Defaulting
	r.applyDefaults(v)

	// 3. Auth Secret 보장 (자동 생성 시 멱등)
	password, secretRef, err := r.ensureAuthSecret(ctx, v)
	if err != nil {
		return applyErrorCondition(ctx, r.Client, v, "AuthSecret", err, r.Recorder)
	}

	// 3b. 자체 시크릿 로테이션 (operator-managed, AuthSpec.RotationInterval).
	//     user-provided secret(PasswordSecretRef)은 외부 소유 — 회전 대상에서 제외.
	//     회전 시 password 를 재할당해 아래 ConfigMap + auth-secret-hash 에 반영(→ STS 롤링).
	if v.Spec.Auth.RotationInterval != "" && v.Spec.Auth.PasswordSecretRef == nil && secretRef != nil {
		newPw, rotated, rerr := r.rotatePasswordIfDue(ctx, v, password, secretRef, time.Now().UTC())
		if rerr != nil {
			return applyErrorCondition(ctx, r.Client, v, "PasswordRotation", rerr, r.Recorder)
		}
		if rotated {
			password = newPw
			logger.Info("password rotated", "name", v.Name)
			commonsevents.Emitf(r.Recorder, v, "PasswordRotated",
				"auth 비밀번호 자동 로테이션 (interval=%s)", v.Spec.Auth.RotationInterval)
		}
	}

	externalReplicaPassword, err := r.externalReplicaPassword(ctx, v)
	if err != nil {
		return applyErrorCondition(ctx, r.Client, v, "ExternalReplica", err, r.Recorder)
	}

	// 4. ConfigMap
	cm, err := resources.BuildConfigMapForValkey(v, password, externalReplicaPassword)
	if err != nil {
		return applyErrorCondition(ctx, r.Client, v, "ConfigMap", err, r.Recorder)
	}
	if err := commonsapply.ConfigMap(ctx, r.Client, r.Scheme, v, cm); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "ConfigMap", err, r.Recorder)
	}

	// 5. Headless + Client Service (TLS 활성 시 6380 port 추가 expose)
	tlsEnabled := v.Spec.TLS != nil && v.Spec.TLS.Enabled
	hs := resources.BuildHeadlessService(v.Name, v.Namespace, false, tlsEnabled, v.Spec.Service)
	if err := commonsapply.Service(ctx, r.Client, r.Scheme, v, hs); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "HeadlessService", err, r.Recorder)
	}
	cs := resources.BuildClientService(v.Name, v.Namespace, tlsEnabled, v.Spec.Service)
	if err := commonsapply.Service(ctx, r.Client, r.Scheme, v, cs); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "ClientService", err, r.Recorder)
	}

	// 6a. TLS Issuer (cert-manager) — AutoSelfSigned=true 시 namespace-scope SelfSigned
	//     Issuer 자동 생성 (외부 chart 의 tls.autoGenerated 패턴 동등).
	//     Certificate 보다 먼저 apply.
	if v.Spec.TLS != nil && v.Spec.TLS.Enabled && v.Spec.TLS.CertManager != nil &&
		v.Spec.TLS.CertManager.AutoSelfSigned {
		issuer := resources.BuildSelfSignedIssuer(v.Name, v.Namespace)
		if err := commonsapply.Unstructured(ctx, r.Client, r.Scheme, v, issuer, true); err != nil {
			return applyErrorCondition(ctx, r.Client, v, "Issuer", err, r.Recorder)
		}
	}

	// 6b. TLS Certificate (cert-manager) — Valkey 단일/replication 도 ADR-0014 AI-005
	//     에 따라 standalone TLS 통합. cert-manager CRD 미설치 시 fail-soft.
	if cert := resources.BuildCertificateForValkey(v); cert != nil {
		if err := commonsapply.Unstructured(ctx, r.Client, r.Scheme, v, cert, true); err != nil {
			return applyErrorCondition(ctx, r.Client, v, "Certificate", err, r.Recorder)
		}
	}

	// 5c. AutoUpdate — effective version 을 spec.Version 에 주입.
	//     아래 imageOrDefault(L STS) + Status.Version 으로 자동 전파된다.
	if applyAutoUpdate(&v.Spec, cachev1alpha1.SupportedValkeyVersions, time.Now().UTC()) {
		logger.Info("auto-update applied",
			"version", v.Spec.Version.Version, "channel", v.Spec.AutoUpdateChannel())
		commonsevents.Emitf(r.Recorder, v, "AutoUpdate",
			"자동 버전 업데이트: %s channel 내 %s 적용",
			v.Spec.AutoUpdateChannel(), v.Spec.Version.Version)
	}

	// 6. StatefulSet
	stsParams := resources.STSParams{
		CRName:               v.Name,
		Namespace:            v.Namespace,
		Replicas:             desiredReplicas(v),
		Image:                imageOrDefault(v.Spec.Version),
		PullPolicy:           v.Spec.Version.ImagePullPolicy,
		Resources:            buildResourceReq(v.Spec.Resources),
		StorageClass:         v.Spec.Storage.StorageClassName,
		StorageSize:          v.Spec.Storage.Size,
		Storage:              v.Spec.Storage,
		PasswordRef:          secretRef,
		ClusterMode:          false,
		Pod:                  v.Spec.Pod,
		AuthSecretHash:       hashAuthSecret(password),
		RevisionHistoryLimit: v.Spec.RevisionHistoryLimit,
		Modules:              v.Spec.Modules,
	}
	if v.Spec.TLS != nil && v.Spec.TLS.Enabled {
		switch {
		case v.Spec.TLS.CustomCert != nil && v.Spec.TLS.CustomCert.SecretName != "":
			stsParams.TLSSecretName = v.Spec.TLS.CustomCert.SecretName
		case v.Spec.TLS.CertManager != nil && v.Spec.TLS.CertManager.IssuerRef.Name != "":
			stsParams.TLSSecretName = resources.CertificateSecretName(v.Name)
		}
		// TLS cert hash — cert-manager rotation 시 pod rolling restart 트리거.
		tlsHash, err := hashTLSSecret(ctx, r.Client, v.Namespace, stsParams.TLSSecretName)
		if err != nil {
			return applyErrorCondition(ctx, r.Client, v, "TLSCertHash", err, r.Recorder)
		}
		stsParams.TLSCertHash = tlsHash
	}
	if v.Spec.Monitoring != nil && v.Spec.Monitoring.Enabled {
		stsParams.ExporterImg = exporterImage(v.Spec.Monitoring)
		stsParams.ExporterResources = exporterResources(v.Spec.Monitoring)
	}
	sts := resources.BuildStatefulSet(stsParams)

	// Scale 가드: ValkeyCluster 와 비대칭 — Replication mode 의 default 는
	// *자동 적용* (Spec.ScalePolicy 미명시 시 Deliberate=true 와 동등). slot
	// 재분배 가 없는 일반 replica 수 변경은 안전하므로 자동 적용 default.
	// Deliberate=false 명시 시만 PendingScale 기록 + STS 보존.
	preserveReplicas, pendingScale, scaleErr := r.evaluateScalePolicy(ctx, v, desiredReplicas(v))
	if scaleErr != nil {
		return applyErrorCondition(ctx, r.Client, v, "ScalePolicy", scaleErr, r.Recorder)
	}
	v.Status.PendingScale = pendingScale
	v.Status.Capabilities = computeValkeyCapabilities(v)
	SetCapabilityMetrics(v.Namespace, v.Name, AllCapabilities, v.Status.Capabilities)

	// HPA 활성 시 STS.replicas 는 HPA 가 관리 — operator 가 덮어쓰면 매 reconcile
	// 마다 HPA 결정과 충돌. ADR-0027 §Spec 의 "Spec.Replicas 는 default" 정책.
	if v.Spec.Autoscaling != nil && v.Spec.Autoscaling.Enabled {
		preserveReplicas = true
	}
	if err := commonsapply.StatefulSet(ctx, r.Client, r.Scheme, v, sts, preserveReplicas); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "StatefulSet", err, r.Recorder)
	}

	// 6.4 HPA — Spec.Autoscaling.Enabled=true 시 HPA 자동 생성 (ADR-0027).
	// 미활성 또는 toggle off 시 기존 HPA 자동 삭제 (commons apply.HPA 가 nil 처리).
	hpa := resources.BuildHorizontalPodAutoscaler(v)
	if err := commonsapply.HPA(ctx, r.Client, r.Scheme, v, resources.HPAName(v.Name), v.Namespace, hpa); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "HPA", err, r.Recorder)
	}

	// 6.5 PVC online expansion — STS VCT 가 immutable 이므로 기존 PVC 직접 patch.
	// webhook 에서 size 감소는 reject. 증가만 도달.
	if !v.Spec.Storage.Ephemeral && v.Spec.Storage.ExistingClaim == "" {
		if err := commonspvc.ExpandDataPVCs(ctx, r.Client, v.Namespace, []string{v.Name}, v.Spec.Storage.Size); err != nil {
			return applyErrorCondition(ctx, r.Client, v, "PVCResize", err, r.Recorder)
		}
	}

	// 6.6 Encryption-at-rest audit/enforce — Spec.Storage.EncryptionRequired=true 시
	// StorageClass 가 encryption 표시자를 노출하는지 검사.
	// EncryptionEnforce=true 추가 시 미표시 → reconcile 실패 (compliance hard-fail).
	if v.Spec.Storage.EncryptionRequired {
		encrypted, hint, err := auditEncryptionAtRest(ctx, r.Client, v.Spec.Storage.StorageClassName)
		if err != nil {
			commonsevents.EmitWarningf(r.Recorder, v, "EncryptionAuditFailed",
				"Failed to audit StorageClass for encryption: %v", err)
		} else if !encrypted {
			if v.Spec.Storage.EncryptionEnforce {
				return applyErrorCondition(ctx, r.Client, v, "EncryptionEnforce",
					fmt.Errorf("StorageClass %q does not advertise encryption-at-rest: %s",
						v.Spec.Storage.StorageClassName, hint), r.Recorder)
			}
			commonsevents.EmitWarningf(r.Recorder, v, "EncryptionNotVerified",
				"Storage.EncryptionRequired=true but StorageClass %q: %s",
				v.Spec.Storage.StorageClassName, hint)
		}
	}

	// 7. PDB — HA default: replicas >= 2 + PDB 미명시 시 auto-create (minAvailable=N-1).
	//    명시적 opt-out: Spec.PodDisruptionBudget = {Enabled: false} → 기존 PDB cleanup.
	//    CDEX-M1 (Codex stage 3 finding): false 시 *기존 PDB 삭제* 의무 (mongodb_controller.go:313 sister).
	if shouldAutoCreatePDB(v.Spec.PodDisruptionBudget, desiredReplicas(v)) {
		pdb := resources.BuildPDB(v.Name, v.Namespace, desiredReplicas(v), v.Spec.PodDisruptionBudget)
		if err := commonsapply.PDB(ctx, r.Client, r.Scheme, v, pdb); err != nil {
			return applyErrorCondition(ctx, r.Client, v, "PDB", err, r.Recorder)
		}
	} else if err := EnsurePDBDeleted(ctx, r.Client, v.Name, v.Namespace); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "PDB", err, r.Recorder)
	}
	if v.Spec.NetworkPolicy != nil && v.Spec.NetworkPolicy.Enabled {
		np := resources.BuildNetworkPolicy(v.Name, v.Namespace, false, v.Spec.NetworkPolicy)
		if err := commonsapply.NetworkPolicy(ctx, r.Client, r.Scheme, v, np); err != nil {
			return applyErrorCondition(ctx, r.Client, v, "NetworkPolicy", err, r.Recorder)
		}
	}

	// 8. Status: STS 상태 반영
	current := &corev1.Pod{}
	_ = current
	stsKey := types.NamespacedName{Name: resources.StatefulSetName(v.Name), Namespace: v.Namespace}
	stsObj, err := fetchSTSStatus(ctx, r.Client, stsKey)
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	// 9a. Replication 자동 failover 검토 (ADR-0017) — primary NotReady 30s+ 시
	// 새 primary 선출. readyReplicas 가 desired 미만일 때만 의미. non-fatal —
	// 실패 시 다음 reconcile 재시도.
	if v.Spec.Mode == cachev1alpha1.ModeReplication && v.Spec.Replicas > 1 && !isExternalReplica(v) {
		tlsCfg, _ := r.tlsConfigForValkey(ctx, v)
		if err := r.reconcileFailover(ctx, v, password, tlsCfg); err != nil {
			logger.Info("failover reconciliation failed", "error", err.Error())
		}
	}

	// 9. Replication: primary 가 ready 되면 모든 replica 가 REPLICAOF primary 호출.
	if v.Spec.Mode == cachev1alpha1.ModeReplication && desiredReplicas(v) > 1 &&
		!isExternalReplica(v) && stsObj.readyReplicas == desiredReplicas(v) {
		tlsCfg, tlsErr := r.tlsConfigForValkey(ctx, v)
		if tlsErr != nil {
			logger.Info("Replication TLS config pending", "error", tlsErr.Error())
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
		if err := r.ensureReplication(ctx, v, password, tlsCfg); err != nil {
			logger.Info("Replication setup pending", "error", err.Error())
			return ctrl.Result{RequeueAfter: requeueProgress}, nil
		}
	}

	// 10. Status update
	v.Status.ObservedGeneration = v.Generation
	v.Status.ReadyReplicas = stsObj.readyReplicas
	v.Status.Version = v.Spec.Version.Version
	v.Status.Endpoint = fmt.Sprintf("%s.%s.svc:%d", resources.ClientServiceName(v.Name), v.Namespace, resources.PortClient)
	v.Status.CurrentPrimary = r.determinePrimary(v)

	switch {
	case stsObj.readyReplicas == 0:
		v.Status.Phase = cachev1alpha1.PhasePending
	case stsObj.readyReplicas < desiredReplicas(v):
		v.Status.Phase = cachev1alpha1.PhaseInitializing
	default:
		v.Status.Phase = cachev1alpha1.PhaseRunning
	}

	// 비-Running 상태에서 pod 의 *실제 차단 사유* 를 status.conditions[Ready].message 로
	// surface — 9 일간 CrashLoopBackOff stuck 사고 (RDB version 80) RCA. best-effort.
	var readyMsg string
	if v.Status.Phase != cachev1alpha1.PhaseRunning {
		readyMsg = r.diagnoseUnhealthyPods(ctx, v)
	}

	conds := v.GetConditions()
	*conds = filterConditionsByType(*conds, "Ready")
	*conds = append(*conds, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             string(v.Status.Phase),
		Message:            readyMsg,
	})
	// aggregate status (단계 10 전반의 산출) — 산출 로직이 함수 전반에 분산되어
	// 클로저 재적용 대상이 아니다. conflict 시 commons 가 refetch 후 server 상태로
	// 갱신하며, 다음 reconcile 이 status 를 재산출한다 (level-triggered).
	if err := commonsstatus.UpdateWithRetry(ctx, r.Client, v); err != nil {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}

	if v.Status.Phase != cachev1alpha1.PhaseRunning {
		return ctrl.Result{RequeueAfter: requeueProgress}, nil
	}
	return ctrl.Result{RequeueAfter: requeueSteady}, nil
}

// applyDefaults — operator 측 defaulting (CRD default 미커버 영역).
func (r *ValkeyReconciler) applyDefaults(v *cachev1alpha1.Valkey) {
	if v.Spec.Mode == "" {
		v.Spec.Mode = cachev1alpha1.ModeStandalone
	}
	if v.Spec.Replicas == 0 {
		v.Spec.Replicas = 1
	}
	if v.Spec.Mode == cachev1alpha1.ModeStandalone {
		v.Spec.Replicas = 1
	}
	if v.Spec.Version.Version == "" {
		v.Spec.Version.Version = cachev1alpha1.DefaultValkeyVersion
	}
	if v.Spec.Version.Image == "" {
		v.Spec.Version.Image = cachev1alpha1.DefaultValkeyImage
	}
}

func desiredReplicas(v *cachev1alpha1.Valkey) int32 { return v.Spec.Replicas }

// evaluateScalePolicy — Replication mode 의 Spec.Replicas 변경 감지 + 가드.
//
// ValkeyCluster 와 *비대칭*:
//   - ValkeyCluster: Deliberate=false default (slot 재분배 위험 → 명시 동의 필요)
//   - Valkey (Replication): Deliberate=true default (단순 replica 수 변경 안전)
//
// 결정 규칙:
//   - STS 미존재 (초기 부트스트랩): preserve=false, pending=nil → 정상 desired 적용.
//   - 현재 == desired: preserve=false, pending=nil.
//   - 현재 != desired & ScalePolicy.Deliberate=true 또는 ScalePolicy=nil:
//     preserve=false, pending=nil → 즉시 적용.
//   - 현재 != desired & Deliberate=false 명시: preserve=true, pending 기록.
func (r *ValkeyReconciler) evaluateScalePolicy(
	ctx context.Context, v *cachev1alpha1.Valkey, desired int32,
) (preserveReplicas bool, pendingScale *cachev1alpha1.PendingScale, err error) {
	stsKey := types.NamespacedName{Name: resources.StatefulSetName(v.Name), Namespace: v.Namespace}
	sts := &appsv1.StatefulSet{}
	if getErr := r.Get(ctx, stsKey, sts); getErr != nil {
		if errors.IsNotFound(getErr) {
			return false, nil, nil
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
	// Replication mode default: ScalePolicy 미명시 시 자동 적용.
	deliberate := v.Spec.ScalePolicy == nil || v.Spec.ScalePolicy.Deliberate
	if deliberate {
		return false, nil, nil
	}
	pending := &cachev1alpha1.PendingScale{
		CurrentReplicas: current,
		DesiredReplicas: desired,
		RequestedAt:     metav1.Now().Format("2006-01-02T15:04:05Z07:00"),
		Reason: fmt.Sprintf(
			"Replication scale deferred: set Spec.ScalePolicy.Deliberate=true to apply (%d → %d)",
			current, desired),
	}
	return true, pending, nil
}

// determinePrimary — primary pod 결정 (ADR-0017).
//
// 우선순위:
//  1. Status.CurrentPrimary 가 set 되어 있으면 보존 (failover 후 다음 reconcile
//     이 pod-0 으로 되돌리지 않도록).
//  2. 미설정 (첫 부트스트랩) 시 pod-0 default.
//
// reconcileFailover 가 Status.CurrentPrimary 를 갱신하면 다음 reconcile 의
// 본 helper 가 새 primary 보존.
func (r *ValkeyReconciler) determinePrimary(v *cachev1alpha1.Valkey) string {
	if v.Status.CurrentPrimary != "" {
		return v.Status.CurrentPrimary
	}
	return fmt.Sprintf("%s-0", v.Name)
}

// diagnoseUnhealthyPods — readyReplicas == 0 상태에서 pod 의 *실제 차단 사유* 한 줄.
// status.conditions[Ready].message 로 surface 하여 `kubectl get valkey -o yaml` 만으로
// CrashLoopBackOff / ImagePullBackOff / Terminated 등 진단 가능.
// best-effort: list 실패 또는 unhealthy pod 없음 → "".
func (r *ValkeyReconciler) diagnoseUnhealthyPods(ctx context.Context, v *cachev1alpha1.Valkey) string {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods,
		client.InNamespace(v.Namespace),
		client.MatchingLabels(resources.SelectorLabels(v.Name)),
	); err != nil {
		return ""
	}
	for _, pod := range pods.Items {
		if !pod.DeletionTimestamp.IsZero() {
			continue
		}
		if msg := firstUnhealthyContainerMessage(pod.Name, pod.Status.InitContainerStatuses); msg != "" {
			return msg
		}
		if msg := firstUnhealthyContainerMessage(pod.Name, pod.Status.ContainerStatuses); msg != "" {
			return msg
		}
	}
	return ""
}

// firstUnhealthyContainerMessage — Waiting (CrashLoopBackOff 류) 또는 Terminated
// (ExitCode != 0) container 의 사유를 한 줄로 요약. corev1.ContainerStatus 만 의존 —
// fake client + 표 기반 단위 테스트로 검증 가능.
func firstUnhealthyContainerMessage(podName string, statuses []corev1.ContainerStatus) string {
	for _, s := range statuses {
		if s.State.Waiting != nil && isTerminalWaitingReason(s.State.Waiting.Reason) {
			return fmt.Sprintf("Pod %s container %s waiting %s: %s",
				podName, s.Name, s.State.Waiting.Reason, s.State.Waiting.Message)
		}
		if s.State.Terminated != nil && s.State.Terminated.ExitCode != 0 {
			return fmt.Sprintf("Pod %s container %s terminated exitCode=%d: %s",
				podName, s.Name, s.State.Terminated.ExitCode, s.State.Terminated.Message)
		}
	}
	return ""
}

// isTerminalWaitingReason — kubelet 가 pod 시작 자체 차단 시 보고하는 Waiting reason.
// 일시 Pending (ContainerCreating, PodInitializing) 은 의도적으로 제외 — 정상 부트스트랩
// 신호 surfacing 시 노이즈.
func isTerminalWaitingReason(reason string) bool {
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

func imageOrDefault(vv cachev1alpha1.ValkeyVersion) string {
	if vv.ImageRef != "" {
		return vv.ImageRef
	}
	if vv.Image != "" && vv.Version != "" {
		return fmt.Sprintf("%s:%s", vv.Image, vv.Version)
	}
	return defaultImage
}

func buildResourceReq(r cachev1alpha1.ResourcesSpec) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{Requests: r.Requests, Limits: r.Limits}
}

func exporterImage(m *cachev1alpha1.MonitoringSpec) string {
	if m == nil || m.Exporter == nil || m.Exporter.Image == "" {
		// 구체 버전 pin SSOT — api 패키지 const (CRD default marker 와 동기).
		return cachev1alpha1.DefaultExporterImage
	}
	return m.Exporter.Image
}

// exporterResources — metrics sidecar 의 resources. 빈 ResourceRequirements 면
// K8s default (Burstable QoS) 로 fallback. CR spec.monitoring.exporter.resources
// 가 sts container 까지 정확히 mapping.
func exporterResources(m *cachev1alpha1.MonitoringSpec) corev1.ResourceRequirements {
	if m == nil || m.Exporter == nil {
		return corev1.ResourceRequirements{}
	}
	return buildResourceReq(m.Exporter.Resources)
}

// ensureAuthSecret — PasswordSecretRef 미지정 시 자동 생성. 항상 (password, ref, err) 반환.
func (r *ValkeyReconciler) ensureAuthSecret(ctx context.Context, v *cachev1alpha1.Valkey) (string, *corev1.SecretKeySelector, error) {
	if v.Spec.Auth.PasswordSecretRef != nil {
		ref := v.Spec.Auth.PasswordSecretRef
		s := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: v.Namespace}, s); err != nil {
			return "", nil, fmt.Errorf("get user-provided secret: %w", err)
		}
		key := ref.Key
		if key == "" {
			key = resources.SecretPasswordKey
		}
		return string(s.Data[key]), ref, nil
	}

	secretName := resources.DefaultSecretName(v.Name)
	existing := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: v.Namespace}, existing); err == nil {
		return string(existing.Data[resources.SecretPasswordKey]),
			&corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: resources.SecretPasswordKey},
			nil
	} else if !errors.IsNotFound(err) {
		return "", nil, err
	}

	password, err := resources.GeneratePassword()
	if err != nil {
		return "", nil, err
	}
	if err := commonsreconcile.SecretIfNotExists(ctx, r.Client, r.Scheme, v, secretName, func() *corev1.Secret {
		return resources.BuildAuthSecret(v.Name, v.Namespace, password)
	}); err != nil {
		return "", nil, err
	}
	return password, &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
		Key:                  resources.SecretPasswordKey,
	}, nil
}

func isExternalReplica(v *cachev1alpha1.Valkey) bool {
	return v.Spec.ExternalReplica != nil && v.Spec.ExternalReplica.Enabled
}

func (r *ValkeyReconciler) externalReplicaPassword(
	ctx context.Context,
	v *cachev1alpha1.Valkey,
) (string, error) {
	if !isExternalReplica(v) || v.Spec.ExternalReplica.Auth == nil || !v.Spec.ExternalReplica.Auth.Enabled {
		return "", nil
	}
	ref := v.Spec.ExternalReplica.Auth.PasswordSecretRef
	if ref == nil {
		return "", fmt.Errorf("externalReplica.auth.passwordSecretRef is required when externalReplica.auth.enabled=true")
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: v.Namespace}, secret); err != nil {
		return "", fmt.Errorf("get external replica auth secret: %w", err)
	}
	key := ref.Key
	if key == "" {
		key = resources.SecretPasswordKey
	}
	return string(secret.Data[key]), nil
}

type stsStatus struct {
	readyReplicas int32
	totalReplicas int32
}

// fetchSTS 계열 함수는 fetchSTSStatus (sts_helpers.go) 로 통합 — 양 controller 공용.

// ensureReplication — replica pod 들이 *현재 primary* (Status.CurrentPrimary
// 또는 pod-0) 를 가리키게 한다.
//
// ADR-0017 통합: primaryOrdinal(v) 가 Status.CurrentPrimary 우선 → failover
// 후 새 primary 기준 정렬. 기존 부트스트랩 (Status 미설정) 시 pod-0 fallback.
//
// TLS 활성 (tlsCfg != nil) 시 control-plane 통신 은 tls-port (6380) 사용.
func (r *ValkeyReconciler) ensureReplication(ctx context.Context, v *cachev1alpha1.Valkey, password string, tlsCfg *tls.Config) error {
	port := int32(resources.PortClient)
	if tlsCfg != nil {
		port = resources.PortTLS
	}
	primaryIdx := primaryOrdinal(v)
	primaryAddr := fmt.Sprintf("%s:%d", resources.PodFQDN(v.Name, primaryIdx, v.Namespace), port)
	primary := dialValkey(primaryAddr, password, tlsCfg)
	if err := vk.PromoteToPrimary(ctx, primary); err != nil {
		_ = primary.Close()
		return fmt.Errorf("promote primary: %w", err)
	}
	_ = primary.Close()

	primaryHost := resources.PodFQDN(v.Name, primaryIdx, v.Namespace)
	for i := int32(0); i < v.Spec.Replicas; i++ {
		if int(i) == primaryIdx {
			continue
		}
		addr := fmt.Sprintf("%s:%d", resources.PodFQDN(v.Name, int(i), v.Namespace), port)
		c := dialValkey(addr, password, tlsCfg)
		err := vk.EnsureReplicaOf(ctx, c, primaryHost, int(port))
		_ = c.Close()
		if err != nil {
			return fmt.Errorf("ensure replicaof %s: %w", addr, err)
		}
	}
	return nil
}

// dialValkey — TLS 적용 가능한 redis client 빌더 (cluster controller 의 dialPod 와 같음).
func dialValkey(addr, password string, tlsCfg *tls.Config) *redis.Client {
	opts := vk.DialOptions{Address: addr, Password: password}
	if tlsCfg != nil {
		opts.UseTLS = true
		opts.TLSConf = tlsCfg
	}
	return vk.NewSingleClient(opts)
}

// tlsConfigForValkey — control-plane TLS config (buildValkeyTLSConfig 위임).
func (r *ValkeyReconciler) tlsConfigForValkey(ctx context.Context, v *cachev1alpha1.Valkey) (*tls.Config, error) {
	return buildValkeyTLSConfig(ctx, r.Client, v.Namespace, v.Name, v.Spec.TLS)
}

func (r *ValkeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// events API 마이그레이션 완료 (RFC-0023 Phase 2, 2026-05-11) — events.EventRecorder 사용.
	r.Recorder = mgr.GetEventRecorder("valkey-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Valkey{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		WithOptions(controller.Options{
			// Controller v2 (ROADMAP 2.x) — fan-out + rate-limiter tuning.
			// MaxConcurrentReconciles: 다중 Valkey CR 동시 reconcile (기본 1 → 3).
			// RateLimiter: per-item exponential backoff (5ms~5m) — 일시 장애 시 과도한
			// requeue 폭주 억제. overall token-bucket 은 controller-runtime 기본 유지.
			MaxConcurrentReconciles: 3,
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
				5*time.Millisecond, 5*time.Minute),
		}).
		Named("valkey").
		Complete(r)
}
