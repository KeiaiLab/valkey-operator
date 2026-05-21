# 混沌测试 — valkey-operator (简体中文)

> English: [chaos-testing.md](chaos-testing.md) — canonical / 正本

如何运行 ADR-0041 基于 chaos-mesh 的 4 场景混沌工程 e2e 套件。

## 前置条件

1. **Kind cluster** (或任意 Kubernetes) 已就绪:
   ```sh
   make setup-test-e2e   # 或者: kind create cluster --name valkey-e2e
   ```

2. **部署 valkey-operator**:
   ```sh
   make docker-build IMG=ghcr.io/keiailab/valkey-operator:e2e-dev
   make deploy IMG=ghcr.io/keiailab/valkey-operator:e2e-dev
   ```

3. **安装 chaos-mesh**:
   ```sh
   make chaos-mesh-install
   # 手动: kubectl apply -f https://mirrors.chaos-mesh.org/v2.7.2/chaos-mesh.yaml
   ```

4. **目标 ValkeyCluster** (namespace `valkey-chaos-e2e`):
   ```yaml
   apiVersion: cache.keiailab.io/v1alpha1
   kind: ValkeyCluster
   metadata: { name: vc-chaos, namespace: valkey-chaos-e2e }
   spec:
     shards: 3
     replicasPerShard: 1
     autoFailover: true
     version: { version: "9.0.4" }
   ```

## 运行

```sh
make chaos-e2e
# 或者覆盖目标 namespace:
CHAOS_TEST_NAMESPACE=my-ns make chaos-e2e
```

## 场景 (4 个)

| ID | Chaos 类型 | 行为 | 恢复检查 |
|---|---|---|---|
| 1 | PodChaos (pod-kill) | 5 min 内每 1 min 随机 kill 一个 pod | `cluster_state=ok` 5 min 内恢复 |
| 2 | NetworkChaos (partition) | 30 s master ↔ replica 网络隔离 | failover 或恢复在 3 min 内完成 |
| 3 | IOChaos (`ENOSPC` fault) | 60 s 模拟 80 % 满盘 | 3 min 内集群 degraded but healthy |
| 4 | IOChaos (latency) | 60 s 在 replica 上注入 100 ms I/O 延迟 | 3 min 内 master 无影响 (不发生 failover) |

每个场景: 下发 chaos CR → 等待 → 自动清理 → 验证集群恢复健康。
`BeforeSuite` 创建的 `vc-chaos` CR 在所有场景之间保留。

## 运维集成

- **开发者本地**: 任何 reconciler 改动之后**建议**执行
  (full e2e + chaos ≈ 30 min)。
- **CI nightly**: ADR-0041 AI-005 (单独 follow-up) — 等待 CI 基础
  设施工作落地后再自动化。
- **生产环境调试**: chaos-mesh **永远不**直接在生产环境运行。仅限
  staging / pre-prod 使用。

## 清理

```sh
make chaos-mesh-uninstall
kubectl delete namespace valkey-chaos-e2e
```

## 新增一个场景

- 新增 chaos CRD: 采用 `chaos-mesh.org/v1alpha1` 下的其它 kind
  (`TimeChaos`、`DNSChaos`、`KernelChaos` 等)。
- 模式: 在 `test/chaos/scenarios_test.go` 中新增一个
  `var _ = Describe(...)` 块,使用 `makeChaos(kind, name, ns,
  spec)` helper。
- chaos-mesh CRD spec 参考: <https://chaos-mesh.org/docs/>

## 排障

| 现象 | 原因 / 处置 |
|---|---|
| `chaos-mesh.org/v1alpha1: NoMatchError` | chaos-mesh CRD 未安装 — `make chaos-mesh-install` |
| `kubectl apply` permission denied | chaos-mesh controller 缺少 **namespace 权限**。检查 `--local kind` 安装选项。 |
| 场景 timeout | 集群规模 / 镜像拉取慢 — 用 `--timeout=30m` 或更长值重跑。 |
| Pod 卡在 `Terminating` | 需要清除 finalizer — `kubectl patch pod ... --type=merge -p '{"metadata":{"finalizers":[]}}'` |

## 参考

- ADR-0041 — chaos-mesh 选型动机 + 候选对比。
- ADR-0040 §gap #4 — chaos engineering e2e。
- chaos-mesh: <https://chaos-mesh.org/>
- Makefile target: `chaos-mesh-install`、`chaos-mesh-uninstall`、
  `chaos-e2e`。
