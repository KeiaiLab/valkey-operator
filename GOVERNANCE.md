<p align="center">
  <b>English</b> |
  <a href="GOVERNANCE.ko.md">한국어</a> |
  <a href="GOVERNANCE.ja.md">日本語</a> |
  <a href="GOVERNANCE.zh.md">中文</a>
</p>

# Governance

> 한국어 버전: [GOVERNANCE.ko.md](GOVERNANCE.ko.md)

This document defines how decisions are made in
`keiailab/valkey-operator`.

## Principles

1. **Openness.** All decisions happen on public channels — GitHub
   issues, pull requests, and RFCs.
2. **Lazy consensus.** Day-to-day changes ship when no one objects.
3. **Explicit consensus.** Architecture changes, CRD changes,
   security-model changes, and license changes require an RFC followed
   by a **2/3 supermajority** of maintainers. Ordinary RFCs (single
   component, tool adoption, policy reinforcement) require a **simple
   majority** (>50%). Changes to this `GOVERNANCE.md` always require a
   2/3 supermajority.
4. **Shared responsibility.** Maintainers are jointly responsible for
   code quality, user safety, and community health.

## Decision classification

### Routine (lazy consensus)

- Bug fixes, doc improvements, new tests, minor/patch dependency
  bumps, refactors with no public API change
- Process: PR → at least one maintainer LGTM → merge
- Comment window: none. Once local gates pass, the PR can merge
  immediately (per RFC-0002 we do not rely on GitHub Actions for the
  gates; pre-commit/pre-push hooks plus the Makefile are the
  enforcement points).

### Medium (explicit consensus)

- New CRD fields, new reconcilers, major dependency upgrades,
  changes to the public API
- Process: open an issue proposing the change → 7-day comment window
  → maintainer majority LGTM → merge
- A single objection triggers a maintainer meeting to debate.

### Architectural (RFC required)

- Introducing a new component, changing the security model, changing
  the license, breaking backward compatibility
- Process:
  1. Submit an ADR or RFC at `docs/kb/adr/NNNN-title.md`
  2. 14-day comment window
  3. 2/3 maintainer approval
  4. Move ADR/RFC `Status` from `Draft` to `Accepted`, then open the
     implementation PR

## Security decisions

CVE reports and changes to the secrets / auth model are handled first
via the private channels in [SECURITY.md](SECURITY.md). Public
consensus follows once a patch release ships.

## Release decisions

A single maintainer may cut a release branch or bump a version under
lazy consensus. Creating a new LTS line or declaring End-of-Life on
an existing one always requires explicit consensus.

## Change history

| Date | Change | Refs |
|---|---|---|
| 2026-05-07 | Document created — 3-repo (mongodb / postgresql / valkey) governance asset alignment | INC-2026-05-07 |
| 2026-05-12 | English becomes canonical; Korean preserved as `GOVERNANCE.ko.md` | i18n PR-K |

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a>
</p>

<p align="center">
  © 2026 keiailab · <a href="LICENSE">Apache-2.0</a> · <a href="https://keiailab.com">keiailab.com</a>
</p>
