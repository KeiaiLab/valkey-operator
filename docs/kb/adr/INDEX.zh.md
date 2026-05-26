# ADR 索引 — valkey-operator (简体中文)

> English: [INDEX.md](INDEX.md) — canonical / 正本

本目录以 Nygard 五段式格式保存 valkey-operator 的不可回退架构决策
(architecture decisions),让决策背后的 *理由* 比代码活得更久。

路径标准: `<repo>/docs/kb/adr/` (全局 `standards/adr.md §1`)。

## 活跃 ADR (按 ID 升序)

| ID | 标题 | 状态 | 日期 |
|----|------|------|------|
| [0001](0001-operator-side-defaulting.md) | Operator 侧默认值填充 (相对于 admission webhook) | 被 0009 取代 | 2026-05-05 |
| [0002](0002-deferred-events-api-migration.md) | 推迟迁移到 client-go events API | 已采纳 | 2026-05-05 |
| [0003](0003-tls-insecure-skip-verify-temporary.md) | 在 cert-manager CA 接线完成前临时使用 InsecureSkipVerify | 已采纳 | 2026-05-05 |
| [0004](0004-shardstatus-spec-derived.md) | ShardStatus 由 Spec 推导 (而非 CLUSTER NODES) | 被 0007 取代 | 2026-05-05 |
| [0005](0005-graceful-cluster-teardown.md) | 通过尽力而为 (best-effort) 的 CLUSTER FORGET 实现优雅集群拆除 | 已采纳 | 2026-05-05 |
| [0006](0006-scale-policy-deliberate.md) | ScalePolicy.Deliberate=false 作为默认值 | 已采纳 | 2026-05-05 |
| [0007](0007-shardstatus-from-nodes.md) | ShardStatus 改由 CLUSTER NODES 推导 (取代 0004) | 已采纳 | 2026-05-05 |
| [0008](0008-tls-ca-bundle-loading.md) | TLS RootCAs 从 Spec.TLS.CustomCert.SecretName 加载 | 已采纳 | 2026-05-05 |
| [0009](0009-webhook-validation-defaulting.md) | Validating + Mutating Webhook (取代 0001) | 已采纳 | 2026-05-05 |
| [0010](0010-cert-manager-auto-discovery.md) | cert-manager Certificate 自动发现 | 已采纳 | 2026-05-05 |
| [0011](0011-required-fields-webhook-defaulting.md) | Required 字段由 mutating webhook 直接填充默认值 | 已采纳 | 2026-05-05 |
| [0012](0012-cluster-meet-requires-ip.md) | CLUSTER MEET 不支持 hostname → 先解析 DNS 再使用 IP | 已采纳 | 2026-05-05 |
| [0013](0013-auth-always-enabled.md) | Auth.Enabled 实际上始终开启 (方案 A) | 已采纳 | 2026-05-05 |
| [0014](0014-tls-volume-mount-and-port-routing.md) | TLS Secret 挂载到 STS + operator 通过 6380 (TLS 端口) 走控制面 | 已采纳 | 2026-05-05 |
| [0015](0015-valkeyrestore-init-container-pattern.md) | ValkeyRestore — 基于 Init Container 加载 RDB + 重启 STS | 已采纳 | 2026-05-06 |
| [0016](0016-valkeybackuptarget-crd-external-storage.md) | ValkeyBackupTarget CRD — S3 兼容的外部存储抽象 | 已采纳 | 2026-05-06 |
| [0017](0017-replication-failover-replica-with-largest-offset.md) | Replication 模式 Failover — 选取 master_repl_offset 最大的 replica | 已采纳 | 2026-05-06 |
| [0018](0018-cluster-auto-resharding.md) | Cluster 自动再分片 (SlotMigrationPolicy Auto 启用,PR-B8.1 ADR 正式落档 — controller 实现在 PR-B8.2 后续) | 已采纳 | 2026-05-09 |
| 0019 | *Reserved (用途未定)*。 | Reserved | — |
| 0020 | *Reserved (用途未定)*。 | Reserved | — |
| [0021](0021-helm-chart-kubebuilder-helm-plugin.md) | Helm Chart — 采用 kubebuilder helm/v2-alpha plugin | 被 0024 取代 | 2026-05-06 |
| [0022](0022-s3-client-library-minio-go.md) | S3 客户端库 — 采用 minio-go v7 (sonatype + context7 验证) | 已采纳 | 2026-05-06 |
| [0023](0023-operator-binary-subcommand-upload-download.md) | Operator binary 的 upload/download 子命令 — 镜像统一打包 | 已采纳 | 2026-05-06 |
| [0024](0024-helm-chart-manual-pattern-artifacthub.md) | Helm Chart — 手写 + ArtifactHub publish 模式 (3 仓统一,取代 0021) | 已采纳 | 2026-05-06 |
| [0025](0025-otel-tracer-provider-optional.md) | OTEL Tracer Provider — 可选,OTLP gRPC Exporter | 已采纳 | 2026-05-06 |
| [0026](0026-conversion-webhook-deferred-until-v1alpha1-stable.md) | Conversion Webhook — 待 v1alpha1 Stable 后再引入 v1beta1 (推迟) | 已采纳 | 2026-05-06 |
| [0027](0027-hpa-replication-mode-only-deferred.md) | HPA — 仅 Replication 模式 + Operator 托管 (落地于 2026-05-10) | 已采纳 | 2026-05-10 |
| [0028](0028-helm-kustomize-parity-invariant.md) | Helm vs Kustomize 对等不变式 — 阻断 5 类 sibling 静默失败 | 已采纳 | 2026-05-06 |
| [0029](0029-gitops-deploy-overlay.md) | 引入 GitOps deploy 覆盖层 (3 仓一致) | 已采纳 | 2026-05-06 |
| [0030](0030-rfc-0017-tooling-unification-adoption.md) | 采纳 RFC-0017 operator tooling 统一 (新增 .golangci.yml + Makefile validate + HEALTHCHECK) | 已提案 | 2026-05-09 |
| [0031](0031-auth-rotation-policy.md) | Password Rotation 反映路径 (AuthSpec.RotationPolicy enum,v1alpha2 PR-B7.1 type module — controller 分支在 PR-B7.2 后续) | 已采纳 | 2026-05-09 |
| [0032](0032-custom-modules-init-container.md) | Valkey 自定义模块 — init container 挂载 + 仅官方预设 (v1alpha2 PR-C6.1 type module — controller 分支在 PR-C6.2 后续) | 已采纳 | 2026-05-09 |
| [0033](0033-supply-chain-cosign-slsa.md) | 供应链 — cosign 签名 + SLSA L2 in-toto attestation (Plan §2 D5,PR-A4) | 已采纳 | 2026-05-09 |
| [0034](0034-auth-optional-v1alpha2.md) | Auth 改为可选 + 新增 v1alpha2 (取代 ADR-0013,PR-A2.1 type module) | 已采纳 | 2026-05-09 |
| [0035](0035-networkpolicy-autocreate-optional.md) | NetworkPolicy.AutoCreate 可选开关 (v1alpha2,PR-A3.1 type module — controller 分支在 PR-A3.1.2 后续) | 已采纳 | 2026-05-09 |
| [0036](0036-pod-security-restricted-optional.md) | PodSecurity Restricted 可选开关 (v1alpha2 PodSpec.PodSecurityRestricted,PR-A3.2 type module — controller 分支在 PR-A3.2.2 后续) | 已采纳 | 2026-05-09 |
| [0037](0037-operatorhub-bundle-scaffold.md) | OperatorHub.io bundle 脚手架 — operator-sdk v1.42 + kustomize,5 CRD owned,Makefile bundle/bundle-build target (PR-B9 首版,alm-examples + community-operators PR 后续) | 已采纳 | 2026-05-10 |
| [0038](0038-rfc-0018-pkg-finalizer-migration.md) | 采纳 RFC-0018 — pkg/finalizer 迁移 (controllerutil → commons,5 个 controller,PR-A6 首版,status 单独处理) | 已采纳 | 2026-05-09 |
| [0039](0039-cluster-self-heal-post-init.md) | ValkeyCluster post-init 自愈 — INC-0001 永久修复,ClusterInitialized=true && state!=ok 时重新调用 ensureClusterMeet | 已采纳 | 2026-05-10 |
| [0040](0040-helm-chart-vs-operator-adoption.md) | Helm chart vs Operator 选型策略 (外部 chart / 外部 chart / valkey-operator 决策矩阵 + 5 gap) | 已采纳 | 2026-05-10 |
| [0041](0041-chaos-engineering-chaos-mesh.md) | 混沌工程 — 采用 chaos-mesh (4 个 e2e 场景,ADR-0040 §gap #4) | 已采纳 | 2026-05-10 |
| [0042](archive/0042-commercial-parity-series-closure.md) | Commercial Parity 系列汇总 — archive (保留历史,deprecation 理由见外部 chart 正文) | 已废弃 | 2026-05-10 |
| [0044](0044-artifacthub-signed-official-trust-badges.md) | Artifact Hub 信任徽章 — Signed 强制,Official 走外部评审 | 已采纳 | 2026-05-12 |
| [0045](0045-restore-github-actions-for-oss-ci.md) | 为 OSS CI 恢复 GitHub Actions workflow (相对 RFC-0002 的有界例外) | 已采纳 | 2026-05-12 |
| [0046](0046-slsa3-cosign-supply-chain.md) | 为发布产物 (image + chart + SBOM) 提供 SLSA-3 provenance + cosign keyless 签名 | 已采纳 | 2026-05-12 |
| [0047](0047-community-operators-sync-automation.md) | community-operators 同步自动化 (RFC 0002 例外 ③ 扩展) | 已采纳 | 2026-05-14 |
| [0048](0048-gha-retention-for-public-oss.md) | GitHub Actions 保留策略 — 公开 OSS Operator 外部信任门 (按 operator 家族权衡) | 已采纳 | 2026-05-21 |
| [0050](0050-audit-augmentation.md) | Audit 增强 — postgres 模式 cp (lefthook 3 件 + helm-publish + UPGRADING,audit P1-11/12/13 + OP-2 + OP-10 ✅) | 已采纳 | 2026-05-21 |
| [0051](0051-multi-arch-build-enablement.md) | 多架构构建按需启用 — `PLATFORMS` env 覆盖 (默认仍 amd64,为引入 ARM 节点 + 外部 GA 准备,RFC-0048 姊妹) — 由重复的 0043 重新编号 | 已提案 | 2026-05-19 |
| [0052](0052-v3x-stable-baseline.md) | v3.x-stable 基线认定 (audit ❌ 0 满足,CLAUDE.md §7 v3.x-stable 条件) | 已采纳 | 2026-05-21 |
| [0053](0053-root-md-documentation-policy.md) | 根目录 `.md` 文档策略 + 工具依赖例外 (PR-D 系列的正当性) | 已采纳 | 2026-05-21 |

## 撰写指南

- 格式: Nygard 五段式 (Context / Decision / Consequences / Alternatives Considered / Status)。
- 位置: `docs/kb/adr/NNNN-<英文 kebab-case slug>.md` (全局标准)。
- 编号: 4 位 0 填充,一旦使用的编号 *禁止再用*。Reserved 槽位在 INDEX 中显式标注。
- 本 INDEX.md 在新增 ADR 时 *必须手动同步* — `standards/enforcement.md §2.1`。
- 排序: 按 ID 升序 — 包含 Reserved 项 (让 gap 可见)。

## Reserved 槽位策略

ADR 编号 0018-0020 在规划阶段被预留,但保持 *未撰写* 状态。按照禁止复用原则,新 ADR 从下一个可用编号 (0030+) 起分配。Reserved 槽位一旦撰写,INDEX 行即替换为正式条目。

## 全局参考

- 全局 ADR 标准: `~/Documents/ai-dev/standards/adr.md`
- ADR 覆盖率门: `scripts/check-adr-coverage.sh` (全局)
- 强制标准: `~/Documents/ai-dev/standards/enforcement.md §2.1`
