# Incident 知识库 — INDEX (简体中文)

> English: [INDEX.md](INDEX.md) — canonical / 正本

本目录以无指责 (blameless) 的 postmortem-lite 格式保存 valkey-operator 的
运维事件 (incident)。全局标准: `~/Documents/ai-dev/standards/incident-kb.md`。

| ID | 标题 | Severity | Detected | Resolved |
|---|---|---|---|---|
| [INC-0001](INC-0001-cluster-fail-bootstrap-skip.md) | ValkeyCluster 在 cluster_state:fail 状态下未重新执行 bootstrap | SEV-2 | 2026-05-09 14:27 KST | 2026-05-10 09:18 KST |

## 撰写指南

- 格式: 全局 `standards/incident-kb.md §3` (Postmortem-lite)。
- 触发条件: 运维故障 / 安全事件 / 调试超过 30 分钟的非显而易见 bug / 模式型发现 (3 次重发)。
- 无指责文化: 关注 *系统* 在哪里允许了失败。Action Items 优先聚焦 *系统层面变更*。
- KB 新鲜度: 30 天未更新的 INC 占比超过 30% 时触发提醒 (全局 §6)。
