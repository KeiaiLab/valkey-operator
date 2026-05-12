---
adr: 0046
title: SLSA-3 provenance + cosign keyless signing for release artifacts
status: Accepted
date: 2026-05-12
deciders: keiailab/maintainers
depends_on: ADR-0045
---

# ADR-0046: SLSA-3 provenance + cosign keyless signing for release artifacts

## Status

Accepted — 2026-05-12

## Context

After ADR-0045 restored the GitHub Actions release pipeline, the
remaining commercial-grade supply-chain gap is **artifact attestation
and signing**:

- The current `release.yml` produces:
  - Multi-arch container image with `provenance: true` and `sbom: true`
    from `docker/build-push-action@v6` (buildx in-toto attestations,
    roughly SLSA L2).
  - A separate syft SPDX-JSON SBOM uploaded as a GitHub Release asset.
  - A `git-cliff` release-notes artifact.
  - The Helm chart packaged as `valkey-operator-<version>.tgz`.
- What is **missing**:
  - **SLSA-3 provenance** — buildx provenance does not satisfy SLSA-3
    requirements for an isolated builder, non-falsifiable provenance,
    and explicit invocation tracing. `slsa-framework/slsa-github-generator`
    provides a reusable workflow that produces a SLSA-3-compliant
    provenance attestation tied to the GitHub Actions OIDC identity.
  - **cosign signing** — neither the container image nor the chart
    `.tgz` is cryptographically signed. Adopters cannot verify image
    provenance with `cosign verify` and the OpenSSF Scorecard
    `Signed-Releases` check is failing.
  - **Sigstore transparency log** entry (rekor) — required for keyless
    signing and tamper-evident audit trail.

The `release.yml` already declares `permissions.id-token: write`, so
keyless cosign and the SLSA-3 generator OIDC flow are ready to enable
without further token plumbing.

## Decision

Add the following to `release.yml`, gated to the existing release jobs:

### 1. Container image signing (cosign keyless)

A new `sign-image` job, downstream of `image`, that runs:

```yaml
- uses: sigstore/cosign-installer@v3
- env:
    COSIGN_EXPERIMENTAL: 1
  run: |
    cosign sign --yes \
      "${IMAGE}@${{ needs.image.outputs.digest }}"
```

This produces an OIDC-bound signature pinned to the manifest digest
(not the floating tag) and writes the entry to the Sigstore
transparency log.

### 2. SLSA-3 provenance for the container image

Add a top-level job that calls
`slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@v2.0.0`,
passing the image+digest from the `image` job. The generator produces
a SLSA-3-compliant provenance attestation that adopters can verify
with `slsa-verifier` or `cosign verify-attestation`.

### 3. Helm chart signing

A new `sign-chart` step appended to the `chart-tgz` job that runs
`helm package --sign` is **not** chosen: native helm GPG signing
requires a long-lived signing key. Instead we use cosign keyless
on the `.tgz` blob:

```yaml
- run: |
    cosign sign-blob --yes \
      --output-signature out/valkey-operator-${VERSION}.tgz.sig \
      --output-certificate out/valkey-operator-${VERSION}.tgz.pem \
      out/valkey-operator-${VERSION}.tgz
```

The `.sig` and `.pem` are uploaded as GitHub Release assets alongside
the chart. Adopters verify with:

```bash
cosign verify-blob \
  --certificate valkey-operator-X.Y.Z.tgz.pem \
  --signature   valkey-operator-X.Y.Z.tgz.sig  \
  --certificate-identity-regexp '^https://github\.com/keiailab/valkey-operator/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  valkey-operator-X.Y.Z.tgz
```

### 4. SBOM signing

Same `cosign sign-blob` pattern applied to the syft SPDX-JSON SBOM
before it is uploaded to the Release. The signature pins the SBOM to
the exact build that produced the image.

## Consequences

### Positive

- OpenSSF Scorecard `Signed-Releases` check passes (target ≥7/10).
- Adopters can verify image authenticity with one `cosign verify`
  command, including OIDC identity proof.
- SLSA-3 provenance enables consumers (downstream operators, ArgoCD
  ApplicationSet policies, Kyverno admission rules) to enforce
  `policy/v1beta1: ClusterImagePolicy` against this image.
- Sigstore transparency log entry makes tampering retroactively
  detectable.

### Negative

- Releases now have a hard dependency on the public Fulcio + Rekor
  services. Outage of either delays the release pipeline.
  Mitigation: the existing artifact build still completes; only the
  `sign-*` jobs fail, and they can be re-run via `workflow_dispatch`
  after Sigstore recovers.
- Build time increases by ~30-60s (cosign install + sign + transparency
  log entry).
- Existing releases (`v1.0.x`) are unsigned. We do **not** retroactively
  sign them; verification documentation will note "v1.0.12 and earlier
  are unsigned; signed releases begin with v1.0.13".

### Trade-offs explicitly considered

| Alternative | Rejected because |
|---|---|
| Long-lived GPG signing key | Key management burden; rotation requires coordination; OpenSSF scoring favors Sigstore keyless |
| Notation (CNCF) instead of cosign | cosign has wider tooling support in the Kubernetes ecosystem; Notation parity is a future option |
| SLSA-2 only (skip generator workflow) | Buildx provenance is L2-equivalent but not auditable to the same standard; the marginal cost of L3 is one reusable-workflow invocation |
| Sign tags only (not digests) | Tags are mutable; digest-pinned signatures are the supply-chain standard |

## Verification documentation

Add a new section to `SECURITY.md` titled "Verifying release artifacts"
with the exact `cosign verify` and `slsa-verifier` commands and the
expected certificate identity regex.

## References

- ADR-0045 — restoration of GitHub Actions (prerequisite)
- SLSA v1.0 specification: <https://slsa.dev/spec/v1.0/>
- slsa-framework/slsa-github-generator: <https://github.com/slsa-framework/slsa-github-generator>
- cosign: <https://github.com/sigstore/cosign>
- OpenSSF Scorecard `Signed-Releases` check: <https://github.com/ossf/scorecard/blob/main/docs/checks.md#signed-releases>
