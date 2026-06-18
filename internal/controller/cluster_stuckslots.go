/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	commonsevents "github.com/keiailab/keiailab-commons/pkg/events"
	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/observability"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

// 결함 ⑤ — partial-slot outage 자가복구.
//
// 증상: node churn 후 cluster_state:ok + slots_assigned:16384 이지만 slots_ok 가
// 16384 미만 — 한 shard 의 slot 이 `fail` flag 가 붙은 master 소유로 남아 그만큼의
// 키스페이스가 DOWN. 기존 health gate (cluster_state / slots_assigned) 는 이를
// "정상" 으로 오판해 self-heal 이 멈췄다.
//
// 본 path 는:
//  1. vk.IsClusterDegraded: slots_ok<16384 또는 fail master 가 slot 소유 시 degraded.
//  2. vk.DetectStuckSlotHeals: 각 stuck master 마다 takeover 가능한 healthy replica 를
//     선정 (순수 함수, 단위테스트).
//  3. healStuckSlots: 선정된 replica 에 CLUSTER FAILOVER TAKEOVER 발행 → slot 소유권이
//     replica 로 승계되어 slot 이 ok 로 복귀.
//
// 보수성 / 멱등:
//   - `fail` flag 는 Valkey 가 node-timeout 경과 + gossip 합의 후에만 설정한다. 따라서
//     flag 의 존재 자체가 "node-timeout 만큼 기다렸다" 는 신호 — converging cluster 를
//     thrash 하지 않는다 (pfail/fail? 는 takeover 대상으로 보지 않는다).
//   - takeover 후 master 가 다시 healthy 가 되면 stuck 조건이 사라져 자동 no-op.
//   - healthy replica 가 없는 stuck master 는 TAKEOVER 하지 않고 (데이터 보존) 경고만.

// reconcileStuckSlots — partial-slot outage 게이트 + 복구. allReady && cluster 도달
// 가능한 상태에서 호출. 반환값은 발행한 takeover 수 (0 이면 no-op).
func (r *ValkeyClusterReconciler) reconcileStuckSlots(
	ctx context.Context, vc *cachev1alpha1.ValkeyCluster,
	password string, info *vk.ClusterInfo, nodes []vk.NodeView,
) (int, error) {
	if info == nil || len(nodes) == 0 {
		return 0, nil
	}
	if !vk.IsClusterDegraded(info, nodes) {
		return 0, nil
	}

	logger := log.FromContext(ctx)
	heals := vk.DetectStuckSlotHeals(nodes)
	if len(heals) == 0 {
		// degraded 이지만 안전한 takeover 후보가 없다 (예: fail master 에 healthy
		// replica 부재, 또는 slots_ok 저하가 pfail 같은 transient 상태). 이번 reconcile
		// 에서는 행동하지 않고 가시화만 — 다음 reconcile 에서 gossip 수렴 또는 멤버십
		// 자가복구(결함 ③)가 replica 를 재합류시킨 뒤 재평가한다.
		logger.Info("ValkeyCluster degraded (partial-slot outage) but no safe takeover candidate",
			"slotsOK", info.SlotsOK, "slotsAssigned", info.SlotsAssigned, "state", info.State)
		commonsevents.EmitWarningf(r.Recorder, vc, "PartialSlotOutage",
			"slots_ok=%d/%d — fail master 의 slot 이 stuck 이나 takeover 가능한 healthy replica 없음",
			info.SlotsOK, vk.ClusterTotalSlots)
		return 0, nil
	}

	logger.Info("ValkeyCluster partial-slot outage detected; healing via TAKEOVER",
		"slotsOK", info.SlotsOK, "stuckShards", len(heals))

	return r.healStuckSlots(ctx, vc, password, heals)
}

// healStuckSlots — 각 heal 계획의 replica 에 CLUSTER FAILOVER TAKEOVER 발행.
//
// TAKEOVER 는 합의 없이 replica 가 slot + epoch 를 승계 — master 가 이미 fail 일 때만
// 안전하며 (DetectStuckSlotHeals 가 그것을 보장), split-brain 위험 때문에 그 외에는
// 호출하지 않는다.
func (r *ValkeyClusterReconciler) healStuckSlots(
	ctx context.Context, vc *cachev1alpha1.ValkeyCluster,
	password string, heals []vk.StuckSlotHeal,
) (int, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyCluster/HealStuckSlots")
	defer span.End()
	logger := log.FromContext(ctx)

	tlsCfg, err := r.tlsConfigForCluster(ctx, vc)
	if err != nil {
		return 0, fmt.Errorf("tls config: %w", err)
	}

	var done int
	var firstErr error
	for _, h := range heals {
		c := dialPod(h.TakeoverReplicaAddr, password, tlsCfg)
		takeoverCtx, takeoverSpan := observability.StartCallSpan(ctx, "ValkeyCluster/FailoverTakeover")
		takeErr := vk.ClusterFailoverTakeover(takeoverCtx, c)
		if takeErr != nil {
			takeoverSpan.RecordError(takeErr)
		}
		takeoverSpan.End()
		_ = c.Close()
		if takeErr != nil {
			logger.Error(takeErr, "CLUSTER FAILOVER TAKEOVER failed — will retry next reconcile",
				"replica", h.TakeoverReplicaAddr, "failedMaster", h.FailedMasterAddr)
			if firstErr == nil {
				firstErr = takeErr
			}
			continue
		}
		done++
		MetricStuckSlotTakeoverTotal.WithLabelValues(vc.Namespace, vc.Name).Inc()
		logger.Info("Stuck-slot takeover issued",
			"replica", h.TakeoverReplicaAddr, "promotedFromFailedMaster", h.FailedMasterAddr)
		commonsevents.Emitf(r.Recorder, vc, "StuckSlotTakeover",
			"partial-slot outage 자가복구: replica %s 가 fail master %s 의 slot 승계 (CLUSTER FAILOVER TAKEOVER)",
			h.TakeoverReplicaAddr, h.FailedMasterAddr)
	}
	return done, firstErr
}
