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
- [x] AI-0024-8: postgres-operator 첫 release v0.3.0-alpha.1 publish (2026-05-06).
      GHCR sha256:7658a42e + GH Release prerelease + gh-pages orphan (817399a) +
      Pages built + helm pull 검증 PASS. 3-repo 모두 publish 완료.
- [x] AI-0024-9: mongodb-operator audit 의 trivy silent-fail 동일 결함 보강
      (commit 2b7c44a) — postgres 패턴 정합. 3-repo audit gate 통일.
- [x] AI-0024-10: 3-repo 동일 `renovate.json` 추가 (RFC 0002 §7 예외) —
      자동 CVE 감지 + k8s/otel group + vulnerabilityAlerts (security/p0).
      commits: valkey 2869b93, mongodb bf772ce, postgres 0ab83ef.
- [x] AI-0024-11: helper 의 name 충돌 회피 안내 — ArtifactHub 의 다른 vendor
      'valkey-operator' (v0.0.61-chart) 와 충돌. 등록 시 name 권장:
      `keiailab-valkey-operator` (path: /packages/helm/keiailab-valkey-operator/valkey-operator).
- [x] AI-0024-1: **ArtifactHub UI 자동 등록 완료** (claude-in-chrome MCP, 2026-05-06).
      Browser session 으로 keiailab org context 활성화 → ADD REPOSITORY → 폼 자동
      채우기 (name=keiailab-valkey-operator, URL=https://keiailab.github.io/valkey-operator)
      → submit → UUID 자동 추출 (`16085dd0-0f19-4c6b-ab90-bd97105bdf42`) → helper 로
      placeholder 교체 → commit eb8bcc3 + helm-publish (gh-pages 627868b) 동기화.
      `scripts/artifacthub-register.sh` helper 와 claude-in-chrome MCP 의 결합으로
      "UI 만 지원" 가정 깨짐 — 단 사용자 browser session 활성 + admin 권한 전제.
- [x] AI-0024-12: 3-repo branch protection (postgres + valkey 신규 — force-push
      차단 + linear history + conversation resolution) + 3-repo CODEOWNERS 통일
      (@eightynine01 직접 매핑) + 3-repo `scripts/release-smoke-test.sh` (10항목,
      모두 10/10 PASS).
- [x] AI-0024-13: 3-repo README badges 7종 (License/Go/DB/K8s/GHCR/Helm/ArtifactHub
      shields.io + ArtifactHub native badge endpoint). 시각적 신뢰 + 즉시 link.
      commits: postgres e5cba5d, valkey 13e1294 (mongodb 는 사전 보유).
- [x] AI-0024-14: 3-repo PR template (`.github/PULL_REQUEST_TEMPLATE.md`) +
      SECURITY.md 의 PGP fingerprint (`89A4 0947 6828 CB99 ...`) 직접 명시 통일.
      commits: valkey 13e1294/2a57330, postgres e5cba5d (mongodb PR 사전 보유).
- [x] AI-0024-15: 3-repo `make release-notes` 타겟 — git-cliff 로 conventional
      commits → release notes 자동 (Bug Fixes / Chores / Documentation 분류).
      release 타겟의 `gh release create` 가 `--notes-file /tmp/release-notes-*.md`
      로 자동 사용 (git-cliff 미설치 시 fallback "변경 내역은 CHANGELOG 참조").
      commits: valkey 248201a, postgres 9ce90cd, mongodb 3009f1e.
- [x] AI-0024-16: 3-repo `helm-publish HELM_SIGN=1` 옵션 — `helm package --sign
      --key $HELM_GPG_KEY --keyring $HELM_KEYRING` 으로 .prov 파일 자동 생성.
      gh-pages cp 가 `*.tgz.prov` + `artifacthub-repo.yml` 도 자동 동기 (있으면).
      기본 default 0 (unsigned, 기존 동작 보존). 사용자가 PGP private key import
      후 `make release HELM_SIGN=1` 활성. commits: valkey 248201a, postgres
      9ce90cd, mongodb 3009f1e.
- [x] AI-0024-17: supply chain 강화 cycle 6 — `make sbom` (syft SPDX-2.3) +
      `make helm-docs` (values 표 자동) + release pipeline 의 자동 SBOM asset 첨부.
      release-smoke-test 6단계로 확장: SBOM asset 검증 + trivy post-publish
      HIGH/CRITICAL CVE scan (--exit-code 1, --ignore-unfixed). SLSA / EU CRA
      (2027) SBOM 의무화 대비. commit valkey 8516de3.
