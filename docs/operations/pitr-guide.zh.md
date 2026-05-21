# Point-In-Time Recovery (PITR) — operator 指南 (简体中文)

> English: [pitr-guide.md](pitr-guide.md) — canonical / 正本

PITR 是 ADR-0040 parity 清单上**最大的单项缺口**。本文涵盖 **phase 1**
(API + webhook) 的使用指南,以及进入 **phase 2** (reconciler dispatch)
的入口路径。

## 当前状态 (截至 2026-05-10)

| 领域 | 状态 |
|---|---|
| AOF 备份 (生成) | ✅ GA (BgRewriteAOF,ADR-0016 + minio-go / GCS / Azure) |
| RDB 备份 (生成) | ✅ GA |
| `ValkeyRestore.Spec.PointInTime` API | ✅ GA (#54) |
| Webhook 校验 (Source 3-type + PointInTime+RDB reject) | ✅ GA (#54) |
| **AOF replay-to-timestamp reconciler dispatch** | ❌ phase 2 |
| 手工 PITR (operator 外部工具) | ✅ 可用 |

## Phase 1 用法 — 完整 AOF replay (`PointInTime` 为 nil)

最常见的场景: 将备份内的全部数据复原。行为与 #54 之前一致:

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata:
  name: vk-restore-full
  namespace: valkey
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef:
      name: s3-prod
      path: vk-prod/2026-05-10T00:00:00Z/dump.aof
  restoreType: AOF   # 当备份本身就是 AOF 时
```

reconciler 下载 AOF → init container 将其落到 Valkey 数据目录 →
STS 重启 → Valkey 在启动时 replay 整个 AOF。

## Phase 1 用法 — PITR API (PointInTime 已填,dispatch 尚未实现)

webhook 接受该 spec,且 `status` 会被保留。reconciler 当前与
"完整 AOF replay" 行为一致 (PointInTime 被忽略) — phase 2 之前的
**fail-safe** 行为:

```yaml
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef:
      name: s3-prod
      path: vk-prod/2026-05-10T00:00:00Z/dump.aof
  restoreType: AOF
  pointInTime: "2026-05-10T14:30:00Z"   # 目标恢复时刻
```

**当前行为**: webhook 校验不变式 (拒绝 RDB + PointInTime 的组合)。
reconciler 忽略 `PointInTime`,replay 整个 AOF — 若需要更早的截止
时刻,请提供一份**更短的** AOF。**phase 2 (单独 epic)** 会加入按
该时刻精确截断的 dispatch。

## 手工 PITR (phase 2 落地前的临时方案)

在 phase 2 上线之前的运维流程 — 使用 operator 外部的工具:

1. **下载 AOF**:
   ```sh
   aws s3 cp s3://vk-prod-backups/2026-05-10T00:00:00Z/dump.aof ./dump.aof
   ```

2. **截断 AOF**,只保留目标时刻之前的条目 (直接修改 Valkey AOF 格式):
   ```sh
   # 从 AOF 条目中提取 timestamp (仅适用于 TIMESTAMP-aware AOF
   # —— Valkey 8.0+ 配合 `set aof-timestamp-enabled yes`)。
   valkey-aof-trim --until "2026-05-10T14:30:00Z" dump.aof > dump-truncated.aof
   ```
   **提示**: `valkey-aof-trim` 是外部 / 用户自行编写的工具。Valkey
   官方实用工具计划在 9.x 版本提供。

3. **上传截断后的 AOF**:
   ```sh
   aws s3 cp dump-truncated.aof s3://vk-prod-backups/pitr-2026-05-10T14:30:00Z/dump.aof
   ```

4. **使用截断后的 AOF 恢复**:
   ```yaml
   spec:
     source:
       targetRef:
         name: s3-prod
         path: pitr-2026-05-10T14:30:00Z/dump.aof
     restoreType: AOF
   ```

phase 2 会让上述 1–3 步在 operator 内部**自动完成**。

## Phase 2 入口条件 (单独 epic 候选)

要在本指南中启用 dispatch:

1. ~~**AOF-timestamp 解析库**~~ → ✅ **#68** `internal/aoftime` 包 GA。
2. ~~**面向 reconciler 集成的文件级 helper**~~ → ✅ **#69**
   `TruncateAOFFile` GA。
3. ~~**Reconciler dispatch — download Job 中的 cli 在 in-place 完成截断**~~
   → ✅ **#70** (`DownloadJobParams.PITRCutoff` +
   `cli download --pitr-cutoff`;当设置 `PointInTime` 且
   `RestoreType=AOF` 时,reconciler 会自动 dispatch)。
4. **`valkey-cli --pipe` 集成** — 当前由 init container 在启动时加载
   AOF (Valkey 默认 `appendonly yes`)。仅在需要 **streaming replay**
   的场景下才需要单独集成 `valkey-cli --pipe`;当前的 init-container
   路径已经够用。
5. **`PointInTime ≤ backup CompletedAt` 的 webhook 不变式** (后续) —
   备份完成之后的 `PointInTime` 是语义矛盾 (相当于请求尚不存在的数据)。
6. **Rollback** (后续) — replay 失败时回退到备份时刻。

**当前状态 (#70 之后)**: 设置 `restoreType: AOF` + `PointInTime` 时,
reconciler 会自动下载 → 截断 → init container 从截断后的 AOF 启动。
**完全自动的 PITR** 已经可用。剩余的工作是 webhook 不变式与
rollback (运维安全性)。

## PITR 失败时的恢复 (#72 rollback)

PITR replay 失败 (AOF 损坏、timestamp marker 异常等) 会让 init
container 陷入 CrashLoopBackOff。手工 rollback 流程如下:

### 前置条件

reconciler 会以 `--pitr-backup=/backup/dump.aof.original` 调用
download Job (#72)。该备份文件必须存在于 staging PVC 上。

### 自动 rollback (运维一键)

```sh
# 1. 起一个临时 helper pod,挂载到 staging PVC。
kubectl run rollback-helper --rm -it --restart=Never \
  --image=ghcr.io/keiailab/valkey-operator:latest \
  --overrides='{"spec":{"containers":[{"name":"r","image":"ghcr.io/keiailab/valkey-operator:latest","command":["sh","-c","cp /backup/dump.aof.original /backup/dump.aof"],"volumeMounts":[{"name":"b","mountPath":"/backup"}]}],"volumes":[{"name":"b","persistentVolumeClaim":{"claimName":"<staging-pvc>"}}]}}'

# 2. 重启 Valkey STS (init container 会执行一次完整的
#    AOF replay)。
kubectl rollout restart sts/<cluster-name>
```

### 自动化 (operator 侧,单独 epic)

后续工作 — operator 将:

1. 检测 `Status.Phase=Restoring` + init-container CrashLoopBackOff。
2. 验证备份文件存在。
3. 转入 `Status.Phase=PITRRollbackPending` (经过用户显式批准后自动)。
4. 由 reconciler 自动执行上面的一键脚本。

该自动化属于**破坏性操作** (会覆写 PVC 数据),所以需要先经过 ADR
立项,并要求用户显式批准。

## #70 使用示例 (实际行为)

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata: { name: pitr-restore }
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef: { name: s3-prod, path: backup/dump.aof }
  restoreType: AOF
  pointInTime: "2026-05-10T14:30:00Z"
```

内部流程:

1. `handlePending`: webhook (#54) 校验不变式 → Mounting。
2. `handleMounting`: 创建 download Job,带上
   `--pitr-cutoff=2026-05-10T14:30:00Z`。
3. `cli download` (#70): S3 → `/backup/dump.aof` → in-place 截断
   到 cutoff。
4. `handleRestoring`: 走既有的 init-container 路径 → 集群从截断后
   的 AOF 启动。
5. Verifying → Completed。

## #68 使用示例 (Go 集成)

```go
import "github.com/keiailab/valkey-operator/internal/aoftime"

aofBytes, _ := os.ReadFile("dump.aof")
if !aoftime.HasTimestamps(aofBytes) {
    // PITR 不可行 — 仅支持完整 replay。
    return errors.New("AOF lacks timestamps (set aof-timestamp-enabled yes for PITR)")
}
cutoff := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)
offset := aoftime.TruncateOffset(aofBytes, cutoff)
truncated := aofBytes[:offset]
// 将 `truncated` 流式喂给 `valkey-cli --pipe` → 只恢复 cutoff
// 之前的条目。
```

## 关联

- runbook §3.3 — Restore (灾难恢复)。
- ADR-0015 — `ValkeyRestore` init-container 模式。
- ADR-0016 — `ValkeyBackupTarget` 外部存储。
- #54 — `PointInTime` API + webhook。

## 参考

- Valkey AOF 规范: <https://valkey.io/topics/persistence/>
- AOF timestamp-enabled (8.0+): `aof-timestamp-enabled` 指令。
- 外部工具: `redis-cli --pipe` (Valkey 兼容)。
