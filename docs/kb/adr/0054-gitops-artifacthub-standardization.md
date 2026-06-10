# ADR-0054: GitOps overlay + ArtifactHub 검증 파이프라인 표준화

- Date: 2026-06-02
- Status: Accepted
- Authors: @phil

## Context

keiailab operator 4종(mongodb-operator / postgres-operator / valkey-operator + keiailab-commons
라이브러리)의 cross-repo 표준이 비일치 상태였다:

- **GitOps overlay 경로 drift**: mongodb는 `examples/gitops/`, postgres/valkey는
  `deploy/overlays/prod/` — 동일 GitOps 패턴이 서로 다른 경로에 분산.
- **ArtifactHub 검증 자동화 부재**: ArtifactHub Signed badge 전제 조건인 PGP
  signingKey(`F1A6893583E632A757FF6767F3CC8C6AEC9CEB08`) 메타데이터 검증이 smoke 테스트
  없이 `ah lint`에만 의존.
- **CI 게이트 비대칭**: valkey가 4종 중 가장 성숙한 reference 구현.

valkey-operator는 이 표준화의 **reference 구현**이다:

- ADR-0024(Helm chart manual pattern + ArtifactHub): `charts/valkey-operator/` +
  `charts/artifacthub-repo.yml` + `scripts/helm-publish.sh --sign` 패턴.
- ADR-0044(ArtifactHub Signed + Official trust badges): PGP signingKey fingerprint
  `F1A6893583E632A757FF6767F3CC8C6AEC9CEB08` 등록 + ArtifactHub REST 확인.

본 ADR은 ADR-0024/0044의 **계승 + `artifacthub-verify.yml` smoke 자동화 추가**를
공식 기록한다.

## Decision

**2-레이어 분리**를 전 4종에 적용한다:

- **Layer 1 — ArtifactHub publish** (4종 모두): helm chart(`charts/<name>/`) → gh-pages
  → ArtifactHub Signed badge. 공통 PGP signingKey fingerprint
  `F1A6893583E632A757FF6767F3CC8C6AEC9CEB08`를 `charts/artifacthub-repo.yml`에 등록.
- **Layer 2 — GitOps 배포 overlay** (operator 3종만, keiailab-commons 제외):
  kustomize(`deploy/overlays/prod/`), namespace=`data`, base namespace delete patch 적용.
  keiailab-commons는 `type: library`로 배포 대상이 아니므로 Layer 2에서 제외.

**ArtifactHub 검증 파이프라인**:
- `.github/workflows/artifacthub-verify.yml`: `ah lint`(메타데이터 린트) + smoke 테스트
  (gh-pages 인덱싱 확인 + ArtifactHub REST 등록 확인 + `.tgz.prov` 도달성 검증).

**서명 구분**:
- `charts/artifacthub-repo.yml` PGP signingKey → ArtifactHub `Signed` badge.
- cosign(`release.yml`, ADR-0046) → GitHub Release `Verified` 레이블.
- 두 서명은 **완전히 별개**다 — 혼동 금지.

**valkey-operator 특이사항**:
- ADR-0024/0044 기존 결정을 유지. 경로 및 signingKey 변경 없음.
- `artifacthub-verify.yml` smoke 워크플로 신규 추가로 검증 자동화 완성.
- 본 ADR은 4종 cross-repo 표준화 컨텍스트를 공식 기록하는 목적.

**전파 방식**: Approach A(self-contained) — valkey reference를 각 repo에 복사+적응.
org-level reusable workflow(`uses:`) 방식은 배제. 이유: OSS fork 가능성 +
`keiailab/.github` org repo 2026-05-27 제거됨.

**GH Actions 사용 정당화**: RFC-0002(GitHub Actions 영구 금지)는 GitLab/인프라
closed-source org billing SPOF(2026-04-28 트리거) 컨텍스트의 결정이다. 본 대상은
**GitHub OSS public repo** + **사용자 명시 지시**("GHActions 통해서 artifacthub.io
파이프라인 검증"). 거버넌스 우선순위(사용자 명시 > Tier-1 글로벌)상 OSS public repo의
GH Actions 사용은 정책 위반이 아니다. ADR-0045/0048(GHA retention for public OSS)와
정합.

## Consequences

**긍정적**:
- valkey reference 구현이 4종 표준의 진본 패턴으로 공식화.
- `ah lint` + smoke 자동 검증으로 ArtifactHub 등록 회귀 방지.
- cosign(ADR-0046 SLSA3) ↔ ArtifactHub Signed badge 구분 명확화.
- ADR-0024/0044 기존 결정 계승으로 변경 최소화.

**부정적 / 트레이드오프**:
- `.tgz.prov` 생성은 현재 로컬 `scripts/helm-publish.sh --sign`에서만 동작 — CI
  자동화는 GPG private key secret 결정 후 후속 적용.
- ArtifactHub REST smoke는 gh-pages publish → ArtifactHub 인덱싱 지연(수 분)으로
  flaky 가능 → 재시도 로직 필요.
- 4종 각각 `artifacthub-verify.yml` 유지 필요(self-contained overhead).

## Alternatives Considered

**org-level reusable workflow(`uses:` 호출)**: 배제. `keiailab/.github` org repo가
2026-05-27 제거됨. OSS repo는 self-contained를 선호(fork 시 의존성 없음). 본 ADR의
self-contained 패턴은 ADR-0024에서 이미 확립됨.

**GH Actions 완전 배제(로컬 4계층만)**: ArtifactHub smoke는 gh-pages publish 후
원격 상태를 확인해야 하므로 로컬에서 실행 불가. ADR-0045/0048 narrow exception 범위
내에서 유지.

## Refs

- ADR-0024: Helm chart manual pattern + ArtifactHub (본 ADR의 기반)
- ADR-0044: ArtifactHub Signed + Official trust badges (본 ADR의 기반)
- ADR-0045: GitHub Actions 유지 — Public OSS External Trust Gate
- ADR-0046: SLSA3 cosign supply chain (cosign ↔ ArtifactHub badge 구분)
- ADR-0048: GHA Retention for Public OSS
- RFC-0002: GitHub Actions 영구 금지(GitLab/인프라 closed-source 한정)
