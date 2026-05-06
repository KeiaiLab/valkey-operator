# ADR-0024: Helm Chart — 수기 작성 + ArtifactHub publish 패턴 (3-repo 통일)

- Date: 2026-05-06
- Status: Accepted
- Supersedes: ADR-0021
- Authors: @phil

## Context

3 개 keiailab operator repo (`mongodb-operator`, `postgres-operator`,
`valkey-operator`) 의 GitOps / Helm 파이프라인을 *동일 패턴* 으로 통일해야 한다.

기존 상태 (조사 결과 2026-05-06):

| 항목 | mongodb-op | postgres-op | valkey-op |
|---|---|---|---|
| Helm chart 위치 | `charts/mongodb-operator/` | `charts/postgresql-operator/` | (없음) |
| ArtifactHub 메타 | `charts/artifacthub-repo.yml` | `charts/artifacthub-repo.yml` | (없음) |
| ArtifactHub 등록 | ✓ | ✓ | (없음) |
| `Makefile` release 6단계 | ✓ | ✓ | (없음) |
| `gh-pages` 브랜치 | ✓ | (helm-publish 첫 호출 때 auto-orphan) | (없음) |
| Decision (이전) | 수기 chart | 수기 chart | ADR-0021 → kubebuilder helm/v2-alpha plugin |

ADR-0021 은 *plan 만 결정* 되었고 `dist/chart/` 산출물은 생성되지 않았다 (paper-only).
사용자 지시: "*3개의 폴더 모두 mongodb-operator 와 동일하게 GitOps 적용*".

## Decision

**3-repo 통일된 수기 chart + ArtifactHub publish 패턴 채택**.

각 repo 의 표준 구조:

```
charts/
├── artifacthub-repo.yml          # ArtifactHub 메타 (repositoryID + PGP signing key)
└── <name>-operator/              # chart 본체
    ├── Chart.yaml                # version, appVersion, artifacthub.io/* annotations
    ├── values.yaml               # 사용자 override 기본값
    ├── values.schema.json        # JSON Schema 검증 (선택)
    ├── README.md                 # 사용법
    ├── LICENSE
    ├── crds/                     # CustomResourceDefinition 정적 사본
    └── templates/                # Deployment / RBAC / Service / etc.
```

`Makefile` 의 표준 타겟:

| 타겟 | 역할 |
|---|---|
| `make setup-hooks` | RFC 0002 L1+L2 hook 설치 (pre-commit + pre-push) |
| `make gate` | lint + test + helm-lint + helm-template + audit |
| `make audit` | govulncheck + gosec + trivy fs |
| `make helm-lint` | `helm lint $(HELM_CHART)` |
| `make helm-template` | default + all-features-on 두 시나리오 render |
| `make release-preflight` | git clean + Chart.yaml ↔ VERSION 일치 검증 |
| `make release VERSION=vX.Y.Z` | 6단계 sequential gate |
| `make helm-publish` | gh-pages 브랜치 publish (auto-orphan 분기) |

`make release` 의 6단계:

1. 로컬 게이트 (gate)
2. release-preflight (git clean + Chart.yaml 버전 일치)
3. Docker image build + push (`docker buildx --platform linux/amd64`, default builder)
4. Git tag + push
5. GitHub Release (alpha/beta/rc → `--prerelease`) + chart `.tgz` 첨부
6. Helm chart publish to gh-pages → ArtifactHub 자동 인덱싱 (~30분)

ArtifactHub publish 모델:
- gh-pages = Helm repo (`https://keiailab.github.io/<repo>`).
- ArtifactHub 가 gh-pages 의 `index.yaml` 을 polling.
- `charts/artifacthub-repo.yml` 의 `repositoryID` (UUID) + `signingKey` (PGP) 가
  ArtifactHub provenance 에 사용됨. 3 repo 모두 keiailab 공통 PGP key 재사용.

## Consequences

