# ADR-0021: Helm Chart — kubebuilder helm/v2-alpha plugin 채택

- Date: 2026-05-06
- Status: Superseded by ADR-0024
- Authors: @phil

> **Supersede 사유 (2026-05-06)**: 본 ADR 은 결정만 되었고 실제 chart 산출물
> (`dist/chart/`) 은 생성되지 않은 paper-only 상태였다. 사용자 지시 ("3개 폴더 모두
> mongodb-operator 와 동일하게 GitOps 적용") 에 따라 mongodb-operator /
> postgres-operator 와 통일된 *수기 chart + ArtifactHub publish* 패턴 (`charts/<n>/`,
> `charts/artifacthub-repo.yml`) 으로 전환. ADR-0024 참조.

## Context

Plan §3 Track D (Helm chart). 외부 사용자의 *Helm 사용 환경 진입 장벽* 해소
필요 — `make deploy` 또는 `kubectl apply -f install.yaml` 외에 표준
`helm install` 도 지원.

후보 옵션:
1. **`kubebuilder edit --plugins=helm/v2-alpha`** — kubebuilder 가 `dist/chart/`
   에 chart 자동 생성. PROJECT 파일에 plugin 등록.
2. **수기 chart** — `helm create` + manifest 변환 + values.yaml 작성.
3. **OLM (Operator Lifecycle Manager) bundle** — OperatorHub 호환.

요구사항:
- 우리 manifest (CRD, RBAC, manager Deployment, webhooks) 와 *drift 없음*
- 변경 시 chart 재생성 절차가 명료
- ImagePullSecret / replicas / resources 등 사용자 override 지원
- cert-manager 의존성 명시

## Decision

**kubebuilder helm/v2-alpha plugin 채택** (옵션 1).

생성:
```sh
kubebuilder edit --plugins=helm/v2-alpha
```

결과:
- `dist/chart/Chart.yaml` — chart metadata
- `dist/chart/values.yaml` — 사용자 override (image / replicas / resources / TLS)
- `dist/chart/templates/` — manifest 자동 변환 (CRD, RBAC, Deployment, Webhooks)
- `PROJECT` 파일 — `helm.kubebuilder.io/v2-alpha` plugin 등록

변경 시 재생성 절차:
1. `make manifests` (controller-gen 으로 CRD/RBAC 갱신)
2. `kubebuilder edit --plugins=helm/v2-alpha --force` (chart 재생성)
3. `dist/chart/values.yaml` 의 사용자 커스텀 override 가 *force 후 manual
   re-apply* 필요 — README 또는 본 ADR 에 명시.

## Consequences

긍정:
- **drift 0** — kubebuilder 가 동일 manifest 출처에서 chart 생성. config/
  와 자동 동기.
- **유지보수 비용 작음** — 변경 시 단일 명령 재생성.
- **CRD 자동 포함** — chart `crds/` 디렉토리 또는 templates 에 CRD 포함
  (helm install 시 자동 적용).

부정:
- **values.yaml customization re-apply** — `--force` 재생성 시 사용자
  custom override 가 덮어써짐. 마이그레이션 노트 필요.
- **OperatorHub (OLM) 호환 별개** — 본 ADR 은 Helm 만. OLM bundle 은
  별개 ADR-0024 (추후).
- **plugin alpha 단계** — kubebuilder helm plugin 이 `v2-alpha`. 향후 stable
  으로 전환 시 마이그레이션 필요.

## Alternatives Considered

1. **수기 chart** (옵션 2 거절):
   - manifest 변환 작업 매번 필요 → drift 위험.
   - 거절: 유지보수 비용 vs 자동 생성 trade-off.

2. **OLM bundle** (옵션 3 거절 — 본 ADR 범위 외):
   - OperatorHub 가 별개 distribution 채널.
   - Helm 과 *상호배타 아님* — 별개 ADR-0024 로 추후 채택 가능.

3. **Helm Chart 수기 + GitHub Actions 자동 검증**: GHA 사용 RFC 0002 제약
   (예외 §7-③ 적용 검토 필요). 거절: 자동 생성이 더 단순.

## Action Items

- [ ] AI-001: `kubebuilder edit --plugins=helm/v2-alpha` 실행 + 결과 검토.
- [ ] AI-002: `dist/chart/values.yaml` 의 default 값 검증 (image / replicas /
      resources / TLS / metrics).
- [ ] AI-003: README 에 `helm install` 절차 추가.
- [ ] AI-004: 첫 v0.1.0 release 시 `helm package dist/chart` + GitHub Release
      asset upload (manual).
- [ ] AI-005: `helm install` e2e 검증 (kind cluster + cert-manager).

## 예외 / 한계

- 본 ADR 은 *operator* image 와 *Valkey* image 의 분리:
  - operator chart: dist/chart (operator manager + CRDs + webhooks).
  - Valkey instance: ValkeyCluster CR YAML (chart 외부 — 사용자 작성).
  - 두 chart 가 별개 — chart-of-charts 또는 umbrella chart 는 별개 ADR.

Refs: Plan §3 Track D, HANDOFF.md cycle 4 §2.2.
