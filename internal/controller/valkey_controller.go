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
	"time"

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
	if v.Spec.Monitoring != nil && v.Spec.Monitoring.Enabled {
		stsParams.ExporterImg = exporterImage(v.Spec.Monitoring)
	}
	sts := resources.BuildStatefulSet(stsParams)
	if err := applyStatefulSet(ctx, r.Client, r.Scheme, v, sts, false); err != nil {
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

	// 9. Replication: primary 가 ready 되면 모든 replica 가 REPLICAOF primary 호출.
	if v.Spec.Mode == cachev1alpha1.ModeReplication && desiredReplicas(v) > 1 && stsObj.readyReplicas == desiredReplicas(v) {
		if err := r.ensureReplication(ctx, v, password); err != nil {
			logger.Info("Replication setup pending", "error", err.Error())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	// 10. Status update
	v.Status.ObservedGeneration = v.Generation
	v.Status.ReadyReplicas = stsObj.readyReplicas
	v.Status.Version = v.Spec.Version.Version
	v.Status.Endpoint = fmt.Sprintf("%s.%s.svc:%d", resources.ClientServiceName(v.Name), v.Namespace, resources.PortClient)
	primary := fmt.Sprintf("%s-0", v.Name)
	v.Status.CurrentPrimary = primary

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

// ensureReplication — replica pod 들이 primary 를 가리키게 한다 (M2).
// pod-0 = primary, pod-1..N = replica.
func (r *ValkeyReconciler) ensureReplication(ctx context.Context, v *cachev1alpha1.Valkey, password string) error {
	primaryAddr := fmt.Sprintf("%s:%d", resources.PodFQDN(v.Name, 0, v.Namespace), resources.PortClient)
	primary := vk.NewSingleClient(vk.DialOptions{Address: primaryAddr, Password: password})
	if err := vk.PromoteToPrimary(ctx, primary); err != nil {
		_ = primary.Close()
		return fmt.Errorf("promote primary: %w", err)
	}
	_ = primary.Close()

	primaryHost := resources.PodFQDN(v.Name, 0, v.Namespace)
	for i := int32(1); i < v.Spec.Replicas; i++ {
		addr := fmt.Sprintf("%s:%d", resources.PodFQDN(v.Name, int(i), v.Namespace), resources.PortClient)
		c := vk.NewSingleClient(vk.DialOptions{Address: addr, Password: password})
		err := vk.EnsureReplicaOf(ctx, c, primaryHost, resources.PortClient)
		_ = c.Close()
		if err != nil {
			return fmt.Errorf("ensure replicaof %s: %w", addr, err)
		}
	}
	return nil
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
