# 运维文档索引 (简体中文)

> English: [README.md](README.md) — canonical / 正本

> 中文用户提示: 所有文档都附带一份 `<name>.ko.md` 的韩文版本 (本 family 的 i18n 起点为韩文)。

本目录汇总了 `valkey-operator` 的运维流程。
**所有文档以英文为正本**,每篇都配有 `<name>.ko.md` 的韩文姊妹文件 (例外: `artifacthub-trust.md` 自始即用英文撰写,无韩文副本)。

## 日常运维

| 文档 | 用途 |
|---|---|
| [runbook.md](runbook.md) | 故障响应与日常运维。所有 Prometheus alert 的 `runbook_url` annotation 的 SSOT。 |
| [troubleshooting.md](troubleshooting.md) | 针对没有触发告警 (或告警尚未配置) 的问题: 症状 → 原因 → 诊断 → 修复 流程图。 |
| [metrics-glossary.md](metrics-glossary.md) | 所有 `valkey_cluster_*` 指标 — 标签 cardinality、采集位置、被哪些 alert 消费。 |
| [capacity-planning.md](capacity-planning.md) | Sizing 指南 — 各拓扑下的内存、复制系数、分片数、p95/p99 延迟目标。 |
| [webhook.md](webhook.md) | Admission webhook 架构、cert-manager 证书路径、webhook 拒绝排查。 |
| [pdb-per-shard.md](pdb-per-shard.md) | 分片型 `ValkeyCluster` 的按分片 `PodDisruptionBudget` 运维 — drain 场景、模式切换、写入可用性保证。 |

## 备份、恢复、容灾

| 文档 | 用途 |
|---|---|
| [pitr-guide.md](pitr-guide.md) | Point-in-time recovery: phase-1 API + webhook、phase-2 reconciler dispatch、手动绕行、回滚。 |
| [chaos-testing.md](chaos-testing.md) | chaos-mesh 4 类场景 e2e 流程: pod-kill、network partition、IO ENOSPC、IO latency。 |

## 发布 & 供应链

| 文档 | 用途 |
|---|---|
| [release-checklist.md](release-checklist.md) | 发布前 gate 清单: build、47 项 SSOT gate、供应链 (SLSA + cosign)、docs、operations。 |
| [post-merge-cleanup.md](post-merge-cleanup.md) | Squash-merge 之后本地分支清理。 |
| [artifacthub-trust.md](artifacthub-trust.md) | Artifact Hub `Signed` / `Official` 信任徽章的运维流程。 |

## 迁移

| 文档 | 用途 |
|---|---|
| [sentinel-migration.md](sentinel-migration.md) | Sentinel → valkey-operator Replication 模式迁移 runbook (ADR-0017 backstop)。 |

## 横向引用

- 架构决策: [`docs/kb/adr/INDEX.md`](../kb/adr/INDEX.md)
- 事故历史: [`docs/kb/incident/`](../kb/incident/)
- 发布产物校验命令: [`SECURITY.md → "Verifying release artifacts"`](../../.github/SECURITY.md#verifying-release-artifacts-signed-releases--v1013)
- 路线图 (项目方向): [`ROADMAP.md`](../ROADMAP.md)

## i18n 状态

`docs/operations/` 下的全部运维文档目前都已 **以英文为正本**。i18n 工作通过 PR #93 / #97 / #98 / #103 / #104 / #105 / #106 / #107 / #108 / #109 / #110 / #111 完成落地。韩文原文以 `<name>.ko.md` 姊妹文件原样保留 (`artifacthub-trust.md` 因自始即为英文,无韩文副本)。
