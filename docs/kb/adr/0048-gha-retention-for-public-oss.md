# ADR-0048: GitHub Actions Retention — Public OSS Operator External Trust Gate

| Meta | Value |
|---|---|
| Status | Accepted (per operator family trade-off — sister ADR-0018 &#x0023; mongodb-ADR-0032 chose strict removal) |
| Date | 2026-05-21 |
| Author | keiailab |
| Supersedes | (none) |
| Related | ADR-0024 (helm chart manual pattern + ArtifactHub publish), ADR-0033 (supply-chain cosign + SLSA), ADR-0045 (restore GitHub Actions for OSS CI), ADR-0047 (community-operators sync automation) |

## Context

Global RFC-0002 (GitHub Actions Permanent Ban, 2026-04-29) was triggered by
the 2026-04-28 organization billing outage which caused a 24h+ merge freeze
across all repos. The intent: avoid single-SaaS SPOF for *internal
infrastructure repos*.

However, public open-source K8s operators have *different* requirements:

1. External contributors need a *trusted, automated gate* to verify their
   PRs. They cannot run the maintainers' private lefthook profile or
   `make verify` on a non-Linux laptop in reasonable time. ADR-0045
   already documents the live evidence from this repo's commit `3c69429`
   experiment.
2. Security scanners (CodeQL, OpenSSF Scorecard, Trivy) are *external
   trust signals* recognized by downstream consumers. Artifact Hub's
   "Signed" / "Official" trust badges (ADR-0044) and the Scorecard badge
   are part of the package metadata.
3. Helm chart auto-publish (GitHub Pages via `gh-pages`) and release
   artifact signing (cosign keyless + SLSA-3, per ADR-0033 / ADR-0046)
   are *part of the public release workflow*. The chart's canonical URL
   is the GitHub Pages site; ADR-0024 defines the manual chart + ArtifactHub
   publish pattern that depends on this.
4. dependabot/renovate require GHA-compatible `package-ecosystem` scanning
   to drive their PR cadence.

This ADR consolidates the partial exception ADRs already recorded in this
repo — **ADR-0024** (helm chart manual + ArtifactHub publish; depends on
`helm-publish.yml`), **ADR-0033** (supply-chain cosign + SLSA L2 in-toto
attestation; runs in `release.yml`), **ADR-0045** (the foundational
deviation ADR that restored the 14 workflows after commit `3c69429`
removed them), and **ADR-0047** (community-operators sync automation;
RFC-0002 §7 Exception ③ extension) — into a single *integrated rationale*
for the full `.github/workflows/` retention. The prior partial ADRs each
justified a slice of the deviation (or its evidence-driven restoration);
this ADR is the SSOT for the *whole* `.github/workflows/` directory in
this repo, written now that all four partial ADRs have stabilized.

## Decision

Retain `.github/workflows/` (14 workflow files) with **dual operation**:
GitHub Actions primary gate + local 4-tier (pre-commit, pre-push, Makefile,
PR reviewer evidence check) as fallback. This depth-defense pattern
mitigates the SPOF risk that motivated RFC-0002.

### Workflow Classification (14 files in this repo)

| Category | Workflows (this repo) | Rationale |
|---|---|---|
| **External Trust Gate** | `codeql.yml`, `scorecard.yml`, `dco.yml`, `dependency-review.yml`, `kube-linter.yml`, `security-scan.yml`, `go-licenses.yml` | External-recognized security/compliance signals; downstream consumers verify the Scorecard badge (per ADR-0044 Artifact Hub trust badges); CodeQL's deep static-analysis dataflow exceeds local `gosec`; DCO maintains the Signed-off-by trail required for community-operators upstream (ADR-0047); `go-licenses` blocks AGPL/BUSL re-introduction. |
| **Auto Deploy** | `helm-publish.yml`, `release.yml` | RFC-0002 §7 Exception ① (GitHub Pages) and Exception ③ (release tag → GitHub Release body). Auto Helm chart publish to `gh-pages` (per ADR-0024 manual + ArtifactHub pattern) plus cosign-signed release artifacts (ADR-0033 / ADR-0046 SLSA-3 keyless cosign). `release.yml` also hosts the community-operators sync job per ADR-0047. |
| **Local 4-Tier Backup** | `ci.yml` (lint+test+build), `helm-lint.yml`, `helm-install-test.yml`, `markdown-link-check.yml` | Same checks also enforced by pre-commit / pre-push / Makefile (per ADR-0030 RFC-0017 tooling unification: `.golangci.yml` + `Makefile validate`). GHA is primary; local is depth-defense. If GHA is down, maintainers can still merge using `make verify` + local hooks. |
| **Ops Tools** | `stale.yml` | Issue / PR lifecycle automation; not a merge gate. Safe to lose during a GHA outage. |

### Branch protection alignment

`main` branch protection lists the GHA job names from the **External Trust
Gate** and **Local 4-Tier Backup** categories as `required_status_checks`.
Maintainers must keep this list in sync when renaming jobs in workflow
files; divergence is treated as an operational defect (the operational
discipline note in §Consequences applies).

## Consequences

**Positive**:

- External contributors see clear, automated PR gates without needing local
  setup parity. ADR-0045's live evidence (commit `3c69429` removal then
  restoration) is the precedent.
