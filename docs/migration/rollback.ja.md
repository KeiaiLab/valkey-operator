# Migration Rollback

> English: [rollback.md](rollback.md) — canonical / 正本

> ValkeyCluster 移行のステップ 4 (client cutover) 以降に client 異常を検知した場合、即時に切り戻す手順。

## 検知シグナル

- `valkey-cli ping` の応答時間が 500ms を超える
- client の error rate が 1% を超える
- `kubectl get valkeycluster` の Ready=False
- PrometheusAlert `ValkeyClusterDegraded` が fire する

## Rollback の実行

### Step R1: Service selector を切り戻す (30 秒)

```bash
kubectl -n <ns> patch svc <name>-headless -p '{"spec":{"selector":{"app":"<original-statefulset-label>"}}}'
```

既存 StatefulSet の selector に再ルーティングする — Pod が既に終了している場合は R2 へ。

### Step R2: 既存 StatefulSet を再起動する (5 分)

```bash
kubectl -n <ns> scale statefulset <name> --replicas=$N
kubectl -n <ns> wait --for=condition=Ready pod -l app=<name> --timeout=300s
```

PVC が保持されているため RDB からの起動が可能。

### Step R3: ValkeyCluster を遮断する (10 秒)

```bash
kubectl -n <ns> scale --replicas=0 \
    -l valkey.keiailab.com/cluster=<name> statefulset
# もしくは
kubectl -n <ns> delete valkeycluster <name>
```

### Step R4: データギャップを復元する (移行中に write が損失した場合)

```bash
# 最新の ValkeyBackup の RDB を diff → 既存 StatefulSet へ import
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

## Verify rollback

- client の error rate が正常化する (< 0.1%)
- `valkey-cli INFO` の keyspace が整合
- `kubectl get valkeybackup` の最新 RDB とデータを比較する

## SLO

- R1 + R2: **5 分未満**
- データギャップ: **30 秒未満 (移行ステップ 4 以降の write のみ)**

## Refs

- ROADMAP.md (P-C.3.3)
- `zero-downtime.md` (forward の流れ)
