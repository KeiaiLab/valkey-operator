<p align="center">
  <a href="SUPPORT.md">English</a> |
  <a href="SUPPORT.ko.md">한국어</a> |
  <a href="SUPPORT.ja.md">日本語</a> |
  <b>中文</b>
</p>

# 支持

> 中文用户: 本文档的所有渠道均欢迎使用英文或中文沟通。

感谢您使用 `valkey-operator`。本页面将告知您如何获取帮助。

## 确定您的需求

| 情况 | 前往何处 |
|---|---|
| **您认为发现了一个安全漏洞 (security vulnerability)。** | **请勿在公开 issue 中提交。** 请参阅 [SECURITY.md](.github/SECURITY.md) — 使用 GitHub Security Advisory 或 `security@keiailab.com` (PGP 签名)。 |
| 您有 "这是否应当像 X 这样工作?" 或 "如何配置 Y?" 之类的问题。 | [GitHub Discussions](https://github.com/keiailab/valkey-operator/discussions)。可搜索,且会被未来的运维者索引。 |
| 您发现了 bug — 行为与文档不一致。 | 请使用 **Bug report** 模板 [开启 issue](https://github.com/keiailab/valkey-operator/issues/new/choose)。 |
| 您希望添加新功能或变更行为。 | 请使用 **Feature request** 模板 [开启 issue](https://github.com/keiailab/valkey-operator/issues/new/choose)。请先查阅 [ROADMAP.md](ROADMAP.md) 确认是否已在规划中。 |
| 您有 "这应该放进 FAQ" 的问题。 | 请使用 **Question** 模板 [开启 issue](https://github.com/keiailab/valkey-operator/issues/new/choose)。 |
| 您遇到 Prometheus 告警,需要 MTTR (Mean Time To Recovery) 步骤。 | [`docs/operations/runbook.md`](docs/operations/runbook.md) §9 (每条告警的 `runbook_url` 注解都会指向此处)。 |
| 您看到异常行为但没有告警。 | [`docs/operations/troubleshooting.md`](docs/operations/troubleshooting.md) — 症状 → 原因 → 诊断 → 处置流程图。 |
| 您希望贡献代码或文档。 | 请参阅 [CONTRIBUTING.md](.github/CONTRIBUTING.md)。 |

## 开启 issue 之前请

1. 搜索 [现有 issue](https://github.com/keiailab/valkey-operator/issues?q=is%3Aissue) 和 [Discussions](https://github.com/keiailab/valkey-operator/discussions) — 您的问题可能已有答案。
2. 尝试 [故障排查流程图](docs/operations/troubleshooting.md)。
3. 在报告中准备好以下信息:
   - `valkey-operator` 版本 (`kubectl get deploy -n valkey-operator-system -o jsonpath='{.items[0].spec.template.spec.containers[0].image}'`)
   - Kubernetes 版本 (`kubectl version`)
   - Helm chart 版本 (`helm list -A | grep valkey-operator`)
   - 您能提供的最小复现案例
   - `kubectl describe <Valkey|ValkeyCluster> <name>` 的输出

## 响应预期

本项目是基于尽力而为 (best-effort) 时间维护的开源项目。决策与评审流程详见 [GOVERNANCE.md](.github/GOVERNANCE.md)。我们通常在数个工作日内回复 issue;安全报告按照 [SECURITY.md](.github/SECURITY.md) 中的 SLA 处理 (72 小时内首次 ack,7 天内 severity triage)。

如果您需要付费支持关系或严格 SLA,请通过 `security@keiailab.com` 联系我们,我们可以讨论可选方案。

## 商业供应商

目前 `valkey-operator` 不背书任何付费支持供应商 (commercial vendor)。如有变更,将在此处添加该供应商的服务条款以及对应的 upstream 功能。

## 行为准则 (Code of Conduct)

以上所有渠道均受 [Code of Conduct](.github/CODE_OF_CONDUCT.md) 管辖。参与前请务必阅读。

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
