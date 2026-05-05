# ADR-0012: CLUSTER MEET 는 hostname 미지원 → DNS 해석 후 IP 사용

- Date: 2026-05-05
- Status: Accepted
- Authors: @phil

## Context

ValkeyCluster bootstrap 의 `ensureMeet` 단계에서 다음 에러 가 무한 반복 :

```
cluster meet valkeycluster-sample-1.valkeycluster-sample-headless.default.svc:6379:
ERR Invalid node address specified
```

원인: Valkey (Redis-fork) 의 `CLUSTER MEET` 명령은 *IP 주소만* 받는다. hostname
은 거부 — `INET_PTON` 에서 직접 fail. Redis 7+ 의 `cluster-announce-hostname`
및 `cluster-preferred-endpoint-type hostname` 설정 으로 *cluster gossip 자체* 는
hostname 으로 동작 가능하나, **`CLUSTER MEET` 명령 인자 자체** 는 여전히 IP
필수 (소스: `cluster.c::clusterCommandSpecial`).

기존 코드 (`internal/valkey/cluster.go::ensureMeet`) 는 StatefulSet 의 headless
service FQDN (`<pod>.<svc>.<ns>.svc:6379`) 을 그대로 인자로 넘김 → 모든 K8s
배포 환경 에서 cluster bootstrap 영구 실패.

## Decision

`internal/valkey/cluster.go` 에 `resolveAddrIP` 헬퍼 추가:
- IP literal 이면 그대로 반환.
- hostname 이면 `net.Resolver.LookupIPAddr` 로 해석.
- 다중 IP 반환 시 IPv4 우선 (cluster bus 가 v4/v6 혼용 시 prefer 명시 필요).

`ensureMeet` 가 MEET 호출 직전 위 함수 로 IP 정규화 후 `ClusterMeet(host, port)`
실행. 에러 메시지 에 원본 hostname + 해석된 IP 둘 다 포함 — 디버깅 시 어느
단계 에서 실패 했는지 즉시 식별.

## Consequences

긍정:
- StatefulSet headless service FQDN 그대로 사용 가능 → operator 가 pod IP 를
  추적할 필요 없음 (DNS 가 동적 처리).
- CLUSTER NODES 응답 의 IP 와 MEET 한 IP 가 일치 → known 집합 비교 (멱등) 가
  잡음 없이 동작.
- Pod 재시작 으로 IP 가 바뀌어도 다음 reconcile 의 LookupIPAddr 가 새 IP 반환,
  CLUSTER NODES 의 known 집합 비교 가 mismatch 감지 → 재 MEET.

부정 / 트레이드오프:
- DNS 해석 latency 가 매 reconcile 에 추가 (수 ms ~ 수십 ms). cluster size N
  대해 N-1 회 lookup. mitigation: 매 cluster 의 reconcile 주기 ~ 5s 단위라
  무시 가능.
- DNS 캐시 stale → CLUSTER NODES 가 새 IP 보고 / Lookup 이 옛 IP 반환 의 race.
  현재 영향 미미 (kube-dns + StatefulSet 의 빠른 update). 추후 문제 시
  pod IP 직접 조회 (Pod resource list 에서 status.podIP) 옵션 검토.
- IPv4 prefer 가 dual-stack 클러스터 에서 의도와 다를 수 있음 — IPv6-only
  배포 시 첫 번째 IP 반환 (IPv6) 로 fallback 작동. 추후 ServiceAddrFamily
  설정 노출 검토.

## Alternatives Considered

1. **`cluster-announce-hostname` + `preferred-endpoint-type hostname`**: Valkey
   8.x 에서 cluster gossip 은 동작 하나 `CLUSTER MEET` 명령 자체 는 여전히 IP
   필수. Operator 가 MEET 호출 단계 에서 막힘. 거절.
2. **Pod IP 를 K8s API 로 직접 조회**: 정확하나 RBAC 추가 (이미 부여 — pods
   list/watch) + 코드 의존 증가. DNS 가 충분 — 거절.
3. **`StatefulSet` 대신 headless `Service` per-pod**: 인프라 복잡도 폭증. 거절.

## Action Items

- [x] AI-001: `resolveAddrIP` 추가 + `ensureMeet` 통합
- [ ] AI-002: integration test 에 hostname 인자 → IP 해석 → MEET 케이스 추가
- [ ] AI-003: dual-stack 환경 테스트 (IPv6 only kind 클러스터)

Refs: 무 (신규 결정)
