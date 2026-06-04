/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// 도메인 타입별 controllerutil.CreateOrUpdate wrapper.
// internal/controller/resources_apply.go 패턴 차용. 핵심 학습:
// - immutable 필드 (Selector / ServiceName / VolumeClaimTemplates / ClusterIP) 는
//   Create 시점에만 설정.
// - Deployment 의 nil-pointer 필드 (RevisionHistoryLimit, ProgressDeadlineSeconds) 는
//   nil-guard 필수 — server-default 와 ping-pong 시 generation 폭주.

func applyConfigMap(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner client.Object, desired *corev1.ConfigMap) error {
	target := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, c, target, func() error {
		target.Labels = desired.Labels
		target.Annotations = desired.Annotations
		target.Data = desired.Data
		target.BinaryData = desired.BinaryData
		return controllerutil.SetControllerReference(owner, target, scheme)
	})
	return err
}

func applyService(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner client.Object, desired *corev1.Service) error {
	target := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, c, target, func() error {
		target.Labels = desired.Labels
		target.Annotations = desired.Annotations
		if target.CreationTimestamp.IsZero() {
			target.Spec.ClusterIP = desired.Spec.ClusterIP
			target.Spec.IPFamilies = desired.Spec.IPFamilies
			target.Spec.IPFamilyPolicy = desired.Spec.IPFamilyPolicy
		}
		target.Spec.Ports = desired.Spec.Ports
		target.Spec.Selector = desired.Spec.Selector
		target.Spec.Type = desired.Spec.Type
		target.Spec.PublishNotReadyAddresses = desired.Spec.PublishNotReadyAddresses
		target.Spec.SessionAffinity = desired.Spec.SessionAffinity
		target.Spec.LoadBalancerSourceRanges = desired.Spec.LoadBalancerSourceRanges
		target.Spec.ExternalTrafficPolicy = desired.Spec.ExternalTrafficPolicy
		return controllerutil.SetControllerReference(owner, target, scheme)
	})
	return err
}

// applyStatefulSet — preserveReplicas 는 ScalePolicy.Deliberate=false 가드 또는
// HPA 적용 중 spec.Replicas 보존을 위함.
func applyStatefulSet(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner client.Object, desired *appsv1.StatefulSet, preserveReplicas bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		target := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
		_, err := controllerutil.CreateOrUpdate(ctx, c, target, func() error {
			target.Labels = desired.Labels
			target.Annotations = desired.Annotations
			if target.CreationTimestamp.IsZero() {
				target.Spec = desired.Spec
			} else {
				if !preserveReplicas {
					target.Spec.Replicas = desired.Spec.Replicas
				}
				target.Spec.Template = desired.Spec.Template
				target.Spec.UpdateStrategy = desired.Spec.UpdateStrategy
				target.Spec.MinReadySeconds = desired.Spec.MinReadySeconds
				target.Spec.PersistentVolumeClaimRetentionPolicy = desired.Spec.PersistentVolumeClaimRetentionPolicy
			}
			return controllerutil.SetControllerReference(owner, target, scheme)
		})
		return err
	})
}

func applyNetworkPolicy(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner client.Object, desired *networkingv1.NetworkPolicy) error {
	target := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, c, target, func() error {
		target.Labels = desired.Labels
		target.Annotations = desired.Annotations
		target.Spec.PodSelector = desired.Spec.PodSelector
		target.Spec.PolicyTypes = desired.Spec.PolicyTypes
		target.Spec.Ingress = desired.Spec.Ingress
		target.Spec.Egress = desired.Spec.Egress
		return controllerutil.SetControllerReference(owner, target, scheme)
	})
	return err
}

// applyServiceMonitor — Prometheus Operator CRD 가 클러스터에 미설치인 경우 NotFound /
// NoMatch 에러를 fail-soft 처리 (nil 반환). 사용자가 prometheus-operator 설치 후 다음
// reconcile 에서 자동 생성.
func applyServiceMonitor(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner client.Object, desired *unstructured.Unstructured) error {
	if desired == nil {
		return nil
	}
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(desired.GroupVersionKind())
	target.SetName(desired.GetName())
	target.SetNamespace(desired.GetNamespace())
	_, err := controllerutil.CreateOrUpdate(ctx, c, target, func() error {
		target.SetLabels(desired.GetLabels())
		target.SetAnnotations(desired.GetAnnotations())
		target.Object["spec"] = desired.Object["spec"]
		return controllerutil.SetControllerReference(owner, target, scheme)
	})
	if err != nil && (apierrors.IsNotFound(err) || meta.IsNoMatchError(err)) {
		return nil
	}
	return err
}

// applyHPA — Spec.Autoscaling.Enabled=true 시 HPA CR 생성/갱신.
// desired==nil 면 *기존 HPA 삭제* (Autoscaling toggle off 회복).
// ADR-0027.
func applyHPA(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner client.Object, name, namespace string, desired *autoscalingv2.HorizontalPodAutoscaler) error {
	if desired == nil {
		// disable: 기존 HPA 삭제 (있으면).
		existing := &autoscalingv2.HorizontalPodAutoscaler{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, existing); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return c.Delete(ctx, existing)
	}
	target := &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, c, target, func() error {
		target.Labels = desired.Labels
		target.Annotations = desired.Annotations
		target.Spec = desired.Spec
		return controllerutil.SetControllerReference(owner, target, scheme)
	})
	return err
}

func applyPDB(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner client.Object, desired *policyv1.PodDisruptionBudget) error {
	target := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, c, target, func() error {
		target.Labels = desired.Labels
		target.Annotations = desired.Annotations
		target.Spec.Selector = desired.Spec.Selector
		target.Spec.MinAvailable = desired.Spec.MinAvailable
		target.Spec.MaxUnavailable = desired.Spec.MaxUnavailable
		return controllerutil.SetControllerReference(owner, target, scheme)
	})
	return err
}
