# Changelog (简体中文)

> English: [CHANGELOG.md](CHANGELOG.md) — canonical / 正本

> 所有重要变更的历史记录。英文为正本,本文件为按中文运维语调翻译的
> 副本。各 release 的简报请参阅 GitHub Release 页面。

本项目所有重要变更均记录于本文件。
格式遵循 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
版本规则遵循
[Semantic Versioning](https://semver.org/spec/v2.0.0.html)。

自动生成: `git-cliff` (P1 §2.3 标准) — release tag 时通过 PR 自动
更新。

## [Unreleased]

## [1.0.13] - 2026-05-13

### Added

- ADR-0045: 为 OSS CI 恢复 GitHub Actions workflow (RFC-0002 的
  范围受限例外) (#89)。
- ADR-0046: 为 release artifact (image + chart + SBOM) 引入
  SLSA-3 provenance 和 cosign keyless 签名 (#92)。
- `.github/FUNDING.yml` — 公开 GitHub Sponsors 资助渠道 (#91)。
- `.github/workflows/scorecard.yml` — 每周执行 OpenSSF Scorecard
  分析并上传 SARIF (#94)。
- `.github/workflows/dependency-review.yml` — 阻止引入 High+ CVE
  或非许可类许可证依赖的 PR (#94)。
- `.github/workflows/codeql.yml` — 使用 `security-extended` 规则
  集的 CodeQL Go SAST (#99)。
- `.github/workflows/dco.yml` — 服务端 DCO sign-off 校验,与本地
  lefthook commit-msg hook 保持一致 (#99)。
- `.github/ISSUE_TEMPLATE/config.yml` — 关闭空 issue,并暴露
  Security Advisory / Discussions / Runbook contact 链接 (#96)。
- 将英文版 README / CONTRIBUTING / SECURITY / GOVERNANCE /
  MAINTAINERS / ADOPTERS 提升为 canonical;原韩文版以 `.ko.md`
  兄弟文件形式保留 (#93, #97, #98)。
- README 新增 "Known limitations" 一节 — 用于保持 SECURITY.md
  的 cross-link 可用 (#93 后续)。
- 新增 `.editorconfig`,统一管控 Go (tab)、YAML / JSON /
  Markdown (2-space、trim 策略)、Makefile (必须使用 tab)、shell
  脚本的格式策略。

### Changed

- 将 8 个 workflow 中所有 GitHub Actions 引用全部 pin 到 commit
  SHA + 尾部版本注释,满足 OpenSSF Scorecard 的
  `Pinned-Dependencies` 检查 (#95)。
- 启用 `setup-go check-latest: true`,并在 `go.mod` 中声明
  `toolchain go1.26.3`,使 stdlib CVE 修复 (1.26.2 / 1.26.3 共 16
  项) 自动应用于每次 CI 运行 (#92)。
- `security-scan.yml` 现在对 *所有 PR* 都执行;此前仅在 diff 触
  及 go.mod / Dockerfile 时执行 — 安全扫描不应因 diff 落在别处
  而被跳过 (#92)。
- 强化 `main` 的 branch protection: 必须通过 7 个 status check
  (golangci-lint, unit + envtest, build, govulncheck, trivy-fs,
  trivy-image, Review dependencies)、strict mode、linear
  history、conversation resolution、禁止 force-push 与删除、
  enforce-admins 开启。
- 仓库安全开关全部开启: Dependency graph、Automated security
  fixes、Secret scanning、Secret-scanning push protection。
- README 中 Go badge 从 1.25+ 升至 1.26+ (#90)。
- CONTRIBUTING.md 中 Go 前置要求升至 1.26,以维持
  `TestGoVersionDockerfileVsGoMod` 回归守护通过 (#84 后续)。
- `release.yml` 的 `sbom` job 现在同时声明 `contents: read` +
  `packages: read`,使 syft 在保留 OIDC token 的同时仍能访问
  GHCR manifest (#92 review 后续)。
- 收紧 `cosign --certificate-identity-regexp`,使其精确匹配
  `release.yml` workflow,而不是本仓库的任意 workflow (#92
  review 后续)。

### Security

- 自 v1.0.13 起,所有签名后的 image / chart / SBOM 都会生成对应的
  Sigstore Rekor 透明日志条目。
- 容器镜像通过 `slsa-framework/slsa-github-generator` 生成 SLSA-3
  provenance attestation (自 v1.0.13)。
- SECURITY.md 明确给出验证 artifact 所需的精确 `cosign verify` /
  `slsa-verifier verify-image` 命令以及证书 identity 的 regex。

### Dependencies

- `actions/setup-go check-latest: true` 使 CI 的 Go runtime 自动
  获取 stdlib CVE 修复。
- 合并到 main 的 7 项 dependabot 更新:
  - Docker base: `golang 1.26.3`、`distroless/static@e3f9456`
    (#80, #81)。
  - Go modules: k8s 0.36 + controller-runtime 0.24 + utils +
    operator-commons 0.7.0 + otel 组 + ginkgo 2.28.3 + gomega
    1.40.0 (#84–#88)。

## [1.0.12] - 2026-05-12

### Changed

- 因发现已有的 `v1.0.11` tag 指向了未完成的 release commit,故发布
  一次干净的 Artifact Hub 刷新版本。
- 刷新已发布 chart 的 metadata,使其展示 Alpine 3.23 的 Valkey
  runtime image,以及 operator image tag `1.0.12`。
- chart README 与 Artifact Hub 可见 metadata 维持英文表述。

### Fixed

- 移除最终 release commit 之前误引入的、被禁止的 GitHub Actions
  workflow 文件。

## [1.0.11] - 2026-05-12

### Changed

- 刷新 Artifact Hub 上的 chart metadata,使已发布的包展示 Alpine
  3.23 的 Valkey runtime image。
- 为当前 release 表面发布 chart / app 版本 `1.0.11`。
- Artifact Hub trust-badge 文档维持英文表述。

### Fixed

- 修正仍在告示 `docker.io/valkey/valkey:9.0.4` 的过期 Helm 仓库
  包。

## [1.0.10] - 2026-05-10

### Added

- OperatorHub.io bundle 骨架 + ADR-0037 (PR-B9 first cut, #21)。
- `alm-examples` 内联 JSON 样例 5 份 (PR-B9.2, #22)。
- 添加 CITATION.cff (OSS 元数据, #20)。

### Changed

- Chart 升至 v1.0.10 + 仅 amd64 构建 (CLAUDE.md §2 对齐, #27)。
- 重新生成 `v1alpha2 zz_generated.deepcopy.go` (controller-gen
  同步, cfd0398)。
- bundle: 移除 `generate-kustomize-manifests` 步骤 (PR-B9.4,
  与 mongodb ADR-0023 对齐, #23)。

### Fixed

- `ValkeyCluster` post-init self-heal — INC-0001 的长期 fix
  (ADR-0039, #25)。
- 为 v1alpha1 添加 `storageversion` 标记并重新执行 controller-gen
  (PR-A2.2.5, #19)。
- 启用 `ReadOnlyRootFilesystem=true` — 现代安全基线的最后一层
  (3aa5480)。

### Docs

- INC-0001 ValkeyCluster bootstrap skip — 生产集群持续 19 小时
  fail 的恢复记录 (#24)。
- INC-0001 `cluster_state=fail` 恢复 runbook + 明确 ADR-0039
  self-heal (AI-0004, #26)。
- ADR-0026 说明部分恢复的进度 (PR-A2.2.* 累计, #18)。
- HANDOFF 更新 PR-A2.2.5 的合入结果与下一个入口点 (1818031)。

## [1.0.9] - 2026-05-10

### Added

- 新增 v1alpha2 类型定义模块 + `AuthSpec.Required` toggle
  (PR-A2.1, #6)。
- 新增 5 个 v1alpha2 Hub marker 类型 (PR-A2.2.1, #15)。
- 实现 v1alpha1 的 `ConvertTo` / `ConvertFrom` 5 类型主体
  (PR-A2.2.2, #16)。
- `cmd`: 注册 v1alpha2 的 SchemeBuilder (PR-A2.2.3.a, #17)。
- 添加 Valkey Custom Modules 类型 (v1alpha2, PR-C6.1, ADR-0032,
  #14)。
- 添加 `AuthSpec.RotationPolicy` enum (v1alpha2, PR-B7.1,
  ADR-0031, #12)。
- `PodSecurity` Restricted 可选 toggle (v1alpha2, PR-A3.2,
  ADR-0036, #10)。
- `NetworkPolicy.AutoCreate` 可选 toggle (v1alpha2, PR-A3.1,
  ADR-0035, #9)。
- 迁移到 RFC-0018 的 `pkg/finalizer` (controller, PR-A6 first
  cut, ADR-0038, #8)。
- release: cosign 签名 + SLSA L2 in-toto attestation + ADR-0033
  (PR-A4, #5)。

### Changed

- operator-commons v0.5.0 → v0.6.0 — RFC-0018 的 `SetAvailable`
  / `SetReadyFalse` 现可使用 (#7)。

### Docs

- ADR-0018 转为正式版 — Cluster Auto-Resharding (PR-B8.1, #13)。
- 新增 Sentinel migration runbook (PR-C7, 对 ADR-0017 被拒后的
  补强, #11)。

## [1.0.8] - 2026-05-09

### Fixed

- 修复 `monitoring.exporter.resources` 未能 reconcile 到 metrics
  sidecar 的运维集成缺陷 (1eb6faf):
  - 新增 `STSParams.ExporterResources corev1.ResourceRequirements`
    字段 (`internal/resources/statefulset.go`)。
  - 让 `BuildStatefulSet` 的 metrics container 应用
    `p.ExporterResources`。
  - 由 Valkey 与 ValkeyCluster controller 经
    `exporterResources(spec.Monitoring)` helper 传入。
  - 空 ResourceRequirements (默认) → K8s Burstable QoS,与原先
    行为等价兼容。

### Changed

- Chart 升至 1.0.7 (8408005)。

## [1.0.7] - 2026-05-09

### Changed

- audit (4-repo cross-cut, 2026-05-09): 采纳 RFC-0017 —
  新增 `.golangci.yml` + `.custom-gcl.yml` (postgres 标准 cp +
  depguard 整理),为 Makefile 增加 `validate` 目标 (kustomize +
  helm lint + helm template)。登记 ADR-0030。本仓库的
  `.lefthook.yml` 升格为 RFC-0017 §3.1 的标准原本 (无内容变更,
  0aea740)。
- operator-commons v0.4.0 → v0.5.0 (4833f13)。
- 新增 `.codecov.yml` — 4-repo 的目标 70% 绝对 floor 统一
  (d381587)。

### Fixed

- `.golangci.yml` 中暂时禁用 depguard (golangci-lint v2.8 schema
  拒绝空的 deny list) — 待 valkey 引入 internal boundary 后,与
  ADR 一起重新启用。已启用 17 个 linter (含 logcheck plugin,
  9dae535)。
- lint: valkey 剩余 37 项,达到 lint 0 issue (goconst 17 +
  unparam 17 + gocyclo 3, 8ba60a7)。
- lint: 修复 lll / prealloc / revive 共 5 项 (安全清理,
  5d16c94)。
- lint: 自动修复 modernize 20 + copyloopvar 2 项 (例如
  `slices.Contains`, 8820460)。

### Docs

- CHANGELOG 条目 + deps log (audit 收尾, bd667ad)。

## [1.0.6] - 2026-05-08

### Added

- 新增 TLS `clientAuth` 字段 — required / optional / disabled 的
  mTLS toggle (0c804c9)。
- renovate: auto-update PR 的入口 (Go modules + image tag,
  ba3c9af)。

## [1.0.5] - 2026-05-08

### Fixed

- 修复 Artifact Hub 拉取 `1.0.4` chart 的 icon
  `https://valkey.io/img/Valkey-Logo-RGB-Color.svg` 时收到 404、
  并产生 tracking warning 的问题;改为指向当前 Valkey 站点上能
  返回 200 的 `https://valkey.io/img/valkey-horizontal.svg`。

## [1.0.4] - 2026-05-08

### Added

- Service builder: 启用 TLS 时暴露 client-tls (6380) 端口。
  为 BuildClientService 与 BuildHeadlessService 增加 tlsEnabled
  参数;外部 client 可以使用 `rediss://` scheme 进行 connect
  (当 `tls-auth-clients=yes` 时,client cert 仍需另行签发 — 本
  patch 仅提供 server-side TLS 的外部暴露基础设施)。

## [1.0.1] - 2026-05-07

### Fixed

- 修复 `ValkeyBackup` 针对 `ValkeyCluster` 目标只保存第一个 pod
  的 `dump.rdb` 的问题。现在按 shard 的 primary pod 生成
  `shard-N/dump.rdb` 结构,可直接与 cluster restore 的默认 shard
  layout 对齐。
- 修复 `ValkeyRestore` 在恢复 `ValkeyCluster` 目标时,仅将
  pause / unpause annotation 应用于 `Valkey` CR 的问题。
- multi-pod restore 在校验现有 source PVC 时,除 `ReadOnlyMany`
  外,也允许可只读挂载的 `ReadWriteMany` PVC。

## [1.0.0] - 2026-05-07

### Added

- 首个 stable 版本。包含 Valkey `9.0.4` 默认值、与 `8.0.9` /
  `8.1.7` milestone 的兼容性、`ValkeyCluster` sharded HA、自动
  failover、`ValkeyBackup`、`ValkeyRestore`、
  `ValkeyBackupTarget`、默认启用 restricted PodSecurity,以及
  `linux/amd64` / `linux/arm64` multi-arch operator image。

## [0.1.0-alpha.5] - 2026-05-07

### Fixed

- **Runtime P0 — restricted PodSecurity 命名空间中 Valkey Pod
  创建失败** (`internal/resources/statefulset.go`):
  Valkey StatefulSet 容器未配置 `allowPrivilegeEscalation=false`、
  `capabilities.drop=[ALL]`、`seccompProfile.type=RuntimeDefault`
  的默认值,导致在 `data-staging` 命名空间下 Pod 创建被拒绝。
  现已向默认 Valkey 容器与 metrics sidecar 注入 restricted
  SecurityContext。

## [0.1.0-alpha.4] - 2026-05-07

### Fixed

- **Release P0 — operator image build metadata 缺失**
  (`Makefile`):
  `make release` 未将 Dockerfile 的 `VERSION` / `COMMIT` /
  `BUILD_DATE` build args 传入,实发版镜像的 `/manager --version`
  与 `valkey_cluster_build_info` 显示为 `dev/none/unknown`。现已
  在 release target 中注入 tag、git commit、UTC build date。
- **Release P0 — chart affinity 与 image platform 不一致**
  (`Makefile`): chart 默认 affinity 允许 `linux/amd64` 与
  `linux/arm64` 节点,但 release image 仅推送了 `linux/amd64`。
  将 release build 改为 `linux/amd64,linux/arm64` multi-arch。

### Added

- 新增回归测试,确保 release target 强制注入 build metadata 并
  使用 multi-arch platform。

## [0.1.0-alpha.3] - 2026-05-07

### Added

- 整合 Valkey latest default: API default、CRD default、Helm
  values、ArtifactHub examples / images、samples、GitOps workload
  CR 一并更新为 `9.0.4`。
- 将 `SupportedValkeyVersions` whitelist 明示为 `8.0.9`、
  `8.1.6`、`8.1.7`、`9.0.4`,以文档化对最新版 + 8.0 / 8.1
  milestone patch 的兼容基线。
- ValkeyCluster 9.0.4 sharded 3x1 Kind smoke: 6 个 pod Ready、
  `cluster_state=ok`、16384 slots,通过 SET / GET 验证。

### Fixed

- 对从 Redis 8.2.x RDB 直接 restore 到 Valkey 9.0.4 时,因 RDB
  格式不匹配引发 CrashLoopBackOff 的路径,改为通过
  `ValkeyRestore.status.phase=Failed` 进行 fail-fast。

## [0.1.0-alpha.2] - 2026-05-07

ADR-0057 Phase A1 (运维集群前置部署) 过程中发现的 chart RBAC
缺陷修复。

### Fixed
- **chart RBAC P0 — `features.{cluster,backup}.enabled=false`
  时 informer 启动失败**
  (`charts/valkey-operator/templates/clusterrole.yaml`):
  原 chart 依据 `features.cluster.enabled` /
  `features.backup.enabled` 条件性地授予
  `valkeyclusters` / `valkeybackups` / `valkeybackuptargets` /
  `valkeyrestores` 的 RBAC;但 operator manager
  (`cmd/main.go`) *始终* 会注册全部 controller — 因此 flag=false
  时 informer 会因 `forbidden` 而启动失败。RBAC 与代码错位是
  production-grade 的关键阻塞。RBAC 简化为 *始终授予全部 CRD
  权限*,feature flag 只在 controller 代码侧处理。

### Verified (运维集群 Phase A1 + A2)
- valkey-operator pod 1/1 Running,Certificate / Issuer /
  ValidatingWebhookConfiguration Ready。
- Valkey CR `valkey-test` (Standalone、valkey 8.1.6、1Gi
  ceph-rbd) 1/1 Running。
- SET / GET smoke:
  `SET phase-a2-smoke "OK-2026-05-07"` → `OK`,`GET` → 正常
  round-trip。
- `INFO server`: valkey_version=8.1.6、tcp_port=6379。

### Refs
- ADR-0057 (基础设施 bootstrap 43fd542): self-hosted
  valkey-operator 的采纳路线。
- 运维事故分析 + Phase A 推进: keiailab/mongodb-operator
  HANDOFF.md (2026-05-07)。

### Added (GitOps deploy 对齐)

- `deploy/overlays/prod/` GitOps 入口 — 将
  `config/{crd,rbac,manager}` 对齐到 prod ns,并移除自动生成的
  Namespace。前提是 ArgoCD 的单向同步。
- `deploy/valkey-cluster.yaml` — production ValkeyCluster 样例
  (db ns、shards=3、replicasPerShard=1、ceph-block、
  auth.enabled=true)。
- `deploy/README.md` — 运维 runbook。
- ADR-0029 — 引入 GitOps deploy overlay (与 mongodb-operator /
  postgresql-operator 形成 3-repo 对齐)。

### Added (cycles 20-90 — Quality 体系 + production-grade UX)

**Quality 体系 (39 个 SSOT 闸口)**:
- ADR governance (4 个闸口): file / INDEX / Status / Superseded
  / Nygard 三段式。
- Alert rules (4): schema / fields / metric / runbook anchor 同
  步。
- RBAC (双向 2 个): `kubebuilder:rbac` ↔ `role.yaml`。
- Sample CR (3): strict unmarshal + dir-mapping + metadata。
- ClusterRef.Kind (2 — 3-way): enum ↔ switch case。
- LICENSE + Chart annotation (2)。
- Chart artifacts (6): images / CRDExamples / CRD sync /
  values / NOTES / README YAML。
- Markdown link + anchor (2)。
- Webhook + Reconciler 注册 (2)。
- `dist/install.yaml` (2): 结构 + `OPERATOR_IMAGE` env。
- Release-checklist 自同步 (1, 双向 cycle 60)。
- Kustomize ↔ chart sync family (3): resources / probes /
  securityContext。
- Cross-feature interaction family (3): NP + webhook /
  tracing / backup。
- `features.*` RBAC + reconciler 同步 (1)。
- value ↔ template binding (1)。
- chart args ↔ operator flag (1)。

**自动化 (从源头杜绝失误)**:
- `make manifests` 自动同步 chart CRD。
- pre-push lefthook 6-hook (full-lint + gitleaks + go-mod-tidy
  + helm-lint + helm-template + unit-test)。
- `make sbom` (syft SPDX) + trivy post-scan 自动挂载到 release
  pipeline。

**Production-grade UX**:
- ldflags chain (cycles 53-57): `cmd/main.go` → Dockerfile →
  `docker-build` → `docker-buildx` → `release.sh` → Prometheus
  `build_info` gauge。
- chart features 5 项 (cycles 65 / 72 / 73 / 74 / 82): tracing
  + NetworkPolicy + webhook + watch.namespaces + autoscaling
  全部诚实地呈现。
- 6 层文档体系: README + chart README + NOTES.txt +
  CONTRIBUTING + release-checklist + HANDOFF (覆盖各类用户角色的
  入口)。
- runbook §7.1 — 环境变量诊断指南。
- 3 层 DX: lefthook auto + `make ssot-check` (1.4s) +
  `make gate` (30s)。

**已实现功能 (cycles 72-74 — chart 4 个未使用 value 中解决了
3 个)**:
- `charts/valkey-operator/templates/networkpolicy.yaml` —
  operator pod default-deny。
- `charts/valkey-operator/templates/webhook.yaml` — 依赖
  cert-manager 的 admission webhook。
- `WATCH_NAMESPACES` env — namespace-scoped watch
  (`cache.DefaultNamespaces`)。

**已实现功能 (cycles 99-106 — kubebuilder boilerplate completion
+ Helm parity)**:
- cycle 100 — runbook §7.0 production TLS 强化指南
  (`insecureSkipVerify` → cert-manager)。
- cycle 101 — `config/manager` + chart values 的 nodeAffinity
  (amd64 + arm64 + linux) — 拦截 mixed-arch 下的
  ImagePullBackOff。
- cycle 102 — 启用 `config/default/kustomization.yaml` 中的
  `- ../prometheus` — kustomize 用户也能自动安装 ServiceMonitor
  + PrometheusRule。
- cycle 103 — `charts/.../prometheusrule.yaml` — 阻止 Helm 用户
  的 10 条 alert silent loss。
- cycle 104 — `charts/.../metrics-auth-rbac.yaml` — 阻止 secure
  metrics 中 Prometheus 因 401 而 silent fail。
- cycle 106 — `charts/.../deployment.yaml` 的 webhook serving
  配置 (`--webhook-cert-path` + 9443 + cert mount) — 启用
  webhook 时 operator 可在 9443 上正确 listen。

**发现并修复 production gap (27 项)** + **清理内部债务 (3 项)**
+ **5 个 hot-path benchmark** + **8 个缺陷 family 的 progressive
completion**。

### Added (iter 7+ — bootstrap · 验证循环)
- README quickstart (基于 kind): 5 步 bootstrap + 数据 plane
  smoke + 运维场景矩阵。 [iter 6]
- ADR-0011: 针对 Required 字段 (无 omitempty) 的 mutating
  webhook defaulting 模式。 [iter 4]
- ADR-0012: CLUSTER MEET 不支持 hostname → 先做 DNS 解析后再用
  IP。 [iter 4]
- ADR-0013: 强制 `Auth.Enabled` 为 true (采纳方案 A)。 [iter 5]
- `internal/valkey/cluster.go::resolveAddrIP`: hostname → IP
  正规化 (优先 IPv4)。
- `internal/webhook/v1alpha1/valkey_webhook.go`: Version +
  `Auth.Enabled` 正规化。
- `internal/webhook/v1alpha1/valkeycluster_webhook.go`: Shards
  / ReplicasPerShard / Version / Auth 的 defaulting。
- `api/v1alpha1/common_types.go`: `DefaultValkeyVersion` /
  `DefaultValkeyImage` 常量。
- `internal/controller/valkeycluster_controller.go`: 添加 pods
  RBAC (用于 status reconciliation)。
- `config/samples/cache_v1alpha1_valkeybackup.yaml`: 填入有意义
  的 ClusterRef。
- `.dockerignore`: `*.tmpl`、`*.lua`、`*.sh` 模式 — 保留 embed
  资产。
- 启用 lefthook (pre-commit + pre-push + commit-msg) +
  Conventional Commits 模式。

### Fixed (iter 7+)
- ValkeyBackup controller 测试 fixture 缺失 ClusterRef (webhook
  validation 无法通过)。
- ValkeyCluster bootstrap 无限重试: CLUSTER MEET 拒绝 hostname →
  改用 DNS 解析。
- defaulting webhook 未填充必需字段 (Version / Shards /
  ReplicasPerShard) 导致 reconcile 无限循环。
- 缺失 pods RBAC 导致无法更新 ValkeyCluster status。
- lefthook commit-msg 使用了 `{1}` 而非 `$1`。
- lefthook golangci-lint 在 cross-directory 的 staged files 上
  出错。

### Verified (iter 7+ 实测)
- e2e suite: 5/5 PASS (manager 启动、metrics endpoint、
  cert-manager、mutating / validating webhook CA injection)。
- integration test: 14 case PASS (真 valkey:8 容器 + 6 节点集群
  bootstrap)。
- unit test: 4 个包 PASS
  (`internal/{controller,resources,valkey,webhook}`)。
- 韧性: primary pod force kill → STS 重建 → operator 重新
  promote → 数据保留 (canary `preserved`)。
- 扩缩容: 3 → 5 → 2,`master_link_status:up`,数据保留。
- 集群模式: 3 shards × 2 instances,`cluster_state:ok`,
  `slots:16384/16384` OK。
- TLS + mTLS 集群 (cert-manager + selfsigned ClusterIssuer):
  Phase=Running,slots=16384/16384 OK,数据 plane SET / GET 成功
  (cluster mode `-c`,跨多 shard 分散)。
- NetworkPolicy 资源一致性: deny-by-default + selfPeer ingress
  (6379) + ownerReferences (Standalone)。cluster mode 时追加
  16379。强制效果验证需要 Calico / Cilium CNI (kindnet 不支持)。
- operator metrics endpoint (HTTPS:8443、ServiceAccount token
  鉴权): `controller_runtime_*` 指标正常暴露。自定义
  `valkey_cluster_*` 指标在 ValkeyCluster reconcile 时 emit。

### Added (iter 1-6 — 之前的循环)
- 实现 ValkeyCluster Reconcile 14 步 (cluster mode 的 CRD
  bootstrap → CLUSTER MEET / ADDSLOTS / REPLICATE → status
  polling)。 [iter 1]
- `internal/valkey/cluster.go`: 将 `CreateCluster` 按阶段拆为
  幂等单元 (`ensureMeet` / `ensureSlots` / `ensureReplicas`),
  支持 partial-state 恢复。 [iter 2]
- `internal/valkey/nodes.go`: `CLUSTER NODES` 应答解析器
  (`NodeView`、`SlotRange`)。 [iter 2]
- 集成测试 (`//go:build integration`): 真 valkey:8 容器 6 节点
  集群 — 4 个场景 PASS。 [iter 2-4]
- Finalizer 优雅清理: `gracefulClusterTeardown` (best-effort
  `CLUSTER FORGET`,30s timeout)。 [iter 2]
- Prometheus metrics: 7 条时间序列 (`state_ok`、
  `assigned_slots`、`shards`、`ready_replicas`、
  `reconcile_total`、`reconcile_errors_total`、`phase`)。
  [iter 3]
- `ScalePolicy.Deliberate` 守护: 未同意时记录
  `Status.PendingScale` 并保留 STS replicas。 [iter 3]
- ServiceMonitor (`monitoring.coreos.com/v1` 的 unstructured)
  自动创建 + 单独的 metrics Service。 [iter 3]
- 通过 ConfigMap directive 集成 AutoFailover
  (`cluster-replica-no-failover yes`)。 [iter 3]
- `make integration-test` Makefile target。 [iter 2]
- `buildShardStatusFromNodes`: 基于 CLUSTER NODES 的 ShardStatus
  (准确反映 failover,ADR-0007)。 [iter 4]
- 加载 TLS RootCAs: `Spec.TLS.CustomCert.SecretName.ca.crt` →
  x509 CertPool (ADR-0008)。 [iter 4]
- Validating + Mutating Webhook (两个 CRD): 8 种组合校验 +
  immutable 守护 (Mode、Storage、TLS toggle)。 [iter 5]
- ShardStatus 的 pod ordinal 映射: `buildPodAddrMap` (K8s Pod
  list → "vk-N")。 [iter 5]
- 自动创建 cert-manager Certificate: 明示
  `Spec.TLS.CertManager.IssuerRef` 时自动建 Certificate CR,
  自动发现 secretName (ADR-0010)。 [iter 6]
- Version upgrade 检测: `decidePhase` 检测到
  `Spec.Version != Status.Version` 时进入 Phase=Upgrading。
  [iter 6]
- ADR 0001-0010 (10 条,其中 2 条 supersede): defaulting →
  webhook (0001 → 0009)、ShardStatus spec → NODES (0004 →
  0007)、TLS 分阶段集成 (0003 → 0008 → 0010)。 [iter 1-6]
- lefthook 配置 (`.lefthook.yml`)。 [iter 3]

### Changed
- `valkey/replication.go`: `SlaveOf` → `ReplicaOf` (Redis 5.0+
  deprecated API)。新增对 modern Valkey `role:replica` 的识别。
  [iter 1]
- `valkeycluster_controller.go`: `pollClusterState` 改为全节点
  fallback (`queryAnyNode`) — 消除 pod-0 SPOF。 [iter 1]
- `dialPod` 现在会传入 `Spec.TLS.Enabled` (之前被忽略)。
  [iter 1]
- SetupWithManager: 增加 `Owns(PDB, NetworkPolicy)` — 用于 drift
  检测。 [iter 1]

### Fixed
- 编译错误: `&appsv1StatefulSet{}.s` →
  `(&appsv1StatefulSet{}).Inner()` (Go struct literal 的
  addressability 限制)。 [iter 1]
- 为 `ensureReplicas` 添加 gossip 收敛 retry —
  `replicateWithRetry` (10 次 backoff,吸收 "Unknown node")。
  [iter 2]
- 让 `parseReplicationInfo` 识别 modern Valkey 的
  `role:replica` — 之前每次 reconcile 都会重新调用 ReplicaOf
  (破坏幂等性)。 [iter 1]

### Documentation
- ADR 索引 (`docs/kb/adr/INDEX.md`)。
- 本 CHANGELOG。 [iter 3、iter 6 更新]

### Test Coverage Snapshot (iter 6 末)
- `internal/controller`: 50.5%。
- `internal/resources`: 33%+。
- `internal/valkey`: 33.7%。
- **`internal/webhook/v1alpha1`: 80.7%** (新增包)。
- 单元测试: 60+ 项。
- 集成测试: 4 个场景 (真 Valkey 6 节点)。

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
