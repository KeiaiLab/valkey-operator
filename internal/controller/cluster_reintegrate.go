/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/observability"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

// 결함 ③ — cluster 멤버십 자가복구.
//
// 증상: 노드가 재시작 후 새 node id 를 얻거나 nodes.conf 를 잃어 멤버십에서 이탈하면
// (CLUSTER NODES 에 안 보임), operator 가 다시 합류시키지 못한다. 기존 health gate 는
// slot 레벨(cluster_state:ok)만 검사하고 *replica 수 / 멤버십* 은 보지 않아 idle 상태로
// shard 가 desired 보다 적은 replica 로 운영된다.
//
// 본 path 는:
//  1. detectReintegration: CLUSTER NODES + desired 토폴로지를 비교해 *어떤 pod 가
//     멤버가 아닌지* + *각 replica 가 어느 primary 를 따라야 하는지* 결정 (순수 함수).
//  2. reintegratePods: 각 누락 pod 에 대해 CLUSTER MEET (현재 pod IP) → 필요 시
//     CLUSTER REPLICATE <target-master-id>. 기존 ensureClusterMeet / CreateCluster 의
//     멤버십 helper 와 동일한 client 추상화 위에서 동작하며 멱등하다 (이미 멤버이며
//     올바른 master 를 가리키는 노드는 건드리지 않는다).

// reintegrationAction — 단일 pod 의 재합류 계획.
type reintegrationAction struct {
	// Ordinal — STS pod ordinal (0..total-1).
	Ordinal int
	// IsReplica — true 면 MEET 후 ReplicateTargetOrdinal 의 primary 를 따르게 한다.
	IsReplica bool
	// ReplicateTargetOrdinal — replica 가 따라야 할 primary 의 pod ordinal (desired
	// 토폴로지 기준). IsReplica=false (primary 자신 이탈) 면 의미 없음.
	ReplicateTargetOrdinal int
}

// observedMember — detectReintegration 의 입력. 관측된 cluster 멤버십을
// pod ordinal 기준으로 정규화한 것. controller 가 CLUSTER NODES + Pod IP 매핑으로 채운다.
type observedMember struct {
	// IsMember — 해당 ordinal 의 pod 가 CLUSTER NODES 에 (자신의 IP 로) 보이는가.
	IsMember bool
	// MasterOrdinal — replica 인 경우 현재 따르고 있는 primary 의 ordinal. 모르면 -1.
	MasterOrdinal int
}

// desiredMasterOrdinal — replica pod ordinal → 따라야 할 primary pod ordinal.
//
// CreateCluster 의 배치 규칙과 동일: replica index j (= ordinal - shards) 는
// primary (j % shards) = pod ordinal (j % shards) 를 따른다.
func desiredMasterOrdinal(ordinal, shards int) int {
	j := ordinal - shards
	return j % shards
}

// detectReintegration — 순수 결정 로직. desired 토폴로지(shards, replicasPerShard)와
// 관측된 멤버십(ordinal→observedMember)을 비교해 재합류가 필요한 pod 들의 계획을 만든다.
//
// 규칙:
//   - primary pod (ordinal < shards) 가 멤버 아님 → MEET (replica 아님).
//   - replica pod (ordinal >= shards) 가 멤버 아님 → MEET + REPLICATE(desired master).
//   - replica pod 가 멤버이지만 *틀린 master* 를 따름 → REPLICATE(desired master).
//   - 이미 올바른 멤버 → skip (멱등).
//
// 반환 순서: primary 먼저, 그다음 replica (master 가 먼저 멤버여야 replica 가 붙는다).
func detectReintegration(shards, replicasPerShard int, observed map[int]observedMember) []reintegrationAction {
	if shards <= 0 {
		return nil
	}
	total := shards * (1 + replicasPerShard)
	var primaries, replicas []reintegrationAction

	for ord := 0; ord < total; ord++ {
		m, seen := observed[ord]
		isReplica := ord >= shards
		if !isReplica {
			// primary — 멤버가 아니면 MEET 필요.
			if !seen || !m.IsMember {
				primaries = append(primaries, reintegrationAction{Ordinal: ord, IsReplica: false})
			}
			continue
		}
		wantMaster := desiredMasterOrdinal(ord, shards)
		switch {
		case !seen || !m.IsMember:
			// 멤버 이탈 — MEET + REPLICATE.
			replicas = append(replicas, reintegrationAction{
				Ordinal: ord, IsReplica: true, ReplicateTargetOrdinal: wantMaster,
			})
		case m.MasterOrdinal != wantMaster:
			// 멤버지만 틀린 master (또는 master 모름) → REPLICATE 로 교정.
			replicas = append(replicas, reintegrationAction{
				Ordinal: ord, IsReplica: true, ReplicateTargetOrdinal: wantMaster,
			})
		}
	}
	return append(primaries, replicas...)
}

