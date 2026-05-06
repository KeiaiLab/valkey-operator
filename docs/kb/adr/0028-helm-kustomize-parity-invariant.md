# ADR-0028: Helm vs Kustomize Parity Invariant

- Date: 2026-05-06
- Status: Accepted
- Authors: @phil

## Context

valkey-operator 는 두 deploy method 지원: (a) `kustomize build` 로 생성된
`dist/install.yaml` (`make build-installer`), (b) Helm chart (`charts/valkey-
operator/`). 두 방식 모두 *동일 operator binary* 를 배포 하지만 *주변
인프라 (RBAC / observability / NetworkPolicy / webhook)* 는 *각자 다른
template 시스템* 으로 작성됨.

본 세션 (cycles 20-114) 에서 발견된 *Helm vs kustomize 의 silent feature
차이* family (5 sibling):

1. cycle 37 — chart CRD 사본 stale (autoFailover 미반영) → Helm 사용자 *기능
   silent 불가*.
2. cycle 102 — kustomize default 의 PROMETHEUS resource 누락 → kustomize
   사용자 *10 alerts silent 미작동*.
3. cycle 103 — chart PrometheusRule template 미보유 → Helm 사용자 *10 alerts
   silent 미작동*.
4. cycle 104 — chart metrics-auth ClusterRole 미보유 → Helm 사용자 *secure
   metrics 401 silent fail*.
5. cycle 109 — chart metrics-reader ClusterRole 미보유 → cluster admin
   convenience helper 부재.

각 sibling 의 *root cause* 동일: *kustomize 와 chart 가 별개 template 시스템*
이므로 한쪽 변경 시 *다른쪽 누락 가능*. *production-grade UX 의 silent failure*
표면.

## Decision

**Helm chart 와 kustomize install.yaml 은 *동일한 operator 인프라* 를 생성해야
한다 (Parity Invariant).**

구체 적용:
1. **신규 K8s resource 추가 시 양쪽 동시 갱신**:
   - kustomize 측: `config/<category>/*.yaml` + `config/default/kustomization.yaml`
     의 resources/patches.
   - chart 측: `charts/valkey-operator/templates/<resource>.yaml` (조건부 +
     values 통합).
2. **kubebuilder default 의 *commented out* 리소스 검토**:
   - `#- ../prometheus` 같은 default off 가 *production-grade UX 에 silent loss*
     야기 시 → 활성 (cycle 102) 또는 chart 측 별도 구현 (cycle 103).
3. **chart 측 *opt-in* 의 명확한 활성화 조건**:
   - `webhook.enabled` / `networkPolicy.enabled` / `metrics.serviceMonitor.enabled`
     같은 *single toggle* 로 *모든 layer (K8s 등록 + cert + deployment serving +
     operator setup + RBAC) 동시 활성/비활성*.
4. **operator runtime env ↔ chart template 동기**:
   - operator 코드 가 읽는 모든 env (cycle 64 OPERATOR_IMAGE, cycle 65 OTEL,
     cycle 74 WATCH_NAMESPACES, cycle 80 ENABLE_*_RECONCILER, cycle 111
     ENABLE_WEBHOOKS) 가 chart deployment 에서 *조건부 자동 주입*.

## Consequences

**긍정:**
- 사용자 가 *어느 deploy path 도 동일한 production-grade 인프라* 활용.
- silent feature drift 차단 (cycles 37/102/103/104 sibling family 의 근본 차단).
- ADR-0024 (chart 수기 작성) 의 *production-grade trade-off* 명확 — *수기
  비용 < parity invariant 가치*.

**부정:**
- *kustomize 변경 시 chart 동시 갱신 비용* — kubebuilder 의 모든 generated
  resource 가 *2번 표현*.
- chart template 의 *조건부 분기 복잡성* (5-element webhook coupling 등).
- *kustomize-only* 또는 *chart-only* 기능 추가 시 *의식적 deferred 결정 필요*
  (e.g., chart 의 helper aggregation roles 미보유 — sister chart 정합).

## Verification

본 invariant 의 *자동 검증*:
- `TestCRDBaseChartSync` (cycle 37) — config/crd/bases/ ↔ chart/crds/ byte-level.
- `TestKustomizeChartResourcesSync` (cycle 61) — resources (limits/requests).
- `TestKustomizeChartProbesSync` (cycle 62) — probes (liveness/readiness).
- `TestKustomizeChartSecurityContextInvariants` (cycle 63) — PSS-restricted.
- `TestChartFeaturesReconcilerEnvSync` (cycle 81) — features.* ↔ ENABLE_*.
- `TestNetworkPolicyWebhookPortPresent` (cycle 87) — NP + webhook coupling.
- `TestNetworkPolicyTracingEgressPresent` (cycle 88) — NP + tracing.
- `TestNetworkPolicyBackupEgressPresent` (cycle 89) — NP + backup.
- `TestInstallYAMLOperatorImageEnvMatchesContainerImage` (cycle 64) —
  OPERATOR_IMAGE env ↔ container image.
- `TestInstallYAMLStructure` (cycles 49/102) — install.yaml 의 K8s kinds 검증
  (ServiceMonitor + PrometheusRule 포함 강제).

본 게이트 collection 이 *Parity Invariant 의 자동 강제*.

## Alternatives Considered

- **kubebuilder helm/v2-alpha plugin** (ADR-0021 → 0024 superseded): chart 자동
  생성 — 그러나 features.* 조건부 분기 등 *production 요구* 표현 부족. *수기
  유지 + parity invariant 자동 검증* 이 더 robust.
- **chart 만 사용 + kustomize 폐기**: 일부 사용자 (CI/CD 파이프라인, helmfile
  사용 안 하는 환경) 가 *kustomize 의존*. 양쪽 유지 — 사용자 선택 보장.

## Action Items

- [x] 5 SSOT sync gate (TestCRDBaseChartSync 등) 자동 검증 활성.
- [x] cycles 37/102/103/104/109 의 5-sibling parity completion 완료.
- [x] release-checklist.md §2 에 parity gate 항목 등록.
- [ ] 향후 신규 K8s resource 추가 시 본 ADR 의 4-step 패턴 (양쪽 갱신 + opt-in
      toggle + env 동기) 반드시 적용.
- [ ] kustomize-only / chart-only 기능 추가 시 *의식적 deferred 결정* 명시
      (sister chart 패턴 정합 또는 ADR 작성).
