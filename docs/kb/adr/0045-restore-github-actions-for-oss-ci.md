---
adr: 0045
title: Restore GitHub Actions workflows for OSS CI (deviation from RFC-0002)
status: Accepted
date: 2026-05-12
deciders: keiailab/maintainers
deviates_from: ai-dev/rfcs/0002-no-github-actions.md
supersedes: (commit 3c69429 "chore(ci): remove prohibited GitHub Actions workflows")
---

# ADR-0045: Restore GitHub Actions workflows for OSS CI

## Status

Accepted — 2026-05-12

## Context

In commit `3c69429` we removed all `.github/workflows/*.yml` files to comply
with the global policy [`RFC-0002: GitHub Actions permanently banned`].
That policy was written for **internal infrastructure repositories** where
the entire team operates locally and a billing outage on GitHub Actions
once cascaded into an organization-wide merge freeze (2026-04-28 incident).

`valkey-operator` is materially different:

1. **External contributor surface.** This is a public open-source Kubernetes
   operator published to ghcr.io and Artifact Hub. External contributors
   open PRs from forks and cannot run our private lefthook profile or
   `make verify` on a non-Linux laptop in a reasonable time.
2. **Trust signals.** Commercial-grade OSS adopters expect a visible CI
   matrix on every PR (`Checks` tab), reproducible release provenance,
   and supply-chain attestations (SLSA, cosign). The OpenSSF Scorecard
   badge prominently displayed on the README is scored largely on
   GitHub Actions-driven signals (`Branch-Protection`, `CI-Tests`,
   `Token-Permissions`, `Signed-Releases`).
3. **Already-installed conventions.** README badges, branch protection,
   CODEOWNERS routing, `.github/PULL_REQUEST_TEMPLATE.md`, and Artifact
   Hub publication all presume an Actions-driven release pipeline.
   Removing the workflows broke the contract these surfaces advertise.
4. **Required status checks.** With workflows absent, the main branch
   protection rule cannot enforce `required_status_checks` — a hard
   requirement for commercial-grade open source.

The post-incident analysis behind RFC-0002 was sound, but its remedy was
"never use Actions on internal infra repos". It did not anticipate
externally-facing OSS projects where Actions is the contract surface.

## Decision

Restore the five workflows removed in commit `3c69429`:

| Workflow | Trigger | Purpose |
|---|---|---|
| `ci.yml` | `pull_request`, `push` | golangci-lint, build manager binary, unit + envtest |
| `security-scan.yml` | `pull_request`, `push` | govulncheck, trivy-fs, trivy-image |
| `helm-lint.yml` | `pull_request` paths | helm template + chart-testing |
| `helm-publish.yml` | tag `valkey-operator-*` | GitHub Pages chart repository |
| `release.yml` | tag `v*` | container image + chart + signing |

In addition, enable `required_status_checks` on `main` for at least:

- `golangci-lint`
- `unit + envtest`
- `build manager binary`
- `govulncheck`
- `trivy-fs`
- `trivy-image`

This is a **scoped deviation** from RFC-0002, *not* a reversal. Internal
infrastructure repositories remain bound by RFC-0002 §1.

## Scope of the deviation

The deviation applies **only** to `keiailab/valkey-operator` and its
sister OSS operator repositories that publish to a public registry:

- `keiailab/valkey-operator` (this repo)
- `keiailab/mongodb-operator`
- `keiailab/postgres-operator`
- `keiailab/operator-commons`

Each sister repository SHOULD record its own ADR referencing this one
rather than relying on a transitive deviation.

## Consequences

### Positive

- External contributors get instant feedback on PRs without running a
  local toolchain.
- `required_status_checks` becomes enforceable, closing the
  "no checks → state=CLEAN" hole observed during the 2026-05-12 dependabot
  sweep (a PR with a failing test was still marked mergeable because the
  required-checks list was empty).
- OpenSSF Scorecard score for `CI-Tests`, `Branch-Protection`, and
  `Signed-Releases` recovers.
- SLSA-3 provenance and `cosign` signing of container images and Helm
  charts become tractable as follow-up work (ADR-0046 placeholder).

### Negative

- We re-inherit the single-point-of-failure that motivated RFC-0002.
  Mitigations:
  - All workflows MUST also be runnable locally via `make verify`
    targets (lefthook + Makefile parity, RFC-0002 §2 evidence pattern).
  - PR authors MAY paste `make verify` output in the PR body when GitHub
    Actions is degraded, and maintainers MAY merge based on local
    evidence after explicit sign-off in the PR thread.
- Org-wide Actions billing affects this repo. We accept the risk because
  the Actions free tier is generous for a public repo and the merge-freeze
  blast radius is limited to OSS repositories (internal repos are
  unaffected).

### Trade-offs explicitly considered

| Alternative | Rejected because |
|---|---|
| Keep workflows removed, rely on lefthook | External contributors cannot install our lefthook profile; PR feedback loop becomes hours/days |
| Mirror to GitLab CI | Doubles maintenance, splits the OSS contribution surface |
| Only restore `release.yml` (RFC-0002 §7 exception) | `required_status_checks` still impossible; PR CI signal still missing |
| Self-host runners | Cost + maintenance not justified for current PR volume |

## Compliance

- The restored `.github/workflows/` are reviewed for the patterns banned
  by RFC-0002 §1 (e.g., long-running scheduled jobs that block merges
  globally). The restored set runs only on `pull_request`, `push` to
  `main`, and tag events — no `schedule:` cron, no global merge-queue
  dependency.
- `make verify` MUST remain fully runnable locally; CI workflows
  duplicate but do not replace the local pipeline.

## Follow-ups

- ADR-0046: SLSA-3 provenance + cosign signing for `valkey-operator`
  container image and Helm chart (depends on this ADR).
- Sister repos (`mongodb-operator`, `postgres-operator`,
  `operator-commons`): write parallel ADRs referencing this one.
- `ai-dev/rfcs/0002`: append a clarification that public OSS operator
  repositories are out of scope.

## References

- Commit `71d322b` — original Actions setup (ported from mongodb-operator)
- Commit `3c69429` — removal under RFC-0002
- 2026-05-12 PR sweep (this commit) — restoration evidence
- RFC-0002 §7 — narrow exceptions previously granted
- OpenSSF Scorecard: <https://scorecard.dev/viewer/?uri=github.com/keiailab/valkey-operator>
