# Documentation — valkey-operator

> All documents are canonical **English**. Where a Korean sibling
> exists it is preserved as `<name>.ko.md` (and similarly for `.ja.md`
> and `.zh.md`). This page is the **single entry point** for everything
> under `docs/`.

## Operational guides

| Document | Purpose |
|---|---|
| [Operations index](operations/README.md) | Day-to-day runbook + troubleshooting + metrics + webhook + PDB + sizing |
| [Runbook](operations/runbook.md) | Incident response and daily operations — SSOT for every Prometheus alert's `runbook_url` |
| [Troubleshooting](operations/troubleshooting.md) | Symptom → cause → diagnostic → remediation |
| [Metrics glossary](operations/metrics-glossary.md) | Every `valkey_cluster_*` metric, cardinality, alert consumer |
| [Capacity planning](operations/capacity-planning.md) | Sizing — memory, replication factor, shard count, p95/p99 latency targets |
| [Webhook](operations/webhook.md) | Admission webhook architecture, cert-manager path, denial debugging |
| [PDB per shard](operations/pdb-per-shard.md) | Per-shard `PodDisruptionBudget` operations, drain scenarios |
| [PITR guide](operations/pitr-guide.md) | Point-in-time recovery — API + webhook + manual workaround |
| [Chaos testing](operations/chaos-testing.md) | chaos-mesh 4-scenario e2e procedure |

## Release & supply chain

| Document | Purpose |
|---|---|
| [Release checklist](operations/release-checklist.md) | Pre-release gate inventory: build, SSOT gates, supply chain, docs |
| [Post-merge cleanup](operations/post-merge-cleanup.md) | Local-branch hygiene after squash-merge |
| [Artifact Hub trust](operations/artifacthub-trust.md) | Artifact Hub `Signed` and `Official` badge operational procedure |
| [Upgrading](UPGRADING.md) | Minor / major upgrade migration, RBAC patch for static-manifest users |

## Migration

| Document | Purpose |
|---|---|
| [Migration index](migration/README.md) | StatefulSet → ValkeyCluster CR migration runbook catalog |
| [Sentinel migration](operations/sentinel-migration.md) | Existing Sentinel HA → Replication-mode + AutoFailover (ADR-0017 backstop) |

## Architecture & decision records

| Document | Purpose |
|---|---|
| [ADR index](kb/adr/INDEX.md) | All 52+ Architecture Decision Records |
| [Observability — OpenTelemetry](observability/otel.md) | OTel trace propagation, OTLP exporter, controller-reconcile span |
| [Valkey 9.x feature flags](version/9x-flags.md) | New cluster-mode flags + operator integration path |

## Knowledge base

| Document | Purpose |
|---|---|
| [Incident KB](kb/incident/INDEX.md) | Postmortem-lite records (blameless) |
| [Dependency change log](kb/deps/2026-05.md) | go.mod / go.sum diff history |

## Operator family

[keiailab operator family](family.md) — postgres-operator · mongodb-operator ·
**valkey-operator** · operator-commons (shared Go library).
Available in [한국어](family.ko.md) · [日本語](family.ja.md) · [中文](family.zh.md).

## i18n status

Korean siblings live next to each English canonical (`<name>.ko.md`).
Japanese (`.ja.md`) and Chinese (`.zh.md`) coverage is **focused on
top-level files** (README, ROADMAP, CONTRIBUTING, family). Operational
runbooks are progressively localized as adoption demand surfaces.
