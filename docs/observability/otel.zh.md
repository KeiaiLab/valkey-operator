# OpenTelemetry 接入指南

> English: [otel.md](otel.md) — canonical / 正本

> valkey-operator 的 OTel 链路传播指南。控制器 reconcile span +
> OTLP gRPC exporter + Jaeger / Tempo / Honeycomb 后端。

ADR-0025 (OTel tracer provider 可选)。exporter 为 **可选**:当
`OTEL_EXPORTER_OTLP_ENDPOINT` 未设置时,operator 使用 no-op tracer,
**零开销**。一旦设置,每一次 reconcile 循环与每一个关键路径调用都
会发出 OTLP gRPC span。

## 架构

```
Reconciler (cmd/manager/main.go) ──► OTel SDK ──► OTLP Exporter ──► Collector ──► Jaeger / Tempo / Honeycomb
        │                                  │
        └── reconcile span (kind/Reconcile)
            ├── child span — 外部调用 (Failover/INFO_replication, ...)
            ├── child span — 子阶段 (ValkeyBackup/Copying, ...)
            └── ...
```

## 启用方式

### Helm chart 用户

```sh
helm upgrade valkey-operator charts/valkey-operator \
  --set tracing.endpoint=tempo.observability.svc:4317 \
  --set tracing.serviceName=valkey-operator
```

当 `tracing.endpoint` 为空时,OTel 保持未启用状态 (no-op tracer,
**零开销**)。

### Kustomize / 直接编写 manifest 的用户

在 operator Deployment 的 env 中追加:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "tempo.observability.svc:4317"  # 与 Tempo / Jaeger / Honeycomb 兼容
  - name: OTEL_SERVICE_NAME
    value: "valkey-operator"
  # 可选: OTEL_RESOURCE_ATTRIBUTES=env=prod,team=cache
```

## Span 层级 — 22 个 span

operator 在 5 个控制器中共发出 **22 个独立 span**。数量与源码位置
均已对照活跃代码库 (`internal/observability/tracing.go` 的
`StartReconcileSpan` / `StartCallSpan` helper) 进行核对。

### Reconcile 根 span (5)

| Span | 源文件 | 用途 |
|---|---|---|
| `Valkey/Reconcile` | `internal/controller/valkey_controller.go` | Valkey CR 的 reconcile 循环 |
| `ValkeyCluster/Reconcile` | `internal/controller/valkeycluster_controller.go` | ValkeyCluster CR 的 reconcile 循环 |
| `ValkeyBackup/Reconcile` | `internal/controller/valkeybackup_controller.go` | ValkeyBackup CR 的 reconcile 循环 |
| `ValkeyRestore/Reconcile` | `internal/controller/valkeyrestore_controller.go` | ValkeyRestore CR 的 reconcile 循环 |
| `ValkeyBackupTarget/Reconcile` | `internal/controller/valkeybackuptarget_controller.go` | ValkeyBackupTarget CR 的 reconcile 循环 |

每个 reconcile 根 span 都会携带 `k8s.namespace`、`k8s.name`、
`k8s.kind` 三个属性 (遵循 `semconv` 标准)。

### 复制集故障切换 (3)

| Span | 源文件 | 用途 |
|---|---|---|
| `Failover/INFO_replication` | `internal/controller/failover.go` | 从每个 replica 上采集 `master_repl_offset` |
| `Failover/PromoteToPrimary` | `internal/controller/failover.go` | 对当选 replica 下发 `REPLICAOF NO ONE` |
| `Failover/EnsureReplicaOf_all` | `internal/controller/failover.go` | 将存活的 replica 重新连接到新的 primary |

ADR-0017 (复制集故障切换 — 选取 offset 最大的 replica)。

### 备份流水线 (4)

| Span | 源文件 | 用途 |
|---|---|---|
| `ValkeyBackup/TriggerBGSAVE` | `internal/controller/valkeybackup_controller.go` | 对 primary 下发 `BGSAVE` |
| `ValkeyBackup/LASTSAVE` | `internal/controller/valkeybackup_controller.go` | 轮询 `LASTSAVE`,直至快照时间戳更新 |
| `ValkeyBackup/Copying` | `internal/controller/valkeybackup_controller.go` | 基于 Job 的 PVC 拷贝 (ADR-0015 init-container 源) |
| `ValkeyBackup/Uploading` | `internal/controller/valkeybackup_controller.go` | 上传至 S3 / GCS / Azure Blob 的 Job (ADR-0016 + ADR-0023) |

### 恢复流水线 (5)

| Span | 源文件 | 用途 |
|---|---|---|
| `ValkeyRestore/Mounting` | `internal/controller/valkeyrestore_controller.go` | PVC / 外部源的下载 |
| `ValkeyRestore/EnsureTargetRefSource` | `internal/controller/valkeyrestore_controller.go` | 为 `Spec.Source.TargetRef` 启动下载 Job |
| `ValkeyRestore/Restoring` | `internal/controller/valkeyrestore_controller.go` | STS init-container patch + 滚动重启 |
| `ValkeyRestore/Verifying` | `internal/controller/valkeyrestore_controller.go` | STS 还原 (移除 init container) + 移除 `paused` 注解 |
| `ValkeyRestore/VerifyDataPlane` | `internal/controller/valkeyrestore_controller.go` | 恢复后执行 `INFO keyspace` (填充 `Status.RestoredKeys`) |

### 集群生命周期 (4)

| Span | 源文件 | 用途 |
|---|---|---|
| `ValkeyCluster/EnsureClusterMeet` | `internal/controller/valkeycluster_controller.go` | `CLUSTER MEET` + `ADDSLOTS` + `REPLICATE` 引导 |
| `ValkeyCluster/CreateCluster` | `internal/controller/valkeycluster_controller.go` | `vk.CreateCluster` (嵌套于 `EnsureClusterMeet` 之下) |
| `ValkeyCluster/QueryAnyNode` | `internal/controller/valkeycluster_controller.go` | `INFO` + `CLUSTER NODES` 轮询 |
| `ValkeyCluster/GracefulTeardown` | `internal/controller/valkeycluster_controller.go` | 由 finalizer 驱动的 `CLUSTER FORGET` |

### 备份目标可达性 (1)

| Span | 源文件 | 用途 |
|---|---|---|
| `ValkeyBackupTarget/BucketExists` | `internal/controller/valkeybackuptarget_controller.go` | 对 S3 / GCS / Azure 的可达性探测 (置 `Status.Phase=Reachable`) |

## 实现

Tracer 在 `internal/observability/tracing.go` 中接入:

```go
const TracerName = "github.com/keiailab/valkey-operator"

