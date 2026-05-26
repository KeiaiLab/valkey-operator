# 文档 — valkey-operator

> English: [README.md](README.md) — canonical / 正本

> 所有文档以 **英文版本** 为正本。凡有中文姊妹版的文档,以
> `<name>.zh.md` 形式与英文并排存放 (`.ko.md` 与 `.ja.md` 同理)。
> 本页面是 `docs/` 下全部文档的 **唯一入口**。

## 运维指南

| 文档 | 用途 |
|---|---|
| [Operations 索引](operations/README.md) | 日常 runbook + 故障排查 + 指标 + webhook + PDB + 容量规划 |
| [Runbook](operations/runbook.md) | 故障响应与日常运维 — 每一条 Prometheus 告警 `runbook_url` 的 SSOT |
| [故障排查](operations/troubleshooting.md) | 症状 → 根因 → 诊断 → 处置 |
| [指标术语表](operations/metrics-glossary.md) | 每个 `valkey_cluster_*` 指标、cardinality、告警消费方 |
| [容量规划](operations/capacity-planning.md) | 容量评估 — 内存、复制系数、分片数、p95/p99 延迟目标 |
| [Webhook](operations/webhook.md) | Admission webhook 架构、cert-manager 路径、拒绝事件调试 |
| [按分片的 PDB](operations/pdb-per-shard.md) | 按分片粒度的 `PodDisruptionBudget` 运维、drain 场景 |
| [PITR 指南](operations/pitr-guide.md) | 时点恢复 (Point-in-time recovery) — API + webhook + 人工兜底 |
| [混沌测试](operations/chaos-testing.md) | chaos-mesh 4 场景 e2e 流程 |

## 发布与供应链

| 文档 | 用途 |
|---|---|
| [发布检查清单](operations/release-checklist.md) | 发布前门禁盘点:构建、SSOT gate、供应链、文档 |
| [合并后清理](operations/post-merge-cleanup.md) | squash-merge 之后的本地分支整理 |
| [Artifact Hub trust](operations/artifacthub-trust.md) | Artifact Hub `Signed` 与 `Official` badge 的运维流程 |
| [升级指南](UPGRADING.md) | Minor / major 升级迁移、静态 manifest 用户的 RBAC patch |

## 迁移

| 文档 | 用途 |
|---|---|
| [迁移索引](migration/README.md) | StatefulSet → ValkeyCluster CR 迁移 runbook 汇总 |
| [Sentinel 迁移](operations/sentinel-migration.md) | 既有 Sentinel HA → Replication 模式 + AutoFailover (ADR-0017 兜底) |

## 架构与决策记录

| 文档 | 用途 |
|---|---|
| [ADR 索引](kb/adr/INDEX.md) | 全部 52+ 份架构决策记录 |
| [可观测性 — OpenTelemetry](observability/otel.md) | OTel 链路传播、OTLP exporter、控制器 reconcile span |
| [Valkey 9.x feature flag](version/9x-flags.md) | 新增的集群模式 flag + operator 集成路径 |

## 知识库

| 文档 | 用途 |
|---|---|
| [Incident KB](kb/incident/INDEX.md) | 轻量级 postmortem 记录 (blameless) |
| [依赖变更日志](kb/deps/2026-05.md) | go.mod / go.sum diff 历史 |

## 社区健康

按 ADR-0053 (根目录 `.md` 策略 + 工具依赖例外) 的规定,GitHub
检测的 community health 文件存放在 `.github/` 下:

| 文档 | 路径 |
|---|---|
| 贡献指引 | [.github/CONTRIBUTING.md](../.github/CONTRIBUTING.md) |
| 行为准则 | [.github/CODE_OF_CONDUCT.md](../.github/CODE_OF_CONDUCT.md) |
| 安全策略 + artifact 验证 | [.github/SECURITY.md](../.github/SECURITY.md) |
| 支持资源 | [.github/SUPPORT.md](../.github/SUPPORT.md) |
| 项目治理 | [.github/GOVERNANCE.md](../.github/GOVERNANCE.md) |

## i18n 状态

韩文姊妹版与英文正本并排存放 (`<name>.ko.md`)。
日文 (`.ja.md`) 与中文 (`.zh.md`) 的覆盖范围 **优先聚焦顶层文档**
(README、ROADMAP、CONTRIBUTING)。运维 runbook 则随着采纳
需求逐步本地化。
