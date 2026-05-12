# CloudPirates valkey 0.20.2 호환 매핑 (한국어)

> English: [cloudpirates-valkey-compatibility.md](cloudpirates-valkey-compatibility.md) — canonical / 정본


대상: ArtifactHub `cloudpirates-valkey/valkey` chart `0.20.2`
(appVersion `9.0.0`). 본 문서는 chart values 를 valkey-operator CRD 로 옮길 때
운영자가 확인할 단일 매핑표다.

## 매핑 원칙

- Helm chart 의 values 이름을 그대로 복제하지 않고, Kubernetes operator 가
  안정적으로 소유할 수 있는 CRD 계약으로 제공한다.
- Sentinel / HTTPRoute 처럼 dual control-plane 또는 TCP/L7 mismatch 를 만드는
  항목은 직접 구현하지 않고, 운영 안정성이 높은 대체 경로를 제공한다.
- literal password 값은 CRD 에 직접 넣지 않는다. Secret 을 먼저 만들고
  `SecretKeySelector` 로 참조한다.

## 기능 매핑

| CloudPirates value | valkey-operator 경로 | 상태 |
|---|---|---|
| `architecture=standalone` | `Valkey.spec.mode=Standalone` | 지원 |
| `architecture=replication`, `replicaCount` | `Valkey.spec.mode=Replication`, `spec.replicas` | 지원 |
| cluster sharding | `ValkeyCluster.spec.shards`, `replicasPerShard` | chart 이상 지원 |
| `revisionHistoryLimit` | `spec.revisionHistoryLimit` | 지원 |
| `image.registry/repository/tag`, digest tag | `spec.version.imageRef` 또는 `image` + `version` | 지원 |
| `image.pullPolicy` | `spec.version.imagePullPolicy` | 지원 |
| `imagePullSecrets` | `spec.pod.imagePullSecrets` | 지원 |
| `auth.enabled` | `spec.auth.enabled` | 지원 |
| `auth.existingSecret`, key | `spec.auth.passwordSecretRef` | 지원 |
| `auth.password` literal | Secret 생성 후 `passwordSecretRef` | 직접 literal 비지원 |
| `tls.enabled` | `spec.tls.enabled` | 지원 |
| `tls.existingSecret` | `spec.tls.customCert.secretName` | 지원. key 는 `tls.crt`, `tls.key`, `ca.crt` 로 정규화 |
| `tls.authClients` | `spec.tls.clientAuth=required|disabled` | 지원 |
| `config.maxMemory`, `maxMemoryPolicy`, `extraConfig` | `spec.additionalConfig` | 지원 |
| `config.save` | `spec.persistence.rdbSaveSchedule` 또는 `additionalConfig.save` | 지원 |
| `config.existingConfigmap` | operator 생성 config + `additionalConfig` 병합 | 직접 교체 비지원 |
| `externalReplica.*` | `spec.externalReplica.*` | 지원. v1alpha1 은 Standalone 한정 |
| `service.type`, annotations/labels | `spec.service.type`, annotations/labels | 지원 |
| `service.port`, `targetPort` | Valkey 표준 6379 / TLS 6380 | 고정 지원 |
| `ipFamily` | `spec.service.ipFamilyPolicy`, `ipFamilies` | 지원 |
| `ingress.*` | `spec.service.type=LoadBalancer` 또는 Helm `extraObjects` | HTTP Ingress 직접 비채택 |
| `gatewayAPI.httpRoute.*` | Helm `extraObjects` 로 `TCPRoute` 등 L4 resource 관리 | HTTPRoute 직접 비채택 |
| `resources` | `spec.resources` | 지원 |
| `persistence.enabled=false` | `spec.storage.ephemeral=true` | 지원. dev/test 전용 |
| `persistence.existingClaim` | `spec.storage.existingClaim` | 지원 |
| `persistence.storageClass/size/accessModes` | `spec.storage.storageClassName/size/accessModes` | 지원 |
| `persistence.annotations/labels` | `spec.storage.annotations/labels` | 지원 |
| `livenessProbe/readinessProbe/startupProbe` | `spec.pod.livenessProbe/readinessProbe/startupProbe` | 지원 |
| `nodeSelector/tolerations/affinity` | `spec.pod.nodeSelector/tolerations/affinity` | 지원 |
| `hostAliases` | `spec.pod.hostAliases` | 지원 |
| `priorityClassName` | `spec.pod.priorityClassName` | 지원 |
| `podLabels/podAnnotations` | `spec.pod.labels/annotations` | 지원 |
| `extraEnvVars` | `spec.pod.extraEnv` | 지원 |
| `metrics.enabled` | `spec.monitoring.enabled` | 지원 |
| `metrics.serviceMonitor.*` | `spec.monitoring.serviceMonitor.*` | 지원 |
| `metrics.exporter.resources` | `spec.monitoring.exporter.resources` | 지원 |
| `sentinel.*` | `mode=Replication` + AutoFailover | 직접 비채택. `sentinel-migration.md` 참조 |
| `initContainer.resources` | operator bootstrap init container 불필요 | 직접 비채택 |
| `extraObjects` | chart `values.extraObjects` | 지원 |

## 예시: CloudPirates 스타일 운영 CR

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

## Sentinel 사용자의 전환 경로

CloudPirates `sentinel.enabled=true` 는 Sentinel-aware client 가
`SENTINEL get-master-addr-by-name` 으로 primary 를 찾는 모델이다. 본 operator 는
Kubernetes Service 와 controller leader election 을 사용하므로 client 를
Service-aware 방식으로 전환한다.

운영 절차는 `docs/operations/sentinel-migration.md` 를 따른다.
