<p align="center">
  <a href="SECURITY.md">English</a> |
  <a href="SECURITY.ko.md">한국어</a> |
  <a href="SECURITY.ja.md">日本語</a> |
  <b>中文</b>
</p>

# 安全策略 (Security Policy)

> 英文版: [SECURITY.md](.github/SECURITY.md) — canonical / 正本

## 报告漏洞 (vulnerability)

**请勿通过公开 issue 提交安全报告 (security report)。** 在补丁 (patch) 发布之前公开披露,会让所有用户面临风险。

### 私下报告渠道

请任选其一:

1. **GitHub Security Advisory** (推荐):
   <https://github.com/keiailab/valkey-operator/security/advisories/new>
2. **邮件**: `security@keiailab.com` (PGP 可选):
   - PGP fingerprint:
     `89A4 0947 6828 CB99 2338  C378 651E 51AF 520B CB78`
   - Public key: 位于 `gh-pages` 分支的 `artifacthub-repo.yml`,或
     <https://keiailab.github.io/valkey-operator/artifacthub-repo.yml>
   - 同一个 key 也被 `mongodb-operator` 和 `postgres-operator` 共用
     (3-repo 统一密钥)。

### 报告中应包含的内容

- 受影响的版本 (release tag 或 commit SHA)
- 复现步骤 (尽可能小的可稳定复现示例)
- 影响评估 (impact assessment) (如可能,请附 CVSS 自评分)
- 报告者身份 — 如希望被署名致谢 (credit),请注明

## 响应 SLA

| 阶段 | 目标 |
|---|---|
| 初次确认 (acknowledgement) | 72 小时以内 |
| 严重程度 (severity) 分级 | 7 天以内 |
| 补丁发布 (patch release) | 按严重程度 (Critical: 14 天、High: 30 天、Medium: 60 天) |
| 公开披露 (public disclosure) | 补丁发布后 14 天 (协调披露 (coordinated disclosure) 可按需调整) |

## 支持的版本 (Supported versions)

| Version | Supported |
|---------|-----------|
| 0.x (alpha) | ✅ 仅最新 minor 版本 |
| 1.0+ (stable) | TBD — 首个 stable release 之后更新 |

本项目目前处于 `v1alpha1` 阶段,**不提供向后兼容性 (backward compatibility) 保证**;安全修复仅会随最新 release 发布。

## 运维侧安全建议 (Operational security recommendations)

运行 `valkey-operator` 时:

1. **强制启用 TLS**: 设置 `Spec.TLS.Enabled=true` (使用 cert-manager
   或用户提供的 `CustomCert`)。详见 ADR-0010 和 ADR-0014。
2. **认证 (auth) 实际上始终启用**: 根据 ADR-0013,无论
   `Spec.Auth.Enabled` 取值如何,operator 都会自动生成 32 字节随机密码。
3. **NetworkPolicy**: 设置 `Spec.NetworkPolicy.Enabled=true` 以限制
   pod-to-pod ingress。请在真正强制执行 NetworkPolicy 的 CNI
   (Calico、Cilium) 上进行验证。
4. **Pod Security Standard: restricted**: 在 namespace 上应用
   `pod-security.kubernetes.io/enforce=restricted`。
5. **凭据 (credentials) 单独存放于 Secret**: `ValkeyBackupTarget` 上的
   S3 credentials 应放入由 RBAC 把守的专用 `Secret` 中
   (ADR-0016)。
6. **备份 (backup) 优先使用外部存储**: 推荐使用
   `Destination.Type=TargetRef` 配合外部 S3。仅依赖 PVC 的备份在
   集群本身丢失时会一同丢失。
7. **验证容器镜像 (container image)**: operator image 仅基于通过
   Sonatype 和 Context7 审核的依赖构建 (ADR-0022)。如需构建自定义
   变体,请对结果运行 `trivy` 或 `grype` 扫描。

## 依赖安全 (Dependency security)

每个引入依赖的 ADR 都会引用相应的 **Sonatype Trust Score** 和
**Context7** 验证 (标准示例见 `docs/kb/adr/0022-*.md`)。

Dependabot 和 Renovate 的自动更新 PR 会被优先 review。

## 验证 release 产物 (signed releases — v1.0.13+)

从 **v1.0.13** 起,所有发布的 container image、Helm chart 和 SPDX SBOM 均通过 **Sigstore cosign** keyless OIDC 进行签名,并附带 **SLSA-3 provenance attestation** (ADR-0045、ADR-0046)。v1.0.13 之前的 release 未签名;对这些 release 运行下面的验证命令会如预期那样失败。

### 验证 container image

```bash
COSIGN_EXPERIMENTAL=1 cosign verify \
  --certificate-identity-regexp '^https://github\.com/keiailab/valkey-operator/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/keiailab/valkey-operator:<version>
```

### 验证 image 的 SLSA-3 provenance

```bash
slsa-verifier verify-image \
  --source-uri github.com/keiailab/valkey-operator \
  --source-tag v<version> \
  ghcr.io/keiailab/valkey-operator:<version>
```

### 验证 Helm chart

从 GitHub Release 页面下载 `valkey-operator-<version>.tgz`、`.tgz.sig`
以及 `.tgz.pem`,然后执行:

```bash
cosign verify-blob \
  --certificate   valkey-operator-<version>.tgz.pem \
  --signature     valkey-operator-<version>.tgz.sig \
  --certificate-identity-regexp '^https://github\.com/keiailab/valkey-operator/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  valkey-operator-<version>.tgz
```

### 验证 SBOM

对 `.spdx.json` / `.sig` / `.pem` 三件套使用相同的 `cosign verify-blob` 模式。SBOM 签名会把 bill-of-materials 固定绑定到生成该 image 的构建上。

### 成功验证意味着什么

- 该产物是由本仓库内的 GitHub Actions workflow 生成的 (证书
  identity 证明了 OIDC subject)。
- 自签名之后,该产物未被修改 (Sigstore Rekor 透明日志 (transparency log)
  条目具备防篡改性)。
- 对于 container image,SLSA-3 attestation 进一步证明构建是在一个隔离的
  托管 GitHub runner 上,基于文档化的 `release.yml` workflow 完成的。

## 已知限制 (Known limitations)

- 英文: [README.md → "Known limitations"](README.md#known-limitations)
- 韩文: [README.ko.md → "잠재적 운영 이슈"](README.ko.md#잠재적-운영-이슈-현재-알려진-한계)
- 另见: GitHub Issues 中带有 `security` 标签的 issue。

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
