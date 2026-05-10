# ADR-0041: Chaos Engineering — chaos-mesh 채택

- Date: 2026-05-10
- Status: Accepted
- Authors: @eightynine01

## Context

ADR-0040 §gap #4 에 따라 valkey-operator e2e 에 chaos engineering 시나리오 추가
필요. 현 `test/e2e/failover_test.go`, `cluster_recovery_test.go` 는 *결정론적
시나리오* 만 커버 — 특정 master 강제 종료 후 회복 검증. 그러나 production SEV-1
의 다수가 random pod kill / network partition / disk fill / slow disk I/O 같은
*비결정론적 장애* 임.

도구 후보 평가:

| 도구 | 라이선스 | CNCF 상태 | k8s native | 시나리오 커버 | 운영 부담 |
|---|---|---|---|---|---|
| **chaos-mesh** | Apache 2.0 | Graduated (2024) | ✅ CRD 기반 | pod / network / IO / time / kernel / DNS | 중 (operator + dashboard) |
| litmus | Apache 2.0 | Incubating | ✅ CRD + hub | 100+ experiments via hub | 중-상 (hub 학습 곡선) |
| chaos-monkey-k8s | Apache 2.0 | (없음) | ✅ 단순 cron | pod kill 만 | 낮 |
| pumba | GPLv3 | (없음) | ❌ docker 기반 | 풍부 | (k8s 외) |

## Decision

**chaos-mesh** 채택.

근거:
1. **CNCF Graduated** (2024) — vendor neutral 거버넌스 + 장기 유지보수 보장.
2. **CRD 기반** — operator 프로젝트와 동일한 declarative 패턴. unstructured apply
   로 SDK 의존성 추가 불필요 (cert-manager / Prometheus Operator 와 동일 패턴).
3. **시나리오 다양성** — PodChaos, NetworkChaos, IOChaos, TimeChaos, DNSChaos,
   KernelChaos. 본 operator 의 4 핵심 시나리오를 모두 단일 도구로 구현.
4. **kubernetes-native** — 별도 hub / experiment registry 학습 곡선 없음. 직접
   YAML 작성.
5. **GPLv3 회피** — pumba 는 GPL 이라 사내 도구 통합 시 라이선스 검토 필요.

거절 사유:
- **litmus**: hub 모델은 강력하나 본 operator 의 4 시나리오 한정 사용엔 over-spec.
  experiment-as-code 패턴이 자체 학습 비용.
- **chaos-monkey-k8s**: pod kill 만 지원 — network partition / IO 부재.
- **pumba**: docker-native, k8s native 아님 + GPLv3.

## Consequences

**긍정:**
- e2e nightly 에서 4 시나리오 자동 실행 — production SLO 회귀 사전 차단.
- 시나리오 추가 비용 낮음 — YAML / unstructured 추가만.
- chaos-mesh CRD 미설치 환경 (개발자 local) 에서 fail-soft (build tag `chaos`
  off 시 e2e 통과).

**부정:**
- chaos-mesh operator 설치 부담 — Helm chart 또는 manifest. 본 ADR 은 *e2e
  nightly* 에서만 사용 — 개발자 local 은 optional.
- nightly 실행 시간 증가 (예상 +5min) — chaos injection + cluster 회복 대기.
- chaos-mesh 자체 버전 의존성 — `v2.7.x` (2026-04 기준 stable) 고정. RBAC /
  privileged container 필요 (개발 환경 한정).

## Alternatives Considered

1. **결정론적 e2e 만 확장** — 추가 비용 낮으나 *비결정론적 장애 커버 부재* 의
   gap 그대로. ADR-0040 §gap #4 의 "production SEV-1 다수가 비결정론적" 명제
   해소 못함. 거절.

2. **단일 시나리오만 시작 (random pod kill)** — chaos-mesh PodChaos 만 사용.
   network partition / IO 시나리오는 phase 2 follow-up. 거절: 4 시나리오 모두
   *gap #4 의 acceptance criteria* — 한꺼번에 도입.

3. **외부 SaaS chaos 도구 (Gremlin)** — 상용. 비용 + 외부 의존. 거절.

## Action Items

- [ ] AI-001: `test/chaos/` 디렉토리 + build tag `chaos` Ginkgo suite
- [ ] AI-002: 4 시나리오 (random pod kill / network partition / disk fill /
      slow disk I/O) chaos-mesh CRD 적용 + 회복 검증
- [ ] AI-003: Makefile `chaos-e2e` target — chaos-mesh 설치 + suite 실행 +
      cleanup
- [ ] AI-004: `docs/operations/chaos-testing.md` — 개발자 local 실행 가이드
- [ ] AI-005: e2e nightly 통합 (별도 follow-up — CI 인프라 변경 필요)

## References

- chaos-mesh: https://chaos-mesh.org/
- CNCF Graduation: 2024-Q4
- ADR-0040 §gap #4 — chaos engineering e2e
- Plan: `~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-hazy-sketch.md`
