<p align="center">
  <b>English</b> |
  <a href="../docs/i18n/ko/CONTRIBUTING.md">한국어</a> |
  <a href="../docs/i18n/ja/CONTRIBUTING.md">日本語</a> |
  <a href="../docs/i18n/zh/CONTRIBUTING.md">中文</a>
</p>

# Contributing

> 한국어 버전: [CONTRIBUTING.ko.md](../docs/i18n/ko/CONTRIBUTING.md)

Thanks for your interest in `valkey-operator`. This document describes
the PR process, how to run the tests, and when an Architecture Decision
Record (ADR) is required.

## Getting started

### Prerequisites

| Tool | Minimum | Notes |
|---|---|---|
| Go | 1.26 | Matches `go.mod` |
| Docker | 24+ | buildx default builder |
| kind | 0.27+ | Local end-to-end tests |
| kubectl | 1.34+ | k3s/kind compatible |
| cert-manager | 1.16+ | Webhook serving cert |
| make | GNU make | Drives every Makefile target |

### First build and test

```sh
git clone https://github.com/keiailab/valkey-operator.git
cd valkey-operator

# Install pre-commit hooks (lefthook).
brew install lefthook       # or `go install github.com/evilmartians/lefthook@latest`
lefthook install

# Unit tests (envtest binaries are fetched automatically).
make test

# Integration test (spawns a real Valkey container, needs Docker).
make integration-test

# End-to-end (deploys the operator on a kind cluster).
make test-e2e
```

## Pull-request workflow

1. **Open an issue first** for any non-trivial change (architecture,
   API, security). A short alignment thread saves rewrites later.
2. **DCO sign-off is mandatory.** Every commit must end with a
   `Signed-off-by:` trailer (`git commit -s`). The commit-msg lefthook
   hook enforces this and unsigned PRs cannot be merged. See the
   [Developer Certificate of Origin](https://developercertificate.org/).
3. **Conventional Commits.** Subject line follows
   `<type>(<scope>): <subject>`, e.g. `feat(backup): TTL auto-cleanup`.
   The body can be English or Korean.
4. **Tests required.** Any behaviour change ships with at least one
   unit test that exercises it; `make test` must pass.
5. **lefthook must pass.** `gofmt`, `go vet`, and `golangci-lint`
   run on every commit; a failing hook blocks the commit.
6. **PR body should include:**
   - The user-visible scenario (why this change is needed)
   - Verification commands and trimmed output (`make test`,
     `kubectl apply -f …`, etc.)
   - The blast radius — which areas you re-tested for regressions
   - Links to any related ADR or issue
7. **Review SLA**: best-effort first review within 24 hours.

## Architecture Decision Records (ADR)

Write an ADR (in `docs/kb/adr/NNNN-<slug>.md`) when the change involves:

- A new CRD or a semantic change to an existing CRD field
- A new third-party dependency (the ADR cites both the
  `sonatype-guide` and `context7` evaluations)
- Security, authentication, or data-flow surface changes
- The third or later attempt to solve the same problem differently
  (convergence ADR)

Use Nygard's five-section template (Context / Decision / Consequences
/ Alternatives Considered / Status). Always update
`docs/kb/adr/INDEX.md` in the same commit.

## Code style

- **Go**: `gofmt` and `golangci-lint` (run via lefthook). `errcheck` is
  enforced.
- **Comments**: English or Korean both welcome. Explain *why*, not
  *what* — the code already shows what it does.
- **Tests**: prefer the fake client; use `envtest` only for genuine
  controller integration paths. Always use `WithStatusSubresource` so
  spec and status remain isolated.

## Design exploration

Before a large change:

1. Check existing plans under `docs/plans/`.
2. If you considered six or more design branches, capture the
   decision as an ADR up front rather than after the fact.
3. Stick to atomic commits — one logical step per commit, each one
   passing all four lefthook stages.

## Quality system (SSOT gates)

This repository ships 35+ Single-Source-of-Truth synchronization gates
(accumulated across release cycles 20–77). They make "advertised
surface == actual behaviour" a build invariant rather than a wish.

### Where the gates live

- `internal/observability/*_test.go` — all 33+ SSOT gate tests
- Inventory: [docs/operations/release-checklist.md §2](../docs/operations/release-checklist.md)

### Examples of what the gates block (pre-merge)

- A new metric without an alert-rules + runbook anchor
- A new ADR without an `INDEX.md` row or required Nygard sections
- A new `kubebuilder:rbac` marker without a matching update to
  `config/rbac/role.yaml` (run `make manifests`)
- A new Helm `values` key that no template actually references
  (catches silent typos)
- A new SSOT gate that isn't yet listed in the release checklist §2
  (the gate inventory is itself a gate)

### Automation that prevents drift in the first place

- `make manifests` syncs chart CRDs automatically (cycle 38)
- `git push` runs a six-hook lefthook pipeline — full lint, gitleaks,
  `go mod tidy`, helm lint, helm template, unit test
- The pre-push `go mod tidy` step blocks direct/indirect drift
  (cycle 47)

### Hot-path benchmarks

- `go test -bench=. ./internal/valkey/` — baselines for the five
  parsers. A 2× slowdown vs. baseline is a regression signal.

### Self-explaining gate failures

Most gates print the exact fix command in their failure message, e.g.:

- `TestCRDBaseChartSync`: `cp config/crd/bases/X charts/.../crds/X && git commit`
- `TestRBACMarkerResourcesInRole`: `run make manifests`
- `TestReleaseChecklistGatesSync`: add the new gate to release-checklist §2

A new contributor never has to guess which adjacent surface needs the
update — the failing test tells them.

## Security issues

Do **not** open public issues for vulnerabilities. See
[SECURITY.md](SECURITY.md) for the private reporting channels (GitHub
Security Advisory and a PGP-signed email address).

## License

This project is Apache License 2.0. By contributing you agree that your
contribution is distributed under the same license.
