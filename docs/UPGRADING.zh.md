# 升级 valkey-operator

> English: [UPGRADING.md](UPGRADING.md) — canonical / 正本

本文档汇总 valkey-operator 在 minor / major 版本升级时所需的迁移操作。
Helm 用户只要执行 chart 升级即可应用全部变更;而使用静态 manifest
(`kubectl apply -f`) 的用户则需要手工 patch RBAC 等少数条目。

## 0. 版本策略 (semver)

| 变更类型 | semver bump | 示例 |
|---|---|---|
| 新增 controller / CR / API | minor (v1.X → v1.X+1) | 新增 ValkeyBackupTarget |
| 既有 API 签名变更 (breaking) | major (v1.X → v2.0) | ValkeyCluster.spec.storage 结构变更 |
| bug fix / 依赖 bump | patch (v1.X.Y → v1.X.Y+1) | controller-runtime 0.19→0.20 |

## 1. v1.0.x → v1.0.13 (当前版本)

### Helm 用户

```bash
helm repo update
helm upgrade valkey-operator keiailab-valkey-operator/valkey-operator \
  --namespace valkey-operator-system \
  --version 1.0.13
```

chart 自身会同步 RBAC、CRD 与 Deployment。无需任何额外操作。

### 静态 manifest 用户 — RBAC 迁移

确认 `make build-installer` 产物 `dist/install.yaml` 的差异:

```bash
kubectl diff -f dist/install.yaml
kubectl apply -f dist/install.yaml
```

现有 ClusterRole 上新增的权限 (本 patch 暂无 RBAC 变更):

| API group | Resource | 原因 | 引入时点 |
|---|---|---|---|
| (无) | — | — | — |

### v1alpha1 → v1alpha2 conversion webhook

v1alpha2 首次引入。v1alpha1 的 CR 会通过 conversion webhook 自动
转换 — 用户无需任何操作。但使用 `kubectl apply -f` 新建 v1alpha1
manifest 时会出现 *deprecated* 警告 — 推荐改用 v1alpha2。

## 2. Sprint 1 引入 ( )

ADR-0049 (`docs/kb/adr/0049-sprint-1-commons-pvc-topology-adoption.md`)。

```bash
# 升级 go.mod 中 的依赖版本后
go mod tidy
```

- **删除的代码**: `internal/controller/pvc_resize.go` (-136 LOC) + 对应测试
  (-166 LOC) + `internal/resources/statefulset.go` 中的内联
  `defaultTopologySpread` (-22 LOC) → 合计 -322 LOC
- **调用点替换**:
  - `valkey_controller.go:235` — `commonspvc.ExpandDataPVCs(ctx, c, ns, []string{crName}, size)`
  - `valkeycluster_controller.go:239` — 同上
  - `statefulset.go` — `commonstopology.Defaulted(constraints, replicas, selector)`

迁移影响:
- Reconcile 行为完全一致 (纯重构,对外行为零变更)
- CRD spec 无变更
- Helm chart 无影响

## 3. v1.0.x → v2.0.0 (规划中 — v3.x-stable 宣告时点)

CLAUDE.md §7 的 *商用产品级别* (P0+P1+P2+OP+C 全部 ✅) 达成时。

- 所有 CR 的 API 稳定性升至 `Stable` (v1)
- breaking change *最小化* — major bump 是 *语义信号*
- 保证 5 个仓库的一致性:参阅
  `commons/docs/quality/production-grade-checklist.md`

## 4. GHA 双轨策略 (ADR-0048)

本仓库是 RFC-0002 (永久禁用 GitHub Actions) 的 *例外* — 由于 public
OSS operator 需要满足外部信任门槛 (CodeQL / OpenSSF Scorecard /
cosign / SLSA / Artifact Hub trust badges),因此保留 14 个 GHA
workflow,并与本地 4 层 hook (lefthook) 双轨并行 (ADR-0048)。

升级期间,GHA workflow 的改动会由 `dependabot/github_actions/*` PR
自动处理。若想由 *人工 PR* 向 `.github/workflows/` 新增文件,需要
*单独的 ADR* + 用户显式批准。

## 5. 通用迁移检查清单

升级前:
- [ ] CRD 变更 (`api/v1alpha1/` 与 v1alpha2 conversion webhook 的兼容性)
- [ ] `make verify` (lint + test + build + audit) 通过
- [ ] 既有 e2e 套件 PASS (`make integration-test`)
- [ ] chaos-mesh 场景 PASS (ADR-0041,4 个场景)
- [ ] 已合并 dependabot 依赖 bump 的相关 PR

升级后:
- [ ] 更新 Helm chart 的 `dependencies:` (keiailab-commons library chart)
- [ ] 验证各 CR 的 spec 兼容性 (尤其是 storage、resources)
- [ ] 确认 reconcile 结果 (`kubectl get valkey,valkeycluster -A`)
- [ ] 运维指标 (`Reconcile{Total,Latency,Errors}`) 处于正常区间
- [ ] 集群模式:确认 `ClusterInitialized=true` + `state=ok` (ADR-0039)

## 6. 不兼容变更通告策略

- **Deprecation**: 在新版 minor 中加 `// Deprecated:` 注释,
  2 个 minor 之后移除
- **Breaking**: major bump + 在本 UPGRADING.md 中单列章节 + 撰写 ADR
- **不做事后通告**: 所有 breaking 变更必须 *至少提前 1 个 minor*
  完成 deprecation

## 参考

- ADR 索引: `docs/kb/adr/INDEX.md`
- audit: `make audit-quality` (5 个仓库的度量,commons ADR-0013)
- i18n: `commons/docs/i18n/README.md`
- 家族 family: `docs/family.md`
- Helm chart: https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
