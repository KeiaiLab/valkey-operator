# valkey-operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org/)
[![Valkey](https://img.shields.io/badge/Valkey-8.0+-FF4438?logo=redis)](https://valkey.io/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes)](https://kubernetes.io/)
[![Container Image](https://img.shields.io/badge/ghcr.io-keiailab%2Fvalkey--operator-blue?logo=github)](https://github.com/keiailab/valkey-operator/pkgs/container/valkey-operator)
[![Helm Chart](https://img.shields.io/badge/dynamic/yaml?url=https://raw.githubusercontent.com/keiailab/valkey-operator/main/charts/valkey-operator/Chart.yaml&label=helm%20v)](https://keiailab.github.io/valkey-operator)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/keiailab-valkey-operator)](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/keiailab/valkey-operator/badge)](https://scorecard.dev/viewer/?uri=github.com/keiailab/valkey-operator)
[![GitHub Discussions](https://img.shields.io/github/discussions/keiailab/valkey-operator?label=discussions&logo=github)](https://github.com/keiailab/valkey-operator/discussions)

> 한국어 README: [README.ko.md](README.ko.md)

A Kubebuilder-based Kubernetes operator for [Valkey](https://valkey.io/)
(the BSD-3 fork of Redis). One controller manages three operational
topologies behind a uniform CRD surface.

| CRD | Purpose | Topology |
|---|---|---|
| `Valkey` | Single instance, or one primary with N replicas | Standalone / Replication |
| `ValkeyCluster` | Sharded Valkey Cluster (16384 slots) | 3+ shards × (1 primary + 0–5 replicas) |
| `ValkeyBackup` | One-shot RDB or AOF backup | PVC (`<backup>-backup`), external storage optional |
| `ValkeyBackupTarget` | S3-compatible external storage abstraction | Shared between Backup and Restore (ADR-0016) |
| `ValkeyRestore` | Restore an RDB into a Valkey or ValkeyCluster instance | Init Container pattern (ADR-0015) |

The operator reconciles `StatefulSet`, `ConfigMap`, `Secret`,
`Service` (headless + ClusterIP), `PodDisruptionBudget`,
`NetworkPolicy`, `cert-manager` `Certificate`, and Prometheus
`ServiceMonitor` — all with spec-drift detection.

## Quickstart (kind)

Every command below is exercised on every release; the kind cluster
bootstrap is the canonical local-dev path.

### 1. Prerequisites

| Tool | Minimum | Notes |
|---|---|---|
| Go | 1.26 | Matches `go.mod` |
| Docker | 24+ | buildx default builder |
| kind | 0.27+ | Local cluster |
| kubectl | 1.34+ | k3s/kind compatible |
| cert-manager | 1.16+ | Webhook serving cert |

### 2. kind cluster + cert-manager

```sh
make setup-test-e2e
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy --all
```

### 3. Build, load, deploy

```sh
make docker-build IMG=valkey-operator:dev
kind load docker-image valkey-operator:dev --name valkey-operator-test-e2e
make install                          # CRDs
make deploy IMG=valkey-operator:dev   # operator + RBAC + webhook
kubectl -n valkey-operator-system rollout status deploy/valkey-operator-controller-manager
```

### 4. Apply sample CRs

```sh
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeycluster.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeybackup.yaml
```

### 5. Data-plane smoke

```sh
PASS=$(kubectl get secret valkey-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" ping     # PONG
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" set k v  # OK
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" get k    # v

# Cluster mode — `-c` follows MOVED redirects automatically
PASS=$(kubectl get secret valkeycluster-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkeycluster-sample-0 -- valkey-cli -a "$PASS" cluster info | head -3
# cluster_state:ok / cluster_slots_assigned:16384 / cluster_slots_ok:16384
```

## Helm

```sh
helm repo add valkey-operator https://keiailab.github.io/valkey-operator
helm install valkey-operator valkey-operator/valkey-operator \
    --namespace valkey-operator-system --create-namespace
```

The chart is also published to
[Artifact Hub](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator)
with the `Signed` trust badge (ADR-0044, ADR-0046).

## Key features

- **Three topologies, one operator.** Standalone, Replication, and
  Valkey Cluster all share a single reconciler set with a uniform
  status surface.
- **Automatic failover** for Replication mode — selects the replica
  with the largest `master_repl_offset` and promotes it with
  `REPLICAOF NO ONE` (ADR-0017).
- **Backup / Restore** — RDB or AOF to a PVC, S3, or any
  S3-compatible endpoint (MinIO, Ceph RGW). Restore uses an Init
  Container pattern so the main container loads the RDB
  transparently (ADR-0015, ADR-0016, ADR-0022, ADR-0023).
- **TLS + mTLS** via cert-manager auto-discovery (ADR-0010,
  ADR-0014) or a user-provided `Secret`.
- **Always-on auth.** A random 32-byte password is generated when
  `Auth.Enabled` is unset (ADR-0013).
- **NetworkPolicy** — opt-in, restricts pod-to-pod traffic to
  6379/16379 (CNI-enforced).
- **Observability.** OTEL tracing with 22 spans (zero overhead when
  `OTEL_EXPORTER_OTLP_ENDPOINT` is unset), Prometheus alert rules,
  ServiceMonitor auto-generation.
- **Supply chain.** SBOM (syft SPDX) + Trivy scan + cosign keyless
  signing + SLSA-3 provenance starting with v1.0.13 (ADR-0046).
  See [SECURITY.md](SECURITY.md) for verification commands.

## Documentation

| Topic | Where |
|---|---|
| Detailed Korean walkthrough | [README.ko.md](README.ko.md) |
| Runbook (Backup, Restore, Scaling, Upgrade, Emergency) | [docs/operations/runbook.md](docs/operations/runbook.md) |
| Release pre-flight checklist | [docs/operations/release-checklist.md](docs/operations/release-checklist.md) |
| Architecture Decision Records | [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md) |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Security policy + artifact verification | [SECURITY.md](SECURITY.md) |
| Project governance | [GOVERNANCE.md](GOVERNANCE.md) |
| Adopters | [ADOPTERS.md](ADOPTERS.md) |

## Production readiness

This operator is in `v1alpha1`, but it ships the quality system of a
commercial product:

- **29 SSOT-parity gates** — alert / runbook / RBAC / CRD / sample /
  chart artifacts drift-blocked by lefthook pre-push.
- **Automatic chart-CRD sync** by `make manifests`; `git push`
  blocks on a stale `go mod tidy`.
- **Microbenchmarks** for the five hot-path parsers
  (`go test -bench=. ./internal/valkey/`).
- **Operator runbook** with 9 sections plus per-alert
  Trigger/Diagnosis/Mitigation/Escalation.
- **Supply chain.** Apache-2.0 license, PGP-signed security
  disclosures, signed Helm chart + image starting v1.0.13.
- **Reusable conventions** are shared across the sibling operators
  `mongodb-operator`, `postgres-operator`, and `operator-commons`.

## Roadmap

The roadmap below is qualitative — no calendar commitments. Progress
is tracked by feature completion, not by quarter.

Already shipped (alpha):

- ✅ Standalone / Replication / ValkeyCluster topologies
- ✅ Backup to PVC and S3-compatible storage
- ✅ Restore via Init Container (ADR-0015)
- ✅ Replication automatic failover (ADR-0017)
- ✅ Prometheus alerts + runbook
- ✅ OTEL tracing
- ✅ Helm chart + Artifact Hub publication

Next:

- [ ] End-to-end automation on kind + MinIO
- [ ] ValkeyCluster automatic resharding (ADR-0018)
- [ ] HPA integration for Replication mode (ADR-0027, deferred until v1alpha1 stable)
- [ ] Conversion webhook for `v1beta1` (ADR-0026, deferred)
- [ ] First `v0.1.0` GA after Track A/B/E stabilization and a 24-hour soak test

Decision rationale lives in [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md).
Feature requests go on [Issues](https://github.com/keiailab/valkey-operator/issues)
or in GitHub Discussions.

## Known limitations

This is `v1alpha1` software, exercised on every release but not yet
GA. Current known caveats:

- `Spec.Auth.Enabled=false` is honoured as a no-op — the operator
  always provisions auth (ADR-0013). If you need an unauthenticated
  cluster, do not deploy this operator.
- IPv6-only environments are untested; `CLUSTER MEET` prefers IPv4
  hostnames (ADR-0012).
- `NetworkPolicy.Enabled` only emits the resource; *actual*
  enforcement depends on a policy-aware CNI (Calico, Cilium).
- Replication automatic failover gives no strong split-brain
  guarantee under network partitions — see ADR-0017 for the trade-off.
- ValkeyCluster restore requires `ReadOnlyMany` or `ReadWriteMany`
  source PVC accessMode; RWO is not supported.
- `cluster-announce-hostname` is not used; revisit if you run on a
  Kubernetes-aware DNS service that resolves pod hostnames into
  routable IPs differently from the in-cluster DNS the operator
  already uses.

A fuller Korean-language list lives in
[README.ko.md → 잠재적 운영 이슈](README.ko.md#잠재적-운영-이슈-현재-알려진-한계).

## Uninstall

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). External contributions are
welcome; please open an issue first for any non-trivial change so we
can align on the API surface before you write code.

Run `make help` to see every Makefile target. Background reading:
[Kubebuilder book](https://book.kubebuilder.io/introduction.html).

## Reporting vulnerabilities

Do **not** open a public issue. Use the private channels in
[SECURITY.md](SECURITY.md) — GitHub Security Advisory or
`security@keiailab.com` (PGP key in `artifacthub-repo.yml`).

## License

Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0
(<http://www.apache.org/licenses/LICENSE-2.0>). Distributed on an
"AS IS" basis, without warranties or conditions of any kind. See the
[LICENSE](LICENSE) file for the full text.
