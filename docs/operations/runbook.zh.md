# 运维 Runbook — valkey-operator (简体中文)

> English: [runbook.md](runbook.md) — canonical / 正本

故障响应与日常运维流程。仅收录核心场景 — 细节请参阅对应的 ADR / Issue。

## 1. 健康检查

```sh
# operator pod 状态
kubectl -n valkey-operator-system get pods -l control-plane=controller-manager

# 全部 CR 的 phase
kubectl get vk,vc,vkb,vbt,vkr -A

# operator 指标 (HTTPS:8443)
kubectl -n valkey-operator-system port-forward \
  svc/valkey-operator-controller-manager-metrics-service 8443:8443
curl -k https://localhost:8443/metrics | grep valkey_cluster_state_ok
```

## 2. 通用故障响应

### 2.1 CR 出现 `Phase=Failed`

```sh
kubectl describe vk <name>     # Status.Conditions 中的 Reason / Message
kubectl get events --field-selector involvedObject.name=<name>
```

处理步骤:
1. 对 `Reason` 进行分类 (TargetNotFound / AuthSecret / ConfigMap /
   TLS / ...)。
2. 排除根因后,重新创建 CR 或通过
   `kubectl annotate ... cache.keiailab.io/retry=true` 触发再次 reconcile。

### 2.2 Pod CrashLoopBackOff

```sh
kubectl logs <pod> -p          # 上一个容器的日志
kubectl logs <pod> -c valkey
```

常见根因:

- TLS Secret 未挂载 → 参考 ADR-0014
  (`/tls/tls.crt: No such file`)。
- Auth password 不匹配 → 重新生成 Auth Secret (删除并重建 CR)。

### 2.3 `ValkeyCluster cluster_state=fail`

**自愈机制 (ADR-0039, 2026-05-10)**: operator 在
`ClusterInitialized=true` 状态下若检测到 `cluster_state != ok`
(或 `slots != 16384`),会自动重新调用 `ensureClusterMeet`。本节
针对自愈本身失败 (持续卡住 5 分钟以上) 时的人工介入流程。

```sh
PASS=$(kubectl get secret <name>-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster info
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster nodes
```

#### 诊断顺序

1. **检查 Pods 是否 Ready**:
   `kubectl get pod -n <ns> -l app.kubernetes.io/instance=<name>`。
   如果存在 pod 不是 `Running 2/2`,请优先修复 (PVC pending、
   NetworkPolicy 拒绝等)。
2. **operator 日志**:
   `kubectl logs -n <op-ns> deploy/<op-name>`,查找 *INC-0001
   self-heal* 尝试行:
   ```
   ValkeyCluster post-init fail detected; attempting re-bootstrap (INC-0001 self-heal)
     state=fail slotsAssigned=0 slotsOK=0
   ```
   该日志若反复出现 30 分钟以上,即代表自愈已失败 — 进入第 3 步。
3. **`nodes.conf` 的 `myself` IP**: 对每个 pod 的
   `/data/nodes.conf`,确认 `myself` 行的 IP 与实际 pod IP
   (`kubectl get pod -o wide`) 是否一致。不一致即 INC-0001 复现
    — 进入第 4 步。

#### 人工恢复 (INC-0001 模式)

恢复前先做数据损失评估:

```sh
# 各 pod 的 key 数量
for i in 0 1 2 3 4 5; do
  echo "pod-$i: $(kubectl exec <name>-$i -- valkey-cli -a "$PASS" dbsize | tail -1)"
done

# 采样部分 key (判断是否为生产数据)
kubectl exec <name>-0 -- valkey-cli -a "$PASS" --scan | head -20
```

**若包含生产数据**: 必须 *先* 备份再恢复 (`make backup`,或
`valkey-cli BGSAVE`)。优先选用 `ValkeyBackup` CR。

**若仅为测试数据 (或可接受丢失)**:

