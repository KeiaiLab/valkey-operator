<p align="center">
  <a href="ROADMAP.md">English</a> |
  <a href="ROADMAP.ko.md">한국어</a> |
  <a href="ROADMAP.ja.md">日本語</a> |
  <b>中文</b>
</p>

# ROADMAP — valkey-operator

> English (canonical / 正本): [ROADMAP.md](ROADMAP.md)

本路线图 (Roadmap) *并非日期承诺*,而是以可验证的功能清单方式跟踪进展。基于时间的 deadline 按照项目的
[`standards/workflow.md`](https://github.com/keiailab/valkey-operator/blob/main/docs/kb/adr/INDEX.md)
("禁止时间型路线图" 规则) 被有意排除;进展按功能完成度衡量。

## 复选框含义

| 标记 | 含义 |
|---|---|
| `[x]` | 代码与测试两者皆有;通过 e2e 或 unit test 提供回归守护 (regression guard) |
| `[~]` | 部分实现 — 字段已定义但 helper 尚未集成,或某项验证仍未完成 |
| `[ ]` | 未开始 (设计或 PoC 阶段) |

每个 sub-task 右侧的 *Verify* 行引用了用于确认该复选框的精确命令或 e2e 文件。

## 当前 (1.x 线 — Active)

### 稳定性与成熟度

- [x] **PodSecurity restricted compliance**
  - [x] 4 处 SecurityContext helper 统一 — `internal/resources/security.go`
  - [x] resources 构造器的 restricted PSA 回归守护
  - [x] controller 与 webhook 侧 podSpec 转换路径全程守护
    — `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validatePodSecurityRestricted` (6 项 —
    runAsNonRoot/runAsUser/privileged/allowPrivilegeEscalation、9 个 unit
    test、#78)
  - Verify: 为 namespace 打上
    `pod-security.kubernetes.io/enforce=restricted` 标签后,pod 进入 Ready

- [x] **集群模式 (Cluster mode,5 shard × replica=2)**
  - [x] 基于 ordinal 的 restore Init Container —
    `internal/controller/valkeycluster_controller.go`
  - [x] 16384 个 slot 自动分配
  - [x] 自动 failover (经 chaos 测试) —
    `test/e2e/cluster_recovery_test.go`、`failover.go`
  - [x] Primary kill → master 重新选举 —
    `test/e2e/failover_test.go`
  - Verify: `test/e2e/cluster_recovery_test.go` PASS、16384 个 slot
    完整保留、数据保全

- [x] **HPA / PDB / NetworkPolicy 自动化 (opt-in)**
  - [x] HPA (ADR-0027、Replication mode) — chart
    `autoscaling.enabled`
  - [x] PDB 默认 — `internal/controller/pdb_default.go`
  - [x] NetworkPolicy default-deny + 明确 allow — chart
    `networkPolicy.enabled`
  - Verify: `pdb_default_test.go` PASS、
    `kubectl get pdb/networkpolicy`

- [x] **备份 / 恢复 — S3 + PVC ROX + VolumeSnapshot**
  - [x] S3 (minio-go) 备份 —
    `internal/controller/valkeybackup_controller.go`
  - [x] PVC ROX 多挂载恢复 —
    `internal/controller/valkeyrestore_controller.go`
  - [x] VolumeSnapshot lifecycle —
    `internal/controller/backup_volumesnapshot.go`
  - [x] Multipod snapshot replication 恢复 —
    `multipod_volumesnapshot_replication_test.go`
  - [x] `ValkeyBackupTarget` CRD (外部备份目标) —
    `api/v1alpha2/valkeybackuptarget_types.go`
  - Verify: `test/e2e/backup_restore_test.go` PASS

- [x] **chart RBAC conditional 修复** (2026-05-07、commit `06237be`)
  - [x] 在
    `features.{cluster,backup}.enabled=false` 时防止 informer 启动失败
  - Verify: 以
    `--set features.cluster.enabled=false` 安装 chart 后,operator pod 变为 Ready

- [x] **版本升级 reconcile 修复**
  - [x] Fresh 场景路径正确 (iteration 7 诊断)
  - [x] Restore → patch chain 回归守护 (iteration 18 V2) —
    `test/e2e/backup_restore_test.go` "Restored instance 8.1.6 → 9.0.4
    version patch chain (V2)"
  - [x] RDB v80 兼容性 (`foo=bar1` 保留)
  - Verify: 上述 e2e PASS = 两个狭义 blocker 永久解决

- [x] **Valkey 9.x 支持 (默认 9.0.4)**
  - [x] Chart `image.tag: 9.0.4` 默认值 —
    `charts/valkey-operator/values.yaml`
  - [x] RDB 格式 v80 兼容性已验证
  - Verify: 启动一个新实例并运行
    `valkey-cli INFO server | grep redis_version`

- [x] **API 版本演进**
  - [x] v1alpha2 活跃 — `api/v1alpha2/`
  - [x] v1alpha1 → v1alpha2 conversion webhook —
    `api/v1alpha2/conversion.go`
  - [x] 5 个 CRD (Valkey、ValkeyCluster、ValkeyBackup、ValkeyRestore、
    ValkeyBackupTarget)
  - Verify: `kubectl apply -f <v1alpha1.yaml>` 后确认其作为 v1alpha2 对象保存

- [x] **PVC 在线扩容 (Online PVC resize)** —
  `internal/controller/pvc_resize.go`

- [x] **Webhook 准入校验 (5 CRD)** —
  `internal/webhook/v1alpha2/`
  - [x] RBD storageClass 基础校验 —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateStorageClassName` (DNS-1123 subdomain)
  - [x] Topology-spread 一致性校验 —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateTopologySpread` (MaxSkew / TopologyKey /
    WhenUnsatisfiable / 重复 key、#77)
  - [ ] 将 replicaCount 下限校验接入 webhook
  - Verify: invalid spec 被 webhook 拒绝

- [x] **加密审计 (TLS / 加密监控)** —
  `internal/controller/encryption_audit.go`、
  `encryption_enforce_test.go`

### 运维与交付

- [x] Helm chart 已发布 — `keiailab.github.io/valkey-operator`
- [x] 3-repo (mongodb / postgres / valkey) 治理 (Governance) 资产
  对齐 (CODE_OF_CONDUCT / GOVERNANCE / MAINTAINERS / ROADMAP)
- [x] **GitHub Actions release pipeline 恢复** (ADR-0045) —
  面向外部开源 (OSS) 仓库,对 RFC-0002 的有限范围偏离;
  详见 [ADR-0045](docs/kb/adr/0045-restore-github-actions-for-oss-ci.md)
- [x] **SLSA-3 provenance + cosign keyless signing** 应用于镜像、
  Helm chart 与 SBOM (ADR-0046) — 验证命令见
  [SECURITY.md](.github/SECURITY.md)。自 v1.0.13 起生效。
- [ ] **生产集群采用**
  - [ ] CRD-install manifest
  - [ ] ArgoCD application 注册
  - [ ] 将生产 Valkey 工作负载从 plain StatefulSet 迁移至 operator
  - Verify: ArgoCD Synced/Healthy 且
    `kubectl get valkey/valkeycluster -A`
- [x] **迁移手册 (Migration runbook)** — plain StatefulSet → ValkeyCluster CR (PR #136)
  - [x] 记录零停机 (zero-downtime) 流程 — `docs/migration/zero-downtime.md` (PR #136)
  - [x] 基于 secondary-promote 的 cutover — `docs/migration/secondary-promote.md` (PR #136)
  - [x] 回滚流程 — `docs/migration/rollback.md` (PR #136)
  - Verify: staging dry-run + RTO / RPO 测量结果记录
- [x] **release-smoke-test.sh** — 移植自 mongodb-operator 模式 (PR #136)
  - [x] 5 个阶段: image / SBOM / trivy / chart index / smoke — `scripts/release-smoke-test.sh` (PR #136)
  - Verify: `bash hack/release-smoke-test.sh <tag>` 12/12 PASS

### 可观测性与安全

- [x] **Prometheus ServiceMonitor 自动** —
  `internal/resources/servicemonitor.go`、
  `servicemonitor_test.go`、chart
  `metrics.serviceMonitor.enabled=true`
- [x] **OpenSSF Scorecard + dependency-review + CodeQL SAST + DCO
  workflows** — 详见 `.github/workflows/`
- [x] Grafana 仪表板 (cluster shard 分布 / replication (PR open)
  lag / memory pressure)
  - [x] 4 个面板: cluster overview、replication、memory、latency — `charts/valkey-operator/dashboards/{cluster-overview,replication,memory,latency}.json` (PR open)
  - [x] Helm chart ConfigMap 集成 — `charts/valkey-operator/templates/grafana-dashboards.yaml` (PR open)
- [ ] OpenTelemetry trace 传播
  - [ ] 为 controller reconcile span 插桩 (instrument)
  - [ ] 接入 OTLP exporter
- [x] 镜像 SBOM (SPDX) + trivy HIGH/CRITICAL fixed-only 扫描 (PR open)
  - [x] 采用 3-repo 共享脚本 — `scripts/sbom-attach.sh` (PR open)
  - [x] release 时自动附加 — `cosign attest` + `gh release upload` (PR open)

## 下一阶段 (2.x 线 — Planning)

### 功能

- [ ] **Valkey 9.x 新功能跟进** — flag / cluster-mode
  变更
- [ ] **多集群联邦 (Multi-cluster federation)**
  - [ ] ClusterRole 分离
  - [ ] 拓扑感知路由
  - [ ] 新 CRD `ValkeyFederation`
- [ ] **跨区域备份复制 (Cross-region backup replication)**
  - [ ] S3 SSE-KMS 密钥管理
  - [ ] 自动 lifecycle policy
- [ ] **在线无 schema 迁移 (Online schema-less migration)**
  - [ ] RDB diff 工具
  - [ ] LWW 冲突解决 (conflict resolution)
- [ ] **加权读副本路由 (Weighted read-replica routing)** (latency-aware)

### 架构

- [ ] **Controller v2**
  - [ ] workqueue rate-limiter 调优
  - [ ] reconcile fan-out 优化
- [ ] **CRD v1 毕业 (graduation)**
  - [ ] Schema 稳定化
  - [ ] v1alpha2 → v1 conversion webhook
  - Verify: 6 个月内零 BREAKING CHANGE,且 3-repo
    兼容

## Non-Goals (有意排除的范围)

- ❌ **多租户隔离 (Multi-tenancy isolation)** — 仅 namespace 级别。更强的
  隔离应由独立集群承担。
- ❌ **内置密钥轮换 (secret rotation) 逻辑** — 委托给 ESO
  (External Secrets Operator) + OpenBao。
- ❌ **Sentinel mode** — 不支持 Redis-Sentinel
  兼容。Cluster mode 是前进路线。
- ❌ **基于日历的路线图 deadline** — 见
  `standards/workflow.md`。

## 变更日志

| 日期 | 变更 | 引用 |
|---|---|---|
| 2026-05-12 | English 成为正本;韩文保留为 `ROADMAP.ko.md`;ADR-0045 (GH Actions 恢复) + ADR-0046 (SLSA-3 + cosign) 在运维与安全章节标注 | i18n initiative |
| 2026-05-11 | 添加 webhook `validateStorageClassName` — RBD storageClass DNS-1123 基础校验 `[x]` | ralph-loop iter#2 |
| 2026-05-11 | 全量重写 — 事实修正 (ServiceMonitor 等)、更细粒度的 sub-task、暴露新项 (VolumeSnapshot multipod、conversion webhook) | parallel-leaping-seal plan |
| 2026-05-07 | 文档创建 — 3-repo 治理资产对齐 | INC-2026-05-07 |

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
