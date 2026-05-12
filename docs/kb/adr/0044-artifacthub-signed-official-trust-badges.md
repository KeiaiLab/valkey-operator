# ADR-0044: Artifact Hub trust badges — Signed mandatory, Official external review

- Date: 2026-05-12
- Status: Accepted
- Authors: @eightynine01
- Refs: ADR-0024, ADR-0033, `docs/operations/artifacthub-trust.md`

## Context

The current Artifact Hub package status shows that repository verified publisher
has passed, but package `signed=false` and repository `official=false`.
The `valkey-operator-1.0.10.tgz.prov` file served from `gh-pages` also returns
404, so Artifact Hub cannot enable the `Signed` badge.

ADR-0024 provided a `HELM_SIGN=1` option, but the default release path was
unsigned. In that state, a release can succeed even when the release operator
forgets the signing option, causing an operational trust signal regression.

By contrast, `Official` is an externally reviewed status defined by Artifact
Hub. It cannot be self-declared by adding `official: true` to repository files.
Artifact Hub must approve an official status request showing that the publisher
owns the relevant software.

## Decision

1. Change the Helm chart release default to `HELM_SIGN=1`.
2. `make release` and `make helm-publish` must assume signed chart packages and
   `.tgz.prov` generation.
3. Helm `--key` expects a UID substring, not a fingerprint, so
   `HELM_GPG_KEY=Keiailab Helm` is the default. The fingerprint is split into
   `HELM_GPG_FINGERPRINT` and used only for documentation and verification.
4. The release smoke test must check GitHub Release assets, fetch `gh-pages`
   provenance, import the public signing key from `artifacthub-repo.yml`, and
   run `helm verify`.
5. `Official` must not be claimed in code. `docs/operations/artifacthub-trust.md`
   records prerequisites and the official status request content, and a user
   with Artifact Hub publisher permissions submits the external request.

## Consequences

Positive:

- The `Signed` badge becomes the default release path starting with the next
  normal release.
- Missing `.tgz.prov` files are detected immediately in both GitHub Release and
  `gh-pages` smoke checks.
- The invalid release configuration that uses a Helm key fingerprint as `--key`
  is removed.
- The responsibility boundary for `Official` is explicit, preventing false
  completion claims based only on code changes.

Negative:

- `make release` and `make helm-publish` fail on development machines without
  the PGP secret key. This is intentional fail-closed behavior.
- The existing `v1.0.10` package remains `signed=false` in this session without
  the private key. The existing artifact must be re-signed, or a new signed
  patch release must be published.
- `Official` depends on Artifact Hub maintainer review latency.

## Alternatives Considered

1. **Keep `HELM_SIGN=0` and rely on a manual release option** — rejected. The
   previous state already produced unsigned releases and depended on humans
   remembering the option.
2. **Generate a new PGP key ad hoc and publish immediately** — rejected. The
   official release signing key needs rotation, storage, and revocation policy.
   Replacing the trust root with a temporary key reduces long-term
   verifiability.
3. **Self-declare Official status with a chart annotation** — rejected. Artifact
   Hub official status is an external review state granted at repository or
   package level.

## Status

Accepted. The next release must pass `make helm-signing-preflight` and publish a
signed chart. A publisher with external account permissions submits the Artifact
Hub official request.
