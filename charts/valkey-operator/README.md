# Valkey Operator Helm Chart

A Kubernetes Operator for deploying and managing Valkey instances and Clusters
(Valkey is the BSD-licensed Redis OSS fork stewarded by the Linux Foundation).

## Features

- **Valkey instance**: Standalone or replica deployment with persistent storage
- **Valkey Cluster**: Redis Cluster compatible distributed mode (opt-in)
- **TLS Encryption**: Automatic TLS with cert-manager integration
- **Authentication**: ACL-based auth with secret-managed credentials
- **Monitoring**: Native cluster metrics with ServiceMonitor support
- **Backup/Restore**: S3 / PVC backup targets and point-in-time restore (opt-in)
- **Auto-scaling**: HPA support for replica counts (opt-in)
- **PodDisruptionBudget**: opt-in disruption guards
- **NetworkPolicy**: deny-by-default ingress automation

## Prerequisites

- Kubernetes 1.26+
- Helm 3.8+
- kubectl configured to communicate with your cluster

### Optional Dependencies

- [cert-manager](https://cert-manager.io/) for TLS certificate management
- [Prometheus Operator](https://prometheus-operator.dev/) for metrics collection
- S3-compatible storage for backups (e.g., AWS S3, MinIO, Ceph ObjectStore)

## Installation

### Add the Helm Repository

```bash
helm repo add valkey-operator https://keiailab.github.io/valkey-operator
helm repo update
```

### Install the Chart

```bash
helm install valkey-operator valkey-operator/valkey-operator \
  --namespace valkey-operator-system \
  --create-namespace
```

### Install with Optional Features Enabled

```bash
helm install valkey-operator valkey-operator/valkey-operator \
  --namespace valkey-operator-system \
  --create-namespace \
  --set features.cluster.enabled=true \
  --set features.backup.enabled=true \
  --set features.autoscaling.enabled=true
```

## Usage

### Standalone Valkey instance

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: my-valkey
  namespace: cache
spec:
  mode: Standalone
  replicas: 1
  version:
    version: "8.1.6"
  storage:
    storageClassName: standard
    size: 5Gi
  auth:
    secretRef:
      name: valkey-credentials
```

### Valkey Cluster (Redis Cluster compatible)

> Requires `features.cluster.enabled=true` at chart install time.

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyCluster
metadata:
  name: my-valkey-cluster
  namespace: cache
spec:
  version:
    version: "8.1.6"
  shards: 3
  replicasPerShard: 1
  storage:
    storageClassName: standard
    size: 10Gi
```

### Backup & Restore

> Requires `features.backup.enabled=true`.

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackupTarget
metadata:
  name: s3-target
  namespace: cache
spec:
  type: s3
  s3:
    bucket: valkey-backups
    endpoint: https://s3.amazonaws.com
    credentialsRef:
      name: s3-credentials
---
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata:
  name: my-valkey-backup
  namespace: cache
spec:
  sourceRef:
    name: my-valkey
    kind: Valkey
  targetRef:
    name: s3-target
  compression: true
```

## Uninstall

```bash
helm uninstall valkey-operator -n valkey-operator-system
kubectl delete crd valkeys.cache.keiailab.io
kubectl delete crd valkeyclusters.cache.keiailab.io
kubectl delete crd valkeybackups.cache.keiailab.io
kubectl delete crd valkeybackuptargets.cache.keiailab.io
kubectl delete crd valkeyrestores.cache.keiailab.io
```

> ⚠️ Deleting CRDs will remove all Valkey custom resources cluster-wide.
> Back up your data before uninstalling in production.

## Troubleshooting

```bash
# Check operator pod
kubectl get pods -n valkey-operator-system

# Operator logs
kubectl logs -n valkey-operator-system -l app.kubernetes.io/name=valkey-operator -f

# Inspect a Valkey resource
kubectl describe valkey my-valkey -n cache
```

## Links

- Source: https://github.com/keiailab/valkey-operator
- Valkey project: https://valkey.io/
- Issues: https://github.com/keiailab/valkey-operator/issues

## Values

<!-- helm-docs 가 본 section 을 자동 갱신: `make helm-docs` 또는 `helm-docs --chart-search-root charts/valkey-operator`. values.yaml 의 `# --` 주석을 추출. -->

