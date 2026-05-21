# 迁移回退

> English: [rollback.md](rollback.md) — canonical / 正本

> ValkeyCluster 迁移完成第 4 步 (客户端 cutover) 后,如发现客户端异常,立即原路回退。

## 检测信号

- `valkey-cli ping` 响应时间 > 500ms
- 客户端 error rate > 1%
- `kubectl get valkeycluster Ready=False`
- PrometheusAlert `ValkeyClusterDegraded` fire

## 回退执行

### Step R1: Service selector 回滚 (30 秒)

```bash
kubectl -n <ns> patch svc <name>-headless -p '{"spec":{"selector":{"app":"<original-statefulset-label>"}}}'
```

将流量重新路由到原 StatefulSet 的 selector — 如果 Pod 已被销毁,继续 R2。

### Step R2: 重启既有 StatefulSet (5 分钟)

```bash
kubectl -n <ns> scale statefulset <name> --replicas=$N
kubectl -n <ns> wait --for=condition=Ready pod -l app=<name> --timeout=300s
```

PVC 已保留,所以可以从 RDB 启动。

### Step R3: 阻断 ValkeyCluster (10 秒)

```bash
kubectl -n <ns> scale --replicas=0 \
    -l valkey.keiailab.com/cluster=<name> statefulset
# 或者
kubectl -n <ns> delete valkeycluster <name>
```

### Step R4: 数据缺口补齐 (迁移期间存在写入丢失时)

```bash
# 用最新 ValkeyBackup 的 RDB diff → 导回既有 StatefulSet
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

## 回退验证

- 客户端 error rate 回归正常 (< 0.1%)
- `valkey-cli INFO` keyspace 一致
- 与 `kubectl get valkeybackup` 中最新 RDB 数据比对

## SLO

- R1 + R2: **< 5 分钟**
- 数据缺口: **< 30 秒 (仅限第 4 步之后的写入)**

## Refs

- ROADMAP.md (P-C.3.3)
- `zero-downtime.zh.md` (正向流程)
