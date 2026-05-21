<p align="center">
  <img src="https://keiailab.com/assets/logo.svg" alt="keiailab" width="120"/>
</p>

# valkey-operator

> **Kubernetes 的 Apache-2.0 Valkey Operator — Standalone + Cluster + 备份/恢复,BSD-3 license-clean**

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"/></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go Version"/></a>
  <a href="https://valkey.io/"><img src="https://img.shields.io/badge/Valkey-8.0+-FF4438?logo=redis" alt="Valkey"/></a>
  <a href="https://kubernetes.io/"><img src="https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes" alt="Kubernetes"/></a>
  <a href="https://github.com/keiailab/valkey-operator/pkgs/container/valkey-operator"><img src="https://img.shields.io/badge/ghcr.io-keiailab%2Fvalkey--operator-blue?logo=github" alt="Container Image"/></a>
  <a href="https://keiailab.github.io/valkey-operator"><img src="https://img.shields.io/badge/dynamic/yaml?url=https://raw.githubusercontent.com/keiailab/valkey-operator/main/charts/valkey-operator/Chart.yaml&label=helm%20v" alt="Helm Chart"/></a>
  <a href="https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/keiailab-valkey-operator" alt="Artifact Hub"/></a>
  <a href="https://scorecard.dev/viewer/?uri=github.com/keiailab/valkey-operator"><img src="https://api.scorecard.dev/projects/github.com/keiailab/valkey-operator/badge" alt="OpenSSF Scorecard"/></a>
  <a href="https://github.com/keiailab/valkey-operator/discussions"><img src="https://img.shields.io/github/discussions/keiailab/valkey-operator?label=discussions&logo=github" alt="GitHub Discussions"/></a>
  <a href="https://github.com/keiailab/operator-commons/blob/main/docs/quality/audit-history.md"><img src="https://img.shields.io/badge/keiailab-v3.x--stable-success?style=flat-square" alt="keiailab v3.x-stable"/></a>
  <a href="https://github.com/keiailab/operator-commons/blob/main/scripts/audit-production-grade.sh"><img src="https://img.shields.io/badge/audit-100%25-success?style=flat-square" alt="audit"/></a>
</p>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.ja.md">日本語</a> |
  <b>中文</b>
</p>

---

