# 故障排查 — valkey-operator (简体中文)

> English: [troubleshooting.md](troubleshooting.md) — canonical / 正本


运维过程中 *症状 → 可能根因 → 诊断命令 → 处置* 的流程图式速查表。各告警的
MTTR 流程以 `runbook.md §9` 为 SSOT — 本文档专注于 **不会触发任何告警 (或在告警
出现之前) 的盲区症状**。

故障发生时按 *症状分类 → 跳转到本文档的 §X.Y 小节 → 诊断 → 处置* 的顺序使用。

## 1. CR 始终无法进入任何 phase

### 1.1 `kubectl apply` 直接被 webhook 拒绝

**症状**: `apply` 命令被 admission webhook 报错拦截。

```sh
kubectl apply -f my-valkey.yaml
# Error: admission webhook "vvalkey-v1alpha1.kb.io" denied the request: ...
```

**可能根因 + 诊断**:

| 根因 | 验证命令 |
|---|---|
| webhook configuration 未安装 / cert 未就绪 | `kubectl get validatingwebhookconfiguration -l app.kubernetes.io/name=valkey-operator` + `kubectl describe ...` |
| `Spec.TLS.CertManager` 与 `CustomCert` 同时设置 | `kubectl explain valkey.spec.tls` (按 ADR-0010 webhook 校验互斥) |
| Valkey 版本不在白名单中 | 查看 `internal/version/matrix.go::SupportedValkeyVersions` (当前 `8.0.9` / `8.1.6` / `8.1.7` / `9.0.4`) |
| ResourceQuota 超限 | `kubectl describe resourcequota -n <ns>` |

**处置**: 修复上表对应行的根因后重新 apply。若 webhook 自身行为异常,按
`runbook.md §6.1` 强制重启 manager。

### 1.2 CR 已创建但 phase 始终停在 `Pending`

**症状**: `kubectl get vk` 显示 `PHASE: Pending` 超过 5 分钟。

**诊断**:

```sh
kubectl describe vk <name>      # Events + Conditions
kubectl logs -n valkey-operator-system -l control-plane=controller-manager \
  --since=10m | grep "<ns>/<name>"
```

**常见根因**:

- AuthSecret 引用的 `Secret` 在其他 namespace → `Reason: SecretNotFound`。
  operator 是 namespace 内作用域,不支持跨 namespace 的 Secret 引用。
- StorageClass 不被支持,或
  `volumeBindingMode: WaitForFirstConsumer` + 没有可用节点 →
  PVC 一直 Pending → STS Pod 一直 Pending → reconcile 无任何进展。
- `ENABLE_CLUSTER_RECONCILER=false` 但创建了 `kind: ValkeyCluster` (chart
  中 `features.cluster.enabled=false` 的环境)。Helm 安装路径有守卫,但
  手写的 CR 会静默卡住。

## 2. 数据面无响应 (尽管 `cluster_state=ok`)

### 2.1 客户端连接超时

**症状**: 集群报告 healthy,但应用看到 connection refused / timeout。

**诊断顺序**:

1. **Service endpoints 检查**:
   ```sh
   kubectl get svc,endpointslices -l cache.keiailab.io/cluster=<name>
   ```
   `Endpoints: 0` 表示 label selector 不匹配 (operator 修改了 STS template
   但漏改了某个 patched-only 的 label)。强制触发一次 reconcile:
   `kubectl annotate vk <name> cache.keiailab.io/retry=true --overwrite`。

2. **NetworkPolicy 拒绝** (启用了 ADR-0035 AutoCreate 时):
   ```sh
   kubectl get networkpolicy -l cache.keiailab.io/cluster=<name>
   kubectl describe networkpolicy <name>-allow
   ```
   若 ingress 的 `podSelector` 与客户端 pod 的 label 不匹配,流量会被默认
   拒绝。

3. **TLS 握手失败** (启用 TLS 的集群):
   ```sh
   kubectl exec -it <client-pod> -- valkey-cli -h <svc> -p 6379 --tls --insecure ping
   ```
   若失败,按 `runbook.md §2.2` 处理 Pod CrashLoopBackOff / TLS Secret
   未挂载的链路。

### 2.2 部分 key 超时 (Cluster 模式)

**症状**: 同一集群内,key A 成功,但 key B 返回 MOVED 或超时。

**根因**: 某个分片的 primary 不可达,且其 replica 尚未自动晋升。

```sh
# 1. key 落在哪个 slot 上?
kubectl exec -it <pod> -- valkey-cli -c -h <svc> CLUSTER KEYSLOT <key>

# 2. 该 slot 由哪个分片持有?
kubectl exec -it <pod> -- valkey-cli -c -h <svc> CLUSTER SLOTS | grep <slot>

# 3. 查看该分片 pod 的日志
kubectl logs <shard-N-0>
```

