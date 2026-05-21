<p align="center">
  <img src="https://keiailab.com/assets/logo.svg" alt="keiailab" width="120"/>
</p>

<p align="center">
  <a href="family.md">English</a> |
  <a href="family.ko.md">한국어</a> |
  <a href="family.ja.md">日本語</a> |
  <b>中文</b>
</p>

# keiailab operator family (operator 家族)

> 构建在共享基础 (`operator-commons` Go 库 + Helm partial + Apache-2.0 stack) 之上的 4 个姊妹 Kubernetes Operator。

本页面在 `valkey-operator` 仓库中编写,是整个 family 的 canonical 交叉引用页面。

## Family 概览

| 项目 | 数据库 | 状态 | 仓库 |
|---|---|---|---|
| **`postgres-operator`** | PostgreSQL 18+ | active | https://github.com/keiailab/postgres-operator |
| **`mongodb-operator`** | MongoDB 7.0+ | active | https://github.com/keiailab/mongodb-operator |
| **`valkey-operator`** | Valkey 8.0+ (Redis fork, BSD-3) | active | https://github.com/keiailab/valkey-operator |
| **`operator-commons`** | 共享 Go 库 | v0.7.0 | https://github.com/keiailab/operator-commons |

## 共享内容 (What we share)

四个项目都收敛于相同的运维原语 (operational primitives):

- **Apache-2.0** 一致 — 无 SSPL,SaaS 表面无 copyleft
- **`operator-commons`** 共享 Go 库 (v0.7.0+) — finalizer、label、status sugar、security context builder、NetworkPolicy / ServiceMonitor partial
- **Helm chart skeleton** — RFC-0027 `default` falsy-toggle 防护、RFC-0026 component-keyed values、cycle 26 hardening 6 marker (priorityClassName / lifecycle / SA / minReadySeconds / automount / revisionHistoryLimit)
- **OLM bundle parity** — scorecard v1alpha3 6-test matrix
- **i18n** — README + 11 个 canonical 文档以 英语 / 한국어 / 日本語 / 中文 提供 (cleanup supercycle 2026-05-21 的 Wave 4)

## 不做的事 (What we do NOT do)

- ❌ **嵌入或封装 third-party operator** — license-clean,无 copyleft 义务
- ❌ **使用 GitHub Actions 作为 release gate** — 本地 4-layer hook 系统 (见 RFC-0002)
- ❌ **基于时间的 roadmap deadline** — 功能 checklist + 完成度百分比
- ❌ **供应商锁定容器镜像** — 仅使用 keiailab-published Apache-2.0 镜像

## 从哪里开始

| 任务 | 入口 |
|---|---|
| 在 Kubernetes 上部署 `valkey-operator` | [README.md](../README.md) Quickstart 章节 |
| 阅读架构 (architecture) | [ARCHITECTURE.md](../ARCHITECTURE.md) |
| 提交 issue 或功能请求 | https://github.com/keiailab/valkey-operator/issues |
| 讨论设计或 roadmap | https://github.com/keiailab/valkey-operator/discussions |
| 贡献代码 | [CONTRIBUTING.md](../.github/CONTRIBUTING.md) |
| 报告安全问题 | [SECURITY.md](../.github/SECURITY.md) |
| 学习品牌 (brand) / voice | [BRANDING.md](../BRANDING.md) |
| 跟踪 adopter / 使用者 | [ADOPTERS.md](../ADOPTERS.md) |
| 查找 maintainer | [MAINTAINERS.md](../MAINTAINERS.md) |
| 审查治理 (governance) 模型 | [GOVERNANCE.md](../.github/GOVERNANCE.md) |
| 查看即将的工作 | [ROADMAP.md](../ROADMAP.md) |

## Family 间兼容性 (operator-commons)

三个数据库 operator 都以匹配的版本 (当前 `v0.7.0+`) import `github.com/keiailab/operator-commons`:

```go
import (
    "github.com/keiailab/operator-commons/pkg/version"
    "github.com/keiailab/operator-commons/pkg/security"
    "github.com/keiailab/operator-commons/pkg/labels"
    "github.com/keiailab/operator-commons/pkg/monitoring"
    "github.com/keiailab/operator-commons/pkg/finalizer"
    "github.com/keiailab/operator-commons/pkg/status"
)
```

`operator-commons` 的 breaking change 要求三个数据库 operator 同步 bump — 在 supercycle Wave 5 的 `make cross-validation` target 中验证。

## i18n

本页面 (及所有 canonical 项目文档) 提供四种语言:

- [English](family.md) (canonical, 正本)
- [한국어](family.ko.md)
- [日本語](family.ja.md)
- **中文** (本文件)

对于技术内容,英语版本为正本;本地化版本以 native 表达反映相同的决策。

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a>
</p>

<p align="center">
  © 2026 keiailab · <a href="../LICENSE">Apache-2.0</a> · <a href="https://keiailab.com">keiailab.com</a>
</p>
