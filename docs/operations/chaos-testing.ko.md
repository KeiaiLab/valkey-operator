# Chaos Testing — valkey-operator (한국어)

> English: [chaos-testing.md](chaos-testing.md) — canonical / 정본


ADR-0041 chaos-mesh 기반 4 시나리오 chaos engineering e2e 실행 가이드.

## 사전 준비

1. **Kind cluster** (또는 임의 K8s) 활성:
   ```sh
   make setup-test-e2e   # 또는 kind create cluster --name valkey-e2e
   ```

2. **valkey-operator deploy**:
   ```sh
   make docker-build IMG=ghcr.io/keiailab/valkey-operator:e2e-dev
   make deploy IMG=ghcr.io/keiailab/valkey-operator:e2e-dev
   ```

3. **chaos-mesh 설치**:
   ```sh
   make chaos-mesh-install
   # 또는 수동: kubectl apply -f https://mirrors.chaos-mesh.org/v2.7.2/chaos-mesh.yaml
   ```

4. **테스트 대상 ValkeyCluster** (namespace=`valkey-chaos-e2e`):
   ```yaml
   apiVersion: cache.keiailab.io/v1alpha1
   kind: ValkeyCluster
   metadata: { name: vc-chaos, namespace: valkey-chaos-e2e }
   spec:
     shards: 3
     replicasPerShard: 1
     autoFailover: true
     version: { version: "9.0.4" }
   ```

## 실행

```sh
make chaos-e2e
# 또는 namespace override
CHAOS_TEST_NAMESPACE=my-ns make chaos-e2e
```

## 시나리오 (4종)

| ID | Chaos 유형 | 동작 | 회복 검증 |
|---|---|---|---|
| 1 | PodChaos (pod-kill) | 5분간 1분 간격 random pod kill | cluster_state=ok 5분 내 회복 |
| 2 | NetworkChaos (partition) | 30s master ↔ replica 차단 | failover 또는 회복 3분 내 |
| 3 | IOChaos (ENOSPC fault) | 60s 80% disk 가득 시뮬레이션 | cluster degraded but healthy 3분 내 |
| 4 | IOChaos (latency) | 60s replica I/O 100ms 지연 | master 영향 없음 (failover 미발생) 3분 내 |

각 시나리오는 chaos CR 적용 → 시간 진행 → 자동 cleanup → cluster healthy 회복
까지 검증 (BeforeSuite 의 vc-chaos CR 은 보존, 매 시나리오 후 회복).

## 운영 통합

- **개발자 local**: 새 reconciler 변경 후 *권장* 실행 (full e2e + chaos = ~30분).
- **CI nightly**: ADR-0041 AI-005 (별도 follow-up) — CI 인프라 작업 후 자동화.
- **production debug**: chaos-mesh 는 production 직접 실행 *금지* — staging /
  pre-prod 환경 전용.

## 정리

```sh
make chaos-mesh-uninstall
kubectl delete namespace valkey-chaos-e2e
```

## 시나리오 추가 가이드

- 신규 chaos CRD: `chaos-mesh.org/v1alpha1` 의 다른 kind (TimeChaos, DNSChaos,
  KernelChaos 등) 채택 가능.
- 패턴: `test/chaos/scenarios_test.go` 에 새 `var _ = Describe(...)` block 추가
  + `makeChaos(kind, name, ns, spec)` helper 사용.
- chaos-mesh CRD spec reference: https://chaos-mesh.org/docs/

## 트러블슈팅

| 증상 | 원인 / 수습 |
|---|---|
| `chaos-mesh.org/v1alpha1: NoMatchError` | chaos-mesh CRD 미설치 — `make chaos-mesh-install` |
| `kubectl apply` permission denied | chaos-mesh controller 가 *namespace 권한* 부족. install 시 `--local kind` 옵션 누락 가능 |
| 시나리오 timeout | cluster size / image pull 지연 — `--timeout=30m` 또는 더 긴 값으로 재실행 |
| Pod 가 `Terminating` 무한대기 | finalizer 제거 필요 — `kubectl patch pod ... --type=merge -p '{"metadata":{"finalizers":[]}}'` |

## 참조

- ADR-0041: chaos-mesh 채택 사유 + 후보 비교
- ADR-0040 §gap #4: chaos engineering e2e
- chaos-mesh: https://chaos-mesh.org/
- Makefile targets: `chaos-mesh-install`, `chaos-mesh-uninstall`, `chaos-e2e`
