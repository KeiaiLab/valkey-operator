# 零停机迁移 — 从普通 StatefulSet 迁移到 ValkeyCluster

> English: [zero-downtime.md](zero-downtime.md) — canonical / 正本

> 将基于 StatefulSet 的 Valkey/Redis 部署 *无服务中断* 迁移到 ValkeyCluster CR。

## 前提假设

- 既有 StatefulSet: `<name>-headless` Service,`replicas: N` (M 个 primary + (N-M) 个 replica)
- 客户端: 同一 namespace 下的应用 Pod,使用 `redis://<name>-headless:6379`
- 数据: PVC 必须保留,且已启用 RDB snapshot 或 AOF

## 验证顺序 (每一步 PASS 后再进入下一步)

### 1. 迁移前审计 (5 分钟)

```bash
kubectl -n <ns> get statefulset <name> -o yaml | head -50
kubectl -n <ns> exec <name>-0 -- valkey-cli INFO replication
kubectl -n <ns> get pvc -l app=<name>
```

PASS 标准: 复制集健康 + PVC 正常 + connections >0。

### 2. 备份基线 (10 分钟)

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

PASS 标准: ValkeyBackup phase=Completed + 引用 RDB 文件的 md5。

### 3. ValkeyCluster 影子部署 (5 分钟)

复用同一份 PVC,*不增挂 ReadWriteOnce*,并以 *新的 ClusterIP Service* 部署 ValkeyCluster CR:

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

operator 识别到 import annotation → 新 Pod 从既有 PVC 中的 RDB 启动。

PASS 标准: `kubectl -n <ns> get valkeycluster <name>` Ready=True。

### 4. 客户端 cutover (DNS 刷新,30 秒)

```bash
# 将既有 Service 的 selector 切换到新 ValkeyCluster 的 Pod label 上
kubectl -n <ns> patch svc <name>-headless -p '{"spec":{"selector":{"app.kubernetes.io/managed-by":"valkey-operator"}}}'
# 或者使用 ExternalName,或者直接更新客户端配置
```

仅切换既有 Service 的 selector — DNS 不变,客户端无感知。

PASS 标准: 客户端侧 `valkey-cli ping` 返回 PONG + 连续 KEY/VALUE 一致。

### 5. 旧 StatefulSet 下线 (5 分钟)

```bash
kubectl -n <ns> scale statefulset <name> --replicas=0
sleep 60
# 持续验证
for i in {1..10}; do
  kubectl -n <ns> exec deploy/test-client -- valkey-cli -h <name>-headless ping
done
kubectl -n <ns> delete statefulset <name>
```

PASS 标准: 客户端 10/10 PONG + 0 次 connection drop。

## 回退 (第 4 步之后发现客户端异常时)

参见 `rollback.zh.md`。

## SLO

- 五步合计: **< 30 分钟**
- 数据一致性: **100%** (RDB md5 一致)
- 客户端停机时间: **0 秒** (DNS 不变)

## Refs

- [ROADMAP.md](../ROADMAP.md) — Migration runbook 进度
- [ADR-0015](../kb/adr/0015-valkeyrestore-init-container-pattern.md) (restore via init container)
- [ADR-0016](../kb/adr/0016-valkeybackuptarget-crd-external-storage.md) (ValkeyBackupTarget S3 abstraction)
