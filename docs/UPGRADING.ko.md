# valkey-operator 업그레이드 가이드 (한국어)

> English: [UPGRADING.md](UPGRADING.md) — canonical / 정본

본 문서는 valkey-operator 의 minor / major 버전 업그레이드 시 필요한 마이그
레이션 작업을 정리한다. Helm 사용자는 chart 업그레이드만으로 모든 변경이
적용되지만, 정적 manifest (`kubectl apply -f`) 사용자는 RBAC 등 일부 항목을
수동으로 patch 해야 한다.

## 0. 버전 정책 (semver)

| 변경 유형 | semver bump | 예시 |
|---|---|---|
| 신규 controller / CR / API 추가 | minor (v1.X → v1.X+1) | ValkeyBackupTarget 신설 |
| 기존 API 시그니처 변경 (breaking) | major (v1.X → v2.0) | ValkeyCluster.spec.storage struct 변경 |
| bug fix / dependency bump | patch (v1.X.Y → v1.X.Y+1) | controller-runtime 0.19→0.20 |

## 1. v1.0.x → v1.0.13 (현재)

### Helm 사용자

```bash
helm repo update
helm upgrade valkey-operator keiailab-valkey-operator/valkey-operator \
  --namespace valkey-operator-system \
  --version 1.0.13
```

chart 자체가 RBAC, CRD, Deployment 를 모두 동기화하므로 추가 작업은 필요하지
않다.

### 정적 manifest 사용자 — RBAC 마이그레이션

`make build-installer` 결과인 `dist/install.yaml` 의 차이를 확인한다:

```bash
kubectl diff -f dist/install.yaml
kubectl apply -f dist/install.yaml
```

기존 ClusterRole 에 추가된 신규 권한 (현재 patch 에서는 RBAC 변경 없음):

| API group | Resource | 사유 | 추가 시점 |
|---|---|---|---|
| (없음) | — | — | — |

### v1alpha1 → v1alpha2 conversion webhook

v1alpha2 가 신규로 도입되었다. 기존 v1alpha1 CR 은 conversion webhook 을
통해 자동 변환되므로 사용자 조치는 필요하지 않다. 다만 `kubectl apply -f`
로 v1alpha1 manifest 를 신규 생성하면 *deprecated* 경고가 출력되며,
v1alpha2 사용을 권장한다.

## 2. Sprint 1 채택 ( )

ADR-0049 (`docs/kb/adr/0049-sprint-1-commons-pvc-topology-adoption.md`).

```bash

## 3. v1.0.x → v2.0.0 (예정 — v3.x-stable 선언 시점)

CLAUDE.md §7 의 *상용 제품 수준* (P0+P1+P2+OP+C 모두 ✅) 도달 시 진행한다.

- 모든 CR 의 API stability 를 `Stable` 로 승격 (v1)
- breaking change 는 *최소화* — major bump 는 *의미 신호* 로 사용한다
- 5 repo 일관성 보장: `commons/docs/quality/production-grade-checklist.md`
  참조

있다.

## 4. GHA dual-track 정책 (ADR-0048)

본 repo 는 RFC-0002 (GitHub Actions 영구 금지) 의 *예외* 다 — public OSS
operator 의 external trust gate (CodeQL / OpenSSF Scorecard / cosign /
SLSA / Artifact Hub trust badge) 가 필요하므로 GHA 14 workflow 를 유지하면서
로컬 4계층 (lefthook) 과 dual-track 으로 운영한다 (ADR-0048).

업그레이드 시 GHA workflow 변경은 `dependabot/github_actions/*` PR 로 자동
처리된다. *사람 PR* 으로 `.github/workflows/` 에 신규 파일을 추가하려면
*별도 ADR* 과 사용자 승인이 필요하다.

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
- [ ] 운영 metric (`Reconcile{Total,Latency,Errors}`) 정상
- [ ] cluster 모드: `ClusterInitialized=true` + `state=ok` 확인 (ADR-0039)

## 6. 비호환 변경 안내 정책

- **Deprecation**: 신규 minor 에 `// Deprecated:` 주석을 추가하고 2 minor
  이후 제거한다
- **Breaking**: major bump + 본 UPGRADING.md 의 별도 섹션 + ADR 작성
- **사후 통보 없음**: 모든 breaking 변경은 *최소 1 minor* 사전 deprecation
  을 거친다

## 참고

- ADR 목록: `docs/kb/adr/INDEX.md`
- audit: `make audit-quality` (5 repo 측정, commons ADR-0013)
- i18n: `commons/docs/i18n/README.md`
- 패밀리: `docs/family.md`
- Helm chart: https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
