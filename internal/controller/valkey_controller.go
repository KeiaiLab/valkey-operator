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
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

const (
	finalizerValkey = "cache.keiailab.io/valkey-finalizer"
	defaultImage    = "docker.io/valkey/valkey:8.1.6"
)

// ValkeyReconciler reconciles a Valkey object (Standalone + Replication).
type ValkeyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeys/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete

func (r *ValkeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	v := &cachev1alpha1.Valkey{}
	if err := r.Get(ctx, req.NamespacedName, v); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 1. Finalizer / deletion
	if !v.DeletionTimestamp.IsZero() {
		return handleFinalizerCleanup(ctx, r.Client, v, finalizerValkey, nil)
	}
	if !controllerutil.ContainsFinalizer(v, finalizerValkey) {
		controllerutil.AddFinalizer(v, finalizerValkey)
		if err := r.Update(ctx, v); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 1b. Paused — ValkeyRestore (ADR-0015) 가 STS 를 직접 patch 중일 때
	//     본 controller 의 reconcile 가 init container 를 제거하지 않도록.
	if isPaused(v) {
		logger.V(1).Info("paused — skipping reconcile (cache.keiailab.io/paused=true)",
			"name", v.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 2. Defaulting
	r.applyDefaults(v)

	// 3. Auth Secret 보장 (자동 생성 시 멱등)
	password, secretRef, err := r.ensureAuthSecret(ctx, v)
	if err != nil {
		return applyErrorCondition(ctx, r.Client, v, "AuthSecret", err, r.Recorder)
	}

	// 4. ConfigMap
	cm, err := resources.BuildConfigMapForValkey(v, password)
	if err != nil {
		return applyErrorCondition(ctx, r.Client, v, "ConfigMap", err, r.Recorder)
	}
	if err := applyConfigMap(ctx, r.Client, r.Scheme, v, cm); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "ConfigMap", err, r.Recorder)
	}

	// 5. Headless + Client Service
	hs := resources.BuildHeadlessService(v.Name, v.Namespace, false)
	if err := applyService(ctx, r.Client, r.Scheme, v, hs); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "HeadlessService", err, r.Recorder)
	}
	cs := resources.BuildClientService(v.Name, v.Namespace)
	if err := applyService(ctx, r.Client, r.Scheme, v, cs); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "ClientService", err, r.Recorder)
	}

	// 6a. TLS Certificate (cert-manager) — Valkey 단일/replication 도 ADR-0014 AI-005
	//     에 따라 standalone TLS 통합. cert-manager CRD 미설치 시 fail-soft.
	if cert := resources.BuildCertificateForValkey(v); cert != nil {
		if err := applyServiceMonitor(ctx, r.Client, r.Scheme, v, cert); err != nil {
			return applyErrorCondition(ctx, r.Client, v, "Certificate", err, r.Recorder)
		}
	}

	// 6. StatefulSet
	stsParams := resources.STSParams{
		CRName:       v.Name,
		Namespace:    v.Namespace,
		Replicas:     desiredReplicas(v),
		Image:        imageOrDefault(v.Spec.Version),
		PullPolicy:   v.Spec.Version.ImagePullPolicy,
		Resources:    buildResourceReq(v.Spec.Resources),
		StorageClass: v.Spec.Storage.StorageClassName,
		StorageSize:  v.Spec.Storage.Size,
		PasswordRef:  secretRef,
		ClusterMode:  false,
		Pod:          v.Spec.Pod,
	}
	if v.Spec.TLS != nil && v.Spec.TLS.Enabled {
		switch {
		case v.Spec.TLS.CustomCert != nil && v.Spec.TLS.CustomCert.SecretName != "":
			stsParams.TLSSecretName = v.Spec.TLS.CustomCert.SecretName
		case v.Spec.TLS.CertManager != nil && v.Spec.TLS.CertManager.IssuerRef.Name != "":
			stsParams.TLSSecretName = resources.CertificateSecretName(v.Name)
		}
	}
	if v.Spec.Monitoring != nil && v.Spec.Monitoring.Enabled {
		stsParams.ExporterImg = exporterImage(v.Spec.Monitoring)
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

	if err := applyStatefulSet(ctx, r.Client, r.Scheme, v, sts, preserveReplicas); err != nil {
		return applyErrorCondition(ctx, r.Client, v, "StatefulSet", err, r.Recorder)
	}

	// 7. PDB / NetworkPolicy (opt-in)
	if v.Spec.PodDisruptionBudget != nil && v.Spec.PodDisruptionBudget.Enabled {
		pdb := resources.BuildPDB(v.Name, v.Namespace, desiredReplicas(v), v.Spec.PodDisruptionBudget)
		if err := applyPDB(ctx, r.Client, r.Scheme, v, pdb); err != nil {
			return applyErrorCondition(ctx, r.Client, v, "PDB", err, r.Recorder)
		}
	}
	if v.Spec.NetworkPolicy != nil && v.Spec.NetworkPolicy.Enabled {
		np := resources.BuildNetworkPolicy(v.Name, v.Namespace, false, v.Spec.NetworkPolicy)
		if err := applyNetworkPolicy(ctx, r.Client, r.Scheme, v, np); err != nil {
			return applyErrorCondition(ctx, r.Client, v, "NetworkPolicy", err, r.Recorder)
		}
	}

	// 8. Status: STS 상태 반영
	current := &corev1.Pod{}
	_ = current
	stsKey := types.NamespacedName{Name: resources.StatefulSetName(v.Name), Namespace: v.Namespace}
	stsObj, err := r.fetchStatefulSetStatus(ctx, stsKey)
	if err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 9a. Replication 자동 failover 검토 (ADR-0017) — primary NotReady 30s+ 시
	// 새 primary 선출. readyReplicas 가 desired 미만일 때만 의미. non-fatal —
	// 실패 시 다음 reconcile 재시도.
	if v.Spec.Mode == cachev1alpha1.ModeReplication && v.Spec.Replicas > 1 {
		tlsCfg, _ := r.tlsConfigForValkey(ctx, v)
		if err := r.reconcileFailover(ctx, v, password, tlsCfg); err != nil {
			logger.Info("failover reconciliation failed", "error", err.Error())
		}
	}

	// 9. Replication: primary 가 ready 되면 모든 replica 가 REPLICAOF primary 호출.
	if v.Spec.Mode == cachev1alpha1.ModeReplication && desiredReplicas(v) > 1 && stsObj.readyReplicas == desiredReplicas(v) {
		tlsCfg, tlsErr := r.tlsConfigForValkey(ctx, v)
		if tlsErr != nil {
			logger.Info("Replication TLS config pending", "error", tlsErr.Error())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		if err := r.ensureReplication(ctx, v, password, tlsCfg); err != nil {
			logger.Info("Replication setup pending", "error", err.Error())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
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

	conds := v.GetConditions()
	*conds = filterConditionsByType(*conds, "Ready")
	*conds = append(*conds, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             string(v.Status.Phase),
	})
	if err := updateStatusWithRetry(ctx, r.Client, v); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if v.Status.Phase != cachev1alpha1.PhaseRunning {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
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
		v.Spec.Version.Version = "8.1.6"
	}
	if v.Spec.Version.Image == "" {
		v.Spec.Version.Image = "docker.io/valkey/valkey"
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

func imageOrDefault(vv cachev1alpha1.ValkeyVersion) string {
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
		return "oliver006/redis_exporter:latest"
	}
	return m.Exporter.Image
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
	if err := reconcileSecretIfNotExists(ctx, r.Client, r.Scheme, v, secretName, func() *corev1.Secret {
		return resources.BuildAuthSecret(v.Name, v.Namespace, password)
	}); err != nil {
		return "", nil, err
	}
	return password, &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
		Key:                  resources.SecretPasswordKey,
	}, nil
}

type stsStatus struct {
	readyReplicas int32
	totalReplicas int32
}

func (r *ValkeyReconciler) fetchStatefulSetStatus(ctx context.Context, key types.NamespacedName) (*stsStatus, error) {
	sts := &corev1.PodList{}
	_ = sts
	// Use unstructured / typed approach: fetch StatefulSet via client
	return r.fetchSTS(ctx, key)
}

func (r *ValkeyReconciler) fetchSTS(ctx context.Context, key types.NamespacedName) (*stsStatus, error) {
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

// tlsConfigForValkey — ValkeyClusterReconciler.tlsConfigForCluster 와 동일 로직.
// CustomCert > CertManager 우선순위. 둘 다 ca.crt 미준비 → InsecureSkipVerify fallback.
func (r *ValkeyReconciler) tlsConfigForValkey(ctx context.Context, v *cachev1alpha1.Valkey) (*tls.Config, error) {
	if v.Spec.TLS == nil || !v.Spec.TLS.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: resources.HeadlessServiceName(v.Name) + "." + v.Namespace + ".svc",
	}
	loadAndAttach := func(secretName string) (bool, error) {
		s := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: v.Namespace}, s); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		caBytes, ok := s.Data["ca.crt"]
		if !ok || len(caBytes) == 0 {
			return false, nil
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return false, fmt.Errorf("invalid PEM in %s/%s/ca.crt", v.Namespace, secretName)
		}
		cfg.RootCAs = pool
		if crt, hasCrt := s.Data["tls.crt"]; hasCrt {
			if key, hasKey := s.Data["tls.key"]; hasKey && len(crt) > 0 && len(key) > 0 {
				if pair, err := tls.X509KeyPair(crt, key); err == nil {
					cfg.Certificates = []tls.Certificate{pair}
				}
			}
		}
		return true, nil
	}
	if v.Spec.TLS.CustomCert != nil && v.Spec.TLS.CustomCert.SecretName != "" {
		if ok, err := loadAndAttach(v.Spec.TLS.CustomCert.SecretName); err != nil {
			return nil, err
		} else if ok {
			return cfg, nil
		}
	}
	if v.Spec.TLS.CertManager != nil && v.Spec.TLS.CertManager.IssuerRef.Name != "" {
		if ok, err := loadAndAttach(resources.CertificateSecretName(v.Name)); err != nil {
			return nil, err
		} else if ok {
			return cfg, nil
		}
	}
	cfg.InsecureSkipVerify = true
	return cfg, nil
}

func (r *ValkeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// nolint:staticcheck // 새 events API (mgr.GetEventRecorder) 마이그레이션은 ADR-0002 (예정).
	// applyErrorCondition 헬퍼가 record.EventRecorder 시그니처를 사용하므로 helpers.go +
	// 양 컨트롤러 동시 변경 필요 — 별도 PR 로 분리.
	r.Recorder = mgr.GetEventRecorderFor("valkey-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Valkey{}).
		Owns((&appsv1StatefulSet{}).Inner()).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Named("valkey").
		Complete(r)
}