```sh
# 1. 清空全部 6 个 pod 的 PVC (AOF + nodes.conf)
for i in 0 1 2 3 4 5; do
  kubectl exec <name>-$i -- sh -c 'rm -rf /data/appendonlydir /data/nodes.conf /data/dump.rdb'
done

# 2. 同时重启全部 pod (controller 会重建 STS)
kubectl delete pod <name>-0 <name>-1 <name>-2 <name>-3 <name>-4 <name>-5 --wait=false

# 3. 强制将 ClusterInitialized 置为 false (再次进入 controller bootstrap 路径)
kubectl patch valkeycluster <name> --type=json --subresource=status \
  -p='[{"op":"replace","path":"/status/clusterInitialized","value":false},
       {"op":"replace","path":"/status/shards","value":[]},
       {"op":"replace","path":"/status/clusterState","value":""}]'

# 4. 修改 spec 以触发 reconcile 事件
kubectl patch valkeycluster <name> --type=merge \
  -p '{"spec":{"nodeTimeoutMillis":15001}}'

# 5. 等待 60s 后验证
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster info
# 期望: cluster_state:ok, cluster_slots_assigned:16384, cluster_slots_ok:16384
```

#### 参考

- INC-0001:
  `docs/kb/incident/INC-0001-cluster-fail-bootstrap-skip.md`
  (2026-05-09,历时 19 小时的故障)。
- ADR-0039: `docs/kb/adr/0039-cluster-self-heal-post-init.md`
  (永久修复方案)。
- 告警: PrometheusRule `ValkeyClusterStateNotOK`
  (`for: 5m`,severity critical) — 本节的入口点。

## 3. 备份 / 恢复

### 3.1 日常备份 (保留 PVC)

```sh
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata: { name: vkb-$(date +%Y%m%d), namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  type: RDB
  retainPVC: true
  ttl: 168h        # 7 天
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeybackup/vkb-...
```

### 3.2 外部存储备份 (S3)

```sh
# 前置: 先创建 ValkeyBackupTarget 与凭据 Secret
# (参考 config/samples/)。
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata: { name: vkb-s3-$(date +%Y%m%d), namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  destination:
    type: TargetRef
    targetRef:
      name: s3-prod
      path: $(date +%Y/%m/%d)/dump.rdb
  ttl: 720h        # 30 天
EOF
```

### 3.3 恢复 (灾难恢复)

**警告**: `ValkeyRestore` 会 **覆盖** 目标集群的现有数据。请先单独
做一份备份。

```sh
# Standalone Valkey,以 PVC 为源。
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata: { name: vkr-recovery, namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  source:
    pvc:
      name: vkb-20260506-backup
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeyrestore/vkr-recovery --timeout=10m

# 验证
kubectl get vkr vkr-recovery -o jsonpath='{.status.restoredKeys}'
```

通过
`kubectl get vkr vkr-recovery -o jsonpath='{.status.phase}'` 跟踪进度
→ Pending → Mounting → Restoring → Verifying → Completed。

## 4. 扩缩容

### 4.1 复制集扩缩容 (replicas N → M)

```sh
kubectl patch vk valkey-prod --type=merge -p '{"spec":{"replicas":5}}'
# operator 写入新的 STS replicas;新 replica 会等到
# master_link_status 报告 up 后才视为就绪。
```

### 4.2 ValkeyCluster 分片扩展

尚未实现 — 详见 ROADMAP Phase B (Track B)。人工流程:

```sh
# 创建新的分片 pod,然后手动执行 CLUSTER MEET 与重新分片。
# 运维指南另行追踪。
```

## 5. 升级

### 5.1 Valkey 版本升级

```sh
kubectl patch vk valkey-prod --type=merge -p '{"spec":{"version":{"version":"8.1.7"}}}'
# operator 会将 Phase=Upgrading,并对 STS 执行滚动重启。
# Replication 模式下顺序为先 replica、后 primary — 全程关注
# master_link_status。
```

### 5.2 operator 版本升级

`make deploy IMG=...` 或使用 Helm chart (单独提交)。

## 6. 应急处置

### 6.1 强制重启 operator manager

```sh
kubectl -n valkey-operator-system rollout restart deploy/valkey-operator-controller-manager
```

### 6.2 中止误触发的 `ValkeyRestore`

```sh
# Restore 会在 STS 中注入 init container 并设置 paused 注解。
# 当 operator 无法自愈时,人工清理:
kubectl delete vkr <name>                                # finalizer 会还原 STS 并移除 paused
# 如果 finalizer 自身卡住 (罕见):
kubectl patch vkr <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl annotate vk <target> cache.keiailab.io/paused-     # 人工移除 paused 注解
kubectl edit sts <target>                                  # 删除名为 "valkey-restore-init" 的 init container
```

### 6.3 直接访问数据面

