# valkey-operator (中文)

> English README: [README.md](README.md) — canonical / 正本
>
> 한국어 README: [README.ko.md](README.ko.md)
>
> 日本語 README: [README.ja.md](README.ja.md)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org/)
[![Valkey](https://img.shields.io/badge/Valkey-8.0+-FF4438?logo=redis)](https://valkey.io/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes)](https://kubernetes.io/)
[![Container Image](https://img.shields.io/badge/ghcr.io-keiailab%2Fvalkey--operator-blue?logo=github)](https://github.com/keiailab/valkey-operator/pkgs/container/valkey-operator)
[![Helm Chart](https://img.shields.io/badge/dynamic/yaml?url=https://raw.githubusercontent.com/keiailab/valkey-operator/main/charts/valkey-operator/Chart.yaml&label=helm%20v)](https://keiailab.github.io/valkey-operator)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/keiailab-valkey-operator)](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/keiailab/valkey-operator/badge)](https://scorecard.dev/viewer/?uri=github.com/keiailab/valkey-operator)
[![GitHub Discussions](https://img.shields.io/github/discussions/keiailab/valkey-operator?label=discussions&logo=github)](https://github.com/keiailab/valkey-operator/discussions)

基于 Kubebuilder 的 Kubernetes Operator。统一的 CRD 表面之下,由单一的 controller 集合管理 [Valkey](https://valkey.io/) (Redis 的 BSD-3 fork) 的 3 种运维拓扑。

| CRD | 用途 | 拓扑 |
|---|---|---|
| `Valkey` | 单实例,或 1 primary + N replica | Standalone / Replication |
| `ValkeyCluster` | 分片 (sharded) 的 Valkey Cluster (16384 个 slot) | 3+ 个 shard × (1 primary + 0~5 个 replica) |
| `ValkeyBackup` | 一次性的 RDB 或 AOF 备份 | PVC (`<backup>-backup`),外部存储可选 |
| `ValkeyBackupTarget` | S3 兼容外部存储的抽象 | Backup 和 Restore 之间共享 (ADR-0016) |
| `ValkeyRestore` | 将 RDB 恢复到 Valkey 或 ValkeyCluster 实例 | Init Container 模式 (ADR-0015) |

Operator 调谐 (reconcile) `StatefulSet`、`ConfigMap`、`Secret`、`Service` (headless + ClusterIP)、`PodDisruptionBudget`、`NetworkPolicy`、`cert-manager` 的 `Certificate` 以及 Prometheus 的 `ServiceMonitor` — 全部具备 spec drift 检测能力。

## 快速开始 (kind)

以下所有命令在每个 release 都经过验证。kind 集群 bootstrap 是 canonical 的本地开发路径。

### 1. 前提条件

| 工具 | 最低版本 | 备注 |
|---|---|---|
| Go | 1.26 | 与 `go.mod` 一致 |
| Docker | 24+ | buildx default builder |
| kind | 0.27+ | 本地集群 |
| kubectl | 1.34+ | k3s / kind 兼容 |
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
make install                          # CRDs
make deploy IMG=valkey-operator:dev   # operator + RBAC + webhook
kubectl -n valkey-operator-system rollout status deploy/valkey-operator-controller-manager
```

### 4. 应用示例 CR

```sh
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeycluster.yaml
kubectl apply -f config/samples/cache_v1alpha1_valkeybackup.yaml
```

### 5. 数据平面冒烟测试

```sh
PASS=$(kubectl get secret valkey-sample-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" ping     # PONG
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" set k v  # OK
kubectl exec valkey-sample-0 -- valkey-cli -a "$PASS" get k    # v

# Cluster 模式 — `-c` 会自动跟随 MOVED 重定向
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

Chart 也发布在 [Artifact Hub](https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator),带 `Signed` 信任徽章 (ADR-0044、ADR-0046)。

## 主要功能

- **3 种拓扑,1 个 Operator。** Standalone、Replication 和 Valkey Cluster 全部共享同一个 reconciler 集合,具备统一的状态表面。
- **自动故障转移** (Replication 模式) — 选择 `master_repl_offset` 最大的 replica 作为候选,使用 `REPLICAOF NO ONE` 将其提升为 primary (ADR-0017)。
- **备份 / 恢复 (Backup / Restore)** — RDB 或 AOF 到 PVC、S3,或任意 S3 兼容 endpoint (MinIO、Ceph RGW)。Restore 采用 Init Container 模式,使主容器透明地加载 RDB (ADR-0015、ADR-0016、ADR-0022、ADR-0023)。
- **TLS + mTLS** — 通过 cert-manager 的自动发现 (ADR-0010、ADR-0014) 或用户提供的 `Secret`。
- **常驻认证 (Always-on auth)。** 当 `Auth.Enabled` 未设置时,会生成随机的 32 字节密码 (ADR-0013)。
- **NetworkPolicy** — 选择启用 (opt-in),将 pod 间流量限制为 6379 / 16379 (由 CNI 强制执行)。
- **可观测性 (Observability)。** 带 22 个 span 的 OTEL tracing (当 `OTEL_EXPORTER_OTLP_ENDPOINT` 未设置时零开销),Prometheus 告警规则,ServiceMonitor 自动生成。
- **供应链 (Supply chain)。** 自 v1.0.13 起,SBOM (syft SPDX) + Trivy 扫描 + cosign keyless 签名 + SLSA-3 provenance (ADR-0046)。验证命令请参阅 [SECURITY.md](SECURITY.md)。

## 文档

| 主题 | 位置 |
|---|---|
| 韩文详细演练 | [README.ko.md](README.ko.md) |
| Runbook (Backup、Restore、Scaling、Upgrade、Emergency) | [docs/operations/runbook.md](docs/operations/runbook.md) |
| Release 前检查清单 | [docs/operations/release-checklist.md](docs/operations/release-checklist.md) |
| Architecture Decision Records | [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md) |
| 贡献指南 | [CONTRIBUTING.md](CONTRIBUTING.md) |
| 安全策略与制品 (artifact) 验证 | [SECURITY.md](SECURITY.md) |
| 项目治理 | [GOVERNANCE.md](GOVERNANCE.md) |
| 采用者 (Adopters) | [ADOPTERS.md](ADOPTERS.md) |

## 生产就绪度

本 Operator 处于 `v1alpha1`,但具备商业产品级的质量体系:

- **29 个 SSOT 一致性 (parity) gate** — alert / runbook / RBAC / CRD / sample / chart 制品的 drift 由 lefthook 的 pre-push 阻断。
- **Chart - CRD 自动同步** — 通过 `make manifests` 执行;当 `go mod tidy` 处于陈旧 (stale) 状态时,`git push` 会被阻断。
- **微基准测试 (Microbenchmark)** — 针对 5 个热路径 (hot-path) 解析器 (`go test -bench=. ./internal/valkey/`)。
- **Operator runbook** — 9 个章节加上 per-alert 的 Trigger / Diagnosis / Mitigation / Escalation。
- **供应链。** Apache-2.0 许可证、PGP 签名的安全披露,自 v1.0.13 起 Helm chart + 镜像均已签名。
- **可复用约定** — 与姊妹 Operator (`mongodb-operator`、`postgres-operator`、`operator-commons`) 共享。

## Roadmap

下面的 roadmap 是定性的 — 不做日历承诺。进度按功能完成度追踪,不按季度划分。

已发布 (alpha):

- ✅ Standalone / Replication / ValkeyCluster 拓扑
- ✅ 备份到 PVC 和 S3 兼容存储
- ✅ 通过 Init Container 进行恢复 (ADR-0015)
- ✅ Replication 自动故障转移 (ADR-0017)
- ✅ Prometheus alert + runbook
- ✅ OTEL tracing
- ✅ Helm chart + Artifact Hub 发布

下一步:

- [ ] kind + MinIO 上的端到端 (end-to-end) 自动化
- [ ] ValkeyCluster 自动 re-sharding (ADR-0018)
- [ ] Replication 模式的 HPA 集成 (ADR-0027,在 v1alpha1 稳定之前推迟)
- [ ] `v1beta1` 的 conversion webhook (ADR-0026,推迟)
- [ ] Track A/B/E 稳定化 + 24 小时浸泡测试 (soak test) 后首个 `v0.1.0` GA

决策依据存放在 [docs/kb/adr/INDEX.md](docs/kb/adr/INDEX.md)。功能请求请提交到 [Issues](https://github.com/keiailab/valkey-operator/issues) 或 GitHub Discussions。

## 已知限制

本软件处于 `v1alpha1`,虽然在每个 release 都经过验证,但尚未达到 GA。当前已知的注意事项:

- `Spec.Auth.Enabled=false` 被视为 no-op — operator 始终会配置认证 (ADR-0013)。如果需要无认证 cluster,请勿部署本 operator。
- 仅 IPv6 的环境未经测试。`CLUSTER MEET` 优先使用 IPv4 主机名 (ADR-0012)。
- `NetworkPolicy.Enabled` 只是发出资源;*实际* 强制执行取决于支持策略的 CNI (Calico、Cilium)。
- Replication 自动故障转移在网络分区 (network partition) 下无法提供强 split-brain 保证 — 取舍请参阅 ADR-0017。
- ValkeyCluster 恢复要求源 PVC 的 accessMode 为 `ReadOnlyMany` 或 `ReadWriteMany`;RWO 不受支持。
- 不使用 `cluster-announce-hostname`。如果运行的 Kubernetes 感知 DNS 服务以与 operator 已使用的集群内 DNS 不同的方式将 pod 主机名解析为可路由 IP,请重新评估。

更完整的韩文清单位于 [README.ko.md → 잠재적 운영 이슈](README.ko.md#잠재적-운영-이슈-현재-알려진-한계)。

## 卸载

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

## 贡献

请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。欢迎外部贡献;对于任何非平凡 (non-trivial) 的变更,请先开 issue,以便我们在你写代码之前对齐 API 表面。

执行 `make help` 可查看所有 Makefile target。背景阅读:[Kubebuilder book](https://book.kubebuilder.io/introduction.html)。

## 漏洞报告

请 **不要** 开公开 issue。请使用 [SECURITY.md](SECURITY.md) 中的私有渠道 — GitHub Security Advisory 或 `security@keiailab.com` (PGP key 在 `artifacthub-repo.yml`)。

## 许可证

Copyright 2026 Keiailab.

本软件依据 Apache License, Version 2.0 (<http://www.apache.org/licenses/LICENSE-2.0>) 授权。以 "AS IS" 为基础分发,不附带任何明示或暗示的保证或条件。完整文本请参阅 [LICENSE](LICENSE) 文件。
