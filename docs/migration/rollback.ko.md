# 마이그레이션 Rollback (한국어)

> English: [rollback.md](rollback.md) — canonical / 정본

> ValkeyCluster 마이그레이션 4단계 (client cutover) 후 client 이상 발견 시 즉시 원복.

## 감지 신호

- `valkey-cli ping` 응답 시간 > 500ms
- client error rate > 1%
- `kubectl get valkeycluster Ready=False`
- PrometheusAlert `ValkeyClusterDegraded` fire

## Rollback 실행

### Step R1: Service selector 원복 (30초)

```bash
kubectl -n <ns> patch svc <name>-headless -p '{"spec":{"selector":{"app":"<original-statefulset-label>"}}}'
```

기존 StatefulSet 의 selector 로 재routing — Pod 이미 종료된 경우 R2 로.

### Step R2: 기존 StatefulSet 재기동 (5분)

```bash
kubectl -n <ns> scale statefulset <name> --replicas=$N
kubectl -n <ns> wait --for=condition=Ready pod -l app=<name> --timeout=300s
```

PVC 가 보존되어 있으므로 RDB 로 부팅 가능.

### Step R3: ValkeyCluster 차단 (10초)

```bash
kubectl -n <ns> scale --replicas=0 \
    -l valkey.keiailab.com/cluster=<name> statefulset
# 또는
kubectl -n <ns> delete valkeycluster <name>
```

### Step R4: 데이터 갭 복원 (마이그레이션 중 write 손실 시)

```bash
# 최신 ValkeyBackup 의 RDB diff → 기존 StatefulSet 으로 import
kubectl -n <ns> apply -f - <<YAML
apiVersion: valkey.keiailab.com/v1alpha2
kind: ValkeyRestore
metadata:
  name: <name>-rollback-restore
spec:
  backupRef:
    name: <name>-pre-migration
  target:
    statefulSet: <name>
YAML
```

## Rollback 검증 (Verify rollback)

- client error rate 정상화 (< 0.1%)
- `valkey-cli INFO` keyspace 정합
- `kubectl get valkeybackup` 의 latest RDB 와 데이터 비교

## SLO

- R1 + R2: **< 5분**
- 데이터 갭: **< 30초 (마이그레이션 단계 4 이후 write 만)**

## 참조 (Refs)

- ROADMAP.md (P-C.3.3)
- `zero-downtime.ko.md` (정방향 흐름)
