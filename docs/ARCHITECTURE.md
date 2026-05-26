<p align="center">
  <b>English</b> |
  <a href="ARCHITECTURE.ko.md">한국어</a> |
  <a href="ARCHITECTURE.ja.md">日本語</a> |
  <a href="ARCHITECTURE.zh.md">中文</a>
</p>

# ARCHITECTURE — valkey-operator

> Single-page architecture description. Updated when CRD surface / topology / reconcile pattern changes.

## Overview

- **Purpose**: Kubebuilder-based K8s operator for [Valkey](https://valkey.io) (BSD-3 fork of Redis). One controller manages three topologies behind a uniform CRD surface.
- **Scope**: Standalone / Replication / Cluster (16384-slot) topologies + backup/restore + S3-compatible external storage.
- **Stability tier**: v1.0.13 (GA on standalone + replication + cluster; federation alpha)
- **Latest release**: v1.0.13 (2026-05-13)
- **License**: Apache-2.0
- **Module path**: `github.com/keiailab/valkey-operator`

## CRD surface (5 CRDs)

| CRD | apiVersion | Topology | Description |
|---|---|---|---|
| `Valkey` | `valkey.keiailab.com/v1alpha2` | Standalone / Replication | Single instance or 1 primary + N replicas |
| `ValkeyCluster` | `valkey.keiailab.com/v1alpha2` | Sharded Cluster (16384 slots) | 3+ shards × (1 primary + 0–5 replicas) |
| `ValkeyBackup` | `valkey.keiailab.com/v1alpha2` | — | One-shot RDB or AOF backup to PVC + external storage |
| `ValkeyBackupTarget` | `valkey.keiailab.com/v1alpha2` | — | S3-compatible external storage abstraction (ADR-0016) |
| `ValkeyRestore` | `valkey.keiailab.com/v1alpha2` | — | Restore RDB into Valkey or ValkeyCluster via Init Container (ADR-0015) |

Conversion webhook supports v1alpha1 ↔ v1alpha2.

## Reconcile flow

```
Watch CRD events
      │
      ▼
Reconcile loop
      │
      ├── StatefulSet (per shard)
      ├── ConfigMap (valkey.conf)
      ├── Secret (auth + TLS keys)
      ├── Service (headless + ClusterIP)
      ├── PodDisruptionBudget
      ├── NetworkPolicy (deny-by-default)
      ├── cert-manager Certificate (webhook serving + TLS)
      └── Prometheus ServiceMonitor

All resources reconciled with spec-drift detection.
Cluster topology: slot rebalance + replica re-election on shard scale.
```

## RBAC scope

- ClusterRole: CRD watch + cert-manager Certificate + Prometheus ServiceMonitor
- Role (per ns): StatefulSet / Service / Secret / ConfigMap / PVC / PDB / NetworkPolicy / Job
- ServiceAccount: `valkey-operator`
- Webhook: validation + conversion (TLS via cert-manager)

## Test layers

| Layer | Location | Coverage |
|---|---|---|
| Unit | `internal/**/_test.go`, `api/**/_test.go` | gocovmerge → cover-final.out |
| Integration (envtest) | `test/integration/` | reconcile + conversion + webhook |
| E2E (kind) | `test/e2e/`, `Makefile setup-test-e2e` | release-critical scenarios |
| Scorecard | `bundle/tests/scorecard/` | OLM v1alpha3 6-test parity |

## Build / deploy

- Container image: `ghcr.io/keiailab/valkey-operator:v1.0.13`
- Helm chart: `charts/valkey-operator/` (published at `keiailab.github.io/valkey-operator`)
- OLM bundle: `bundle/`
- ArtifactHub: `keiailab-valkey-operator`
- Quickstart: kind cluster + cert-manager 1.16+ (`make setup-test-e2e`)

## Security supply chain

- **SLSA-3 provenance** (ADR-0046)
- **cosign keyless signing** (ADR-0046)
- **OpenSSF Scorecard** active (badge in README)
- **CodeQL** + **dependency-review** + **DCO** workflows
- **`.gitleaks.toml`** secret scanning (42/44 coverage)
- **go-licenses** dependency-license scan + allowlist

## ADR cross-link (45 ADRs — most ADR-rich of 3 operators)

Notable:
- ADR-0015: Restore via Init Container pattern
- ADR-0016: ValkeyBackupTarget — S3 abstraction
- ADR-0045: GitHub Actions release pipeline restoration
- ADR-0046: SLSA-3 + cosign keyless
- ADR-0047: community-operators upstream sync automation (cycle 25)

Full list: `docs/kb/adr/INDEX.md`.

## Roadmap status

- Done: 31 items (Cluster mode + backup/restore + HPA/PDB/NP + version-upgrade + Valkey 9.x + API evolution + webhook admission + Helm + SLSA-3 + ServiceMonitor + OpenSSF)
- Pending: 38 items (production cluster adoption + migration runbook + smoke test + Grafana + OTel + SBOM + 9.x feature follow-up + multi-cluster federation + cross-region replication + online schema-less migration + weighted replica routing + controller v2 + CRD v1 graduation)

## Non-goals

- ❌ Embedded Redis (we serve Valkey — license-compatible BSD-3 fork)
- ❌ Third-party Valkey chart embedding (we implement natively)
- ❌ Redis Sentinel topology (use 3-shard cluster instead)
- ❌ Valkey version < 8.0

## References

- `README.md` / `README.ko.md`
- `ROADMAP.md`
- `CHANGELOG.md`
- `ADOPTERS.md` / `ADOPTERS.ko.md`
- `CONTRIBUTING.md` / `CONTRIBUTING.ko.md`
- `GOVERNANCE.md` / `GOVERNANCE.ko.md`
- `AGENTS.md`
- `docs/kb/adr/INDEX.md`
