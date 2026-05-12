# Operations docs index

This directory holds operational procedures for `valkey-operator`.
Most documents have an English canonical version and a Korean
sibling (`<name>.ko.md`); some are still Korean-only and are being
translated in follow-up PRs.

## Canonical English (read this first)

| Document | Purpose |
|---|---|
| [runbook.md](runbook.md) | Incident response and daily operations. SSOT for every Prometheus alert's `runbook_url` annotation. |
| [release-checklist.md](release-checklist.md) | Pre-release gate inventory: build, SSOT gates, supply chain, docs, operations. Run before pushing any release tag. |
| [troubleshooting.md](troubleshooting.md) | Symptom → cause → diagnostic → remediation flowchart for issues that fire no alert (or fire before one exists). |

## Korean-only (English translation pending — tracked by ROADMAP)

These deep-dive documents are still Korean canonical. External
operators who need them in English should open an issue requesting
the specific page; translations land as the underlying topic ships
its next user-facing change.

| Document | Topic | Lines |
|---|---|---|
| [artifacthub-trust.md](artifacthub-trust.md) | Artifact Hub trust badges (Verified Publisher, Signed, Official) and the keys/owners contract that produces them. | 95 |
| [capacity-planning.md](capacity-planning.md) | Sizing guide — memory, replication factor, shard count, p95/p99 latency targets per topology. | 160 |
| [chaos-testing.md](chaos-testing.md) | chaos-mesh scenarios exercised in CI and how to reproduce them locally on a kind cluster. | 92 |
| [cloudpirates-valkey-compatibility.md](cloudpirates-valkey-compatibility.md) | Compatibility matrix vs. CloudPirates' Valkey distribution: which data-plane knobs are safe, which are out of scope. | 119 |
| [commercial-parity-status.md](commercial-parity-status.md) | Feature-by-feature parity with Redis Enterprise: what's at parity, what's deliberately skipped, what's planned. | 178 |
| [metrics-glossary.md](metrics-glossary.md) | Every `valkey_cluster_*` metric, its label cardinality, where it is emitted, and which alerts consume it. | 136 |
| [pitr-guide.md](pitr-guide.md) | Point-in-time recovery: AOF retention sizing, restore boundaries, RPO/RTO measurement procedure. | 195 |
| [post-merge-cleanup.md](post-merge-cleanup.md) | After a release: GH Pages cache flush, Artifact Hub propagation wait, Helm repository index refresh. | 120 |
| [sentinel-migration.md](sentinel-migration.md) | Migrating from Redis Sentinel topologies to a Valkey Cluster. Why Sentinel mode is a Non-Goal (ROADMAP §Non-Goals). | 196 |
| [webhook.md](webhook.md) | Admission-webhook architecture, cert-manager certificate path, debugging webhook denials. | 124 |

## Cross-cutting references

- Architecture decisions: [`docs/kb/adr/INDEX.md`](../kb/adr/INDEX.md)
- Incident history: [`docs/kb/incident/`](../kb/incident/)
- Release verification commands: [`SECURITY.md → "Verifying release artifacts"`](../../SECURITY.md#verifying-release-artifacts-signed-releases--v1013)
- Roadmap (project direction): [`ROADMAP.md`](../../ROADMAP.md)

## i18n status

The i18n initiative landed in PRs #93 / #97 / #98 / #103–#107.
Canonical-English coverage as of 2026-05-12: README, CONTRIBUTING,
SECURITY, GOVERNANCE, MAINTAINERS, ADOPTERS, ROADMAP, runbook,
troubleshooting, release-checklist. The remaining 10 deep-dive
documents above ship Korean-only until their next functional update.
