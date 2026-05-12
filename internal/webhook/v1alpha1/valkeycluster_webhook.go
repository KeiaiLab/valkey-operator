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

// Package v1alpha1 — Valkey / ValkeyCluster admission webhook.
//
// 본 webhook 의 역할:
//  1. Defaulting (mutating): CRD marker 가 처리 못 하는 *조합 derivable* 기본값.
//     단순 zero → 상수 기본값은 CRD marker 가 처리 — 본 webhook 은 *조건부 derived*
//     필드만 다룬다.
//  2. Validation: *조합 검증* (CRD marker 의 정적 단일 필드 검증으로 표현 불가).
//     - immutable 필드 가드 (예: Storage.Size 변경 금지).
//     - 모순 조합 reject (예: AutoFailover=true + ReplicasPerShard=0).
//     - TLS Enabled 시 CertManager / CustomCert 둘 중 하나 필수.
package v1alpha1

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	commonswebhook "github.com/keiailab/operator-commons/pkg/webhook"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

var valkeyclusterlog = logf.Log.WithName("valkeycluster-resource")

// SetupValkeyClusterWebhookWithManager registers the webhook for ValkeyCluster in the manager.
func SetupValkeyClusterWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &cachev1alpha1.ValkeyCluster{}).
		WithValidator(&ValkeyClusterCustomValidator{}).
		WithDefaulter(&ValkeyClusterCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-cache-keiailab-io-v1alpha1-valkeycluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeyclusters,verbs=create;update,versions=v1alpha1,name=mvalkeycluster-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyClusterCustomDefaulter — derived defaults (CRD marker 미커버 영역).
type ValkeyClusterCustomDefaulter struct{}

// Default — admission 시점 derived 기본값 적용.
//
// CRD marker 는 *상수 기본값* 만 처리. 본 함수는 *조건부* 보정만 다룬다 — 그래야
// 한 곳에서 logic 추적 가능.
func (d *ValkeyClusterCustomDefaulter) Default(_ context.Context, obj *cachev1alpha1.ValkeyCluster) error {
	valkeyclusterlog.V(1).Info("Defaulting", "name", obj.GetName())

	// AutoFailover 의 zero-value 와 명시 false 를 webhook 에서 구별 불가 — CRD
	// default=true 가 강한 신호이므로 여기서 손대지 않음 (mutating webhook 의 한계).

	// SlotMigration 빈 문자열 → "Auto" (CRD default 와 같지만 명시).
	if obj.Spec.SlotMigration == "" {
		obj.Spec.SlotMigration = cachev1alpha1.SlotMigrationAuto
	}
	// `omitempty` 없는 required 필드는 mutating webhook 이 zero value 로 직렬화하면
	// CRD schema default 가 skip 되어 결국 0 으로 남는다. 따라서 여기서 직접 채움.
	if obj.Spec.Shards == 0 {
		obj.Spec.Shards = 3
	}
	// ReplicasPerShard: AutoFailover=true 가 default 인데 0 이면 webhook validator
	// 가 reject 한다. CRD default=1 이지만 omitempty 부재로 schema default 가 skip
	// 되므로 여기서 채움.
	if obj.Spec.ReplicasPerShard == 0 {
		obj.Spec.ReplicasPerShard = 1
	}
	if obj.Spec.Version.Version == "" {
		obj.Spec.Version.Version = cachev1alpha1.DefaultValkeyVersion
	}
	if obj.Spec.Version.Image == "" {
		obj.Spec.Version.Image = cachev1alpha1.DefaultValkeyImage
	}
	// Auth.Enabled — ADR-0013 옵션 A: 항상 강제 (보안 기본값).
	obj.Spec.Auth.Enabled = true
	return nil
}

// +kubebuilder:webhook:path=/validate-cache-keiailab-io-v1alpha1-valkeycluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=cache.keiailab.io,resources=valkeyclusters,verbs=create;update,versions=v1alpha1,name=vvalkeycluster-v1alpha1.kb.io,admissionReviewVersions=v1

// ValkeyClusterCustomValidator — 조합 / immutable 검증.
type ValkeyClusterCustomValidator struct{}

// ValidateCreate — 신규 CR 검증.
func (v *ValkeyClusterCustomValidator) ValidateCreate(_ context.Context, obj *cachev1alpha1.ValkeyCluster) (admission.Warnings, error) {
	valkeyclusterlog.V(1).Info("ValidateCreate", "name", obj.GetName())
	if errs := validateClusterSpec(obj); len(errs) > 0 {
		return nil, apiError("ValkeyCluster", obj.GetName(), errs)
	}
	return nil, nil
}

// ValidateUpdate — 기존 CR 변경 검증. spec 검증 + immutable 가드.
func (v *ValkeyClusterCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *cachev1alpha1.ValkeyCluster) (admission.Warnings, error) {
	valkeyclusterlog.V(1).Info("ValidateUpdate", "name", newObj.GetName())
	errs := validateClusterSpec(newObj)
	errs = append(errs, validateClusterImmutable(oldObj, newObj)...)
	if len(errs) > 0 {
		return nil, apiError("ValkeyCluster", newObj.GetName(), errs)
	}
	return nil, nil
}

// ValidateDelete — 삭제 검증. 현재는 항상 허용 (finalizer 가 graceful teardown 처리).
func (v *ValkeyClusterCustomValidator) ValidateDelete(_ context.Context, _ *cachev1alpha1.ValkeyCluster) (admission.Warnings, error) {
	return nil, nil
}

// validateClusterSpec — 조합 검증. 정적 단일 필드 검증은 CRD marker 가 담당.
func validateClusterSpec(vc *cachev1alpha1.ValkeyCluster) field.ErrorList {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// iteration 31 (2026-05-07): operator-commons/pkg/webhook v0.4.0 위임.
	if err := commonswebhook.ValidateWithPredicate(
		specPath.Child("version", "version"), vc.Spec.Version.Version,
		cachev1alpha1.IsSupportedValkeyVersion,
		cachev1alpha1.SupportedValkeyVersions,
	); err != nil {
		errs = append(errs, err)
	}

	// AutoFailover=true + ReplicasPerShard=0 → failover 불가 (replica 부재).
	//
	// ADR-0017 Type A' (조건부 unreachable, defensive 유지):
	//   - ValkeyClusterSpec.ReplicasPerShard 는 'json:"replicasPerShard"' (no
	//     omitempty), CRD '+kubebuilder:default=1', mutating defaulter (Default
	//     함수 위) 가 명시 0→1 보강.
	//   - webhook.enabled=true 환경 (operator 정상 운영) 에서는 mutating defaulter
	//     가 0→1 변환 → 본 invariant 도달 안 함.
	//   - webhook.enabled=false 환경 (CRD-only 모드, helm opt-out) 에서는 mutating
	//     defaulter 우회 + CRD default 가 *명시 0* 무력화 못 함 → reachable.
	//   - 따라서 *제거 금지* — defensive 가드. it47 commit 5f3f91c 의 envtest
	//     'unreachable' 분석은 webhook.enabled=true 한정 시나리오.
	if vc.Spec.AutoFailover && vc.Spec.ReplicasPerShard == 0 {
		errs = append(errs, field.Forbidden(
			specPath.Child("autoFailover"),
			"autoFailover=true requires replicasPerShard >= 1 (no replicas means no failover possible)",
		))
	}

	// 총 노드 수 상한 — Valkey cluster 권장 (>100 노드는 운영 부담 + gossip 오버헤드).
	total := vc.Spec.TotalNodes()
	if total > 100 {
		errs = append(errs, field.Invalid(
			specPath,
			total,
			"total nodes (shards * (1 + replicasPerShard)) must not exceed 100",
		))
	}

	// TLS.Enabled=true 면 CertManager 또는 CustomCert 중 하나는 명시.
	if vc.Spec.TLS != nil && vc.Spec.TLS.Enabled {
		// hasCertMgr: CertManager pointer non-nil + (IssuerRef.Name 명시 OR AutoSelfSigned=true).
		hasCertMgr := vc.Spec.TLS.CertManager != nil &&
			(vc.Spec.TLS.CertManager.IssuerRef.Name != "" || vc.Spec.TLS.CertManager.AutoSelfSigned)
		hasCustom := vc.Spec.TLS.CustomCert != nil && vc.Spec.TLS.CustomCert.SecretName != ""
		if !hasCertMgr && !hasCustom {
			errs = append(errs, field.Required(
				specPath.Child("tls"),
				"TLS.Enabled=true requires either tls.certManager (issuerRef or autoSelfSigned) or tls.customCert.secretName",
			))
		}
		if hasCertMgr && hasCustom {
			errs = append(errs, field.Forbidden(
				specPath.Child("tls"),
				"TLS.CertManager and TLS.CustomCert are mutually exclusive — choose one",
			))
		}
		// AutoSelfSigned + IssuerRef.Name 동시 명시 reject — 모호.
		if vc.Spec.TLS.CertManager != nil &&
			vc.Spec.TLS.CertManager.AutoSelfSigned &&
			vc.Spec.TLS.CertManager.IssuerRef.Name != "" {
			errs = append(errs, field.Forbidden(
				specPath.Child("tls", "certManager"),
				"autoSelfSigned and issuerRef.name are mutually exclusive — choose one",
			))
		}
	}

	// SlotMigration=Manual 일 때 AutoFailover=true 는 허용 — slot 이동 정책과 failover
	// 는 독립적 (모순 아님). 검증 항목 아님.

	// Auth.Users 사용 시 Auth.Enabled=true 필수.
	if len(vc.Spec.Auth.Users) > 0 && !vc.Spec.Auth.Enabled {
		errs = append(errs, field.Forbidden(
			specPath.Child("auth"),
			"auth.users requires auth.enabled=true",
		))
	}

	// storage.size 하한 1Gi (cross-cut audit, mongodb-operator it46 step 7
	// commit 8b2414f 와 동일 invariant). RDB snapshot + AOF 합산 floor 보장.
	errs = append(errs, validateStorageSizeMin(specPath.Child("storage", "size"), vc.Spec.Storage.Size)...)
	errs = append(errs, validateStorageMode(specPath.Child("storage"), vc.Spec.Storage)...)

	// storage.storageClassName DNS-1123 subdomain 검증 (ROADMAP RBD storageClass
	// 기본 검증 — ceph-rbd 등 RBD 계열 이름 사전 reject 패턴).
	errs = append(errs, validateStorageClassName(specPath.Child("storage", "storageClassName"), vc.Spec.Storage.StorageClassName)...)

	// auth.users[].passwordSecretRef cross-cut (Valkey single-CR webhook 와 동일).
	errs = append(errs, validateUsersSecretRefs(specPath.Child("auth", "users"), vc.Spec.Auth.Users)...)

	// pod.topologySpreadConstraints 일관성 검증 (ROADMAP topology spread).
	if vc.Spec.Pod != nil {
		errs = append(errs, validateTopologySpread(
			specPath.Child("pod", "topologySpreadConstraints"),
			vc.Spec.Pod.TopologySpreadConstraints,
		)...)
	}

	// pod.{securityContext,containerSecurityContext} PSA restricted 가드.
	errs = append(errs, validatePodSecurityRestricted(specPath.Child("pod"), vc.Spec.Pod)...)

	return errs
}

// validateUsersSecretRefs — ValkeyUser 의 PasswordSecretRef name+key 둘 다 non-
// empty 강제. SecretKeySelector 가 struct value 라 omitempty trap 잠재. ADR-0016
// cross-cut audit pattern.
func validateUsersSecretRefs(path *field.Path, users []cachev1alpha1.ValkeyUser) field.ErrorList {
	var errs field.ErrorList
	for i, u := range users {
		userPath := path.Index(i)
		if u.Name == "" {
			errs = append(errs, field.Invalid(
				userPath.Child("name"), "",
				"users[].name must be non-empty",
			))
		}
		if u.PasswordSecretRef.Name == "" {
			errs = append(errs, field.Invalid(
				userPath.Child("passwordSecretRef", "name"), "",
				"users[].passwordSecretRef.name must be non-empty (no auto-generation for individual users — ADR-0014)",
			))
		}
		if u.PasswordSecretRef.Key == "" {
			errs = append(errs, field.Invalid(
				userPath.Child("passwordSecretRef", "key"), "",
				"users[].passwordSecretRef.key must be non-empty (Secret 의 어느 key 가 password 인지 명시 필요)",
			))
		}
	}
	return errs
}

func validateSecretKeySelector(
	path *field.Path,
	ref *corev1.SecretKeySelector,
	label string,
) field.ErrorList {
	if ref == nil {
		return field.ErrorList{field.Required(path, label+" is required")}
	}
	var errs field.ErrorList
	if ref.Name == "" {
		errs = append(errs, field.Invalid(path.Child("name"), "", label+".name must be non-empty"))
	}
	if ref.Key == "" {
		errs = append(errs, field.Invalid(path.Child("key"), "", label+".key must be non-empty"))
	}
	return errs
}

func validateStorageMode(path *field.Path, s cachev1alpha1.StorageSpec) field.ErrorList {
	if s.Ephemeral && s.ExistingClaim != "" {
		return field.ErrorList{field.Forbidden(
			path,
			"storage.ephemeral=true and storage.existingClaim are mutually exclusive",
		)}
	}
	return nil
}

// validateStorageClassName — storage.storageClassName 의 기본 형식 검증.
//
// Why: 사용자가 명시한 StorageClassName 은 K8s 가 *동일 이름의 StorageClass*
// 리소스를 lookup 한다. 잘못된 이름 (대문자 / 언더스코어 / 길이 초과 등) 은
// 즉시 PVC binding 실패로 이어져 STS pod 가 Pending 영구 정지된다. argos
// 클러스터의 default class `ceph-rbd` 처럼 RBD 계열 이름은 모두 DNS-1123
// subdomain 규칙을 따른다 — webhook 에서 사전 reject 하면 PVC 단계까지 가지
// 않고 즉시 사용자 피드백 가능.
//
// 정책: zero (unset) → cluster default class 사용 → 통과. non-empty →
// DNS-1123 subdomain (lowercase alphanumeric / '-' / '.', 253자 이하) 검증.
func validateStorageClassName(path *field.Path, name string) field.ErrorList {
	if name == "" {
		return nil
	}
	if msgs := validation.IsDNS1123Subdomain(name); len(msgs) > 0 {
		return field.ErrorList{field.Invalid(
			path, name,
			"storage.storageClassName must be a DNS-1123 subdomain: "+msgs[0],
		)}
	}
	return nil
}

// validatePodSecurityRestricted — spec.pod.{securityContext,containerSecurityContext}
// 의 사용자 명시값이 PSA "restricted" profile 을 위반하지 않는지 검증.
//
// Why: operator 의 resources/statefulset.go 는 default 로 restricted 호환 값을
// 주입한다. 그러나 사용자가 spec.pod 에 SecurityContext / ContainerSecurityContext
// 를 *명시* 하면 그 값이 override 되어 PSA enforce restricted namespace 에서
// admission webhook (K8s 자체) 에 reject 된다. operator webhook 이 사전 reject
// 하여 즉시 사용자 피드백.
//
// 정책 (restricted profile 핵심 항목):
//   - PodSecurityContext.RunAsNonRoot == false → reject
//   - ContainerSecurityContext.RunAsNonRoot == false → reject
//   - ContainerSecurityContext.Privileged == true → reject
//   - ContainerSecurityContext.AllowPrivilegeEscalation == true → reject
//   - ContainerSecurityContext.RunAsUser == 0 → reject (root user)
//
// nil 또는 미지정 (omitempty) 은 operator default 가 채우므로 통과.
func validatePodSecurityRestricted(path *field.Path, pod *cachev1alpha1.PodSpec) field.ErrorList {
	if pod == nil {
		return nil
	}
	var errs field.ErrorList
	if pod.SecurityContext != nil && pod.SecurityContext.RunAsNonRoot != nil && !*pod.SecurityContext.RunAsNonRoot {
		errs = append(errs, field.Forbidden(
			path.Child("securityContext", "runAsNonRoot"),
			"runAsNonRoot=false violates PodSecurity restricted profile",
		))
	}
	if pod.SecurityContext != nil && pod.SecurityContext.RunAsUser != nil && *pod.SecurityContext.RunAsUser == 0 {
		errs = append(errs, field.Forbidden(
			path.Child("securityContext", "runAsUser"),
			"runAsUser=0 (root) violates PodSecurity restricted profile",
		))
	}
	c := pod.ContainerSecurityContext
	if c != nil {
		if c.RunAsNonRoot != nil && !*c.RunAsNonRoot {
			errs = append(errs, field.Forbidden(
				path.Child("containerSecurityContext", "runAsNonRoot"),
				"runAsNonRoot=false violates PodSecurity restricted profile",
			))
		}
		if c.RunAsUser != nil && *c.RunAsUser == 0 {
			errs = append(errs, field.Forbidden(
				path.Child("containerSecurityContext", "runAsUser"),
				"runAsUser=0 (root) violates PodSecurity restricted profile",
			))
		}
		if c.Privileged != nil && *c.Privileged {
			errs = append(errs, field.Forbidden(
				path.Child("containerSecurityContext", "privileged"),
				"privileged=true violates PodSecurity restricted profile",
			))
		}
		if c.AllowPrivilegeEscalation != nil && *c.AllowPrivilegeEscalation {
			errs = append(errs, field.Forbidden(
				path.Child("containerSecurityContext", "allowPrivilegeEscalation"),
				"allowPrivilegeEscalation=true violates PodSecurity restricted profile",
			))
		}
	}
	return errs
}

// validateTopologySpread — spec.pod.topologySpreadConstraints 일관성 검증.
//
// Why: corev1.TopologySpreadConstraint 는 K8s API server 가 *create* 시점에 일부
// 검증하지만 (MaxSkew>0, WhenUnsatisfiable enum), CR spec 안에 *복사된 값* 은
// API server 의 PodSpec validation 을 거치지 않고 STS 가 생성된 *이후에야* kubelet
// 단계에서 실패한다. 그 결과 admission 통과 → STS 생성 → pod 스케줄 영구 Pending
// 으로 이어진다. webhook 에서 사전 reject 하여 즉시 사용자 피드백.
//
// 정책:
//   - MaxSkew >= 1 (K8s 요구사항 동등)
//   - TopologyKey non-empty
//   - WhenUnsatisfiable ∈ {DoNotSchedule, ScheduleAnyway}
//   - 같은 TopologyKey 중복 reject — 모순된 분포 정책 사전 차단
func validateTopologySpread(path *field.Path, tscs []corev1.TopologySpreadConstraint) field.ErrorList {
	var errs field.ErrorList
	seenKeys := make(map[string]int, len(tscs))
	for i, c := range tscs {
		ip := path.Index(i)
		if c.MaxSkew < 1 {
			errs = append(errs, field.Invalid(
				ip.Child("maxSkew"), c.MaxSkew,
				"maxSkew must be >= 1",
			))
		}
		if c.TopologyKey == "" {
			errs = append(errs, field.Required(
				ip.Child("topologyKey"),
				"topologyKey must be non-empty (e.g. topology.kubernetes.io/zone)",
			))
		}
		switch c.WhenUnsatisfiable {
		case corev1.DoNotSchedule, corev1.ScheduleAnyway:
		case "":
			errs = append(errs, field.Required(
				ip.Child("whenUnsatisfiable"),
				"whenUnsatisfiable must be DoNotSchedule or ScheduleAnyway",
			))
		default:
			errs = append(errs, field.NotSupported(
				ip.Child("whenUnsatisfiable"), string(c.WhenUnsatisfiable),
				[]string{string(corev1.DoNotSchedule), string(corev1.ScheduleAnyway)},
			))
		}
		if c.TopologyKey != "" {
			if prev, ok := seenKeys[c.TopologyKey]; ok {
				errs = append(errs, field.Duplicate(
					ip.Child("topologyKey"),
					"topologyKey "+c.TopologyKey+" already specified at index "+strconv.Itoa(prev),
				))
			} else {
				seenKeys[c.TopologyKey] = i
			}
		}
	}
	return errs
}

// validateStorageSizeMin — storage.size 하한 1Gi 검증. mongodb-operator it46
// step 7 와 동일 패턴 (cross-cut audit per ADR-0016). zero (unset) 은 CRD
// default 가 채우므로 본 함수 도달 시점엔 양수.
func validateStorageSizeMin(path *field.Path, size resource.Quantity) field.ErrorList {
	if size.IsZero() {
		return nil
	}
	min := resource.MustParse("1Gi")
	if size.Cmp(min) < 0 {
		return field.ErrorList{field.Invalid(
			path, size.String(),
			"storage.size must be >= 1Gi — RDB snapshot + AOF data dir floor",
		)}
	}
	return nil
}

// validateClusterImmutable — 변경 금지 필드 가드.
//
// 변경하면 *데이터 손실 또는 cluster topology 깨짐* 위험인 필드:
//   - Storage.StorageClassName / DataDirPath: PVC 재생성 = 데이터 손실.
//   - TLS.Enabled true → false 또는 false → true: 진행 중인 client 연결 단절.
func validateClusterImmutable(oldObj, newObj *cachev1alpha1.ValkeyCluster) field.ErrorList {
	var errs field.ErrorList
	p := field.NewPath("spec")

	if oldObj.Spec.Storage.StorageClassName != "" &&
		oldObj.Spec.Storage.StorageClassName != newObj.Spec.Storage.StorageClassName {
		errs = append(errs, field.Forbidden(
			p.Child("storage", "storageClassName"),
			"storage.storageClassName is immutable",
		))
	}
	if oldObj.Spec.Storage.DataDirPath != "" &&
		oldObj.Spec.Storage.DataDirPath != newObj.Spec.Storage.DataDirPath {
		errs = append(errs, field.Forbidden(
			p.Child("storage", "dataDirPath"),
			"storage.dataDirPath is immutable",
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
			"tls.enabled is immutable (toggling breaks active client connections)",
		))
	}

	return errs
}

// apiError — field.ErrorList 를 status=Invalid 의 K8s API error 로 변환.
func apiError(kind, name string, errs field.ErrorList) error {
	return apierrors.NewInvalid(
		schema.GroupKind{Group: cachev1alpha1.GroupVersion.Group, Kind: kind},
		name, errs,
	)
}

// _ runtime.Object 를 위한 명시적 import (admission webhook 등록 시 사용).
var _ runtime.Object = &cachev1alpha1.ValkeyCluster{}
