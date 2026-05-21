# Upgrading valkey-operator

본 문서는 valkey-operator 의 마이너/메이저 버전 업그레이드 시 필요한 마이그레이션
작업을 정리한다. Helm 사용자는 chart 업그레이드만으로 모든 변경이 적용되지만,
정적 manifest (`kubectl apply -f`) 사용자는 RBAC 등 일부 항목을 수동으로
patch 해야 한다.

## 0. 버전 정책 (semver)

| 변경 유형 | semver bump | 예시 |
|---|---|---|
| 신규 controller / CR / API 추가 | minor (v1.X → v1.X+1) | ValkeyBackupTarget 신설 |
| 기존 API 시그니처 변경 (breaking) | major (v1.X → v2.0) | ValkeyCluster.spec.storage struct 변경 |
| bug fix / dependency bump | patch (v1.X.Y → v1.X.Y+1) | controller-runtime 0.19→0.20 |
| operator-commons 의존 bump | minor (commons v0.X → v0.X+1) | pkg/pvc + pkg/topology 채택 |

## 1. v1.0.x → v1.0.13 (현재)

### Helm 사용자

```bash
helm repo update
helm upgrade valkey-operator keiailab-valkey-operator/valkey-operator \
  --namespace valkey-operator-system \
  --version 1.0.13
```

차트 자체가 RBAC, CRD, Deployment 를 모두 동기화한다. 추가 작업 불필요.

### 정적 manifest 사용자 — RBAC 마이그레이션

`make build-installer` 결과인 `dist/install.yaml` 의 차이 확인:

```bash
kubectl diff -f dist/install.yaml
kubectl apply -f dist/install.yaml
```

기존 ClusterRole 의 신규 권한 (현재 patch 는 RBAC 변경 없음):

| API group | Resource | 사유 | 추가 시점 |
|---|---|---|---|
| (없음) | — | — | — |

### v1alpha1 → v1alpha2 conversion webhook

v1alpha2 신규 도입. v1alpha1 CR 은 conversion webhook 으로 자동 변환 — 사용자
조치 불필요. 단 `kubectl apply -f` 로 v1alpha1 manifest 신규 생성은 *deprecated*
경고 — v1alpha2 사용 권장.

## 2. Sprint 1 채택 (operator-commons v0.9.0)

ADR-0049 (`docs/kb/adr/0049-sprint-1-commons-pvc-topology-adoption.md`).

```bash
# go.mod 의 operator-commons 의존 bump 후
go mod tidy
```

- **신규 import**: `github.com/keiailab/operator-commons/pkg/pvc`, `pkg/topology`
- **삭제된 코드**: `internal/controller/pvc_resize.go` (-136 LOC) + test
  (-166 LOC) + `internal/resources/statefulset.go` 의 inline
  `defaultTopologySpread` (-22 LOC) → 총 -322 LOC
- **callsite 교체**:
  - `valkey_controller.go:235` — `commonspvc.ExpandDataPVCs(ctx, c, ns, []string{crName}, size)`
  - `valkeycluster_controller.go:239` — 동일
  - `statefulset.go` — `commonstopology.Defaulted(constraints, replicas, selector)`

마이그레이션 영향:
- Reconcile 동작 동일 (refactor 만, 외부 동작 변경 없음)
- CRD spec 변경 없음
- Helm chart 영향 없음

## 3. v1.0.x → v2.0.0 (예정 — v3.x-stable 선언 시점)

CLAUDE.md §7 의 *상용 제품 수준* (P0+P1+P2+OP+C 모두 ✅) 도달 시.

- 모든 CR 의 API stability `Stable` 승격 (v1)
- breaking change *최소화* — major bump 는 *의미 신호*
- 5 repo 일관성 보장: `commons/docs/quality/production-grade-checklist.md` 참조

상세: operator-commons ADR-0013 (audit-production-grade.sh)

## 4. GHA dual-track 정책 (ADR-0048)

본 repo 는 RFC-0002 (GitHub Actions 영구 금지) 의 *예외* — public OSS operator 의
external trust gate (CodeQL / OpenSSF Scorecard / cosign / SLSA / Artifact Hub
trust badges) 필요로 GHA 14 workflow 유지 + 로컬 4계층 (lefthook) 과
dual-track 운영 (ADR-0048).

업그레이드 시 GHA workflow 변경은 `dependabot/github_actions/*` PR 으로 자동.
*사람 PR* 으로 `.github/workflows/` 신규 파일 추가는 *별 ADR* + 사용자 승인 필요.

## 5. 일반 마이그레이션 체크리스트

업그레이드 전:
- [ ] CRD 변경 (`api/v1alpha1/` 와 v1alpha2 conversion webhook 호환)
- [ ] `make verify` (lint + test + build + audit) 통과
- [ ] 기존 e2e 스위트 PASS (`make integration-test`)
- [ ] chaos-mesh 시나리오 PASS (ADR-0041, 4 시나리오)
- [ ] dependabot 의존 bump PR 통합 확인

업그레이드 후:
- [ ] Helm chart 의 `dependencies:` (keiailab-commons library chart) 갱신
- [ ] 각 CR 의 spec 호환성 검증 (특히 storage, resources)
- [ ] reconcile 결과 verify (`kubectl get valkey,valkeycluster -A`)
- [ ] 운영 메트릭 (`Reconcile{Total,Latency,Errors}`) 정상
- [ ] 클러스터 모드: `ClusterInitialized=true` + `state=ok` 확인 (ADR-0039)

## 6. 비호환 변경 안내 정책

- **Deprecation**: 신규 minor 에서 `// Deprecated:` 주석 + 2 minor 후 제거
- **Breaking**: major bump + 본 UPGRADING.md 의 별 섹션 + ADR 작성
- **사후 통보 안 함**: 모든 breaking 변경은 *최소 1 minor* 사전 deprecation

## 참고

- ADR 목록: `docs/kb/adr/INDEX.md`
- operator-commons UPGRADING: https://github.com/keiailab/operator-commons/blob/main/docs/UPGRADING.md
- audit: `make audit-quality` (5 repo 측정, commons ADR-0013)
- i18n: `commons/docs/i18n/README.md`
- 가족 family: `docs/family.md`
- Helm chart: https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
