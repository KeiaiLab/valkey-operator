# Artifact Hub Trust Badge 运维 — valkey-operator (简体中文)

> English: [artifacthub-trust.md](artifacthub-trust.md) — canonical / 正本

如何为 Artifact Hub 上的 `valkey-operator` 包拿到并维持 `Signed` 与
`Official` 两枚徽章。本文将**可复现的核对步骤与申请流程**沉淀为
SSOT,避免每次发版前重新摸索。

## 当前状态

2026-05-12 核对:

```bash
curl -fsSL https://artifacthub.io/api/v1/packages/helm/keiailab-valkey-operator/valkey-operator \
  | jq '{version, signed, official, repository:{verified_publisher:.repository.verified_publisher, official:.repository.official}}'
```

观察到的字段:

- `repository.verified_publisher=true`
- `signed=false`
- `repository.official=false`
- `https://keiailab.github.io/valkey-operator/valkey-operator-1.0.10.tgz.prov` 返回 404

仓库所有权认证**已经完成**。剩下要做的事情有两条线: **发布 Helm
provenance** 与 **走完 Artifact Hub 的 official 审核**。

## 点亮 Signed 徽章

只有当 chart 归档旁边**同时**存在 `<chart>-<version>.tgz.prov` 文件时,
Artifact Hub 才会在前端展示 Helm `Signed` 状态。该 `.tgz.prov` 文件由
`helm package --sign` 生成。

标准发版命令:

```bash
make release VERSION=vX.Y.Z
```

`Makefile` 默认强制 `HELM_SIGN=1`,因此只有当下列前置条件**全部**满足时
发版才会通过:

- `~/.gnupg/secring.gpg`
- `HELM_GPG_KEY=Keiailab Helm`
- `HELM_GPG_FINGERPRINT=F1A6893583E632A757FF6767F3CC8C6AEC9CEB08`

在一台新的开发机上准备私钥:

```bash
gpg --import <keiailab-helm-private-key.asc>
gpg --export-secret-keys > ~/.gnupg/secring.gpg
make helm-signing-preflight
```

发版完成后的核验:

```bash
bash scripts/release-smoke-test.sh vX.Y.Z
```

这一道 smoke test 会**同时**核验三个面: (a) GitHub Releases 上的
`.tgz.prov` 资产、(b) `gh-pages` 分支上的 `.tgz.prov` 文件、(c) 使用
`artifacthub-repo.yml` 中登记的公共签名密钥跑出来的 `helm verify` 结果。

## 点亮 Official 徽章

`Official` 无法仅通过修改仓库里的文件来开启 — 它是 Artifact Hub
**外部审核**给出的状态,表示"publisher 直接拥有该 package 所聚焦的
软件本身"。

已经满足的前置条件:

- Artifact Hub repository ID:
  `16085dd0-0f19-4c6b-ab90-bd97105bdf42`
- Verified publisher: `true`
- Chart README: `charts/valkey-operator/README.md`
- 仓库元数据已对外提供:
  `https://keiailab.github.io/valkey-operator/artifacthub-repo.yml`

仍需推进的步骤:

1. 由 Artifact Hub 的 publisher 或 organization 成员在
   `artifacthub/hub` 仓库提交一个 official status 申请 issue。
2. 申请的是**仓库级别** (repository-level) 的 official status。
3. 在申请正文中原样附上下面这些事实。

```text
Repository name: keiailab-valkey-operator
Package name: valkey-operator
Repository URL: https://keiailab.github.io/valkey-operator
Package URL: https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
Publisher owns the software: yes, keiailab publishes and maintains valkey-operator itself.
Verified publisher: true
Documentation: charts/valkey-operator/README.md
```

在 Artifact Hub 审核结论下来之前,无论是修改仓库代码,还是重新发布
`gh-pages`,都没有办法把 `Official` 徽章翻成 `true` — 这一段属于
**纯粹等待外部裁决**的窗口。
