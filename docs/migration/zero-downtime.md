# Zero-Downtime Migration — plain StatefulSet → ValkeyCluster

> StatefulSet 기반 Valkey/Redis 배포를 ValkeyCluster CR 로 *서비스 중단 없이* 이전.

## 가정

- 기존 StatefulSet: `<name>-headless` Service, `replicas: N` (M primary + (N-M) replica)
- 클라이언트: 동일 namespace 의 Application Pod, `redis://<name>-headless:6379`
- 데이터: PVC 보존 필수, RDB snapshot 또는 AOF 활성

## 검증 순서 (각 단계 PASS 후 다음)

### 1. Pre-migration audit (5분)

```bash
kubectl -n <ns> get statefulset <name> -o yaml | head -50
kubectl -n <ns> exec <name>-0 -- valkey-cli INFO replication
kubectl -n <ns> get pvc -l app=<name>
```

PASS 기준: replication healthy + PVC 정상 + connections >0.

### 2. Backup baseline (10분)

```bash
kubectl -n <ns> apply -f - <<YAML
apiVersion: valkey.keiailab.com/v1alpha2
kind: ValkeyBackup
metadata:
  name: <name>-pre-migration
  namespace: <ns>
spec:
  source:
    statefulSet: <name>
  storage:
    pvc:
      claimName: <name>-backup
      size: 10Gi
YAML
kubectl -n <ns> wait --for=condition=Completed valkeybackup/<name>-pre-migration --timeout=600s
```

PASS 기준: ValkeyBackup phase=Completed + RDB file md5 인용.

### 3. ValkeyCluster shadow apply (5분)

같은 PVC 를 *추가 ReadWriteOnce mount* 없이, *새 ClusterIP Service* 로 ValkeyCluster CR apply:

```bash
kubectl -n <ns> apply -f - <<YAML
apiVersion: valkey.keiailab.com/v1alpha2
kind: ValkeyCluster
metadata:
  name: <name>
  namespace: <ns>
  annotations:
    valkey.keiailab.com/import-from-pvc: "<name>-data-0"
spec:
  shards: 1
  replicasPerShard: $(($N - 1))
  storage:
    storageClassName: <existing-sc>
    size: 10Gi
YAML
```

operator 가 import annotation 인지 → 새 Pod 가 기존 PVC 의 RDB 로 부팅.

PASS 기준: `kubectl -n <ns> get valkeycluster <name>` Ready=True.

### 4. Client cutover (DNS 갱신, 30초)

```bash
# 기존 Service 의 selector 를 새 ValkeyCluster Pod label 로 갱신
kubectl -n <ns> patch svc <name>-headless -p '{"spec":{"selector":{"app.kubernetes.io/managed-by":"valkey-operator"}}}'
# 또는 ExternalName 또는 client config 갱신
```

기존 Service 의 selector 만 갱신 — DNS 변경 없음, client 무영향.

PASS 기준: client side `valkey-cli ping` PONG + 연속 KEY/VALUE 정합.

### 5. Old StatefulSet teardown (5분)

```bash
kubectl -n <ns> scale statefulset <name> --replicas=0
sleep 60
# 연속 verify
for i in {1..10}; do
  kubectl -n <ns> exec deploy/test-client -- valkey-cli -h <name>-headless ping
done
kubectl -n <ns> delete statefulset <name>
```

PASS 기준: client 10/10 PONG + 0 connection drop.

## Rollback (4단계 후 client 이상 시)

`rollback.md` 참조.

## SLO

- 전체 5단계: **< 30분**
- 데이터 정합성: **100%** (RDB md5 일치)
- Client 다운타임: **0초** (DNS 무변경)

## Refs

- [ROADMAP.md](../../ROADMAP.md) — Migration runbook 진척
- [ADR-0015](../kb/adr/0015-valkeyrestore-init-container-pattern.md) (restore via init container)
- [ADR-0016](../kb/adr/0016-valkeybackuptarget-crd-external-storage.md) (ValkeyBackupTarget S3 abstraction)