- Downstream consumers verify external security signals: Security tab
  CodeQL findings, Scorecard badge, DCO compliance trail, Artifact Hub
  trust badges (ADR-0044).
- Helm chart auto-publish to GitHub Pages keeps release velocity (cuts the
  manual `helm package` + `gh-pages` push step out of every release; see
  ADR-0024 pipeline definition).
- cosign keyless + SLSA-3 provenance (ADR-0033 / ADR-0046) runs in
  `release.yml`; removing GHA would force a rewrite to local keyfile cosign
  which is what ADR-0033 explicitly avoided.
- dependabot/renovate operational without an extra runner SaaS.
- The four prior partial-exception ADRs (0024, 0033, 0045, 0047) now have
  a single upstream consolidated decision.

**Negative**:

- GHA SPOF risk remains. Mitigated by the local 4-tier fallback: every
  gate in the External Trust Gate and Local 4-Tier Backup categories has
  a local equivalent that maintainers can run when GHA is down.
- Some workflow files (notably `ci.yml`, `helm-lint.yml`) overlap with
  local hooks. Accepted for depth-defense; the marginal maintenance cost
  of keeping the workflow YAML in sync is small.
- Branch protection's `required_status_checks` list must stay in sync
  with workflow job names. Treated as operational discipline; a rename
  of a job in `ci.yml` without updating branch protection silently
  disables that gate, so renames go through PR review.
- This ADR may be blocked by the same `required_status_checks` it
  documents — if any External Trust Gate workflow fails on the PR, this
  ADR PR cannot merge. The merge path is reviewer override; the irony is
  intentional (the gates are real).

**Neutral**:

- All RFC-0002 §7 stated exceptions (Pages, dependabot, release) are
  already covered. This ADR is a *broader integrated rationale* that
  explains why the 14-file retention as a whole is correct for *this
  class of repo* (public OSS operator), not a request to add new
  exceptions. The prior ADR-0045 framed this as a "scoped deviation";
  this ADR upgrades the framing to "integrated retention rationale" now
  that the supporting evidence (ADR-0033 supply-chain, ADR-0044 trust
  badges, ADR-0047 sync automation) has accumulated.

## Alternatives Considered

1. **Strict RFC-0002 (remove all workflows)** — Rejected. External
   contributor trust loss; the Scorecard badge would disappear from
   Artifact Hub; CodeQL findings on the Security tab would empty;
   release automation regression. Already attempted by commit `3c69429`
   per ADR-0045; consequences were severe enough that the workflows were
   restored.
2. **Partial removal (keep External Trust Gate only, remove `ci.yml`
   and `helm-lint.yml`)** — Rejected. Inconsistency with sister
   full set; the local 4-tier duplicate would add maintenance burden
   without clear benefit. Depth-defense value is small but non-zero and
   the cost is low.
3. **GHA-only (drop the local 4-tier)** — Rejected. Re-introduces the
   exact SPOF that RFC-0002 was created to address; the 2026-04-28
   incident already demonstrated this failure mode (24h+ org-wide merge
   freeze).
4. **Local keyfile cosign instead of GHA OIDC keyless** — Already
   considered and chosen by ADR-0033 §Decision (RFC-0002 conflict
   avoidance). However the keyless OIDC alternative is what GHA enables
   for *future* upgrades when ADR-0033 is revisited; retention of
   `release.yml` keeps that door open.

## References

- **RFC-0002** (2026-04-29) — Global GHA permanent ban (internal-infra
  intent).
- **Sister ADRs (this repo)**:
  - [ADR-0024](0024-helm-chart-manual-pattern-artifacthub.md) — manual
    Helm chart + ArtifactHub publish; depends on `helm-publish.yml`.
  - [ADR-0033](0033-supply-chain-cosign-slsa.md) — cosign sign + SLSA L2
    in-toto attestation; runs in `release.yml`.
  - [ADR-0045](0045-restore-github-actions-for-oss-ci.md) — foundational
    deviation ADR; 14-workflow restoration after commit `3c69429`
    removal.
  - [ADR-0047](0047-community-operators-sync-automation.md) — RFC-0002 §7
    Exception ③ extension; sync job runs in `release.yml`.
- **Cross-operator ADRs (consolidated rationale parity)**:
- **Incident KB**: I-2026-04-28 (GHA billing outage; RFC-0002 trigger).
- **Related repo policy**:
  - [ADR-0030](0030-rfc-0017-tooling-unification-adoption.md) — RFC-0017
    tooling unification; defines the `.golangci.yml` + `Makefile validate`
    pieces that constitute the Local 4-Tier Backup fallback for
    `ci.yml`.
  - [ADR-0044](0044-artifacthub-signed-official-trust-badges.md) —
    Artifact Hub Signed (mandatory) + Official (external review) trust
    badges that the External Trust Gate workflows produce.
  - [ADR-0046](0046-slsa3-cosign-supply-chain.md) — SLSA-3 provenance
    upgrade path that depends on `release.yml` retention.

## Implementation

No code changes. Status `Proposed` → `Accepted` upon merge of this ADR.
The existing 14 workflow files in `.github/workflows/` remain as-is;
this ADR documents the *why* of their retention. Branch protection's
`required_status_checks` list is unchanged.