```sh
PASS=$(kubectl get secret <cr-name>-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec -it <cr-name>-0 -- valkey-cli -a "$PASS"
# 启用 TLS 时: valkey-cli --tls --cacert /tls/ca.crt --cert /tls/tls.crt --key /tls/tls.key -p 6380
```

## 7. 可观测性约定

- **指标** (子系统 `valkey_cluster_*`): `state_ok`、
  `assigned_slots`、`shards`、`ready_replicas`、`reconcile_total`、
  `reconcile_errors_total`、`phase`、`backup_total`、
  `restore_total`、`failover_total`、`build_info` (cycle 57)。
  当 `Spec.Monitoring.ServiceMonitor.Enabled` 启用时,ServiceMonitor
  会自动注册。
- **事件**: `kubectl get events --field-selector involvedObject.kind=Valkey`。
- **日志**: 结构化 (`zap`)。
  `kubectl logs <operator-pod> -f --tail=100`。

### 7.0 Prometheus ServiceMonitor TLS — 生产环境加固 (cycle 100)

**默认配置**: `config/prometheus/monitor.yaml` 中的 ServiceMonitor
默认带有 `insecureSkipVerify: true` — 这是 Kubebuilder 的默认行为。
**在生产环境下这就是 MITM 攻击面**。请按以下步骤启用 cert-manager 验证:

```sh
# 1. 首先在集群范围内安装 cert-manager。
# 2. 取消注释 config/prometheus/kustomization.yaml 中的 patches 块。
sed -i '' 's|^#patches:|patches:|; s|^#  - path: monitor_tls_patch.yaml|  - path: monitor_tls_patch.yaml|; s|^#    target:|    target:|; s|^#      kind: ServiceMonitor|      kind: ServiceMonitor|' \
  config/prometheus/kustomization.yaml
# 3. 同时取消注释 config/default/kustomization.yaml 中
#    [METRICS WITH CERTMANAGER] 的 patches 块。
# 4. 通过 make build-installer 或 make deploy 重新部署。
```

验证通过后,`monitor_tls_patch.yaml` 会将
`insecureSkipVerify` 翻转为 `false`,并引用由 cert-manager 签发的
`metrics-server-cert` Secret,从而提供 **可验证的双向 TLS**。
这正是 ADR-0003 中 "TLS InsecureSkipVerify temporary" 走向生产的
演进路径。

### 7.1 Operator 环境变量 (cycle 80)

用于诊断 *某个集群中实际启用了哪些 reconciler*:

| 环境变量 | 默认值 | 作用 |
|---|---|---|
| `ENABLE_CLUSTER_RECONCILER` | `true` | 设为 `false` 时跳过 ValkeyClusterReconciler — chart `features.cluster.enabled=false` 时自动注入。 |
| `ENABLE_BACKUP_RECONCILER` | `true` | 设为 `false` 时跳过 ValkeyBackup / BackupTarget / Restore 三个 reconciler — chart `features.backup.enabled=false` 时自动注入。 |
| `ENABLE_WEBHOOKS` | `true` | 设为 `false` 时不注册 ValkeyWebhook 与 ValkeyClusterWebhook。**仅限 envtest 使用**;生产环境严禁设置此值。 |
| `WATCH_NAMESPACES` | 未设置 (cluster-wide) | 逗号分隔列表 (`ns1,ns2`)。会写入 `cache.DefaultNamespaces` — 由 chart `watch.namespaces` 自动注入。 |
| `OPERATOR_IMAGE` | `controller:latest` | Upload/Download Jobs 使用的镜像 — 由 chart `valkey-operator.image` helper 自动注入 (cycle 64)。未设置时存在 ImagePullBackOff 风险。 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 未设置 (no-op) | OTLP gRPC endpoint — 由 chart `tracing.endpoint` 自动注入 (cycle 65)。未设置时 22 条 span 不产生任何开销。 |
| `OTEL_SERVICE_NAME` | `valkey-operator` | OTEL 服务标识 — 由 chart `tracing.serviceName` 自动注入。Jaeger / Tempo UI 中用作 service id。 |

**注意**: `ENABLE_*_RECONCILER` / `ENABLE_WEBHOOKS` 均
**区分大小写** — 仅当值为小写字符串 `"false"` 时才生效。其他写法
(`"FALSE"`、`"False"`) 以及其他 "falsy" 字符串 (`"0"`、`"no"`) 都会
按 Kubebuilder 惯例视为 **启用**。

