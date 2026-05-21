# INC-0001: ValkeyCluster 在 cluster_state:fail 状态下未重新执行 bootstrap (简体中文)

> English: [INC-0001-cluster-fail-bootstrap-skip.md](INC-0001-cluster-fail-bootstrap-skip.md) — canonical / 正本

- Detected: 2026-05-09 14:27 (KST) — production cluster,生产集群
- Resolved: 2026-05-10 09:18 (KST)
- Severity: SEV-2 (仅影响单一 cluster,未影响应用流量 — 仅为测试数据)
- Owners: @eightynine01
- Tags: [valkey, cluster, reconcile, status, controller-runtime]

## Impact

- **用户影响**: 0 (cluster 中的 keys 全部为测试数据 — `test_valkey_br_*`、`test_prod_*`、`test_failover_*`,6 个唯一 key × 2 master+replica)。生产应用流量当时尚未接入。
- **系统影响**: ValkeyCluster (生产实例) (3 shards × 2 = 6 pods,16384 slots) — 约 19 小时卡在 `cluster_state: fail` 状态。
- **财务/法律影响**: 无。

## Timeline

- **2026-05-07 06:14**: cluster 初次 deploy。Bootstrap 正常完成。`clusterInitialized: true`。
- **2026-05-08 07:21**: ClusterReady=False / ClusterNotConverged condition 显现 (原因不明 — 可能是 pods 变动或 cluster bus partition)。
- **2026-05-09 14:27**: Pods 重启 (9 小时前)。分配到新 IP。`nodes.conf` 中的 myself IP 仍为 *旧 IP* (例如 10.42.6.172),未刷新。与其他节点 cluster gossip 失败,出现 `cluster_state:fail`。controller 以 *clusterInitialized=true* 状态判断,*跳过* 了 cluster bootstrap 的重新执行,仅尝试 STS reconcile → 反复触发 STS conflict ("the object has been modified")。
- **2026-05-10 00:02**: ReconcileError condition 最后一次 transition。此后 controller queue 进入指数退避,reconcile 频率降低。
- **2026-05-10 09:00 ~ 09:18**: 排障 + 修复:
  1. 重启 controller pod (无效 — clusterInitialized 仍为 true)。
  2. 对 6 个 pod 执行 CLUSTER RESET HARD — 3 个 master pod 因仍持有 keys 被拒绝。
  3. 删除 Pod-1 的 nodes.conf 并重启 — 部分恢复 (仅自身 shard 正常)。
  4. 对 6 个 pod 执行 FLUSHALL + CLUSTER RESET HARD + 删除 AOF/nodes.conf + 同时重启 — 进入 fresh state。
  5. controller 进入 reconcile,但仍不执行 bootstrap — 因为 `clusterInitialized: true` 是阻断点。
  6. 通过 `kubectl patch --subresource=status` 强制将 `clusterInitialized: false` 重置 → controller 立即重新执行 bootstrap → 16384 slots 全部正常。

## Root Cause

5 Whys:

1. **为什么 cluster_state:fail?** Pods 上的 nodes.conf 保留了过期的 myself IP → cluster gossip 失败。
2. **为什么 nodes.conf 过期?** Pods 重启时 PVC 中的 nodes.conf 仍保存 *旧 IP*,valkey 启动时会读取它。
3. **为什么 controller 没有恢复?** controller 跳过了 cluster bootstrap 阶段 (CLUSTER MEET / ADDSLOTS / REPLICATE)。
4. **为什么 bootstrap 阶段被跳过?** `status.clusterInitialized: true` 在 *初始化完成时* 设置,而且 *cluster fail 状态下也不会被重置* — 这是 controller 代码中 *one-shot init* 的假设。
5. **为什么会有 one-shot init 假设?** 早期设计假定 *cluster 一旦 bootstrap 完成就永久 healthy* — 未考虑 pod 重启后 IP 变更的场景。ADR-0017 (保留 failover) 制定时未将 *cluster topology 自动恢复* 纳入范围。

诱因 (contributing factors):
- `nodes.conf` 保存在 PVC 中 (stateful) — 要反映新 IP 需要 valkey-cli 触发 cluster reset 或更新 announce-ip。
- controller 的 ReconcileError condition 只反映了 STS conflict — 未发出 *cluster 自身 fail* 的信号 (因此既无告警也无恢复)。

## Resolution

人工应急处置 (本次 INCIDENT):
1. 评估数据损失: keys 全部为测试数据 → 可以安全 wipe。
2. 删除 6 个 pod 的 PVC 数据 (AOF + nodes.conf + dump.rdb)。
3. 同时重启 6 个 pod (fresh state)。
4. 强制 patch `status.clusterInitialized: false`。
5. 通过 spec mutation 触发 controller reconcile。
6. controller 立即重新执行 cluster bootstrap → 16384 slots 全部正常。

永久修复 (单独 PR — 本 INC 的后续):
- controller 代码在评估 *clusterInitialized* flag 时,同时校验 `cluster_state == "ok"` AND `assignedSlots == 16384`。出现 fail 或 partial assignment 时,*自动重新执行 bootstrap*。
- 新增告警规则: `cluster_state:fail` 持续 30s 以上时触发 PrometheusRule。

## Prevention

短期 (本事件之内):
- ✓ 撰写 INCIDENT KB (本文档)。
- ⏳ 新增告警规则 (`prometheus.io/scrape` annotation 暴露的 metrics 路径 — `cluster_status_ok` metric)。

中期 (单独 PR):
- **PR-INC-0001-fix**: controller 在 `clusterInitialized` 为 true 时也增加 *再校验* 逻辑。出现 `cluster_state != "ok"` || `assignedSlots != 16384` 时重新执行 bootstrap。
- **PR-INC-0001-alert**: PrometheusRule (在 `groups[].rules` 中加入 `ValkeyClusterFail` alert)。

长期 (RFC 后续):
- RFC-0005 (单独 RFC): cluster topology 自愈策略。自动校验 nodes.conf 中的 myself IP + 动态更新 cluster announce-ip。利用 valkey 9.x 的 *cluster-announce-bus-port* + DNS-aware IP advertisement。

## Action Items

- [ ] AI-0001: PR-INC-0001-fix — controller 重新 bootstrap 逻辑 (Owner: @eightynine01,Due: 2026-05-15)。
- [ ] AI-0002: PR-INC-0001-alert — PrometheusRule (`ValkeyClusterFail` warning + critical)。
- [ ] AI-0003: e2e 回归测试 — pod IP 变更场景 + clusterInitialized=true 阻断点校验 (test/e2e/cluster_recovery_test.go)。
- [ ] AI-0004: docs/operations/runbook.md — 编写 cluster_state:fail 应急处置流程文档。

## References

- ADR-0017 (保留 failover) — 本 INC 属于其作用域之外的场景 (cluster topology 恢复)。
- HANDOFF.md 中的 PR-A2.2.5 storageversion fix — *独立事件*,与本 INC 无关。
- mongodb INC-0001 (另一仓库) — 作为跨仓 patterns audit 的候选样本。