// buildObservedMembers — CLUSTER NODES 결과(nodes) + pod ordinal→IP:port 매핑으로
// observedMember 맵을 만든다. desired total 만큼의 ordinal 을 채운다.
//
// addrByOrdinal[ord] = "ip:port" (pod 의 현재 IP). 비어 있으면 IsMember=false.
// nodeIDByOrdinal 은 부수적으로 ordinal→nodeID (멤버인 경우) 매핑을 채워 반환한다 —
// REPLICATE 시 target master 의 node id 가 필요하다.
func buildObservedMembers(
	shards, replicasPerShard int,
	nodes []vk.NodeView,
	addrByOrdinal map[int]string,
) (map[int]observedMember, map[int]string) {
	total := shards * (1 + replicasPerShard)

	// addr → NodeView 인덱스.
	byAddr := make(map[string]*vk.NodeView, len(nodes))
	for i := range nodes {
		byAddr[nodes[i].Addr] = &nodes[i]
	}
	// nodeID → ordinal (master 매핑 역참조용).
	idToOrdinal := make(map[string]int, total)
	for ord := 0; ord < total; ord++ {
		if addr := addrByOrdinal[ord]; addr != "" {
			if nv := byAddr[addr]; nv != nil {
				idToOrdinal[nv.ID] = ord
			}
		}
	}

	observed := make(map[int]observedMember, total)
	nodeIDByOrdinal := make(map[int]string, total)
	for ord := 0; ord < total; ord++ {
		addr := addrByOrdinal[ord]
		nv := byAddr[addr]
		if addr == "" || nv == nil {
			observed[ord] = observedMember{IsMember: false, MasterOrdinal: -1}
			continue
		}
		nodeIDByOrdinal[ord] = nv.ID
		masterOrd := -1
		if nv.IsReplica() && nv.MasterID != "" && nv.MasterID != "-" {
			if mo, ok := idToOrdinal[nv.MasterID]; ok {
				masterOrd = mo
			}
		}
		observed[ord] = observedMember{IsMember: true, MasterOrdinal: masterOrd}
	}
	return observed, nodeIDByOrdinal
}

// ensureClusterMembership — 결함 ③ 자가복구 진입점. allReady && cluster_state=ok 인
// 정상 cluster 에서도 *멤버십 / replica 수* 를 추가 검사해, desired 토폴로지에 비해
// 누락된 멤버를 재합류시킨다.
//
// 멱등: detectReintegration 이 이미 올바른 멤버는 제외하므로 반복 호출이 안전하다.
// 누락이 없으면 즉시 반환(no-op).
func (r *ValkeyClusterReconciler) ensureClusterMembership(
	ctx context.Context, vc *cachev1alpha1.ValkeyCluster, password string,
) (int, error) {
	ctx, span := observability.StartCallSpan(ctx, "ValkeyCluster/EnsureClusterMembership")
	defer span.End()
	logger := log.FromContext(ctx)

	shards := int(vc.Spec.Shards)
	rps := int(vc.Spec.ReplicasPerShard)
	if shards <= 0 {
		return 0, nil
	}

	info, nodes, err := r.queryAnyNode(ctx, vc, password)
	if err != nil || info == nil {
		// cluster 에 도달 불가 — bootstrap path 가 별도로 처리. 여기선 no-op.
		return 0, err
	}
	// ordinal → 현재 pod IP:port 매핑 (CLUSTER MEET 은 IP 를 요구, announce-ip/PR #298 정합).
	addrByOrdinal := r.podIPByOrdinal(ctx, vc)

	observed, nodeIDByOrdinal := buildObservedMembers(shards, rps, nodes, addrByOrdinal)
	actions := detectReintegration(shards, rps, observed)
	if len(actions) == 0 {
		return 0, nil
	}

	logger.Info("ValkeyCluster membership drift detected; re-integrating",
		"missingOrReplicaDrift", len(actions),
		"clusterState", info.State,
		"knownNodes", info.KnownNodes)

	return r.reintegratePods(ctx, vc, password, actions, addrByOrdinal, nodeIDByOrdinal)
}

