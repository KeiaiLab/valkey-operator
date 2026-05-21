# Operations docs index

> 한국어 사용자: 모든 문서가 `<name>.ko.md` 한국어 사본을 함께 보관합니다.

This directory holds operational procedures for `valkey-operator`.
**Every document is canonical English** with a Korean sibling at
`<name>.ko.md` (a single artifacthub-trust.md was already English
and has no Korean copy).

## Day-to-day operations

| Document | Purpose |
|---|---|
| [runbook.md](runbook.md) | Incident response and daily operations. SSOT for every Prometheus alert's `runbook_url` annotation. |
| [troubleshooting.md](troubleshooting.md) | Symptom → cause → diagnostic → remediation flowchart for issues that fire no alert (or fire before one exists). |
| [metrics-glossary.md](metrics-glossary.md) | Every `valkey_cluster_*` metric, its label cardinality, where it is emitted, and which alerts consume it. |
| [capacity-planning.md](capacity-planning.md) | Sizing guide — memory, replication factor, shard count, p95/p99 latency targets per topology. |
| [webhook.md](webhook.md) | Admission-webhook architecture, cert-manager certificate path, debugging webhook denials. |
| [pdb-per-shard.md](pdb-per-shard.md) | Per-shard `PodDisruptionBudget` operations for sharded `ValkeyCluster` — drain scenarios, mode transitions, write-availability guarantees. |

## Backup, restore, recovery

| Document | Purpose |
|---|---|
| [pitr-guide.md](pitr-guide.md) | Point-in-time recovery: phase-1 API + webhook, phase-2 reconciler dispatch, manual workaround, rollback. |
| [chaos-testing.md](chaos-testing.md) | chaos-mesh 4-scenario e2e procedure: pod-kill, network partition, IO ENOSPC, IO latency. |

## Release & supply chain

| Document | Purpose |
|---|---|
| [release-checklist.md](release-checklist.md) | Pre-release gate inventory: build, 47 SSOT gates, supply chain (SLSA + cosign), docs, operations. |
| [post-merge-cleanup.md](post-merge-cleanup.md) | Local-branch hygiene after squash-merge. |
| [artifacthub-trust.md](artifacthub-trust.md) | Artifact Hub `Signed` and `Official` trust badge operational procedure. |

## Migration

| Document | Purpose |
|---|---|
| [sentinel-migration.md](sentinel-migration.md) | Sentinel → valkey-operator Replication-mode migration runbook (ADR-0017 backstop). |

## Cross-cutting references

- Architecture decisions: [`docs/kb/adr/INDEX.md`](../kb/adr/INDEX.md)
- Incident history: [`docs/kb/incident/`](../kb/incident/)
- Release verification commands: [`SECURITY.md → "Verifying release artifacts"`](../../.github/SECURITY.md#verifying-release-artifacts-signed-releases--v1013)
- Roadmap (project direction): [`ROADMAP.md`](../../ROADMAP.md)

## i18n status

Every operational document under `docs/operations/` is now
**canonical English**. The i18n initiative landed across PRs
#93 / #97 / #98 / #103 / #104 / #105 / #106 / #107 / #108 / #109 /
#110 / #111. Korean originals are preserved verbatim as
`<name>.ko.md` siblings (except artifacthub-trust.md, which was
authored in English from the start).
