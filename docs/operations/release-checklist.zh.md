# 发版清单 — valkey-operator (简体中文)

> English: [release-checklist.md](release-checklist.md) — canonical / 正本

在 push 一个 release tag (例如 `v0.1.0`) 之前通过本清单,即可保证
release-grade 的质量。**下面的每一项**都有对应的自动化门禁兜底;
本文是给人类阅读的清单,**真正的强制点不在这里**。

自动化 SSOT: `scripts/release.sh` (手动入口) + `make gate` +
lefthook pre-push hooks。阅读本文是为了**看懂这套体系**;真正卡住
合并的是各道门禁本身。

## 1. 构建与代码质量 (自动)

- [ ] `make lint` — 0 issues (golangci-lint)。
- [ ] `make test` — 所有 unit + envtest 测试 PASS,0 回归。
- [ ] `make helm-lint` — chart 结构合法。
- [ ] `make helm-template` — chart 渲染干净。
- [ ] `make audit` — govulncheck + gosec + trivy fs (已知有 fix 的
      HIGH 与 CRITICAL 发现项: 0)。
- [ ] `go build ./...` — 各 OS/arch 上 0 错误。

lefthook 的 pre-push hook 会在每一次 push 时跑上面这 6 项;release
tag 的 push 同样走这道门禁。

## 2. SSOT 同步门禁 (in-process 单元测试,`internal/observability/`)

只要任意两个表面 (surface) 之间发生 drift,以下门禁就会阻止 PR
合入:

