# ADR-0005: Graceful cluster teardown via best-effort CLUSTER FORGET

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil

## Context

ValkeyCluster CR 삭제 시 K8s GC 가 owner-ref 를 따라 STS / Service / ConfigMap /
Secret / PDB / NetworkPolicy 를 cascade 삭제. STS 삭제 → pod 삭제 → PVC 삭제
(retentionPolicy=Delete 시).

문제: 각 valkey 노드의 `nodes.conf` 에 cluster gossip 멤버십이 영속화 — pod 가
종료되기 전 다른 노드들이 *나* 의 ID 를 알고 있다면, **다음 동명 ValkeyCluster 생성
시 같은 IP 가 부여된 새 pod 의 nodes.conf 에 stale 잔존 멤버 참조** 가 발생할 수 있다.

옵션:
1. **No-op (iter 0~1):** finalizer cleanup hook 이 nil. K8s GC 만 의존.
2. **Hard CLUSTER RESET:** 모든 노드에 `CLUSTER RESET HARD` — flushall 동반, 데이터
   파괴 위험.
3. **Best-effort CLUSTER FORGET (본 결정):** 모든 도달 가능 노드에서 다른 모든
   알려진 노드 ID 에 대해 `CLUSTER FORGET` 발행. 실패 무시.

## Decision

옵션 3 채택. 30s timeout 안에서 best-effort. 어떤 에러도 reconcile 차단하지 않음.

근거:
- "삭제는 막지 않는다" 원칙 (force-tenant CLAUDE.md 정책 차용) — finalizer 가 사용자
  삭제 의도를 *지연시킬 수는 있어도 차단하면 안 됨*.
- CLUSTER FORGET 은 데이터 비파괴적 — *gossip 멤버십에서만 제거*.
- 실패 시나리오 다수 (이미 STS replicas=0, NetworkPolicy 차단, password Secret 삭제,
  TLS 인증서 만료 등) → 모두 무시. PVC retention=Delete 면 자연 정리.

## Consequences

**긍정:**
- 같은 namespace+name 으로 재생성 시 stale 멤버 충돌 회피 (best-effort).
- 30s 안에 완료 → 사용자 삭제 UX 손상 최소.

**부정:**
- 노드 도달 불가능 시 cluster 외부 (다른 namespace 의 cluster 가 같은 노드 ID 알고
  있는 경우) 까지 정리 못함 → 단일 K8s cluster 안에서는 비현실적 시나리오.
- log 로만 forget 결과 표시 — Status condition 미반영. 운영자가 "왜 정리됐는지"
  확인하려면 controller log 조회 필요.

## 후속 작업

- Trigger: PVC retention=Retain 사용자가 재생성 시 stale 멤버 충돌 보고.
- 개선: `Status.Conditions` 에 `TeardownCompleted` condition 추가, `forget_calls_succeeded`
  메트릭 노출.
- M3 (Beta) 에서 `CLUSTER RESET SOFT` 옵션 (데이터 보존, gossip 만 reset) 비교 검토.

## Alternatives Considered

- **PreStop hook 으로 valkey 컨테이너에서 자기 자신 forget:** 다른 노드 입장에서는
  떠난 노드 = stale entry. 본질적 해결 아님.
- **Operator 가 STS replicas 를 0 으로 줄여 graceful drain 후 삭제:** 시간 비용 과대
  + 사용자 의도 (즉시 삭제) 에 반함.
