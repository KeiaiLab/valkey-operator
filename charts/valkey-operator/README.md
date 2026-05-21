# Valkey Operator Helm Chart

A Kubernetes Operator for deploying and managing Valkey instances and Clusters
(Valkey is the BSD-licensed Redis OSS fork stewarded by the Linux Foundation).

## Features

- **Valkey instance**: Standalone or Replication (1 primary + N replicas) with persistent storage
- **Valkey Cluster**: Redis Cluster compatible (16384 slots, auto-resharding)
- **TLS Encryption**: cert-manager integration + auto-generated SelfSigned Issuer (PR #40)
- **Authentication**: password + ACL v2 + auto rolling restart on rotation (PR #46)
- **Monitoring**: 12 Prometheus metrics (PR #47 reconcile latency Histogram, PR #59 SLO alerts, PR #64 capability Gauge) + ServiceMonitor + 12 PrometheusRule alerts
- **Backup/Restore**: S3 / GCS / Azure / PVC / VolumeSnapshot 5 backend (PR #42 + #51 + #57)
- **Auto-scaling**: HPA for Replication mode (PR #44, ADR-0027 implemented) — CPU + Memory targets
- **PodDisruptionBudget**: HA default — auto-create when replicas≥2 (PR #49)
- **NetworkPolicy**: deny-by-default ingress automation
- **TopologySpreadConstraints**: HA default — multi-AZ + multi-node spread when replicas≥2 (PR #48)
- **Encryption-at-rest**: StorageClass audit + enforce mode for compliance (PR #45 + #55)
- **Slow-log**: configurable threshold + max entries (PR #45)
- **PVC auto-resize**: automatic expansion when `spec.storage.size` changes (PR #39)
- **Chaos engineering**: chaos-mesh foundation (PR #41, ADR-0041)
- **PITR foundation**: Source.VolumeSnapshot + PointInTime API (PR #51-#58 + #54)
- **Status.Capabilities**: active features visible at a glance with `kubectl get -o wide` (PR #62)
- **External chart compatibility knobs**: image digest, storage/service/pod knobs,
  external replica, revision history, dual-stack Service, chart extraObjects (ADR-0043)

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
  --set features.backup.enabled=true
  # features.autoscaling.enabled — ADR-0027 deferred (RBAC only, HPA controller not implemented yet)
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
    version: "9.0.4"
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
    version: "9.0.4"
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

## Production Features (cycles 65-74)

This chart provides five opt-in controls for production-grade operations:

### 1. Admission Webhooks

```bash
helm upgrade valkey-operator . --set webhook.enabled=true
```

cert-manager must be installed before enabling this option. The chart creates
Validating and Mutating WebhookConfiguration resources, a SelfSigned Issuer,
a Certificate, and the webhook Service. Invalid custom resources are rejected
at admission time.

### 2. NetworkPolicy

```bash
helm upgrade valkey-operator . --set networkPolicy.enabled=true
```

The operator pod uses default-deny networking with explicit ingress for
Prometheus on 8443 and egress for the Kubernetes API on 443, DNS on 53, and
Valkey on 6379/6380/16379.

### 3. OpenTelemetry Tracing (ADR-0025)

```bash
helm upgrade valkey-operator . \
  --set tracing.endpoint=tempo.observability.svc:4317
```

Enables the OTLP gRPC exporter and emits 22 trace spans. When the endpoint is
empty, the operator uses a no-op tracer with no runtime overhead.

### 4. Namespace-Scoped Watch

```bash
helm upgrade valkey-operator . --set 'watch.namespaces={valkey-prod,valkey-stage}'
```

Limits the reconciliation surface in multi-tenant environments. This is
separate from the cluster-wide ClusterRole; namespaced RBAC should be supplied
by the platform operator when required.

### 5. Version Identification

```bash
# CLI verification:
kubectl exec -n valkey-operator-system \
  -l app.kubernetes.io/name=valkey-operator -- /manager --version
# -> "valkey-operator vX.Y.Z (commit abc1234, built YYYY-MM-DD)"

# Prometheus dashboard:
sum by (version) (valkey_cluster_build_info)
```

Use this to identify the running release tag and detect version skew.

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

<!-- helm-docs automatically updates this section via `make helm-docs` or `helm-docs --chart-search-root charts/valkey-operator`. It extracts the `# --` comments from values.yaml. -->