诊断命令:

```sh
# 当前生效的 env
kubectl exec -n valkey-operator-system <operator-pod> -- env | \
  grep -E "ENABLE_|WATCH_NAMESPACES|OPERATOR_IMAGE|OTEL_"

# 启动日志 (哪些 reconciler 被跳过、watch 范围、版本号)
kubectl logs -n valkey-operator-system <operator-pod> | head -20
```

## 8. ADR / RFC 参考

- ADR-0010 cert-manager 自动识别 / ADR-0013 Auth 强制启用 /
  ADR-0014 TLS volume mount
- ADR-0015 Restore init-container 模式 / ADR-0016
  `ValkeyBackupTarget`
- ADR-0022 minio-go / ADR-0023 子命令模式
- ADR-0045 GH Actions 恢复 / ADR-0046 SLSA-3 + cosign

完整 INDEX: `docs/kb/adr/INDEX.md`。

## 9. 各告警响应 (Prometheus 告警 → MTTR)

每条告警的 `runbook_url` 注解都指向本节。值班人员按
**Trigger → Diagnosis → Mitigation → Escalation** 的顺序逐项处置。

### 9.1 ValkeyClusterStateNotOK
- **Trigger**: `valkey_cluster_state_ok == 0` 持续 5m。`CLUSTER INFO` 中的 `cluster_state` ≠ ok。
- **自愈 (ADR-0039)**: 即便 `ClusterInitialized=true`,operator 也会
  重新调用 `ensureClusterMeet`。请在 operator 日志中查找
  "INC-0001 self-heal" 行。如果 5 分钟窗口内仍未恢复,说明自愈
  本身已失败 — 切换至人工流程。
- **Diagnosis**: 完全按照 §2.3 ("ValkeyCluster cluster_state=fail")
  处理 (诊断顺序 + 人工恢复)。
- **Mitigation**:
  1. 找出缺失的 slot 并执行 `CLUSTER ADDSLOTS` (预期 5 分钟内
     恢复)。
  2. 若 `nodes.conf` 已陈旧,执行 §2.3 "人工恢复" 流程
     (清空 PVC + 重置 `clusterInitialized`)。请先备份数据。
- **References**: INC-0001、ADR-0039。

### 9.2 ValkeyClusterSlotsMismatch
- **Trigger**: `valkey_cluster_assigned_slots != 16384` 持续 5m。
- **Diagnosis**: 使用
  `valkey-cli cluster nodes` 检查 slot 分布。可能只是 resharding 过程中的临时状态。
- **Mitigation**: 若持续 5 分钟以上,人工执行 `CLUSTER ADDSLOTS`
  或重启 operator。

### 9.3 ValkeyClusterNoReadyReplicas
- **Trigger**: `valkey_cluster_ready_replicas == 0` 持续 5m。所有 pod NotReady。
- **Diagnosis**: §2.2 (CrashLoopBackOff) 加上节点层面的信号
  (磁盘压力等)。
  `kubectl get pods -l app.kubernetes.io/name=valkey` 配合 describe。
- **Mitigation**: PVC 重新绑定、镜像拉取问题、OOMKilled 等。
  按根因走 §2 中的对应流程。
- **Escalation**: 如果是节点宕机,扩节点或迁移到其他节点。

### 9.4 ValkeyClusterDegraded
- **Trigger**: `0 < ready_replicas < 2` 持续 5m。部分 pod NotReady。
- **Diagnosis**: 检查每个 NotReady pod 的日志与事件。
- **Mitigation**: 通常仍是 §2.2 的模式。

### 9.5 ValkeyClusterPhaseFailed
- **Trigger**: `valkey_cluster_phase{phase="Failed"} == 1` 持续 1m。
- **Diagnosis**: §2.1 ("Phase=Failed CR")。查看 Conditions 中的
  `LastError`。
- **Mitigation**: 按具体错误处置 (通常是 admission / RBAC /
  StorageClass 相关问题)。

### 9.6 ValkeyOperatorReconcileErrorsHigh
- **Trigger**: `rate(valkey_cluster_reconcile_errors_total[5m]) > 0.1` 持续 5m。
- **Diagnosis**: 在 operator 日志中 grep `level=error`,并配合
  kubectl events。典型原因: RBAC、API server 压力、CR 校验拒绝。