| # | 门禁 | 检查内容 |
|---|---|---|
| 1 | `TestADRFilesAllInIndex` | `docs/kb/adr/` ↔ `INDEX.md` 双向 |
| 2 | `TestADRIndexStatusValid` | Status 列 ∈ {Accepted, Proposed, Deprecated, Superseded by NNNN} |
| 3 | `TestADRIndexSupersededReferencesExist` | 每条 "Superseded" 引用都解析到真实 ADR |
| 4 | `TestADRFilesHaveRequiredSections` | 每份 ADR 都包含 Nygard 的三段 (Context / Decision / Consequences) |
| 5 | `TestAlertRulesSchemaSanity` | PrometheusRule CRD schema (apiVersion / kind / groups) |
| 6 | `TestAlertRulesAllFieldsValid` | 每条告警: 前缀 `Valkey` + expr + for + severity + annotations |
| 7 | `TestAlertRulesMetricNamesRegistered` | 告警 `expr` 中的 `valkey_cluster_*` 指标已在 `metrics.go` 注册 |
| 8 | `TestAlertRulesRunbookAnchorsExist` | `runbook_url` 锚点可在 `runbook.md` 中解析 |
| 9 | `TestRBACMarkerResourcesInRole` | `kubebuilder:rbac` 标记 ↔ `role.yaml` 双向 |
| 10 | `TestSamplesStrictUnmarshal` | `config/samples/` ↔ API 类型 strict-decode (拒绝未知字段) |
| 11 | `TestSamplesDirHasAllExpected` | 注册中的样本映射 ↔ 磁盘上的 samples 目录,双向 |
| 12 | `TestSamplesMetadataValid` | `apiVersion` / `kind` / `metadata.name` 格式 |
| 13 | `TestClusterRefKindEnumMatchesSSOT` | `ClusterReference.Kind` enum ↔ SSOT slice |
| 14 | `TestClusterRefKindAllHaveSwitchCase` | 每种 kind 在 controller 中都有 switch case |
| 15 | `TestLicenseFileExistsAndIsMIT` | `LICENSE` 文件存在且为 MIT |
| 16 | `TestChartLicenseAnnotationMatchesLicenseFile` | Chart annotation ↔ `LICENSE` 文件 |
| 17 | `TestChartImagesAnnotationMatchesAppVersion` | `artifacthub.io/images` tag ↔ Chart `AppVersion` |
| 18 | `TestChartIconURLUsesCurrentValkeyAsset` | Chart icon URL 当前能在 Artifact Hub 上解析 |
| 19 | `TestChartCRDExamplesStrictUnmarshal` | `crdsExamples` ↔ API 类型 strict-decode |
| 20 | `TestCRDBaseChartSync` | `config/crd/bases/` ↔ `charts/.../crds/` 字节级 (sha256) |
| 21 | `TestChartValuesValkeyVersionMatchesAPIDefault` | `values.yaml::valkey.version` ↔ API 默认值 |
| 22 | `TestChartNotesTxtModeValueValidEnum` | `NOTES.txt` 的 `mode:` ↔ `ValkeyMode` enum |
| 23 | `TestChartReadmeYAMLCodeblocksValid` | 每个 Markdown 文件中的 YAML 块都是合法的 `mode`/`apiVersion`/`kind` |
| 24 | `TestMarkdownRelativeLinksResolve` | 每个 `.md` 文件中的相对 `.md` 链接都可解析 |
| 25 | `TestIssueTemplateReadmeAnchorsExist` | issue-template 中的锚点能在 README 中解析 |
| 26 | `TestWebhookSetupFunctionsRegisteredInMain` | webhook setup 函数 ↔ `main.go` |
| 27 | `TestReconcilerTypesRegisteredInMain` | Reconciler 类型 ↔ `main.go` 实例化 |
| 28 | `TestRBACRoleResourcesInMarker` | `role.yaml` 中的 resource → `kubebuilder:rbac` 标记 (拦截孤儿规则) |
| 29 | `TestInstallYAMLStructure` | `dist/install.yaml` 形状 (5 个 CRD + Deployment + RBAC + Webhook + Service) |
| 30 | `TestKustomizeManifestLabelChainSync` | pod labels ⊇ Deployment selector ⊇ Service selector + ServiceMonitor selector ⊆ Service labels |
| 31 | `TestKustomizeChartResourcesSync` | `manager.yaml` ↔ `values.yaml` 的 resources (limits + requests × cpu + memory) |
| 32 | `TestKustomizeChartProbesSync` | manager Deployment ↔ chart probe 的 `initialDelay` 与 `period` |
| 33 | `TestKustomizeChartSecurityContextInvariants` | 两侧均满足 Pod Security "restricted" 不变式 (runAsNonRoot、seccompProfile、allowPrivilegeEscalation=false、readOnlyRootFilesystem、capabilities.drop=ALL) |
| 34 | `TestInstallYAMLOperatorImageEnvMatchesContainerImage` | `dist/install.yaml` 的 `OPERATOR_IMAGE` env 值 ↔ manager container 镜像 (拦截 Upload/Download Job ImagePullBackOff) |
| 35 | `TestChartArgsMatchOperatorFlags` | chart deployment + `config/manager` 的 args ↔ `cmd/main.go` flag 定义 (拦截过期 flag 导致的 CrashLoopBackOff) |
| 36 | `TestValuesTemplateBindingCoverage` | `values.yaml` 中的每个 top-level key 都在 `templates/` 的某处被引用 (拦截 silent ignore) |
| 37 | `TestChartFeaturesReconcilerEnvSync` | chart `features.{cluster,backup}.enabled` ↔ `ENABLE_{CLUSTER,BACKUP}_RECONCILER` env (确保 RBAC 与 reconciler 对齐) |
| 38 | `TestNetworkPolicyWebhookPortPresent` | `webhook.enabled` 时 NetworkPolicy ingress 条件性放行 9443 (拦截 silent reject) |
| 39 | `TestNetworkPolicyTracingEgressPresent` | `tracing.endpoint` 时 NetworkPolicy egress 条件性放行 OTLP 4317/4318 (拦截 span 静默丢失) |
| 40 | `TestNetworkPolicyBackupEgressPresent` | `features.backup.enabled` 时 NetworkPolicy egress 条件性放行 S3 (443/9000) (拦截 `BackupTarget` 永久 Pending) |
| 41 | `TestMetricPhaseLabelsSync` | `metrics.go::allPhases` ↔ API `ValkeyPhase` + `ClusterPhase` enum 并集 (确保 Grafana 上 phase 时间序列完整) |
| 42 | `TestGoVersionDockerfileVsGoMod` | `Dockerfile` `FROM golang:X.Y` ↔ `go.mod` `go X.Y` ↔ `CONTRIBUTING.md` Go 表格 |
| 43 | `TestKubernetesVersionSync` | `Chart.yaml` `kubeVersion` ↔ README badge ↔ chart README 的 Kubernetes 前置要求 |
| 44 | `TestReleaseTargetInjectsBuildMetadataAndAmd64Only` | release 镜像带上 build metadata;release 构建产物为 amd64+arm64 多架构 (ADR-0045 之后,release pipeline 通过 `docker/build-push-action` 产出多架构) |
| 45 | `TestArtifactHubRepositoryMetadataEnablesVerifiedPublisherAndSigningKey` | 保持 `artifacthub-repo.yml` 的 `repositoryID`、`signingKey`、`owners` 同步 |
| 46 | `TestReleasePipelineRequiresSignedHelmCharts` | 默认 `HELM_SIGN=1`;release/helm-publish 产出 `.tgz.prov` 资产 |
| 47 | `TestReleaseSmokeVerifiesHelmProvenance` | GH Release / `gh-pages` 的 provenance 通过 `helm verify` 配合已发布的签名密钥校验 |

运行命令: `go test ./internal/observability/`

