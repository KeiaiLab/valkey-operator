<p align="center">
  <a href="GOVERNANCE.md">English</a> |
  <a href="GOVERNANCE.ko.md">한국어</a> |
  <a href="GOVERNANCE.ja.md">日本語</a> |
  <b>中文</b>
</p>

# 治理 (Governance)

> English version: [GOVERNANCE.md](.github/GOVERNANCE.md)

本文档定义了 `keiailab/valkey-operator` 中决策的制定方式。

## 原则

1. **开放 (Openness)。** 所有决策都发生在公共渠道上 — GitHub issue、pull request 与 RFC。
2. **惰性共识 (Lazy consensus)。** 日常变更只要没有人反对即可 ship。
3. **显式共识 (Explicit consensus)。** 架构变更、CRD 变更、安全模型变更与许可证变更需要先经过 RFC,然后获得 **维护者 2/3 supermajority** 的同意。一般 RFC (单一组件、工具采用、策略强化) 需要 **simple majority** (>50%)。对本 `GOVERNANCE.md` 的修改始终需要 2/3 supermajority。
4. **共同责任 (Shared responsibility)。** 维护者共同对代码质量、用户安全和社区健康负责。

## 决策分类

### 例行 (lazy consensus)

- Bug 修复、文档改进、新增测试、minor/patch 依赖 bump、不变更公共 API 的重构
- 流程: PR → 至少一位维护者 LGTM → 合并
- 评论窗口: 无。本地 gate 通过后,PR 可以立即合并 (根据 RFC-0002,我们不依赖 GitHub Actions 作为 gate;pre-commit / pre-push hook 加 Makefile 是 enforcement 点)。

### 中等 (显式共识)

- 新增 CRD 字段、新增 reconciler、主要依赖升级、公共 API 变更
- 流程: 提出变更的 issue → 7 天评论窗口 → 维护者过半 LGTM → 合并
- 一条反对意见即触发维护者会议进行讨论。

### 架构 (需要 RFC)

- 引入新组件、变更安全模型、变更许可证、破坏向后兼容
- 流程:
  1. 在 `docs/kb/adr/NNNN-title.md` 提交 ADR 或 RFC
  2. 14 天评论窗口
  3. 维护者 2/3 批准
  4. 将 ADR/RFC 的 `Status` 从 `Draft` 改为 `Accepted`,然后开启实现 PR

## 安全决策

CVE 报告与对 secrets / auth 模型的变更首先通过 [SECURITY.md](.github/SECURITY.md) 中的私有渠道处理。公开共识在 patch release 发布后进行。

## 发布决策

单一维护者可以在 lazy consensus 下切出 release branch 或 bump 版本号。创建新的 LTS 线或宣布现有 LTS 线 End-of-Life 始终需要显式共识。

## 变更历史

| 日期 | 变更 | Refs |
|---|---|---|
| 2026-05-07 | 文档创建 — 3 个仓库 (mongodb / postgresql / valkey) 治理资产对齐 | INC-2026-05-07 |
| 2026-05-12 | 英文版成为 canonical;韩文版作为 `GOVERNANCE.ko.md` 保留 | i18n PR-K |

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a>
</p>

<p align="center">
  © 2026 keiailab · <a href="LICENSE">Apache-2.0</a> · <a href="https://keiailab.com">keiailab.com</a>
</p>
