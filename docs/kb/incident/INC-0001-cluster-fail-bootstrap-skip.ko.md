# INC-0001: ValkeyCluster 가 cluster_state:fail 상태에서 bootstrap 재실행 안 됨 (한국어)

> English: [INC-0001-cluster-fail-bootstrap-skip.md](INC-0001-cluster-fail-bootstrap-skip.md) — canonical / 정본

- Detected: 2026-05-09 14:27 (KST) — production cluster (운영 클러스터)
- Resolved: 2026-05-10 09:18 (KST)
- Severity: SEV-2 (단일 cluster 영향, application traffic 미영향 — test data 한정)
- Owners: @eightynine01
- Tags: [valkey, cluster, reconcile, status, controller-runtime]

## Impact (영향)

- **사용자 영향**: 0. cluster 의 keys 가 모두 test data (`test_valkey_br_*`, `test_prod_*`, `test_failover_*`, 6 unique keys × 2 master+replica) 였고 production application traffic 미연결 상태였음.
- **시스템 영향**: ValkeyCluster (운영 인스턴스, 3 shards × 2 = 6 pods, 16384 slots) 가 약 19시간 동안 `cluster_state: fail` 상태로 stuck.
- **재정 / 법적 영향**: 없음.

## Timeline (시간순)

- **2026-05-07 06:14**: cluster 초기 deploy. Bootstrap 정상 완료. `clusterInitialized: true`.
- **2026-05-08 07:21**: ClusterReady=False / ClusterNotConverged condition 발현. 원인 불명 (pods 변동 또는 cluster bus partition 가능성).
- **2026-05-09 14:27**: Pods 재시작 (9시간 전). 새 IP 가 할당됨. 그러나 `nodes.conf` 의 myself IP 는 *이전 IP* (e.g. 10.42.6.172) 그대로 남아 갱신되지 않음. 다른 노드들과 cluster gossip 실패 → `cluster_state:fail`. controller 는 `clusterInitialized=true` 만 보고 cluster bootstrap 재실행을 *건너뜀*. STS reconcile 만 시도 → STS conflict 반복 (`the object has been modified`).
- **2026-05-10 00:02**: ReconcileError condition 의 마지막 transition. 이후 controller queue 가 exponential backoff 에 들어가면서 reconcile 빈도 감소.
- **2026-05-10 09:00 ~ 09:18**: 디버깅 + 수습 진행:
  1. controller pod restart (효과 없음 — clusterInitialized 그대로).
  2. 6 pods 에 CLUSTER RESET HARD 시도 — 3 master pods 가 keys 보유로 거부.
  3. Pod-1 의 nodes.conf 삭제 + restart — partial 회복 (자기 shard 만 OK).
  4. 6 pods 일괄 FLUSHALL + CLUSTER RESET HARD + AOF/nodes.conf 삭제 + 동시 restart → fresh state.
  5. controller 가 reconcile 은 돌지만 bootstrap 은 안 함 — `clusterInitialized: true` 가 차단점.
  6. `kubectl patch --subresource=status` 로 `clusterInitialized: false` 강제 → controller 즉시 bootstrap 재실행 → 16384 slots 모두 OK.

## Root Cause (근본 원인)

5 Whys:

1. **왜 cluster_state:fail 인가?** Pods 의 nodes.conf 가 stale myself IP 를 보유 → cluster gossip 실패.
2. **왜 nodes.conf 가 stale 한가?** Pods 재시작 시 PVC 의 nodes.conf 가 *이전 IP* 를 보존. valkey 부팅 시 이를 그대로 read 한다.
3. **왜 controller 가 회복시키지 못했나?** controller 가 cluster bootstrap (CLUSTER MEET / ADDSLOTS / REPLICATE) 단계를 skip 했다.
4. **왜 bootstrap 단계를 skip 했나?** `status.clusterInitialized: true` 가 *initialization 완료* 시점에 set 되고, *cluster fail 상태에서도 reset 되지 않는다* — controller 코드의 *one-shot init* 가정.
5. **왜 one-shot init 으로 가정했나?** 초기 설계 단계에서 *cluster 가 한 번 bootstrap 되면 영구히 healthy* 라는 가정. pod restart 후 IP 변경 시나리오 미고려. ADR-0017 (failover 보존) 결정 당시 *cluster topology 자동 회복* 영역이 범위 밖이었음.

기여 요인 (contributing factors):
- `nodes.conf` 가 PVC 에 보존 (stateful) — 새 IP 반영을 위해 valkey-cli 가 cluster reset 또는 announce-ip 갱신을 수행해야 한다.
- controller 의 ReconcileError condition 이 STS conflict 만 반영 — *cluster 자체 fail* 신호가 발행되지 않아 alert 미발생 + 자동 recovery 미진행.

## Resolution (수습)

수동 수습 (본 incident 한정):
1. data 손실 평가: keys 가 모두 test data → 안전하게 wipe 가능 판정.
2. 6 pods 의 PVC 데이터 (AOF + nodes.conf + dump.rdb) 일괄 삭제.
3. 6 pods 동시 restart (fresh state).
4. `status.clusterInitialized: false` 강제 patch.
5. controller 의 spec mutation 으로 reconcile 트리거.
6. controller 가 즉시 cluster bootstrap 재실행 → 16384 slots OK.

영구 fix (별 PR — 본 INC 의 후속):
- controller 코드의 *clusterInitialized* flag 평가 시 `cluster_state == "ok"` AND `assignedSlots == 16384` 까지 검증. fail 또는 partial assignment 면 *automatic re-bootstrap*.
- alert rule 추가: `cluster_state:fail` 30s 이상 지속 시 PrometheusRule 발화.

## Prevention (재발 방지)

단기 (본 incident 내):
- ✓ INCIDENT KB 작성 (본 문서).
- ⏳ Alert rule (`prometheus.io/scrape` annotation 의 metrics 경로 — `cluster_status_ok` metric).

중기 (별 PR):
- **PR-INC-0001-fix**: controller 가 `clusterInitialized` true 라도 *재검증* 로직을 추가. `cluster_state != "ok"` || `assignedSlots != 16384` 시 bootstrap 재실행.
- **PR-INC-0001-alert**: PrometheusRule (`groups[].rules` 의 `ValkeyClusterFail` alert).

장기 (RFC 후속):
- RFC-0005 (별 RFC): cluster topology 자가치유 정책. nodes.conf 의 myself IP 자동 검증 + cluster announce-ip 동적 갱신. valkey 9.x 의 *cluster-announce-bus-port* + DNS-aware IP advertisement.

## Action Items

- [ ] AI-0001: PR-INC-0001-fix — controller 재bootstrap 로직 (Owner: @eightynine01, Due: 2026-05-15).
- [ ] AI-0002: PR-INC-0001-alert — PrometheusRule (`ValkeyClusterFail` warning + critical).
- [ ] AI-0003: e2e regression test — pod IP 변경 시나리오 + clusterInitialized=true 차단점 검증 (test/e2e/cluster_recovery_test.go).
- [ ] AI-0004: docs/operations/runbook.md — cluster_state:fail 수습 절차 문서화.

## References

- ADR-0017 (failover 보존) — 본 INC 가 범위 밖 시나리오 (cluster topology 회복).
- HANDOFF.md 의 PR-A2.2.5 storageversion fix — *별 incident*, 본 INC 와 무관.
- mongodb INC-0001 (별 repo) — patterns cross-cut audit 후보.
