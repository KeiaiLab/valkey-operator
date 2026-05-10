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
- **PVC auto-resize**: Spec.Storage.Size 변경 시 자동 expansion (PR #39)
- **Chaos engineering**: chaos-mesh foundation (PR #41, ADR-0041)
- **PITR foundation**: Source.VolumeSnapshot + PointInTime API (PR #51-#58 + #54)
- **Status.Capabilities**: kubectl get -o wide 한눈에 활성 features (PR #62)

자세한 평가: `docs/operations/commercial-parity-status.md` (Redis Enterprise ~94% parity).

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
  # features.autoscaling.enabled — ADR-0027 deferred (RBAC 만 부여, 실제 HPA 미구현)
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

본 chart 는 *production-grade 운영* 을 위한 5 항목 opt-in:

### 1. Admission Webhooks

```bash
helm upgrade valkey-operator . --set webhook.enabled=true
```

cert-manager 사전 설치 필수. Validating + Mutating WebhookConfiguration +
selfSigned Issuer + Certificate + webhook Service 자동 생성. invalid CR
admission 차단.

### 2. NetworkPolicy

```bash
helm upgrade valkey-operator . --set networkPolicy.enabled=true
```

operator pod default-deny + 명시 ingress (Prometheus 8443) + egress (K8s API
443, DNS 53, Valkey 6379/6380/16379).

### 3. OpenTelemetry Tracing (ADR-0025)

```bash
helm upgrade valkey-operator . \
  --set tracing.endpoint=tempo.observability.svc:4317
```

OTLP gRPC exporter 활성. 22 trace spans 발행. endpoint 비어있으면 no-op
tracer (성능 영향 0).

### 4. Namespace-Scoped Watch

```bash
helm upgrade valkey-operator . --set 'watch.namespaces={valkey-prod,valkey-stage}'
```

multi-tenant 환경에서 reconcile 표면 제한 (cluster-wide ClusterRole 와는
별개 — 사용자 자체 namespaced RBAC 권장).

### 5. Version Identification

```bash
# CLI 시점 검증:
kubectl exec -n valkey-operator-system \
  -l app.kubernetes.io/name=valkey-operator -- /manager --version
# → "valkey-operator vX.Y.Z (commit abc1234, built YYYY-MM-DD)"

# Prometheus dashboard:
sum by (version) (valkey_cluster_build_info)
```

운영 시점 release tag 식별 + version skew 감지.

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