## 3. 供应链 (在 release 时强制)

- [ ] `make sbom VERSION=vX.Y.Z` — 生成 syft SPDX-2.3 SBOM。
- [ ] release pipeline 自动把 SBOM 挂到 GitHub Release 上。
- [ ] release pipeline 自动把 chart `.tgz.prov` 挂到 GitHub
      Releases 与 `gh-pages` 上。
- [ ] container 镜像、Helm chart 与 SBOM 上的 **cosign keyless
      签名** (ADR-0046,v1.0.13+)。校验方法见
      [SECURITY.md](../../.github/SECURITY.md#verifying-release-artifacts-signed-releases--v1013)。
- [ ] **SLSA-3 provenance** 通过
      `slsa-framework/slsa-github-generator` 附加到镜像 (ADR-0046)。
- [ ] `bash scripts/release-smoke-test.sh` — 8 个阶段: chart 资产、
      SBOM 资产、Helm provenance 校验、helm pull、镜像 manifest、
      `gh-pages`、trivy CVE 扫描、cosign 校验。
- [ ] 多架构镜像 (linux/amd64 + linux/arm64),通过默认的 buildx
      builder 构建。

## 4. 文档

- [ ] CHANGELOG.md `[Unreleased]` 已晋升到 `[vX.Y.Z]`
      (git-cliff 会自动处理)。
- [ ] README §Roadmap "Next" 条目与本次 release 实际内容一致。
- [ ] ADR INDEX 已更新 (`TestADRFilesAllInIndex` 强制)。
- [ ] runbook.md §9 的每条告警响应流程在新增告警时同步更新。

## 5. 运维门禁

- [ ] kubectl 兼容性: `kubeVersion ≥ 1.26` (Chart.yaml)。
- [ ] `make manifests` 在 working tree 上是 no-op (controller-gen
      drift: 0)。`manifests` 同时会自动同步 chart 中的 CRD。
- [ ] 在 cert-manager / prometheus-operator 缺失时优雅降级
      (NotFound / NoMatch 全部 fail-soft)。

## 6. 用户可见表面 (自动检查)

这些都会沉淀为 OSS 的信任信号:

- LICENSE Apache-2.0 (门禁 #15 与 #16)
- SECURITY.md 含 PGP 指纹
- CONTRIBUTING.md 含前置要求 + PR 流程
- `.github/PULL_REQUEST_TEMPLATE.md` (门禁 #23)
- `.github/ISSUE_TEMPLATE/{bug_report,feature_request,question}.yml`
  (门禁 #24)
- README §Roadmap (门禁 #25 校验该锚点)
- Artifact Hub chart README + `crdsExamples` (门禁 #17、#18、#22)
- Issue triage 标签 (bug/triage 自动打上)

## 7. release tag 推送流程

```bash
# 1. 跑完 §1–§6 的自动化套件,确认全绿。
make gate                                # = lint + test + helm + audit
go test ./internal/observability/        # 47 道 SSOT 门禁
bash scripts/release-smoke-test.sh vX.Y.Z  # 8 个阶段 (image/chart 缺失时跳过)

# 2. 手工运行 release.sh。
bash scripts/release.sh vX.Y.Z

# 3. 手工发布 GH Release 正文 (release.sh 会写入
#    .release_notes.md)。
gh release create vX.Y.Z --notes-file .release_notes.md \
  dist/install.yaml \
  /tmp/valkey-operator-X.Y.Z.spdx.json

# 4. helm-publish (Artifact Hub 会自动检测 gh-pages)。
make helm-publish HELM_SIGN=1 VERSION=vX.Y.Z
```

## 8. v0.1.0 GA 额外门禁 (alpha → GA 晋升)

下列项在 alpha 阶段**不**适用。从 GA tag 开始:

- [ ] 在 kind cluster 上 24 小时 soak test: 0 内存泄漏,0 FD
      泄漏。
- [ ] End-to-end 自动化 (kind + MinIO + ValkeyCluster restore
      场景) PASS。
- [ ] Track B (Failover / Resharding) 的 ADR + 实现已完成。
- [ ] Conversion webhook 就绪 (v1alpha1 → v1beta1,ADR-0026
      已 defer)。
- [ ] HPA 集成 (Replication 模式,ADR-0027 已 defer)。
- [ ] semver `0.1.0` 还是 `1.0.0` 由项目策略决定。

## 9. 自我校验

`docs/operations/release-checklist.md` 自身也处在 SSOT 门禁的覆盖
范围内:

- 一道未来的横切门禁可以校验本文档引用的门禁确实存在于
  `internal/observability/`。
- 本文档中的破损 markdown 链接会被
  `TestMarkdownRelativeLinksResolve` 拦截。
