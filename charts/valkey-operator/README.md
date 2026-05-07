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
- **Auto-scaling**: HPA — *DEFERRED (ADR-0027)*. RBAC 권한만 부여, 실제 HPA reconciler 미구현. v0.1.0 stable 후 활성 예정.
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
