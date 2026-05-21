<p align="center">
  <a href="ADOPTERS.md">English</a> |
  <a href="ADOPTERS.ko.md">한국어</a> |
  <a href="ADOPTERS.ja.md">日本語</a> |
  <b>中文</b>
</p>

# valkey-operator 的采用者 (Adopters)

> English version: [ADOPTERS.md](../../ADOPTERS.md)

本文档是 `keiailab/valkey-operator` 在生产环境运行或正在评估的组织和项目的 **公开** 名单。欢迎自助注册 — 提交 PR 追加一行即可。

## 生产用户 (Production users)

以生产级 SLA 在生产环境运行 `valkey-operator` 的组织。

| 用户 | 组件 | 使用方式 | 初始版本 | 当前版本 | 列入日期 |
|---|---|---|---|---|---|
| **内部生产集群** ([keiailab](https://github.com/keiailab)) | Valkey 9.0.4 (Standalone + sharded Cluster 3×1) | 内部生产工作负载的缓存与 pub/sub 层。6-pod ValkeyCluster, `cluster_state=ok`, ServiceMonitor + alert-rules.yaml + PodSecurity restricted。 | v1.0.0 | v1.0.3 | 2026-05-07 |

## 评估者 (Evaluators)

概念验证 (PoC)、评估中,以及 Bitnami redis-cluster 迁移候选。

| 用户 | 阶段 | 备注 |
|---|---|---|
| _欢迎自助注册_ | — | 提交 PR 追加一行即可。请注意 ValkeyRestore 文档中所述的 Redis 8.2 → Valkey 9.0 RDB 兼容性限制。 |

## 如何添加自己

提交 PR 在上述表格之一追加一行:

```markdown
| **<组织或项目>** ([profile](<URL>)) | <组件 + 拓扑 (topology)> | <使用方式> | <初始版本> | <当前版本> | <YYYY-MM-DD> |
```

如果您希望以匿名方式被列出,请通过 [SECURITY.md](../../../.github/SECURITY.md) 中的安全联系方式与我们联系,维护者将代为登记一行匿名化的组织信息。

## CNCF Sandbox 引用

本名单也作为 CNCF graduation 标准 "≥ 1 public adopter" 的公开依据。

## 从 Bitnami redis-cluster 迁移

如果您正在运营 Bitnami `redis-cluster` (Redis 7.x / 8.x) 并在评估 Valkey,请参阅 `ROADMAP.md` → **Phase B (RDB 兼容性与替代迁移路径)**。部分 Redis 8.2.x 的 RDB 文件无法直接恢复到 Valkey 9.0.4;在这种情况下 `ValkeyRestore` 会 fail fast,运维方不会在静默错误 (silent error) 中无限等待。

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
