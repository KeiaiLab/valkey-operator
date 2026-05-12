# CloudPirates valkey 0.20.2 compatibility mapping

> 한국어 버전: [cloudpirates-valkey-compatibility.ko.md](cloudpirates-valkey-compatibility.ko.md)

Target: the ArtifactHub `cloudpirates-valkey/valkey` chart `0.20.2`
(appVersion `9.0.0`). This document is the **single mapping table**
an operator should consult when porting that chart's `values` to
`valkey-operator` CRDs.

## Mapping principles

- We do **not** clone CloudPirates' value names verbatim. We offer
  a CRD contract that a Kubernetes operator can own with stable
  semantics.
- Items that introduce a dual control-plane (Sentinel) or a TCP/L7
  mismatch (HTTPRoute) are **not implemented directly**; we offer
  an operationally safer alternative path.
- Literal password values are **never** placed in a CRD — create a
  `Secret` first and reference it with `SecretKeySelector`.

## Feature mapping

| CloudPirates value | valkey-operator path | Status |
|---|---|---|
| `architecture=standalone` | `Valkey.spec.mode=Standalone` | Supported |
| `architecture=replication`, `replicaCount` | `Valkey.spec.mode=Replication`, `spec.replicas` | Supported |
| Cluster sharding | `ValkeyCluster.spec.shards`, `replicasPerShard` | Supported (beyond chart scope) |
| `revisionHistoryLimit` | `spec.revisionHistoryLimit` | Supported |
| `image.registry/repository/tag`, digest tag | `spec.version.imageRef` or `image` + `version` | Supported |
| `image.pullPolicy` | `spec.version.imagePullPolicy` | Supported |
| `imagePullSecrets` | `spec.pod.imagePullSecrets` | Supported |
| `auth.enabled` | `spec.auth.enabled` | Supported |
| `auth.existingSecret`, key | `spec.auth.passwordSecretRef` | Supported |
| `auth.password` (literal) | Create a `Secret`, then `passwordSecretRef` | Literal not supported |
| `tls.enabled` | `spec.tls.enabled` | Supported |
| `tls.existingSecret` | `spec.tls.customCert.secretName` | Supported; keys normalized to `tls.crt`, `tls.key`, `ca.crt` |
| `tls.authClients` | `spec.tls.clientAuth=required|disabled` | Supported |
| `config.maxMemory`, `maxMemoryPolicy`, `extraConfig` | `spec.additionalConfig` | Supported |
| `config.save` | `spec.persistence.rdbSaveSchedule` or `additionalConfig.save` | Supported |
| `config.existingConfigmap` | Operator-generated config merged with `additionalConfig` | Direct replacement not supported |
| `externalReplica.*` | `spec.externalReplica.*` | Supported (v1alpha1: Standalone only) |
| `service.type`, annotations/labels | `spec.service.type`, annotations/labels | Supported |
| `service.port`, `targetPort` | Valkey standard 6379 / TLS 6380 | Fixed; supported |
| `ipFamily` | `spec.service.ipFamilyPolicy`, `ipFamilies` | Supported |
| `ingress.*` | `spec.service.type=LoadBalancer` or Helm `extraObjects` | HTTP Ingress not directly adopted |
| `gatewayAPI.httpRoute.*` | L4 via Helm `extraObjects` (`TCPRoute` etc.) | HTTPRoute not directly adopted |
| `resources` | `spec.resources` | Supported |
| `persistence.enabled=false` | `spec.storage.ephemeral=true` | Supported (dev/test only) |
| `persistence.existingClaim` | `spec.storage.existingClaim` | Supported |
| `persistence.storageClass/size/accessModes` | `spec.storage.storageClassName/size/accessModes` | Supported |
| `persistence.annotations/labels` | `spec.storage.annotations/labels` | Supported |
| `livenessProbe/readinessProbe/startupProbe` | `spec.pod.livenessProbe/readinessProbe/startupProbe` | Supported |
| `nodeSelector/tolerations/affinity` | `spec.pod.nodeSelector/tolerations/affinity` | Supported |
| `hostAliases` | `spec.pod.hostAliases` | Supported |
| `priorityClassName` | `spec.pod.priorityClassName` | Supported |
| `podLabels/podAnnotations` | `spec.pod.labels/annotations` | Supported |
| `extraEnvVars` | `spec.pod.extraEnv` | Supported |
| `metrics.enabled` | `spec.monitoring.enabled` | Supported |
| `metrics.serviceMonitor.*` | `spec.monitoring.serviceMonitor.*` | Supported |
| `metrics.exporter.resources` | `spec.monitoring.exporter.resources` | Supported |
| `sentinel.*` | `mode=Replication` + AutoFailover | Not directly adopted. See `sentinel-migration.md`. |
| `initContainer.resources` | Operator does not require a bootstrap init container | Not directly adopted |
| `extraObjects` | chart `values.extraObjects` | Supported |

## Example: CloudPirates-style production CR

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cache-main
  namespace: data
spec:
  mode: Replication
  replicas: 3
  revisionHistoryLimit: 10
  version:
    imageRef: docker.io/valkey/valkey:9.0.4-alpine3.23
    imagePullPolicy: IfNotPresent
  auth:
    enabled: true
    passwordSecretRef:
      name: cache-main-password
      key: password
  service:
    type: LoadBalancer
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
    ipFamilyPolicy: PreferDualStack
    ipFamilies: [IPv4, IPv6]
  storage:
    storageClassName: gp3
    size: 20Gi
    accessModes: [ReadWriteOnce]
    annotations:
      backup.velero.io/backup-volumes: data
  pod:
    labels:
      workload-tier: cache
    imagePullSecrets:
      - name: private-registry
    startupProbe:
      exec:
        command: ["sh", "-c", "valkey-cli -h 127.0.0.1 -p 6379 ping | grep -q PONG"]
      periodSeconds: 5
      failureThreshold: 30
  additionalConfig:
    maxmemory: 1gb
    maxmemory-policy: allkeys-lru
    save: "900 1 300 10 60 10000"
  monitoring:
    enabled: true
    serviceMonitor:
      interval: 30s
```

## Transition path for Sentinel users

CloudPirates' `sentinel.enabled=true` assumes a Sentinel-aware
client that resolves the primary via
`SENTINEL get-master-addr-by-name`. This operator uses a Kubernetes
`Service` and controller leader election, so clients must switch
to the Service-aware model.

Operational procedure: see
[`sentinel-migration.md`](sentinel-migration.md).
