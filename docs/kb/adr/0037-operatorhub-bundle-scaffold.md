# ADR-0037: OperatorHub.io bundle scaffold (PR-B9 first cut)

- Date: 2026-05-10
- Status: Accepted
- Authors: @eightynine01

## Context

Plan §1 Phase 1 갭 D (External visibility — OperatorHub.io 미등록) 가 valkey
operator 의 *최대 OSS 발견성 손실 영역* 이다. ArtifactHub Helm chart 등록은
완료 (repositoryID `16085dd0`) 했으나, OperatorHub.io / OLM (Operator Lifecycle
Manager) 미등록 시 OpenShift OperatorHub 카탈로그 + Kubernetes operator catalog
양쪽에 노출되지 않는다.

OperatorHub.io 등록은 *bundle* 형식 (CSV + CRDs + RBAC + manifests + metadata
annotations) 을 community-operators repo (`k8s-operatorhub/community-operators`)
에 PR 로 제출하는 절차. 본 ADR 은 *bundle 자체를 valkey-operator repo 내에서
생성 가능한 상태* 로 만드는 first cut 결정을 기록한다.

## Decision

1. **operator-sdk v1.42+ 채택** — Makefile 의 `bundle` 타겟이 `operator-sdk
   generate kustomize manifests` + `kustomize build config/manifests` +
   `operator-sdk generate bundle` 를 단일 명령으로 wrap. brew install 필요.
2. **CSV scaffold 위치**: `config/manifests/bases/valkey-operator.cluster
   serviceversion.yaml` — repo 에 commit 보관. 5 CRD 모두 (`valkeys`,
   `valkeyclusters`, `valkeybackups`, `valkeybackuptargets`, `valkeyrestores`)
   `customresourcedefinitions.owned` 에 명시.
3. **메타데이터** (CSV `spec`):
   - `displayName: Valkey Operator`
   - `description`: 5-feature markdown (topologies, failover, cluster, backup,
     supply chain, security baseline) + Documentation/Discussions 링크
   - `keywords`: valkey, redis, cache, database, cluster, replication, backup, restore
   - `categories`: "Database, Storage"
   - `capabilities: Seamless Upgrades` (StatefulSet rolling update)
   - `installModes`: AllNamespaces only (cluster-scoped operator pattern)
   - `maintainers`: Keiailab <help@masblue.studio>
   - `provider`: Keiailab
   - `maturity: alpha` (v1alpha1 API 단계)
   - `minKubeVersion: 1.26.0`
   - `containerImage`: ghcr.io/keiailab/valkey-operator:v1.0.9
   - `repository / support`: GitHub repo + issues
4. **bundle Dockerfile**: operator-sdk 가 `bundle.Dockerfile` 자동 생성. `make
   bundle-build` 타겟 으로 image 빌드 (push 는 community-operators PR 시점에 별).
5. **alm-examples**: 본 ADR 단계에서는 빈 array `[]` (operator-sdk 기본). 후속
   PR-B9.2 에서 5 sample CR 의 inline JSON 으로 채움.
6. **bundle validate** 통과 — operator-sdk 1.42 의 표준 lint 통과 (alm-examples
   warning 만 informational, blocking 아님).

## Consequences

긍정:
- valkey 가 OperatorHub.io / OpenShift OperatorHub catalog 양쪽에 등록 가능한
  *기술적 전제 조건* 충족. 후속 PR-B9.2 (alm-examples) + community-operators
  repo PR 만 남음.
- Repeatable: `make bundle VERSION=1.0.9` 단일 명령. release 자동화 후속 작업
  (release.sh 통합) 가능.
- 5 CRD 의 `customresourcedefinitions.owned` 명시 — OLM 카탈로그가 본 operator
  가 관리하는 자원을 정확히 인지.

부정:
- Makefile + config/manifests/ 영역이 valkey 에 신규 추가. operator-commons /
  postgres / mongodb 와 비대칭 (postgres + mongodb 는 후속 PR 로 정합).
- alm-examples 부재로 OperatorHub UI 의 "Try it" 폼 자동 생성 불가 (PR-B9.2
  까지). 사용자는 여전히 Helm chart 또는 raw YAML 적용 가능.
- bundle 자체 등록 (community-operators PR) 은 본 PR 범위 외 — 사용자 명시
  진행 + GitHub fork + community-operators 검증 cycle 필요.

## Alternatives Considered

1. **operator-sdk 미사용 — 수동 CSV 작성**: 거절 사유. CSV format 변경 (OLM
   bundle spec evolution) 추적 비용 ↑. operator-sdk 가 표준 도구.
2. **alm-examples 본 ADR 에 포함**: 거절 사유. CSV 본문 + sample 5종 인라인
   JSON 결합 시 본 PR 디프 폭증 (~300+ lines). 분리 보존이 review burden ↓.
3. **Makefile 의 release 타겟 자동 통합**: 거절 사유. release.sh 의 cosign sign
   + SBOM + bundle 통합은 별 cycle. 본 PR 은 *bundle 자체 생성 능력* 만 보장.

## References

- Plan §1 Phase 1 D (External visibility / OperatorHub).
- ADR-0033: Cosign + SLSA L2 (supply chain — bundle image 도 동일 sign).
- operator-sdk 1.42 docs: <https://sdk.operatorframework.io/docs/olm-integration/>
- community-operators repo: <https://github.com/k8s-operatorhub/community-operators>
- 후속 PR: PR-B9.2 (alm-examples 채움), PR-B9.3 (community-operators PR 제출).