- **Mitigation**: 临时性问题会自愈。若持续存在,执行 §6.1
  (重启 operator)。

### 9.7 ValkeyOperatorDown
- **Trigger**: `up{job=~"valkey-operator.*"} == 0` 持续 2m。
- **Diagnosis**: §6.1 ("强制重启 operator manager")。检查
  Deployment Available 状态、Pod 状态、节点状态。
- **Mitigation**: 按 §6.1 执行 rollout restart。若是
  `ImagePullBackOff`,检查镜像。
- **Escalation**: 所有 reconcile 都已停止 — 新 CR 不会被处理,
  phase 也无法迁移。SEV-1。

### 9.8 ValkeyBackupFailureRateHigh
- **Trigger**: `rate(valkey_cluster_backup_total{phase="Failed"}[1h]) > 0.0017` (约每小时 6 次) 持续 10m。
- **Diagnosis**: §3 ("备份 / 恢复")。查看 Failed 状态的
  `ValkeyBackup` 上的 `LastError`,以及对应 Job / Upload Pod 的日志。
  通常嫌疑包括凭据、S3 bucket 权限或磁盘空间。
- **Mitigation**: 轮换凭据或更换 `BackupTarget` 的 endpoint。
  评估对保留策略 (TTL) 的影响后重新执行。

### 9.9 ValkeyRestoreFailureRateHigh
- **Trigger**: `rate(valkey_cluster_restore_total{phase="Failed"}[1h]) > 0.0017` 持续 10m。
- **Diagnosis**: §3.3 ("恢复")。检查 source RDB 完整性、init
  container 日志,以及 PVC ROX 挂载情况。
- **Mitigation**: 按 §6.2 中止误触发的 Restore 后再次执行。
  Failed 状态的 Restore CR 在 finalizer 清理后再删除。

### 9.10 ValkeyFailoverHigh
- **Trigger**: `rate(valkey_cluster_failover_total[1h]) > 0.005` (约每小时 18 次) 持续 10m。
- **Diagnosis**: 频繁 failover 表示 primary 不稳定。检查 primary
  是否被 OOMKilled、是否发生网络分区,以及复制延迟情况。
  `valkey-cli info replication`。
- **Mitigation**: 调整资源配额、审查网络策略、将 primary 上的
  负载转移 (使用读 replica)。检查磁盘 I/O 瓶颈。
- **Escalation**: 怀疑 split-brain 时 → 走 §2.3 流程并参阅
  ADR-0017。

### 9.11 ValkeyOperatorReconcileLatencyP95High
- **Trigger**: reconcile 成功的 p95 > 1 s 持续 10m。
- **Diagnosis**: 集群 API server 压力、operator pod CPU 受限,
  或 reconciler 中存在外部调用超时。
  通过 `kubectl top pod -n valkey-operator-system` 观察 CPU。
- **Mitigation**: 上调 operator pod 的 `resources.requests.cpu`,
  或将其与其他 controller 的 API 突发隔离开来。

### 9.12 ValkeyOperatorReconcileLatencyP99Critical
- **Trigger**: reconcile (成功 + 错误) p99 > 5 s 持续 10m。
  已逼近 controller-runtime 默认 context timeout (30 s) 的危险水位。
- **Diagnosis**: 与 9.11 相同,但属于 **严重饱和** 场景。检查
  operator pod 状态,以及 `reconcile_errors_total` 按 component
  标签的分布。
- **Mitigation**: 立即重启 operator pod
  (`kubectl rollout restart deploy/valkey-operator-controller-manager -n valkey-operator-system`)。

### 9.13 ValkeyOperatorReconcileErrorRateHigh
- **Trigger**: reconcile error rate > 5 % 持续 10m。
- **Diagnosis**: 按 `component` 标签拆分
  `valkey_cluster_reconcile_errors_total`,定位是哪个阶段
  (secret / sts / svc / tls / backup) 失败。
- **Mitigation**: 应用 §2.1 `Phase=Failed CR` 流程
  (按 Conditions 的 Reason 分类处理)。

## 10. Replication 模式自动 Failover

ADR-0017 (replication failover — 选择 offset 最大的 replica)。
failover 决策由 operator 主导;不存在 Sentinel sidecar。

### 10.1 触发条件

- `Spec.Mode == Replication` 且 `Spec.Replicas > 1`
- `Spec.AutoFailover == true` (默认开启 — 设为 `false` 可禁用)

