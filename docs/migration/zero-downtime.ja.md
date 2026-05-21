# Zero-Downtime Migration — plain StatefulSet → ValkeyCluster

> English: [zero-downtime.md](zero-downtime.md) — canonical / 正本

> StatefulSet ベースの Valkey/Redis デプロイメントを ValkeyCluster CR へ *無停止で* 移行する。

## 前提

- 既存 StatefulSet: `<name>-headless` Service、`replicas: N` (M primary + (N-M) replica)
- クライアント: 同一 namespace のアプリケーション Pod、`redis://<name>-headless:6379`
- データ: PVC の保持が必須、RDB snapshot または AOF が有効

## 検証手順 (各ステップを PASS してから次へ進む)

### 1. Pre-migration audit (5 分)

```bash
kubectl -n <ns> get statefulset <name> -o yaml | head -50
kubectl -n <ns> exec <name>-0 -- valkey-cli INFO replication
kubectl -n <ns> get pvc -l app=<name>
```

PASS 基準: replication が healthy + PVC 正常 + connections > 0。

### 2. Backup baseline (10 分)

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

PASS 基準: ValkeyBackup phase=Completed + RDB file の md5 を引用する。

### 3. ValkeyCluster shadow apply (5 分)

同一 PVC に対して *追加の ReadWriteOnce mount を作らず*、*新しい ClusterIP Service* で ValkeyCluster CR を apply する:

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

operator が import annotation を認識し、新しい Pod が既存 PVC の RDB から起動する。

PASS 基準: `kubectl -n <ns> get valkeycluster <name>` で Ready=True。

### 4. Client cutover (DNS 切り替え、30 秒)

```bash
# 既存 Service の selector を新 ValkeyCluster Pod の label に更新する
kubectl -n <ns> patch svc <name>-headless -p '{"spec":{"selector":{"app.kubernetes.io/managed-by":"valkey-operator"}}}'
# あるいは ExternalName を使う / client 側の config を更新する
```

既存 Service の selector だけを更新する — DNS は変更されないため client に影響しない。

PASS 基準: client 側で `valkey-cli ping` が PONG を返し、連続した KEY/VALUE の整合性が取れる。

### 5. Old StatefulSet teardown (5 分)

```bash
kubectl -n <ns> scale statefulset <name> --replicas=0
sleep 60
# 連続して verify する
for i in {1..10}; do
  kubectl -n <ns> exec deploy/test-client -- valkey-cli -h <name>-headless ping
done
kubectl -n <ns> delete statefulset <name>
```

PASS 基準: client が 10/10 で PONG を返し、connection drop が 0 件。

## Rollback (ステップ 4 以降に client 異常を検知した場合)

`rollback.md` を参照。

## SLO

- 全体 5 ステップ: **30 分未満**
- データ整合性: **100%** (RDB の md5 が一致)
- Client ダウンタイム: **0 秒** (DNS 変更なし)

## Refs

- [ROADMAP.md](../ROADMAP.md) — Migration runbook の進捗
- [ADR-0015](../kb/adr/0015-valkeyrestore-init-container-pattern.md) (restore via init container)
- [ADR-0016](../kb/adr/0016-valkeybackuptarget-crd-external-storage.md) (ValkeyBackupTarget S3 abstraction)