긍정:
- **3 repo 일관성** — 새 repo 추가 시 mongodb chart 복제 + sed 치환 + 본문 재작성으로
  ~1 일 내 GitOps 부트스트랩 가능.
- **단일 SPOF 제거 (RFC 0002)** — GH Actions 의존 0건. organization billing /
  runner 장애 영향 없음.
- **ArtifactHub pull 모델** — gh-pages 갱신만 책임지면 됨. 멀티 단계 push 실패 시
  helm-publish 만 retry 가능 (idempotent: `helm repo index --merge` 가 중복 entry 거부).
- **kubebuilder helm plugin 의존성 0** — alpha 단계 plugin 의 breaking change /
  v2-alpha → v2 마이그레이션 부담 없음.

부정:
- **수기 동기화 부담** — `config/` (kustomize) 와 `charts/<n>/templates/` 사이 manifest
  drift 가능. 완화책: `make manifests generate` + `make helm-template` 을 게이트에 포함하여
  chart render 회귀 차단. CRD 는 chart 의 `crds/` 디렉토리로 자동 sync (mongodb 패턴).
- **annotations 정확성 책임** — `artifacthub.io/crds` / `crdsExamples` / `images` /
  `changes` 가 실제 CR 정의와 일치해야 함 (drift 시 ArtifactHub UI 가 stale 정보 노출).
  CHANGELOG → annotations.changes 는 release 단계에서 manual sync.

## Alternatives Considered

1. **kubebuilder helm/v2-alpha plugin** (ADR-0021 원안) — paper-only 미실행. 3 repo
   통일성 떨어짐 (mongodb/postgres 와 chart path 다름: `dist/chart/` vs `charts/<n>/`).
2. **OLM bundle** — OperatorHub 호환이 별 목표라 본 ADR 범위 외. 추후 별 ADR.
3. **Helm chart 자동 생성 도구 (helmify, kompose)** — drift 가드는 좋으나 valkey 의
   features.* 가드가 복잡 (cluster/backup/autoscaling 가드, 5 CRD 동시 가드) 해서
   자동 변환의 한계. 수기 chart 가 표현력 우위.

## Action Items

- [x] `charts/valkey-operator/` 생성 (mongodb 복제 + valkey 자원 재작성)
- [x] `charts/artifacthub-repo.yml` 작성 (repositoryID placeholder, PGP key 재사용)
- [x] `Makefile` 에 release/helm-publish/gate/audit/setup-hooks 타겟 추가
- [x] `.lefthook.yml` 에 helm-lint + helm-template + gitleaks hook 추가
- [x] ADR-0021 → Superseded 처리
- [x] AI-0024-2: 첫 release `make release VERSION=v0.1.0-alpha.1` 완료 (2026-05-06).
      GHCR `ghcr.io/keiailab/valkey-operator:v0.1.0-alpha.1` (sha256 2d1463bf...) +
      GH Release prerelease=true + gh-pages orphan branch (commit 37716ff) +
      Pages 자동 활성화 (status: built) + index.yaml live (5063 bytes).
- [x] AI-0024-3: postgres-operator 의 `release` 타겟 buildx 보강 (commit 314af15).
- [x] AI-0024-4: grpc CVE-2026-33186 (CRITICAL) v1.72.2→v1.81.0 (commit a353b44).
- [x] AI-0024-5: otel SDK GO-2026-4394 v1.36.0→v1.43.0 (commit c05b251).
- [x] AI-0024-6: Makefile audit trivy fail-handling 보강 (silent-fail → exit 1,
      postgres 패턴 정합, commit a353b44).
- [x] AI-0024-7: `scripts/artifacthub-register.sh` helper (UUID 검증 + sed 교체 +
      검증 명령 echo, commit a353b44).
- [ ] AI-0024-1: **ArtifactHub UI 에서 valkey-operator repo 신규 등록** (수동) →
      `scripts/artifacthub-register.sh <uuid>` 로 placeholder 교체 후 follow-up
      commit + `make helm-publish` 재실행. 자동화 불가 (UI 만 지원).