### 10.2 算法 (7 步)

1. 读取当前 primary (`Status.CurrentPrimary` 或 `<name>-0`)
   并验证 pod 就绪状态。
2. `NotReady` 时长不足 30 s 视为瞬时抖动,忽略。
3. 对所有 replica (除 primary 外) 执行 `INFO replication`,
   收集 `master_repl_offset`。
4. `selectFailoverCandidate` 选出 offset 最大的 replica
   (平局时取 ordinal 较小者)。
5. `PromoteToPrimary` 对该候选发出 `REPLICAOF NO ONE`
   → 其成为新的 primary。
6. 对其余每个 `Ready` 状态的 replica 发出 `EnsureReplicaOf <new-primary>`。
7. 在内存中更新 `Status.CurrentPrimary`;下一次 reconcile
   会将其写回 CR。

各阶段对应的 OTel span: `Failover/INFO_replication`、
`Failover/PromoteToPrimary`、`Failover/EnsureReplicaOf_all`
(参阅 [`observability/otel.md`](../observability/otel.md))。

### 10.3 禁用方式

```yaml
spec:
  autoFailover: false   # operator 跳过 failover 路径;仅支持人工恢复
```

### 10.4 已知限制

- **网络分区下无强 split-brain 防护** — 可能选出两个 primary。
  缓解措施: 30 s 的 `NotReady` 阈值,以及 operator leader election
  (单一副本)。
- **ValkeyCluster 模式不适用** — Valkey 原生 cluster 模式依赖
  `cluster_replica_validity_factor`。`ValkeyClusterReconciler` 有意
  不参与 failover。
- **e2e 自动化暂缺** — replication-failover e2e 作为独立 cycle 推进。

## 11. 已验证的运维场景

下表记录在发布验证 (release-validation) 过程中实测过的场景。
每一行代表一次针对线上 operator 执行的场景;"行为" 列描述了实际
观察到的结果。

| 场景 | 行为 | 数据 |
|---|---|---|
| primary pod kill (force) | STS 自动重建;operator 重新 promote pod-0 | PVC 保留 |
| replica 扩容 (3 → 5) | 新 replica 自动加入 `master link up` | — |
| replica 缩容 (5 → 2) | 多余 pod 被清理 | 既有数据保留 |
| ValkeyCluster shard pod kill | `cluster_state=ok` 保持 (replica 接管) | 所有 slot 完整 |
| TLS + mTLS ValkeyCluster (cert-manager) | `Phase=Running`、`slots=16384`、数据面 SET/GET 通过 | — |
| TLS + mTLS Valkey Standalone (cert-manager) | `Phase=Running`,6380 端口 ping/set/get 通过 | — |
| TLS + mTLS Valkey Replication (3 replicas) | `master_link_status:up`,写入跨 replica 传播通过 | — |
| ValkeyBackup (RDB) | Pending → InProgress → Completed,生成 89 B 的 `/data/dump.rdb` | — |
| ValkeyBackup M3.5 (基于 Job 的 PVC) | Copying → Completed,`<name>-backup` PVC 中保留 `dump.rdb` | TLS 自动传递 |
| ValkeyRestore (Standalone PVC) | Mounting → Restoring → Verifying → Completed;init container 复制 `/data/dump.rdb` | 填充 `Status.RestoredKeys` |
| ValkeyRestore (`Source.TargetRef` S3) | 临时 PVC + Download Job → 走标准 init-container 流程 | 跨集群恢复 |
| ValkeyRestore (ValkeyCluster,ROX) | 按分片 ordinal → `SHARD_IDX` shell 映射 → 逐 pod cp | 要求源 PVC 为 ROX |
| Replication 自动 Failover (ADR-0017) | primary `NotReady` 30 s+ → offset 最大的 replica 执行 `REPLICAOF NO ONE` → `Status.CurrentPrimary` 更新 | e2e: `test/e2e/failover_test.go` |
| NetworkPolicy 资源创建 | selfPeer + 6379 / 16379 入站 + `ownerReferences` | (依赖 CNI) |
| Operator metrics endpoint (HTTPS:8443) | 暴露 `controller_runtime_*` + `valkey_cluster_*` | — |
| Prometheus 告警规则 | 13 条告警 (state / slots / replicas / phase / errors / latency / operator down) | `config/prometheus/alert-rules.yaml` |