[Valkey](https://valkey.io/) (Redis 的 BSD-3 fork) 的基于 Kubebuilder
的 Kubernetes Operator。一个 controller 在统一的 CRD 接口下管理 3 种
运维拓扑。

| CRD | 用途 | 拓扑 |
|---|---|---|
| `Valkey` | 单实例,或 1 个 primary 与 N 个 replica | Standalone / Replication |
| `ValkeyCluster` | 分片 Valkey Cluster (16384 个 slot) | 3+ 分片 × (1 primary + 0–5 replica) |
| `ValkeyBackup` | 一次性 RDB 或 AOF 备份 | PVC (`<backup>-backup`),外部存储可选 |
| `ValkeyBackupTarget` | S3 兼容的外部存储抽象 | Backup 和 Restore 共享 (ADR-0016) |
| `ValkeyRestore` | 将 RDB 恢复到 Valkey 或 ValkeyCluster 实例 | Init Container 模式 (ADR-0015) |

Operator reconcile `StatefulSet`、`ConfigMap`、`Secret`、`Service`
(headless + ClusterIP)、`PodDisruptionBudget`、`NetworkPolicy`、
`cert-manager` `Certificate` 和 Prometheus `ServiceMonitor` — 全部
带 spec-drift 检测。

## 快速开始 (kind)

以下命令在每次 release 都经过验证,kind 集群引导是本地开发的标准
路径。

### 1. 前置要求

| 工具 | 最低版本 | 备注 |
|---|---|---|
| Go | 1.26 | 与 `go.mod` 一致 |
| Docker | 24+ | buildx 默认 builder |
| kind | 0.27+ | 本地集群 |
| kubectl | 1.34+ | k3s/kind 兼容 |
| cert-manager | 1.16+ | Webhook serving 证书 |

### 2. kind 集群 + cert-manager

```sh
make setup-test-e2e
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy --all
```

### 3. 构建、加载、部署

```sh
make docker-build IMG=valkey-operator:dev
kind load docker-image valkey-operator:dev --name valkey-operator-test-e2e
make install                          # CRD
make deploy IMG=valkey-operator:dev   # operator + RBAC + webhook
kubectl -n valkey-operator-system rollout status deploy/valkey-operator-controller-manager
```

### 4. 应用样例 CR

```sh
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeycluster.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeybackup.yaml
```

### 5. 数据面 (data-plane) 冒烟测试

```sh
PASS=$(kubectl get secret valkey-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" ping     # PONG
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" set k v  # OK
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" get k    # v

# Cluster 模式 — `-c` 自动追随 MOVED 重定向
PASS=$(kubectl get secret valkeycluster-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkeycluster-sample-0 -- valkey-cli -a "$PASS" cluster info | head -3
# cluster_state:ok / cluster_slots_assigned:16384 / cluster_slots_ok:16384
```

## Helm

```sh
helm repo add valkey-operator https://keiailab.github.io/valkey-operator
helm install valkey-operator valkey-operator/valkey-operator \
    --namespace valkey-operator-system --create-namespace
```

Chart 也发布到
[Artifact Hub](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator)
并附 `Signed` trust badge (ADR-0044、ADR-0046)。

## 主要功能

- **3 种拓扑,1 个 Operator。** Standalone、Replication 和 Valkey
  Cluster 共享同一套 reconciler,具有统一的 status 表面。
- **Replication 模式自动 failover** — 选择 `master_repl_offset` 最大
  的 replica,以 `REPLICAOF NO ONE` 提升为 primary (ADR-0017)。
- **备份 / 恢复 (Backup / Restore)** — 将 RDB 或 AOF 保存至 PVC、S3,
  或任何 S3 兼容端点 (MinIO、Ceph RGW)。Restore 使用 Init Container
  模式,使主容器透明地加载 RDB (ADR-0015、ADR-0016、ADR-0022、ADR-0023)。
- **TLS + mTLS** — 通过 cert-manager 自动发现 (ADR-0010、ADR-0014),
  或用户提供的 `Secret`。
- **认证 (Authentication) 常开。** `Auth.Enabled` 未设置时自动生成
  32-byte 随机密码 (ADR-0013)。
- **NetworkPolicy** — opt-in,将 pod 间流量限制为 6379/16379
  (由 CNI 强制)。
- **可观测性 (Observability)。** 22 span 的 OTEL tracing
  (`OTEL_EXPORTER_OTLP_ENDPOINT` 未设置时 overhead 为 0)、
  Prometheus 告警规则、ServiceMonitor 自动生成。
- **供应链 (Supply chain)。** 自 v1.0.13 起 SBOM (syft SPDX) +
  Trivy 扫描 + cosign keyless 签名 + SLSA-3 provenance (ADR-0046)。
  验证命令见 [SECURITY.md](SECURITY.md)。

## 文档

| 主题 | 位置 |
|---|---|
| 文档中心 (所有文档) | [docs/README.md](docs/README.md) |
| Runbook (备份、恢复、扩缩、升级、应急) | [docs/operations/runbook.md](docs/operations/runbook.md) |
| Release 预检清单 | [docs/operations/release-checklist.md](docs/operations/release-checklist.md) |
| Architecture Decision Records | [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md) |
| 贡献指南 | [CONTRIBUTING.md](CONTRIBUTING.md) |
| 安全策略 + artifact 验证 | [SECURITY.md](SECURITY.md) |
| 项目治理 | [GOVERNANCE.md](GOVERNANCE.md) |
| 采用者 | [ADOPTERS.md](ADOPTERS.md) |

## 生产就绪 (Production readiness)

本 Operator 处于 `v1alpha1`,但具备商业产品级别的质量体系:

- **29 个 SSOT-parity 门** — alert / runbook / RBAC / CRD / sample /
  chart artifact 的 drift 由 lefthook pre-push 阻止。
- **Chart-CRD 自动同步** — `make manifests` 自动执行;
  `go mod tidy` 过期时 `git push` 阻止。
- **微基准 (Microbenchmark)** — 测量 5 个 hot-path parser
  (`go test -bench=. ./internal/valkey/`)。
- **Operator runbook** — 9 章 + 每个 alert 的 Trigger / Diagnosis /
  Mitigation / Escalation。
- **供应链。** Apache-2.0 license、PGP 签名的安全公告、自 v1.0.13 起
  Helm chart + image 带签名。
- **复用约定 (Reusable conventions)** — 在兄弟 Operator
  `mongodb-operator`、`postgres-operator`、`operator-commons` 之间共享。

## 路线图 (Roadmap)

Roadmap 是定性的,无日历承诺。进度按功能完成而非按季度跟踪。

已发布 (alpha):

- ✅ Standalone / Replication / ValkeyCluster 拓扑
- ✅ 备份到 PVC 和 S3 兼容存储
- ✅ 通过 Init Container 恢复 (ADR-0015)
- ✅ Replication 自动 failover (ADR-0017)
- ✅ Prometheus alerts + runbook
- ✅ OTEL tracing
- ✅ Helm chart + Artifact Hub 发布

下一步:

- [ ] kind + MinIO 上的 end-to-end 自动化
- [ ] ValkeyCluster 自动 reshard (ADR-0018)
- [ ] Replication 模式的 HPA 集成 (ADR-0027,推迟到 v1alpha1 稳定后)
- [ ] 用于 `v1beta1` 的 conversion webhook (ADR-0026,推迟)
- [ ] Track A/B/E 稳定与 24 小时 soak 测试后的首个 `v0.1.0` GA

决策依据见
[docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md)。功能请求请提至
[Issues](https://github.com/keiailab/valkey-operator/issues) 或
GitHub Discussions。

## 已知限制

本软件处于 `v1alpha1`,每次 release 都经过验证,但尚未 GA。当前已知
注意事项:

- `Spec.Auth.Enabled=false` 作为 no-op 处理 — Operator 始终配置 auth
  (ADR-0013)。如需无认证集群,请勿部署本 Operator。
- 仅 IPv6 环境未验证 — `CLUSTER MEET` 优先使用 IPv4 hostname
  (ADR-0012)。
- `NetworkPolicy.Enabled` 仅 emit 资源,**实际**强制取决于 policy-aware
  CNI (Calico、Cilium)。
- Replication 自动 failover 对网络分区下的 split-brain 不提供强保证 —
  trade-off 见 ADR-0017。
- ValkeyCluster restore 要求源 PVC 的 accessMode 为 `ReadOnlyMany` 或
  `ReadWriteMany` — 不支持 RWO。
- 未使用 `cluster-announce-hostname`;如运行在以与 in-cluster DNS 不同
  方式将 pod hostname 解析为 routable IP 的 Kubernetes-aware DNS 服务
  上,请重新评估。

更完整的韩语清单见
[README.ko.md → 잠재적 운영 이슈](README.ko.md#잠재적-운영-이슈-현재-알려진-한계)。

## 卸载

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

## 贡献

参见 [CONTRIBUTING.md](CONTRIBUTING.md)。欢迎外部贡献。对于非平凡的
变更,请先 open issue 以便在编写代码前就 API 表面达成一致。

`make help` 可查看所有 Makefile target。背景阅读:
[Kubebuilder book](https://book.kubebuilder.io/introduction.html)。

## 漏洞报告

请**不要**开公开 issue。使用 [SECURITY.md](SECURITY.md) 的私密渠道 —
GitHub Security Advisory 或 `security@keiailab.com` (PGP key 在
`artifacthub-repo.yml`)。

## 许可证 (License)

Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0
(<http://www.apache.org/licenses/LICENSE-2.0>)。以 AS IS 方式分发,
不附任何种类的 warranty 或 condition。完整文本见 [LICENSE](LICENSE)。

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
