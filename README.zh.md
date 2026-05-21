# valkey-operator (中文)

> [English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) (placeholder) | **中文** (placeholder)
>
> English README (canonical): [README.md](README.md)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org/)
[![Valkey](https://img.shields.io/badge/Valkey-8.0+-FF4438?logo=redis)](https://valkey.io/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes)](https://kubernetes.io/)

基于 Kubebuilder 的 Kubernetes operator,管理 [Valkey](https://valkey.io/)
(Redis 的 BSD-3 fork)。单一 controller 通过统一 CRD 表面管理三种运行拓扑。

> **状态**: `[~]` 部分实现 (placeholder) — RFC-0025 §1.2 复选框含义。
> 经 native reviewer 质量验证后升级到 `[x]` 完成状态 candidate。

## CRD 概览

| CRD | 用途 | 拓扑 |
|---|---|---|
| `Valkey` | 单实例,或 1 primary + N replica | Standalone / Replication |
| `ValkeyCluster` | 分片 Valkey Cluster (16384 slot) | 3+ shard × (1 primary + 0–5 replica) |
| `ValkeyBackup` | 一次性 RDB 或 AOF 备份 | PVC (`<backup>-backup`), 外部存储可选 |
| `ValkeyBackupTarget` | S3 兼容外部存储抽象 | Backup 与 Restore 共享 (ADR-0016) |
| `ValkeyRestore` | 将 RDB 还原至 Valkey 或 ValkeyCluster 实例 | Init Container 模式 (ADR-0015) |

operator 通过 spec-drift detection 调和 `StatefulSet`、`ConfigMap`、`Secret`、
`Service` (headless + ClusterIP)、`PodDisruptionBudget`、`NetworkPolicy`、
`cert-manager` `Certificate`、Prometheus `ServiceMonitor`。

## Quickstart / 安装 / CRD 示例 / 运维

详见 [English README](README.md)。本 placeholder 将在 native reviewer
翻译完成后扩展。

- Quickstart (kind): [README.md → Quickstart](README.md#quickstart-kind)
- CRD examples: [README.md → CRD examples](README.md)
- 已知限制: [README.md → Known operational issues](README.md)

## Contributing

参见 [CONTRIBUTING.md](CONTRIBUTING.md)。欢迎外部贡献;对非平凡变更请先开
issue,以便在编写代码前对齐 API 表面。

## 报告漏洞

**不要**开 public issue。请使用 [SECURITY.md](SECURITY.md) 中的私有渠道 —
GitHub Security Advisory 或 `security@keiailab.com` (PGP key 见
`artifacthub-repo.yml`)。

## License

Copyright 2026 Keiailab.

根据 Apache License, Version 2.0 许可
(<http://www.apache.org/licenses/LICENSE-2.0>)。按 `AS IS` 基础分发,不附
任何明示或暗示的担保或条件。完整文本见 [LICENSE](LICENSE) 文件。

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a> ·
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/forgewise">forgewise</a>
</p>

<p align="center">© 2026 keiailab · Apache-2.0 · <a href="https://keiailab.com">keiailab.com</a></p>
