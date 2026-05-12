# Release Checklist — valkey-operator

> 한국어 버전: [release-checklist.ko.md](release-checklist.ko.md)

Passing this checklist immediately before pushing a release tag
(`v0.1.0`, etc.) guarantees release-grade quality. **Every item**
below is covered by an automated gate; this document is the
human-readable inventory, not the enforcement point.

Automation SSOT: `scripts/release.sh` (manual entry point) +
`make gate` + lefthook pre-push hooks. Read this document to *see*
the system; the gates themselves are what actually block.

## 1. Build and code quality (automated)

- [ ] `make lint` — 0 issues (golangci-lint).
- [ ] `make test` — all unit + envtest tests PASS, no regressions.
- [ ] `make helm-lint` — chart is structurally valid.
- [ ] `make helm-template` — chart renders cleanly.
- [ ] `make audit` — govulncheck + gosec + trivy fs (HIGH and
      CRITICAL findings with a known fix: 0).
- [ ] `go build ./...` — 0 errors on every OS/arch.

The lefthook pre-push hook runs the six items above on every push;
a release tag push uses the same gate.

## 2. SSOT synchronization gates (in-process unit tests, `internal/observability/`)

These gates block PR merges if any two surfaces drift apart:

| # | Gate | What it checks |
|---|---|---|
| 1 | `TestADRFilesAllInIndex` | `docs/kb/adr/` ↔ `INDEX.md` in both directions |
| 2 | `TestADRIndexStatusValid` | Status column ∈ {Accepted, Proposed, Deprecated, Superseded by NNNN} |
| 3 | `TestADRIndexSupersededReferencesExist` | Every "Superseded" reference resolves to a real ADR |
| 4 | `TestADRFilesHaveRequiredSections` | Each ADR has all three Nygard sections (Context / Decision / Consequences) |
| 5 | `TestAlertRulesSchemaSanity` | PrometheusRule CRD schema (apiVersion / kind / groups) |
| 6 | `TestAlertRulesAllFieldsValid` | Every alert: prefix `Valkey` + expr + for + severity + annotations |
| 7 | `TestAlertRulesMetricNamesRegistered` | `valkey_cluster_*` metrics in alert `expr` are registered in `metrics.go` |
| 8 | `TestAlertRulesRunbookAnchorsExist` | `runbook_url` anchors resolve in `runbook.md` |
| 9 | `TestRBACMarkerResourcesInRole` | `kubebuilder:rbac` markers ↔ `role.yaml` in both directions |
| 10 | `TestSamplesStrictUnmarshal` | `config/samples/` ↔ API types strict-decoded (unknown fields rejected) |
| 11 | `TestSamplesDirHasAllExpected` | Registered sample map ↔ on-disk samples directory, both ways |
| 12 | `TestSamplesMetadataValid` | `apiVersion` / `kind` / `metadata.name` formats |
| 13 | `TestClusterRefKindEnumMatchesSSOT` | `ClusterReference.Kind` enum ↔ SSOT slice |
| 14 | `TestClusterRefKindAllHaveSwitchCase` | Every kind has a controller switch case |
| 15 | `TestLicenseFileExistsAndIsApache2` | `LICENSE` file present and is Apache-2.0 |
| 16 | `TestChartLicenseAnnotationMatchesLicenseFile` | Chart annotation ↔ `LICENSE` file |
| 17 | `TestChartImagesAnnotationMatchesAppVersion` | `artifacthub.io/images` tag ↔ Chart `AppVersion` |
| 18 | `TestChartIconURLUsesCurrentValkeyAsset` | Chart icon URL resolves on Artifact Hub today |
| 19 | `TestChartCRDExamplesStrictUnmarshal` | `crdsExamples` ↔ API types strict-decoded |
| 20 | `TestCRDBaseChartSync` | `config/crd/bases/` ↔ `charts/.../crds/` byte-level (sha256) |
| 21 | `TestChartValuesValkeyVersionMatchesAPIDefault` | `values.yaml::valkey.version` ↔ API default |
| 22 | `TestChartNotesTxtModeValueValidEnum` | `NOTES.txt` `mode:` ↔ `ValkeyMode` enum |
| 23 | `TestChartReadmeYAMLCodeblocksValid` | YAML blocks in every Markdown file are valid `mode`/`apiVersion`/`kind` |
| 24 | `TestMarkdownRelativeLinksResolve` | Every relative `.md` link in every `.md` file resolves |
| 25 | `TestIssueTemplateReadmeAnchorsExist` | Issue-template anchors resolve in README |
| 26 | `TestWebhookSetupFunctionsRegisteredInMain` | Webhook setup funcs ↔ `main.go` |
| 27 | `TestReconcilerTypesRegisteredInMain` | Reconciler types ↔ `main.go` instantiation |
| 28 | `TestRBACRoleResourcesInMarker` | `role.yaml` resources → `kubebuilder:rbac` markers (orphan rule blocked) |
| 29 | `TestInstallYAMLStructure` | `dist/install.yaml` shape (5 CRDs + Deployment + RBAC + Webhook + Service) |
| 30 | `TestKustomizeManifestLabelChainSync` | pod labels ⊇ Deployment selector ⊇ Service selector + ServiceMonitor selector ⊆ Service labels |
| 31 | `TestKustomizeChartResourcesSync` | `manager.yaml` ↔ `values.yaml` resources (limits + requests × cpu + memory) |
| 32 | `TestKustomizeChartProbesSync` | manager Deployment ↔ chart probe `initialDelay` and `period` |
| 33 | `TestKustomizeChartSecurityContextInvariants` | Pod Security "restricted" invariants on both sides (runAsNonRoot, seccompProfile, allowPrivilegeEscalation=false, readOnlyRootFilesystem, capabilities.drop=ALL) |
| 34 | `TestInstallYAMLOperatorImageEnvMatchesContainerImage` | `dist/install.yaml` `OPERATOR_IMAGE` env value ↔ manager container image (Upload/Download Job ImagePullBackOff blocked) |
| 35 | `TestChartArgsMatchOperatorFlags` | chart deployment + `config/manager` args ↔ `cmd/main.go` flag definitions (stale flags → CrashLoopBackOff blocked) |
| 36 | `TestValuesTemplateBindingCoverage` | every top-level `values.yaml` key is referenced somewhere in `templates/` (silent ignore blocked) |
| 37 | `TestChartFeaturesReconcilerEnvSync` | chart `features.{cluster,backup}.enabled` ↔ `ENABLE_{CLUSTER,BACKUP}_RECONCILER` env (RBAC and reconciler stay aligned) |
| 38 | `TestNetworkPolicyWebhookPortPresent` | NetworkPolicy ingress conditional 9443 when `webhook.enabled` (silent reject blocked) |
| 39 | `TestNetworkPolicyTracingEgressPresent` | NetworkPolicy egress conditional OTLP 4317/4318 when `tracing.endpoint` (silent span loss blocked) |
| 40 | `TestNetworkPolicyBackupEgressPresent` | NetworkPolicy egress conditional S3 (443/9000) when `features.backup.enabled` (`BackupTarget` permanent Pending blocked) |
| 41 | `TestMetricPhaseLabelsSync` | `metrics.go::allPhases` ↔ API `ValkeyPhase` + `ClusterPhase` enum union (Grafana phase series stays complete) |
| 42 | `TestGoVersionDockerfileVsGoMod` | `Dockerfile` `FROM golang:X.Y` ↔ `go.mod` `go X.Y` ↔ `CONTRIBUTING.md` Go table |
| 43 | `TestKubernetesVersionSync` | `Chart.yaml` `kubeVersion` ↔ README badge ↔ chart README Kubernetes prerequisite |
| 44 | `TestReleaseTargetInjectsBuildMetadataAndAmd64Only` | release image carries build metadata; release builds amd64+arm64 multi-arch (after ADR-0045 the release pipeline produces multi-arch via `docker/build-push-action`) |
| 45 | `TestArtifactHubRepositoryMetadataEnablesVerifiedPublisherAndSigningKey` | `artifacthub-repo.yml` `repositoryID`, `signingKey`, and `owners` stay in sync |
| 46 | `TestReleasePipelineRequiresSignedHelmCharts` | `HELM_SIGN=1` default; release/helm-publish produce `.tgz.prov` assets |
| 47 | `TestReleaseSmokeVerifiesHelmProvenance` | GH Release / `gh-pages` provenance verified via `helm verify` against the published signing key |

