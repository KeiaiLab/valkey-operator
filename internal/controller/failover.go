/*
Copyright 2026 Keiailab.

Replication mode failover helpers (ADR-0017).
*/

package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/observability"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

// failoverNotReadyThreshold — primary NotReady 가 본 시간 이상 지속해야 failover.
const failoverNotReadyThreshold = 30 * time.Second

// primaryOrdinal — Status.CurrentPrimary 에서 ordinal 추출. 미설정 또는
// 형식 불명 시 0 (pod-0 default).
func primaryOrdinal(v *cachev1alpha1.Valkey) int {
	name := v.Status.CurrentPrimary
	if name == "" {
		return 0
	}
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return 0
	}
	if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
		return n
	}
	return 0
}

// podReadyState — Pod 의 Ready condition 상태 + 마지막 transition 시각.
// NotFound 시 ready=false, since=zero.
func (r *ValkeyReconciler) podReadyState(
	ctx context.Context, podName, namespace string,
) (ready bool, sinceNotReady time.Time, err error) {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{
		Name: podName, Namespace: namespace,
	}, pod); err != nil {
		if errors.IsNotFound(err) {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, err
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			if cond.Status == corev1.ConditionTrue {
				return true, time.Time{}, nil
			}
			return false, cond.LastTransitionTime.Time, nil
		}
	}
	return false, time.Time{}, nil
}