**处置**: 当持有这些 key 的 primary 不可达时,ADR-0017 会自动晋升 offset
最大的 replica。若 5 分钟后仍未恢复,按 `runbook.md §2.3` 的
`cluster_state=fail` 恢复流程处理。

## 3. 性能下降 (延迟上升)

### 3.1 reconcile 抖动 (thrashing)

**症状**: operator pod CPU 占用稳定在 100%,且
`valkey_cluster_reconcile_total` 速率激增。

**诊断**:

```sh
# 速率 (在 Prometheus 中)
rate(valkey_cluster_reconcile_total[1m])
# > 5/s 即为抖动

# 错误发生在哪个 component?
sum by (component) (rate(valkey_cluster_reconcile_errors_total[5m]))
```

**常见根因**:

- 状态更新死循环: operator 检测到 spec drift → 打 patch → 触发 reconcile →
  再次检测到同一 drift。若 `metadata.generation` 没有变化但 reconcile
  反复进入,即可怀疑此模式。
- 外部依赖抖动: cert-manager 的 `Certificate` 在 `Ready` 与 `NotReady`
  之间来回切换,会导致 operator 每轮都重新运行。

**处置**: `kubectl logs ... --follow` 追踪 reconcile 进入的原因。修复 drift
根因 — 通常是 admission webhook 的 mutating defaulter 与 reconciler 的
desired state 之间出现了不一致。

### 3.2 数据面延迟上升 (慢日志)

**症状**: 客户端 p95 延迟从约 1 ms 攀升到约 100 ms。

**可能根因** (虽超出 operator 作用域,但仍可诊断):

| 根因 | 诊断 |
|---|---|
| AOF rewrite 进行中 | `valkey-cli INFO persistence \| grep aof_rewrite_in_progress` |
| RDB snapshot 进行中 | `valkey-cli INFO persistence \| grep rdb_bgsave_in_progress` |
| 存在大 key (BIGKEY) | `valkey-cli --bigkeys` — 单个 100 MB+ 的 key 会暴露慢的 O(N) 命令 |
| 内存换页 (swap) | `kubectl top pod <pod>` 加上节点的 `vmstat 1` |

调优属于 **数据面** 决策,operator 不会直接介入。请按 `runbook.md §6.3`
直接通过 `valkey-cli` 进入排查。

## 4. 备份 / 恢复失败

### 4.1 `ValkeyBackup phase=Failed`

```sh
kubectl describe vkb <name>     # Conditions 中的 Reason
kubectl logs job/<backup-job-name>
```

**常见根因**:

- `TargetRef` 中的 Secret 凭据无效 → S3 客户端返回
  `403 SignatureDoesNotMatch`。请通过
  `kubectl describe vbt <target>` 检查 `status.reachable`。
- bucket 不存在 → `404 NoSuchBucket`。请核对
  `ValkeyBackupTarget` 的 `spec.s3.bucket`。
- PVC 空间耗尽 → `no space left on device`。可用 `INFO memory` 中的
  `used_memory_rss` 估算 RDB 大小。

### 4.2 `ValkeyRestore` 一直挂起

ADR-B02 在 RDB 格式不匹配 (例如 Redis 8.2.1 → Valkey 9.0.4) 时会
fail-fast 并把 `Status.Phase` 置为 `Failed`。其他挂起模式:

- pod CrashLoopBackOff 累计 ≥ 5 次 = restore 容器持续失败。请在日志中
  寻找诸如 `Can't handle RDB format version` 的明确原因。
- `TargetRef` RDB 下载超时 (大对象 + 慢网络) — 请扩大 Job 的 timeout
  配置。

## 5. 通用诊断速查表

```sh
# 一眼查看所有 CR 的 phase
kubectl get vk,vc,vkb,vbt,vkr -A \
  -o custom-columns=NS:.metadata.namespace,KIND:.kind,NAME:.metadata.name,PHASE:.status.phase

# 最近 5 分钟内 reconcile 的原因 (operator 日志)
kubectl -n valkey-operator-system logs -l control-plane=controller-manager --since=5m \
  | jq -R 'fromjson? | select(.msg)' | head -50

# Conditions 差异 (前后 status 对比)
kubectl get vk <name> -o jsonpath='{.status.conditions}' | jq

# 最近的事件
kubectl get events --field-selector involvedObject.name=<name> --sort-by='.lastTimestamp'
```

## 6. 终极处置手段

- `runbook.md §6` 中的应急处置流程 (强制重启 manager、中止 restore、
  直接进入数据面)。
- `INC-0001` 类型的 cluster-fail 场景 → ADR-0039 self-heal **大多数情况**
  可自动恢复;若未自动恢复,请按 `runbook.md §2.3` 进行人工恢复。

## 7. 当问题始终无法收敛

若同一问题反复出现 3 次以上,请按全局 `incident-kb.md §2 trigger` 的规定,
新建 `docs/kb/incident/INC-NNNN-*.md`。若需要系统层面的变更,请按正式路径
推进: RFC → ADR → 代码变更。