Run command: `go test ./internal/observability/`

## 3. Supply chain (enforced at release time)

- [ ] `make sbom VERSION=vX.Y.Z` — produce a syft SPDX-2.3 SBOM.
- [ ] Release pipeline auto-attaches the SBOM to the GitHub Release.
- [ ] Release pipeline auto-attaches chart `.tgz.prov` files to
      GitHub Releases and `gh-pages`.
- [ ] **cosign keyless signature** on the container image, Helm
      chart, and SBOM (ADR-0046, v1.0.13+). Verification recipes in
      [SECURITY.md](../../SECURITY.md#verifying-release-artifacts-signed-releases--v1013).
- [ ] **SLSA-3 provenance** attached to the image via
      `slsa-framework/slsa-github-generator` (ADR-0046).
- [ ] `bash scripts/release-smoke-test.sh` — 8 stages: chart asset,
      SBOM asset, Helm provenance verification, helm pull, image
      manifest, `gh-pages`, trivy CVE scan, and cosign verification.
- [ ] Multi-arch image (linux/amd64 + linux/arm64), built via the
      default buildx builder.

## 4. Documentation

- [ ] CHANGELOG.md `[Unreleased]` is promoted to `[vX.Y.Z]`
      (git-cliff automates this).
- [ ] README §Roadmap "Next" items reflect what is actually in the
      release.
- [ ] ADR INDEX is up to date (`TestADRFilesAllInIndex` enforces).
- [ ] runbook.md §9 per-alert response procedure is updated when a
      new alert lands.

## 5. Operational gates

- [ ] kubectl compatibility: `kubeVersion ≥ 1.26` (Chart.yaml).
- [ ] `make manifests` is a no-op on the working tree
      (controller-gen drift: 0). `manifests` also auto-syncs chart
      CRDs.
- [ ] Graceful fallback when cert-manager / prometheus-operator are
      absent (NotFound / NoMatch fail-soft).

## 6. User-visible surface (automated checks)

These accumulate as OSS trust signals:

- LICENSE Apache-2.0 (gates #15 and #16)
- SECURITY.md with the PGP fingerprint
- CONTRIBUTING.md with prerequisites + PR workflow
- `.github/PULL_REQUEST_TEMPLATE.md` (gate #23)
- `.github/ISSUE_TEMPLATE/{bug_report,feature_request,question}.yml`
  (gate #24)
- README §Roadmap (gate #25 enforces the anchor)
- Artifact Hub chart README + `crdsExamples` (gates #17, #18, #22)
- Issue triage labels (bug/triage auto-applied)

## 7. Release tag push procedure

```bash
# 1. Run the §1–§6 automation suite and confirm green.
make gate                                # = lint + test + helm + audit
go test ./internal/observability/        # 47 SSOT gates
bash scripts/release-smoke-test.sh vX.Y.Z  # 8 stages (skips on missing image/chart)

# 2. Run release.sh manually.
bash scripts/release.sh vX.Y.Z

# 3. Publish the GH Release body manually (release.sh writes
#    .release_notes.md).
gh release create vX.Y.Z --notes-file .release_notes.md \
  dist/install.yaml \
  /tmp/valkey-operator-X.Y.Z.spdx.json

# 4. helm-publish (gh-pages auto-detected by Artifact Hub).
make helm-publish HELM_SIGN=1 VERSION=vX.Y.Z
```

## 8. v0.1.0 GA additional gates (alpha → GA promotion)

These do **not** apply during alpha. From the GA tag onward:

- [ ] 24-hour soak test on a kind cluster: 0 memory leaks, 0 FD
      leaks.
- [ ] End-to-end automation (kind + MinIO + ValkeyCluster restore
      scenario) PASS.
- [ ] Track B (Failover / Resharding) ADR + implementation complete.
- [ ] Conversion webhook ready (v1alpha1 → v1beta1, ADR-0026
      deferred).
- [ ] HPA integration (Replication mode, ADR-0027 deferred).
- [ ] semver `0.1.0` or `1.0.0` choice (project policy).

## 9. Self-validation

`docs/operations/release-checklist.md` is itself inside the SSOT
gate set:

- A future cross-cutting gate can verify that the gates referenced
  here actually exist in `internal/observability/`.
- Broken markdown links in this file are blocked by
  `TestMarkdownRelativeLinksResolve`.