// Reconcile 根 span。
func StartReconcileSpan(ctx context.Context, kind, namespace, name string) (context.Context, trace.Span) {
    ctx, span := otel.Tracer(TracerName).Start(ctx, kind+"/Reconcile")
    span.SetAttributes(
        attribute.String("k8s.namespace", namespace),
        attribute.String("k8s.name", name),
        attribute.String("k8s.kind", kind),
    )
    return ctx, span
}

// 子 span (外部调用 / 重计算)。
func StartCallSpan(ctx context.Context, name string) (context.Context, trace.Span) {
    return otel.Tracer(TracerName).Start(ctx, name)
}
```

每一个调用方都会 `defer` `span.End()`。

## 端到端验证

```bash
# 本地 kind + Jaeger all-in-one
helm install jaeger jaegertracing/jaeger-all-in-one

# 启用 OTel 的 operator
helm upgrade valkey-operator charts/valkey-operator \
  --set tracing.endpoint=jaeger:4317

# 触发一次 reconcile,在 Jaeger UI 中查看
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl port-forward svc/jaeger-query 16686:16686
open http://localhost:16686
```

## 运维用途

- **延迟 p95 / p99** — 每个 span 的耗时驱动阶段级 SLO。
  Reconcile 根 span 的开销目标:**≤ 5 ms**;链路导出延迟
  (Batcher 默认值):**≤ 10 s**。
- **错误率** — `span.RecordError` 对关键路径失败进行分类。
- **层级视图** — 链路 UI 在每个 reconcile 循环下呈现子 span 的
  耗时分布,便于定位瓶颈。
- **采样** — dev 100 %,prod 1 % (可通过
  `OTEL_TRACES_SAMPLER_ARG` 调整)。

## 已知限制

- **仅支持 gRPC** (`otlptracegrpc`)。HTTP exporter 尚未接入 —
  另起一份 ADR。
- **exporter 暂不支持 TLS / OAuth** 认证 — 另起一份 ADR。
- **应用层指标** 不在 tracer 的职责范围内。
  controller-runtime 内置 metrics + operator 自身的
  `valkey_cluster_*` Prometheus 指标负责覆盖
  (参阅 [`operations/metrics-glossary.md`](../operations/metrics-glossary.md))。

## 参考

- [ADR-0025](../kb/adr/0025-otel-tracer-provider-optional.md) — OTel
  tracer provider 可选 + 未设置时使用零开销的 no-op
- `internal/observability/tracing.go` — `SetupTracing`、
  `StartReconcileSpan`、`StartCallSpan`
- OpenTelemetry Go SDK — <https://github.com/open-telemetry/opentelemetry-go>