// reintegratePods — actions 를 실제 CLUSTER MEET / REPLICATE 명령으로 실행한다.
//
// MEET 은 멤버십을 가진 임의의 healthy 노드(seed)에서 발행한다. REPLICATE 는 대상
// pod 자신에게 발행하며, target master 의 *현재 node id* 가 필요하다 — gossip 수렴
// 직후라 모를 수 있으므로 replicateWithRetry 패턴(vk 패키지)을 재사용한다.
func (r *ValkeyClusterReconciler) reintegratePods(
	ctx context.Context,
	vc *cachev1alpha1.ValkeyCluster,
	password string,
	actions []reintegrationAction,
	addrByOrdinal map[int]string,
	nodeIDByOrdinal map[int]string,
) (int, error) {
	tlsCfg, err := r.tlsConfigForCluster(ctx, vc)
	if err != nil {
		return 0, fmt.Errorf("tls config: %w", err)
	}
	dial := func(addr string) *redis.Client { return dialPod(addr, password, tlsCfg) }

	// seed — MEET 발행 노드. 첫 멤버(가급적 primary)를 고른다.
	seedAddr := r.firstMemberAddr(addrByOrdinal, nodeIDByOrdinal, int(vc.Spec.Shards))
	if seedAddr == "" {
		return 0, fmt.Errorf("no healthy seed member to issue CLUSTER MEET")
	}

	var done int
	for _, a := range actions {
		addr := addrByOrdinal[a.Ordinal]
		if addr == "" {
			// pod IP 미상 (아직 스케줄 전 등) — 다음 reconcile 재시도.
			continue
		}
		// 1) MEET (seed → 누락 노드). 이미 멤버면 valkey 가 no-op 처리.
		if err := vk.MeetNode(ctx, dial, seedAddr, addr); err != nil {
			return done, fmt.Errorf("cluster meet ordinal %d (%s): %w", a.Ordinal, addr, err)
		}
		if !a.IsReplica {
			done++
			continue
		}
		// 2) REPLICATE — target master 의 현재 node id.
		targetID := nodeIDByOrdinal[a.ReplicateTargetOrdinal]
		if targetID == "" {
			// master 자체가 아직 멤버가 아니거나 id 미상 — 다음 reconcile 에 수렴.
			// (actions 는 primary 를 먼저 처리하므로 통상 동일 reconcile 내에서 해소.)
			continue
		}
		if err := vk.ReplicateTo(ctx, dial, addr, targetID); err != nil {
			return done, fmt.Errorf("cluster replicate ordinal %d → master ordinal %d: %w",
				a.Ordinal, a.ReplicateTargetOrdinal, err)
		}
		done++
	}
	return done, nil
}

// firstMemberAddr — MEET 발행에 쓸 seed 주소. primary ordinal 중 멤버를 우선,
// 없으면 멤버인 아무 ordinal.
func (r *ValkeyClusterReconciler) firstMemberAddr(
	addrByOrdinal, nodeIDByOrdinal map[int]string, shards int,
) string {
	for ord := 0; ord < shards; ord++ {
		if nodeIDByOrdinal[ord] != "" {
			return addrByOrdinal[ord]
		}
	}
	for ord, id := range nodeIDByOrdinal {
		if id != "" {
			return addrByOrdinal[ord]
		}
	}
	return ""
}

// podIPByOrdinal — STS pod 들을 조회해 ordinal → "podIP:port" 매핑을 만든다.
// CLUSTER MEET 은 hostname 을 거부하고 IP 만 받으므로 FQDN 이 아닌 *현재 pod IP* 를
// 쓴다 (재시작 후 IP 변경 + announce-ip, PR #298 와 정합). IP 미상 pod 는 생략.
func (r *ValkeyClusterReconciler) podIPByOrdinal(ctx context.Context, vc *cachev1alpha1.ValkeyCluster) map[int]string {
	pods := &corev1.PodList{}
	selector := client.MatchingLabels(resources.SelectorLabels(vc.Name))
	out := make(map[int]string)
	if err := r.List(ctx, pods, client.InNamespace(vc.Namespace), selector); err != nil {
		return out
	}
	port := resources.PortClient
	if vc.Spec.TLS != nil && vc.Spec.TLS.Enabled {
		port = resources.PortTLS
	}
	prefix := resources.StatefulSetName(vc.Name) + "-"
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.PodIP == "" {
			continue
		}
		ord, ok := ordinalFromPodName(p.Name, prefix)
		if !ok {
			continue
		}
		out[ord] = fmt.Sprintf("%s:%d", p.Status.PodIP, port)
	}
	return out
}

// ordinalFromPodName — "vk-3" → 3. prefix("vk-") 이후 정수 파싱.
func ordinalFromPodName(name, prefix string) (int, bool) {
	if !strings.HasPrefix(name, prefix) {
		return 0, false
	}
	suffix := name[len(prefix):]
	n := 0
	if suffix == "" {
		return 0, false
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}