// reconcileFailover — Replication mode 의 자동 failover (ADR-0017).
//
// 동작 조건:
//   - Spec.Mode == Replication
//   - desiredReplicas > 1
//   - Spec.IsAutoFailoverEnabled() == true
//
// 알고리즘:
//  1. 현재 primary (primaryOrdinal) Pod Ready 검증. Ready 면 return.
//  2. NotReady 가 30s+ 미만이면 return (transient flap 보호).
//  3. 모든 replica (primary 제외) 의 INFO replication 수집 → offsets map.
//  4. selectFailoverCandidate 으로 가장 큰 offset 선출.
//  5. PromoteToPrimary (REPLICAOF NO ONE) 발행.
//  6. 다른 replica 에 EnsureReplicaOf 발행.
//  7. v.Status.CurrentPrimary = 새 pod 이름 (in-memory; caller 가 Update).
//
// non-fatal 에러 정책 — caller 가 log + 다음 reconcile 재시도.
func (r *ValkeyReconciler) reconcileFailover(
	ctx context.Context, v *cachev1alpha1.Valkey,
	password string, tlsCfg *tls.Config,
) error {
	if !v.Spec.IsAutoFailoverEnabled() {
		return nil
	}
	if v.Spec.Mode != cachev1alpha1.ModeReplication || v.Spec.Replicas <= 1 {
		return nil
	}

	logger := log.FromContext(ctx)
	curOrdinal := primaryOrdinal(v)
	curPrimary := fmt.Sprintf("%s-%d", v.Name, curOrdinal)

	ready, since, err := r.podReadyState(ctx, curPrimary, v.Namespace)
	if err != nil {
		return fmt.Errorf("podReadyState %s: %w", curPrimary, err)
	}
	if ready {
		return nil
	}
	if since.IsZero() || time.Since(since) < failoverNotReadyThreshold {
		// transient flap 또는 너무 이른 시점.
		return nil
	}

	logger.Info("Primary NotReady 30s+ — initiating failover",
		"primary", curPrimary, "since", since)

	port := int32(resources.PortClient)
	if tlsCfg != nil {
		port = resources.PortTLS
	}

	// 모든 replica 의 offset 수집.
	offsets := map[int]int64{}
	for i := int32(0); i < v.Spec.Replicas; i++ {
		if int(i) == curOrdinal {
			continue
		}
		podName := fmt.Sprintf("%s-%d", v.Name, i)
		ok, _, _ := r.podReadyState(ctx, podName, v.Namespace)
		if !ok {
			continue
		}
		addr := fmt.Sprintf("%s:%d",
			resources.PodFQDN(v.Name, int(i), v.Namespace), port)
		c := dialValkey(addr, password, tlsCfg)
		infoCtx, infoSpan := observability.StartCallSpan(ctx, "Failover/INFO_replication")
		info, infoErr := c.Info(infoCtx, "replication").Result()
		if infoErr != nil {
			infoSpan.RecordError(infoErr)
		}
		infoSpan.End()
		_ = c.Close()
		if infoErr != nil {
			logger.V(1).Info("INFO replication failed — skip replica",
				"pod", podName, "err", infoErr.Error())
			continue
		}
		offsets[int(i)] = vk.ParseReplicationOffset(info)
	}

	if len(offsets) == 0 {
		return fmt.Errorf("no Ready replica candidates for failover (primary=%s)",
			curPrimary)
	}

	newOrdinal, ok := selectFailoverCandidate(offsets)
	if !ok {
		return fmt.Errorf("no failover candidate selected")
	}
	newPrimaryPod := fmt.Sprintf("%s-%d", v.Name, newOrdinal)
	logger.Info("Failover candidate selected",
		"newPrimary", newPrimaryPod, "offset", offsets[newOrdinal])

	// 새 primary 에 PromoteToPrimary.
	newPrimaryAddr := fmt.Sprintf("%s:%d",
		resources.PodFQDN(v.Name, newOrdinal, v.Namespace), port)
	newPrimaryClient := dialValkey(newPrimaryAddr, password, tlsCfg)
	promoteCtx, promoteSpan := observability.StartCallSpan(ctx, "Failover/PromoteToPrimary")
	promoteErr := vk.PromoteToPrimary(promoteCtx, newPrimaryClient)
	if promoteErr != nil {
		promoteSpan.RecordError(promoteErr)
	}
	promoteSpan.End()
	if promoteErr != nil {
		_ = newPrimaryClient.Close()
		return fmt.Errorf("PromoteToPrimary %s: %w", newPrimaryPod, promoteErr)
	}
	_ = newPrimaryClient.Close()

	// 다른 replicas 에 EnsureReplicaOf.
	newPrimaryHost := resources.PodFQDN(v.Name, newOrdinal, v.Namespace)
	replicaCtx, replicaSpan := observability.StartCallSpan(ctx, "Failover/EnsureReplicaOf_all")
	for i := int32(0); i < v.Spec.Replicas; i++ {
		if int(i) == newOrdinal {
			continue
		}
		// 기존 primary 도 살아 돌아왔을 수 있음 — 무관하게 새 primary 가리키도록.
		ok, _, _ := r.podReadyState(replicaCtx, fmt.Sprintf("%s-%d", v.Name, i), v.Namespace)
		if !ok {
			continue
		}
		addr := fmt.Sprintf("%s:%d",
			resources.PodFQDN(v.Name, int(i), v.Namespace), port)
		c := dialValkey(addr, password, tlsCfg)
		_ = vk.EnsureReplicaOf(replicaCtx, c, newPrimaryHost, int(port))
		_ = c.Close()
	}
	replicaSpan.End()

	// Status.CurrentPrimary 갱신 (in-memory).
	v.Status.CurrentPrimary = newPrimaryPod
	logger.Info("Failover completed",
		"oldPrimary", curPrimary, "newPrimary", newPrimaryPod)
	return nil
}

// selectFailoverCandidate — replica ordinal → master_repl_offset/slave_repl_offset
// 맵에서 *가장 큰 offset* replica ordinal 선출. tie 시 ordinal 작은 것.
//
// ADR-0017: 가장 latest replica 가 primary 의 마지막 commit 시점에 가장
// 가까움 → 데이터 손실 최소화.
//
// 빈 맵 → ok=false. 모든 offset 0 일 시 ordinal 가장 작은 replica 반환.
func selectFailoverCandidate(offsets map[int]int64) (ordinal int, ok bool) {
	if len(offsets) == 0 {
		return 0, false
	}
	keys := make([]int, 0, len(offsets))
	for k := range offsets {
		keys = append(keys, k)
	}
	sort.Ints(keys) // tie-break: ordinal 작은 것 우선.

	bestIdx := keys[0]
	bestOffset := offsets[bestIdx]
	for _, k := range keys[1:] {
		if offsets[k] > bestOffset {
			bestOffset = offsets[k]
			bestIdx = k
		}
	}
	return bestIdx, true
}
