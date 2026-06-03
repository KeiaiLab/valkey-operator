# valkey-operator

A Kubernetes operator for running [Valkey](https://valkey.io/) — standalone, replicated, or as a sharded cluster — with backup and restore.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![Valkey](https://img.shields.io/badge/Valkey-8%2B-6979DC?logo=redis&logoColor=white)](https://valkey.io/)

Built with [Kubebuilder](https://book.kubebuilder.io/). A single controller manages standalone instances, primary/replica replication, and sharded Valkey Cluster through five custom resources, and reconciles the StatefulSets, Services, ConfigMaps, Secrets, and related objects each one needs.

## Custom resources

| Kind | What it does |
|---|---|
| `Valkey` | A single instance, or one primary with replicas |
| `ValkeyCluster` | A sharded Valkey Cluster (16384 slots across N shards) |
| `ValkeyBackup` | An RDB, AOF, or VolumeSnapshot backup of an instance |
| `ValkeyBackupTarget` | An S3 / GCS / Azure storage endpoint, shared by backups and restores |
| `ValkeyRestore` | Restores a backup into a `Valkey` or `ValkeyCluster` |

All resources use the API group `cache.keiailab.io`.

## Features

- **One operator, three topologies** — standalone, primary/replica replication, and sharded cluster, all through the same resources.
- **Automatic failover** for replication mode — promotes the most up-to-date replica when the primary goes down.
- **Backup and restore** — RDB, AOF, or CSI `VolumeSnapshot`, to a PVC or to S3-compatible storage (also GCS and Azure Blob).
- **TLS** via cert-manager, or your own certificate `Secret`.
- **Authentication on by default** — a password `Secret` is generated automatically when you don't supply one.
- **Prometheus integration** — optional `ServiceMonitor`, a bundled `PrometheusRule`, and Grafana dashboards.
- **Optional `NetworkPolicy` and `PodDisruptionBudget`** generation.

## Installation

Requires Kubernetes 1.26+ and [cert-manager](https://cert-manager.io/) (for the admission webhook). Install cert-manager first if you don't already run it:

```sh
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy --all
```

### Helm

```sh
helm repo add valkey-operator https://keiailab.github.io/valkey-operator
helm install valkey-operator valkey-operator/valkey-operator \
    --namespace valkey-operator-system --create-namespace
```

The container image is published at `ghcr.io/keiailab/valkey-operator`.

### From source

```sh
make install                              # install the CRDs
make deploy IMG=ghcr.io/keiailab/valkey-operator:latest
```

## Usage

Create a standalone instance:

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: my-cache
spec:
  mode: Standalone
  replicas: 1
  version:
    image: docker.io/valkey/valkey
    version: "9.0.4"
  storage:
    size: 8Gi
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
  auth:
    enabled: true
```

Connect to it:

```sh
kubectl apply -f my-cache.yaml
PASS=$(kubectl get secret my-cache-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec my-cache-0 -- valkey-cli -a "$PASS" ping   # PONG
```

For a sharded cluster, use `ValkeyCluster` with `shards` and `replicasPerShard`:

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyCluster
metadata:
  name: my-cluster
spec:
  shards: 3
  replicasPerShard: 1
  autoFailover: true
  version:
    image: docker.io/valkey/valkey
    version: "9.0.4"
  storage:
    size: 8Gi
```

Runnable manifests for every resource — including backup, restore, and external storage targets — live in [`config/samples/`](config/samples/).

## Roadmap

Implemented:

- Standalone, replication, and sharded cluster topologies
- Automatic failover for replication mode
- Backup to a PVC, VolumeSnapshot, or S3 / GCS / Azure
- Restore (RDB / AOF), including from external storage
- TLS via cert-manager, Prometheus alerts, and Grafana dashboards

Planned:

- Automatic cluster resharding
- Point-in-time recovery (AOF replay)
- Horizontal autoscaling for replication mode
- A `v1beta1` API and conversion webhook

The CRDs are `v1alpha1`; expect API changes before a stable release.

## Documentation

- [Documentation index](docs/README.md)
- [Operations runbook](docs/operations/runbook.md)
- [Architecture Decision Records](docs/kb/adr/INDEX.md)

## Contributing

Contributions are welcome. For anything non-trivial, please open an issue first so we can agree on the API surface. See [CONTRIBUTING.md](CONTRIBUTING.md), and run `make help` for the full list of build targets.

To report a security issue, follow [SECURITY.md](.github/SECURITY.md) rather than opening a public issue.

## License

[MIT](LICENSE) © keiailab
