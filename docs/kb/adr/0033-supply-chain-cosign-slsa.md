# ADR-0033: Supply Chain — cosign sign + SLSA L2 in-toto attestation

- Date: 2026-05-09
- Status: Accepted
- Authors: @eightynine01
- Refs: Plan §2 D5 (`~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-synthetic-gem.md`)

## Context

ArtifactHub 비교 분석 (Phase 1) 결과 외부 두 redis Helm 차트가 모두
공급망 보증을 제공하나 valkey-operator 는 SBOM (syft SPDX) + trivy 만
보유:

| 차트 | 공급망 보증 |
|---|---|
| 외부 chart redis v25.5.2 | in-toto attestation + Photon Linux CVE scanning |
| 외부 chart (redis v0.27.6) | cosign signed |
| **valkey-operator (현재)** | SBOM only |

ROADMAP 항목으로 *cosign + SLSA* 가 이미 명시. 본 ADR 은 그 implementation
결정을 보존한다.

추가 제약: RFC-0002 (GHA 영구 금지, 2026-04-29) 로 인해 GitHub Actions
OIDC 기반 keyless cosign 은 *사용 불가*. 대안 keyfile 또는 Sigstore
Rekor public ledger 만 가능.

## Decision

1. **cosign 서명**: image push 후 `cosign sign --key <keyfile>` 으로 서명.
   keyless OIDC 미사용 (RFC-0002 충돌 회피). Sigstore Rekor public ledger
   에 entry 생성 (transparency).

2. **SLSA L2 in-toto provenance attestation**: SBOM (syft SPDX) +
   in-toto Statement v1 + provenance v1 predicate 를
   `cosign attest --type slsaprovenance --key <keyfile>` 로 첨부.
   builder.id = `https://keiailab.io/valkey-operator/scripts/release.sh`.

3. **Keyfile 관리**: `cosign.key` 는 GitHub Secret (`COSIGN_KEY`,
   `COSIGN_PASSWORD`) + 개발자 로컬 keychain 보관. 1Password Connect
   또는 OpenBao 보관 후속 작업 (별도 ADR).

4. **SLSA Level 정당화**:
   - L2 충족: provenance 가 *signed* + builder 가 식별 가능 (release.sh).
   - L3 미달: builder 가 *isolated* 환경 아님 (개발자 로컬 또는 ad-hoc
     server). L3 도달은 hermetic build 시스템 (Nix / Bazel) 도입 후속.
   - L4 미달: 두 명 이상의 검토자 + signed audit log 부재.

5. **수동 release 흐름**:
   - 개발자가 `scripts/release.sh <version>` 실행.
   - Step 6 (docker-buildx) 직후 신규 Step 6.5 가 cosign sign + attest 호출.
   - `release-smoke-test.sh` 의 신규 Step 7 가 cosign verify (사용자
     `COSIGN_PUBLIC_KEY` 설정 필수).

## Consequences

### Positive

- ArtifactHub publishing 시 *signed* badge 노출 가능 (Sigstore Rekor 검증).
- 외부 외부 chart 와 동등 보증 수준 — 사용자 이탈 차단.
- SLSA L2 가 EU Cyber Resilience Act (CRA) 의 *minimum acceptable*
  baseline 으로 인용 가능 (2027 시행 예정).
  도입 가능).

### Negative

- `COSIGN_KEY` 의 발급 / 회전 / 폐기 라이프사이클이 신규 운영 부담.
  완화: ADR 후속에서 OpenBao 통합 plan 작성.
- release.sh 가 `make sign-image / attest-provenance` 의존 — 두 타겟이
  cosign + jq 미설치 시 explicit fail. 완화: env 부재 시 graceful skip
  + 안내.

### Trade-offs

- *keyfile + Sigstore Rekor* (본 ADR) vs *keyless OIDC + Sigstore* —
  후자는 GHA OIDC 의존 (RFC-0002 충돌). 본 ADR 은 keyfile 선택.
- *cosign* (Sigstore 표준) vs *Notary v2* / *PGP signing* — cosign 이
  외부 chart in-toto + 외부 chart 와 호환 + Sigstore 생태계 가장 활발.

## Alternatives Considered

1. **Keyless OIDC (Sigstore Fulcio + Rekor)** — 거부.
   - GHA OIDC 의존 → RFC-0002 (GHA 영구 금지) 충돌.
   - 대안 OIDC provider (keychain / Auth0 / Okta) 가능하나 release.sh
     수동 흐름과 부적합 (interactive login 필요).

2. **PGP signing (helm provenance + image)** — 거부.
   - 이미 `helm package --sign` (HELM_SIGN=1) 패턴 보유 (Makefile
     line 502-504) — chart 만 cover, image 미cover.
   - cosign 이 Sigstore 표준이며 ArtifactHub badge 호환.

3. **Notary v2** — 거부.
   - OCI artifact 표준이나 ArtifactHub UI 노출 부재.
   - cosign 의 ArtifactHub `verifying-publisher` flag 와 정합 안 됨.

4. **GitHub Action 으로 SLSA L3 generator 도입** — 거부.
   - RFC-0002 GHA 금지 규칙 위반.
   - 본 ADR 은 L2 까지만 — L3 도달 path 별도 검토.

## Refs

- RFC-0002 (글로벌): GHA 영구 금지.
- Plan §2 D5: Supply Chain cosign + SLSA L2 P0 결정.
- 외부 패턴: 외부 chart in-toto attestation, 외부 chart cosign signed.
- Sigstore: <https://docs.sigstore.dev/cosign/overview>
- SLSA: <https://slsa.dev/spec/v1.0/levels>
- in-toto: <https://in-toto.io/spec/>
