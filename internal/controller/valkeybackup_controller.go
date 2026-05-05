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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
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
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeybackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=cache.keiailab.io,resources=valkeys;valkeyclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ValkeyBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	b := &cachev1alpha1.ValkeyBackup{}
	if err := r.Get(ctx, req.NamespacedName, b); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Terminal phase 면 추가 작업 없음.
	if b.IsTerminal() {
		return ctrl.Result{}, nil
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
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil

	case cachev1alpha1.BackupPhasePending:
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
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		logger.Info("Backup BGSAVE/BGREWRITEAOF issued",
			"name", b.Name, "type", b.Spec.Type, "target", b.Spec.ClusterRef.Name,
			"preLastSave", preLastSave)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

	case cachev1alpha1.BackupPhaseInProgress:
		// LASTSAVE 가 preLastSave 보다 커지면 RDB 스냅샷 완료.
		var preLastSaveUnix int64
		_, _ = fmt.Sscanf(b.Status.Message, "preLastSave=%d", &preLastSaveUnix)
		curLastSave, err := r.queryLastSave(ctx, b)
		if err != nil {
			logger.Info("LASTSAVE poll failed — will retry", "error", err.Error())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		// 30 분 timeout — RDB 가 매우 큰 dataset 가 아닌 한 충분.
		if b.Status.StartedAt != nil && time.Since(b.Status.StartedAt.Time) > 30*time.Minute {
			return r.markFailed(ctx, b, "BackupTimeout",
				fmt.Sprintf("LASTSAVE did not advance within 30m (pre=%d cur=%d)",
					preLastSaveUnix, curLastSave.Unix()))
		}
		if curLastSave.Unix() <= preLastSaveUnix {
			// 아직 진행 중.
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		// 완료. 실제 dump.rdb 파일 의 PVC 복사 는 M3.5 (Job 기반) — 본 iter 는 LASTSAVE
		// 기준 RDB 스냅샷 발생만 보장.
		now := metav1.Now()
		b.Status.Phase = cachev1alpha1.BackupPhaseCompleted
		b.Status.CompletedAt = &now
		b.Status.Message = fmt.Sprintf("RDB snapshot completed at %s (PVC copy pending — M3.5)",
			curLastSave.Format(time.RFC3339))
		setCondition(b.GetConditions(), metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Completed",
			Message:            b.Status.Message,
			ObservedGeneration: b.Generation,
		})
		if err := updateStatusWithRetry(ctx, r.Client, b); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		logger.Info("Backup LASTSAVE advanced — RDB completed",
			"name", b.Name, "lastSave", curLastSave)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// validateClusterRef — Spec.ClusterRef 가 가리키는 Valkey / ValkeyCluster 존재 확인.
func (r *ValkeyBackupReconciler) validateClusterRef(ctx context.Context, b *cachev1alpha1.ValkeyBackup) error {
	key := types.NamespacedName{Name: b.Spec.ClusterRef.Name, Namespace: b.Namespace}
	switch b.Spec.ClusterRef.Kind {
	case "ValkeyCluster":
		obj := &cachev1alpha1.ValkeyCluster{}
		if err := r.Get(ctx, key, obj); err != nil {
			return fmt.Errorf("get ValkeyCluster %s: %w", key, err)
		}
	case "Valkey":
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
func (r *ValkeyBackupReconciler) markFailed(ctx context.Context, b *cachev1alpha1.ValkeyBackup, reason, msg string) (ctrl.Result, error) {
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
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// triggerBackup — 대상 인스턴스의 primary (Valkey) 또는 임의 노드 (ValkeyCluster)
// 에 BGSAVE / BGREWRITEAOF 발행. preLastSave timestamp 반환 (완료 감지용).
func (r *ValkeyBackupReconciler) triggerBackup(ctx context.Context, b *cachev1alpha1.ValkeyBackup) (time.Time, error) {
	c, err := r.dialBackupTarget(ctx, b)
	if err != nil {
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
	c, err := r.dialBackupTarget(ctx, b)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = c.Close() }()
	return vk.LastSaveTime(ctx, c)
}

// dialBackupTarget — 대상 CR 의 primary 노드 dial. TLS 활성 시 자동 6380 + cert
// 로딩 (대상 인스턴스 의 cert-manager Secret 또는 CustomCert).
func (r *ValkeyBackupReconciler) dialBackupTarget(ctx context.Context, b *cachev1alpha1.ValkeyBackup) (*redis.Client, error) {
	password, err := r.fetchBackupTargetPassword(ctx, b)
	if err != nil {
		return nil, err
	}
	tlsCfg, err := r.tlsConfigForBackupTarget(ctx, b)
	if err != nil {
		return nil, err
	}
	port := int32(resources.PortClient)
	if tlsCfg != nil {
		port = resources.PortTLS
	}
	addr := fmt.Sprintf("%s:%d",
		resources.PodFQDN(b.Spec.ClusterRef.Name, 0, b.Namespace),
		port)
	opts := vk.DialOptions{Address: addr, Password: password}
	if tlsCfg != nil {
		opts.UseTLS = true
		opts.TLSConf = tlsCfg
	}
	return vk.NewSingleClient(opts), nil
}

// tlsConfigForBackupTarget — 대상 CR 의 Spec.TLS 를 조회하여 *operator → 노드*
// control-plane TLS config 구성. ValkeyController / ValkeyClusterController 의
// 동등 함수 와 같은 우선순위 (CustomCert > CertManager > InsecureSkipVerify).
//
// 본 함수 는 대상 CR 의 종류 (Valkey / ValkeyCluster) 별로 TLSSpec / SecretName /
// CertManager 필드를 조회한다.
func (r *ValkeyBackupReconciler) tlsConfigForBackupTarget(ctx context.Context, b *cachev1alpha1.ValkeyBackup) (*tls.Config, error) {
	var (
		tlsSpec  *cachev1alpha1.TLSSpec
		certName = resources.CertificateSecretName(b.Spec.ClusterRef.Name)
		nsName   = types.NamespacedName{Name: b.Spec.ClusterRef.Name, Namespace: b.Namespace}
		headless = resources.HeadlessServiceName(b.Spec.ClusterRef.Name) + "." + b.Namespace + ".svc"
	)
	switch b.Spec.ClusterRef.Kind {
	case "Valkey":
		obj := &cachev1alpha1.Valkey{}
		if err := r.Get(ctx, nsName, obj); err != nil {
			return nil, err
		}
		tlsSpec = obj.Spec.TLS
	case "ValkeyCluster":
		obj := &cachev1alpha1.ValkeyCluster{}
		if err := r.Get(ctx, nsName, obj); err != nil {
			return nil, err
		}
		tlsSpec = obj.Spec.TLS
	}
	if tlsSpec == nil || !tlsSpec.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: headless}

	loadAttach := func(secretName string) (bool, error) {
		s := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: b.Namespace}, s); err != nil {
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
			return false, fmt.Errorf("invalid PEM in %s/ca.crt", secretName)
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
	if tlsSpec.CustomCert != nil && tlsSpec.CustomCert.SecretName != "" {
		if ok, err := loadAttach(tlsSpec.CustomCert.SecretName); err != nil {
			return nil, err
		} else if ok {
			return cfg, nil
		}
	}
	if tlsSpec.CertManager != nil && tlsSpec.CertManager.IssuerRef.Name != "" {
		if ok, err := loadAttach(certName); err != nil {
			return nil, err
		} else if ok {
			return cfg, nil
		}
	}
	cfg.InsecureSkipVerify = true
	return cfg, nil
}

// fetchBackupTargetPassword — 대상 인스턴스 의 auth secret 에서 password 추출.
func (r *ValkeyBackupReconciler) fetchBackupTargetPassword(ctx context.Context, b *cachev1alpha1.ValkeyBackup) (string, error) {
	secretName := resources.DefaultSecretName(b.Spec.ClusterRef.Name)
	s := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: b.Namespace}, s); err != nil {
		return "", fmt.Errorf("get auth secret %s: %w", secretName, err)
	}
	return string(s.Data[resources.SecretPasswordKey]), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ValkeyBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.ValkeyBackup{}).
		Named("valkeybackup").
		Complete(r)
}
