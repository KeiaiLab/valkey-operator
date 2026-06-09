# ARCHITECTURE — valkey-operator (简体中文)

> English: [ARCHITECTURE.md](ARCHITECTURE.md) — canonical / 正本

> 单页架构说明。当 CRD 表面 / 拓扑 / reconcile 模式发生变更时同步更新。

## 概览

- **目标**: 面向 [Valkey](https://valkey.io) (Redis 的 BSD-3 fork) 的 Kubebuilder K8s operator。单一控制器在统一的 CRD 表面之下管理三种拓扑。
- **范围**: Standalone / Replication / Cluster (16384-slot) 三种拓扑 + 备份/恢复 + S3 兼容的外部存储。
- **稳定度等级**: v1.0.13 (standalone + replication + cluster 已 GA; federation 计划中 — 尚未启动)
- **最新发布**: v1.0.13 (2026-05-13)
- **许可证**: MIT
- **模块路径**: `github.com/keiailab/valkey-operator`

## CRD 表面 (5 个 CRD)

| CRD | apiVersion | 拓扑 | 说明 |
|---|---|---|---|
| `Valkey` | `valkey.keiailab.com/v1alpha2` | Standalone / Replication | 单实例 或 1 primary + N replicas |
| `ValkeyCluster` | `valkey.keiailab.com/v1alpha2` | 分片 Cluster (16384 slot) | 3+ 分片 × (1 primary + 0–5 replicas) |
| `ValkeyBackup` | `valkey.keiailab.com/v1alpha2` | — | 一次性 RDB 或 AOF 备份到 PVC + 外部存储 |
| `ValkeyBackupTarget` | `valkey.keiailab.com/v1alpha2` | — | S3 兼容的外部存储抽象 (ADR-0016) |
| `ValkeyRestore` | `valkey.keiailab.com/v1alpha2` | — | 通过 Init Container 将 RDB 恢复到 Valkey 或 ValkeyCluster (ADR-0015) |

Conversion webhook 支持 v1alpha1 ↔ v1alpha2 转换。

## Reconcile 流程

```
Watch CRD events
      │
      ▼
Reconcile loop
      │
      ├── StatefulSet (per shard)
      ├── ConfigMap (valkey.conf)
      ├── Secret (auth + TLS keys)
      ├── Service (headless + ClusterIP)
      ├── PodDisruptionBudget
      ├── NetworkPolicy (deny-by-default)
      ├── cert-manager Certificate (webhook serving + TLS)
      └── Prometheus ServiceMonitor

所有资源均带 spec-drift 检测进行 reconcile。
Cluster 拓扑: 分片扩缩容时进行 slot 再平衡 + replica 重新选举。
```

## RBAC 范围

- ClusterRole: CRD watch + cert-manager Certificate + Prometheus ServiceMonitor
- Role (按 namespace): StatefulSet / Service / Secret / ConfigMap / PVC / PDB / NetworkPolicy / Job
- ServiceAccount: `valkey-operator`
- Webhook: validation + conversion (通过 cert-manager 提供 TLS)

## 测试分层

| 层次 | 位置 | 覆盖范围 |
|---|---|---|
| Unit | `internal/**/_test.go`, `api/**/_test.go` | gocovmerge → cover-final.out |
| Integration (envtest) | `test/integration/` | reconcile + conversion + webhook |
| E2E (kind) | `test/e2e/`, `Makefile setup-test-e2e` | release 关键场景 |
| Scorecard | `bundle/tests/scorecard/` | OLM v1alpha3 6-test parity |

## 构建 / 部署

- 容器镜像: `ghcr.io/keiailab/valkey-operator:v1.0.13`
- Helm chart: `charts/valkey-operator/` (发布于 `keiailab.github.io/valkey-operator`)
- OLM bundle: `bundle/`
- ArtifactHub: `keiailab-valkey-operator`
- Quickstart: kind 集群 + cert-manager 1.16+ (`make setup-test-e2e`)

## 安全供应链

- **SLSA-3 provenance** (ADR-0046)
- **cosign keyless 签名** (ADR-0046)
- **OpenSSF Scorecard** 已启用 (README 中含徽章)
- **CodeQL** + **dependency-review** + **DCO** workflow
- **`.gitleaks.toml`** 密钥扫描 (覆盖 42/44)
- **go-licenses** 依赖许可证扫描 + allowlist

## ADR 交叉引用 (45 条 ADR — 三个 operator 中 ADR 最丰富)

要点:
- ADR-0015: 通过 Init Container 模式实现 Restore
- ADR-0016: ValkeyBackupTarget — S3 抽象
- ADR-0045: GitHub Actions release 流水线恢复
- ADR-0046: SLSA-3 + cosign keyless
- ADR-0047: community-operators 上游同步自动化 (cycle 25)

完整列表见 `docs/kb/adr/INDEX.md`。

## 路线图状态

- 已完成: 31 项 (Cluster 模式 + 备份/恢复 + HPA/PDB/NP + 版本升级 + Valkey 9.x + API 演进 + webhook admission + Helm + SLSA-3 + ServiceMonitor + OpenSSF)
- 待办: 38 项 (production cluster 落地 + 迁移 runbook + smoke test + Grafana + OTel + SBOM + 9.x 特性跟进 + multi-cluster federation + 跨区域复制集 + 在线 schema-less 迁移 + 加权 replica 路由 + controller v2 + CRD v1 graduation)

## Non-goals

- ❌ 内嵌 Redis (我们提供 Valkey — 许可证兼容的 BSD-3 fork)
- ❌ 内嵌 third-party Valkey chart (我们原生实现)
- ❌ Redis Sentinel 拓扑 (改用 3-分片 cluster)
- ❌ Valkey 8.0 以下版本

## 参考

- `README.md` / `README.ko.md`
- `ROADMAP.md`
- `CHANGELOG.md`
- `ADOPTERS.md` / `ADOPTERS.ko.md`
- `CONTRIBUTING.md` / `CONTRIBUTING.ko.md`
- `GOVERNANCE.md` / `GOVERNANCE.ko.md`
- `AGENTS.md`
- `docs/kb/adr/INDEX.md`
