<p align="center">
  <b>English</b> |
  <a href="../docs/i18n/ko/SECURITY.md">한국어</a> |
  <a href="../docs/i18n/ja/SECURITY.md">日本語</a> |
  <a href="../docs/i18n/zh/SECURITY.md">中文</a>
</p>

# Security Policy

> 한국어 버전: [SECURITY.ko.md](../docs/i18n/ko/SECURITY.md)

## Reporting a vulnerability

**Do not file public issues for security reports.** Public exposure
before a patch ships puts every adopter at risk.

### Private reporting channels

Choose either:

1. **GitHub Security Advisory** (recommended):
   <https://github.com/keiailab/valkey-operator/security/advisories/new>
2. **Email**: `security@keiailab.com` (PGP optional):
   - PGP fingerprint:
     `89A4 0947 6828 CB99 2338  C378 651E 51AF 520B CB78`
   - Public key: `artifacthub-repo.yml` on the `gh-pages` branch, or
     <https://keiailab.github.io/valkey-operator/artifacthub-repo.yml>
   - The same key signs `mongodb-operator` and `postgres-operator`
     (3-repo unified key).

### What to include

- Affected versions (release tag or commit SHA)
- Reproduction steps (the smallest reliable repro you can produce)
- Impact assessment (include a CVSS self-score if available)
- Reporter identity — let us know if you would like a credit

## Response SLA

| Stage | Target |
|---|---|
| Initial acknowledgement | within 72 hours |
| Severity triage | within 7 days |
| Patch release | by severity (Critical: 14 days, High: 30 days, Medium: 60 days) |
| Public disclosure | 14 days after the patch ships (coordinated disclosure on request) |

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.x (alpha) | ✅ Latest minor only |
| 1.0+ (stable) | TBD — updated after the first stable release |

The project is currently in `v1alpha1`. There is **no backward
compatibility guarantee**; security fixes ship only on the latest
release.

## Operational security recommendations

When you run `valkey-operator`:

1. **Force TLS.** Set `Spec.TLS.Enabled=true` (cert-manager or a
   user-provided `CustomCert`). See ADR-0010 and ADR-0014.
2. **Auth is effectively always on.** Per ADR-0013 the operator
   provisions a 32-byte random password regardless of
   `Spec.Auth.Enabled`.
3. **NetworkPolicy.** Set `Spec.NetworkPolicy.Enabled=true` to
   restrict pod-to-pod ingress. Verify on a CNI that actually
   enforces NetworkPolicies (Calico, Cilium).
4. **Pod Security Standard: restricted.** Apply
   `pod-security.kubernetes.io/enforce=restricted` to your namespace.
5. **Keep credentials in their own Secret.** S3 credentials on
   `ValkeyBackupTarget` belong in a dedicated `Secret` gated by RBAC
   (ADR-0016).
6. **Prefer external storage for backups.** Use
   `Destination.Type=TargetRef` with external S3. PVC-only backups
   are lost if the cluster itself is lost.
7. **Verify your container image.** The operator image is built only
   from dependencies that passed Sonatype and Context7 review
   (ADR-0022). When you build your own variant, run `trivy` or
   `grype` against the result.

## Dependency security

Every dependency-introducing ADR cites the relevant **Sonatype Trust
Score** and **Context7** verification (see `docs/kb/adr/0022-*.md` for
the canonical example).

Dependabot and Renovate auto-update PRs are reviewed at the front of
the queue.

## Verifying release artifacts (signed releases — v1.0.13+)

Starting with **v1.0.13**, every published container image, Helm chart,
and SPDX SBOM is signed via **Sigstore cosign** keyless OIDC and
attached with a **SLSA-3 provenance attestation** (ADR-0045,
ADR-0046). Releases prior to v1.0.13 are unsigned; the verification
commands below will fail against them as expected.

### Verify the container image

```bash
COSIGN_EXPERIMENTAL=1 cosign verify \
  --certificate-identity-regexp '^https://github\.com/keiailab/valkey-operator/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/keiailab/valkey-operator:<version>
```

### Verify SLSA-3 provenance for the image

```bash
slsa-verifier verify-image \
  --source-uri github.com/keiailab/valkey-operator \
  --source-tag v<version> \
  ghcr.io/keiailab/valkey-operator:<version>
```

### Verify the Helm chart

Download `valkey-operator-<version>.tgz`, `.tgz.sig`, and `.tgz.pem`
from the GitHub Release page, then:

```bash
cosign verify-blob \
  --certificate   valkey-operator-<version>.tgz.pem \
  --signature     valkey-operator-<version>.tgz.sig \
  --certificate-identity-regexp '^https://github\.com/keiailab/valkey-operator/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  valkey-operator-<version>.tgz
```

### Verify the SBOM

Same `cosign verify-blob` pattern with the `.spdx.json` / `.sig` /
`.pem` triple. The SBOM signature pins the bill-of-materials to the
exact build that produced the image.

### What a successful verification means

- The artifact was produced by a GitHub Actions workflow in this
  repository (the certificate identity proves the OIDC subject).
- The artifact has not been modified since signing (the Sigstore
  Rekor transparency log entry is tamper-evident).
- For the container image, the SLSA-3 attestation additionally proves
  the build ran in an isolated, hosted GitHub runner using the
  documented `release.yml` workflow.

## Known limitations

- English: [README.md → "Known limitations"](../README.md#known-limitations)
- Korean: [README.ko.md → "잠재적 운영 이슈"](../README.ko.md#잠재적-운영-이슈-현재-알려진-한계)
- See also: GitHub Issues with the `security` label.

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
