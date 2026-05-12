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

package v1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	commonswebhook "github.com/keiailab/operator-commons/pkg/webhook"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

var valkeylog = logf.Log.WithName("valkey-resource")

// SetupValkeyWebhookWithManager registers the webhook for Valkey in the manager.
func SetupValkeyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1alpha1.Valkey{}).
		WithValidator(&ValkeyCustomValidator{}).
		WithDefaulter(&ValkeyCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-cache-keiailab-io-v1alpha1-valkey,mutating=true,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeys,verbs=create;update,versions=v1alpha1,name=mvalkey-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyCustomDefaulter — derived defaults.
type ValkeyCustomDefaulter struct{}

// Default — Mode=Standalone 일 때 Replicas 강제 1, Mode=Replication 일 때 Replicas
// 미명시 시 2 (primary + 1 replica).
//
// Why: CRD 의 +kubebuilder:default 는 부모 객체가 nil 이면 자식 default 를
// 적용하지 않는다. spec.version 이 통째로 누락된 CR (샘플 포함) 의 경우
// version.version 이 빈 문자열이 되어 CRD 정규식 검증을 실패시켜 reconcile
// 무한 루프를 유발한다. 따라서 defaulting webhook 에서 빈 값을 채워야 한다.
func (d *ValkeyCustomDefaulter) Default(_ context.Context, obj *cachev1alpha1.Valkey) error {
	valkeylog.V(1).Info("Defaulting", "name", obj.GetName())
	if obj.Spec.Mode == cachev1alpha1.ModeStandalone {
		obj.Spec.Replicas = 1
	}
	if obj.Spec.Mode == cachev1alpha1.ModeReplication && obj.Spec.Replicas < 2 {
		obj.Spec.Replicas = 2
	}
	if obj.Spec.Version.Version == "" {
		obj.Spec.Version.Version = cachev1alpha1.DefaultValkeyVersion
	}
	if obj.Spec.Version.Image == "" {
		obj.Spec.Version.Image = cachev1alpha1.DefaultValkeyImage
	}
	if obj.Spec.ExternalReplica != nil && obj.Spec.ExternalReplica.Enabled && obj.Spec.ExternalReplica.Port == 0 {
		obj.Spec.ExternalReplica.Port = 6379
	}
	// Auth.Enabled 는 omitempty 부재로 zero value 가 false 로 직렬화 → CRD
	// schema default=true 가 skip 된다. 본 operator 는 ADR-0013 옵션 A 에 따라
	// 항상 auth 를 강제하므로 webhook 에서 true 로 정규화한다.
	obj.Spec.Auth.Enabled = true
	return nil
}

// +kubebuilder:webhook:path=/validate-cache-keiailab-io-v1alpha1-valkey,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeys,verbs=create;update,versions=v1alpha1,name=vvalkey-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyCustomValidator — 조합 / immutable 검증.
type ValkeyCustomValidator struct{}

// ValidateCreate — 신규 CR 검증.
func (v *ValkeyCustomValidator) ValidateCreate(_ context.Context, obj *cachev1alpha1.Valkey) (admission.Warnings, error) {
	if errs := validateValkeySpec(obj); len(errs) > 0 {
		return nil, apiError("Valkey", obj.GetName(), errs)
	}
	return nil, nil
}

// ValidateUpdate — Mode immutable + spec 검증.
func (v *ValkeyCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *cachev1alpha1.Valkey) (admission.Warnings, error) {
	errs := validateValkeySpec(newObj)
	errs = append(errs, validateValkeyImmutable(oldObj, newObj)...)
	if len(errs) > 0 {
		return nil, apiError("Valkey", newObj.GetName(), errs)
	}
	return nil, nil
}

// ValidateDelete — 항상 허용.
func (v *ValkeyCustomValidator) ValidateDelete(_ context.Context, _ *cachev1alpha1.Valkey) (admission.Warnings, error) {
	return nil, nil
}

func validateValkeySpec(v *cachev1alpha1.Valkey) field.ErrorList {
	var errs field.ErrorList
	p := field.NewPath("spec")

	// iteration 31 (2026-05-07): operator-commons/pkg/webhook v0.4.0 위임.
	// 인라인 IsSupportedValkeyVersion + field.NotSupported → ValidateWithPredicate.
	if err := commonswebhook.ValidateWithPredicate(
		p.Child("version", "version"), v.Spec.Version.Version,
		cachev1alpha1.IsSupportedValkeyVersion,
		cachev1alpha1.SupportedValkeyVersions,
	); err != nil {
		errs = append(errs, err)
	}
	if v.Spec.Mode == cachev1alpha1.ModeStandalone && v.Spec.Replicas > 1 {
		errs = append(errs, field.Invalid(
			p.Child("replicas"), v.Spec.Replicas,
			"replicas must be 1 when mode=Standalone",
		))
	}
	if v.Spec.Mode == cachev1alpha1.ModeReplication && v.Spec.Replicas < 2 {
		errs = append(errs, field.Invalid(
			p.Child("replicas"), v.Spec.Replicas,
			"replicas must be >= 2 when mode=Replication",
		))
	}
	if v.Spec.Autoscaling != nil && v.Spec.Autoscaling.Enabled {
		ap := p.Child("autoscaling")
		if v.Spec.Mode != cachev1alpha1.ModeReplication {
			errs = append(errs, field.Forbidden(
				ap, "Autoscaling is supported only when mode=Replication (ADR-0027)",
			))
		}
		if v.Spec.Autoscaling.MinReplicas < 2 {
			errs = append(errs, field.Invalid(
				ap.Child("minReplicas"), v.Spec.Autoscaling.MinReplicas,
				"autoscaling.minReplicas must be >= 2 (Replication topology)",
			))
		}
		if v.Spec.Autoscaling.MaxReplicas < v.Spec.Autoscaling.MinReplicas {
			errs = append(errs, field.Invalid(
				ap.Child("maxReplicas"), v.Spec.Autoscaling.MaxReplicas,
				"autoscaling.maxReplicas must be >= minReplicas",
			))
		}
	}
	errs = append(errs, validateExternalReplica(p.Child("externalReplica"), v)...)
	if v.Spec.TLS != nil && v.Spec.TLS.Enabled {
		// hasCertMgr: CertManager pointer non-nil + (IssuerRef.Name 명시 OR AutoSelfSigned=true).
		// CRD required marker 가 struct value 빈 객체 통과 허용 (cross-cut audit).
		hasCertMgr := v.Spec.TLS.CertManager != nil &&
			(v.Spec.TLS.CertManager.IssuerRef.Name != "" || v.Spec.TLS.CertManager.AutoSelfSigned)
		hasCustom := v.Spec.TLS.CustomCert != nil && v.Spec.TLS.CustomCert.SecretName != ""
		if !hasCertMgr && !hasCustom {
			errs = append(errs, field.Required(
				p.Child("tls"),
				"TLS.Enabled=true requires either tls.certManager (issuerRef or autoSelfSigned) or tls.customCert.secretName",
			))
		}
		if hasCertMgr && hasCustom {
			errs = append(errs, field.Forbidden(
				p.Child("tls"),
				"TLS.CertManager and TLS.CustomCert are mutually exclusive — choose one",
			))
		}
		// AutoSelfSigned + IssuerRef.Name 동시 명시 reject — namespace-scope auto issuer
		// vs 외부 issuer 모호. 둘 중 하나만 선택.
		if v.Spec.TLS.CertManager != nil &&
			v.Spec.TLS.CertManager.AutoSelfSigned &&
			v.Spec.TLS.CertManager.IssuerRef.Name != "" {
			errs = append(errs, field.Forbidden(
				p.Child("tls", "certManager"),
				"autoSelfSigned and issuerRef.name are mutually exclusive — choose one",
			))
		}
	}
	// storage.size 하한 1Gi (cross-cut audit, ADR-0016).
	errs = append(errs, validateStorageSizeMin(p.Child("storage", "size"), v.Spec.Storage.Size)...)
	errs = append(errs, validateStorageMode(p.Child("storage"), v.Spec.Storage)...)

	// storage.storageClassName DNS-1123 subdomain 검증 (ROADMAP RBD storageClass
	// 기본 검증 — Valkey CR 동일 invariant).
	errs = append(errs, validateStorageClassName(p.Child("storage", "storageClassName"), v.Spec.Storage.StorageClassName)...)

	// auth.users[].passwordSecretRef omitempty trap — ValkeyUser 의 secret ref
	// 가 struct value 라 빈 객체 통과. controller 자동 생성 path 없음 (Auth.
	// PasswordSecretRef nil 만 random 자동 생성 — ADR-0014 intentional design).
	errs = append(errs, validateUsersSecretRefs(p.Child("auth", "users"), v.Spec.Auth.Users)...)

	// pod.topologySpreadConstraints 일관성 검증 (ROADMAP topology spread). Pod nil
	// 이면 default TSC 가 controller 가 주입 — 사용자 명시 시만 검증.
	if v.Spec.Pod != nil {
		errs = append(errs, validateTopologySpread(
			p.Child("pod", "topologySpreadConstraints"),
			v.Spec.Pod.TopologySpreadConstraints,
		)...)
	}

	// pod.{securityContext,containerSecurityContext} PSA restricted 가드.
	errs = append(errs, validatePodSecurityRestricted(p.Child("pod"), v.Spec.Pod)...)

	if len(v.Spec.Auth.Users) > 0 && !v.Spec.Auth.Enabled {
		errs = append(errs, field.Forbidden(
			p.Child("auth"),
			"auth.users requires auth.enabled=true",
		))
	}
	return errs
}

func validateExternalReplica(path *field.Path, v *cachev1alpha1.Valkey) field.ErrorList {
	if v.Spec.ExternalReplica == nil || !v.Spec.ExternalReplica.Enabled {
		return nil
	}
	var errs field.ErrorList
	if v.Spec.Mode != cachev1alpha1.ModeStandalone {
		errs = append(errs, field.Forbidden(
			path, "externalReplica is supported only when mode=Standalone; use operator Replication/Cluster for internal HA",
		))
	}
	if v.Spec.ExternalReplica.Host == "" {
		errs = append(errs, field.Required(
			path.Child("host"),
			"externalReplica.host is required when externalReplica.enabled=true",
		))
	}
	if v.Spec.ExternalReplica.Port < 0 || v.Spec.ExternalReplica.Port > 65535 {
		errs = append(errs, field.Invalid(
			path.Child("port"), v.Spec.ExternalReplica.Port,
			"externalReplica.port must be between 1 and 65535",
		))
	}
	if v.Spec.ExternalReplica.Auth != nil && v.Spec.ExternalReplica.Auth.Enabled {
		errs = append(errs, validateSecretKeySelector(
			path.Child("auth", "passwordSecretRef"),
			v.Spec.ExternalReplica.Auth.PasswordSecretRef,
			"externalReplica.auth.passwordSecretRef",
		)...)
	}
	return errs
}

func validateValkeyImmutable(oldObj, newObj *cachev1alpha1.Valkey) field.ErrorList {
	var errs field.ErrorList
	p := field.NewPath("spec")
	if oldObj.Spec.Mode != "" && oldObj.Spec.Mode != newObj.Spec.Mode {
		errs = append(errs, field.Forbidden(
			p.Child("mode"),
			"spec.mode is immutable (Standalone↔Replication transitions require manual migration)",
		))
	}
	if oldObj.Spec.Storage.StorageClassName != "" &&
		oldObj.Spec.Storage.StorageClassName != newObj.Spec.Storage.StorageClassName {
		errs = append(errs, field.Forbidden(
			p.Child("storage", "storageClassName"),
			"storage.storageClassName is immutable",
		))
	}
	if !oldObj.Spec.Storage.Size.IsZero() &&
		newObj.Spec.Storage.Size.Cmp(oldObj.Spec.Storage.Size) < 0 {
		errs = append(errs, field.Forbidden(
			p.Child("storage", "size"),
			"storage.size cannot be decreased (Kubernetes PVC shrink is unsupported)",
		))
	}
	oldTLS := oldObj.Spec.TLS != nil && oldObj.Spec.TLS.Enabled
	newTLS := newObj.Spec.TLS != nil && newObj.Spec.TLS.Enabled
	if oldTLS != newTLS {
		errs = append(errs, field.Forbidden(
			p.Child("tls", "enabled"),
			"tls.enabled is immutable",
		))
	}
	return errs
}
