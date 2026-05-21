# OpenTelemetry 統合ガイド

> English: [otel.md](otel.md) — canonical / 正本

> valkey-operator の OTel trace 伝播ガイド。Controller の reconcile span
> + OTLP gRPC exporter + Jaeger / Tempo / Honeycomb バックエンド。

ADR-0025 (OTel tracer provider optional)。exporter は **任意** であり、
`OTEL_EXPORTER_OTLP_ENDPOINT` が未設定であれば operator は no-op tracer に
fallback し、**オーバーヘッドはゼロ** となる。設定されている場合のみ、
すべての reconcile loop とクリティカルパスの呼び出しが OTLP gRPC span
を emit する。

## アーキテクチャ

```
Reconciler (cmd/manager/main.go) ──► OTel SDK ──► OTLP Exporter ──► Collector ──► Jaeger / Tempo / Honeycomb
        │                                  │
        └── reconcile span (kind/Reconcile)
            ├── child span — 外部呼び出し (Failover/INFO_replication, ...)
            ├── child span — sub-phase (ValkeyBackup/Copying, ...)
            └── ...
```

## 有効化

### Helm chart 利用者

```sh
helm upgrade valkey-operator charts/valkey-operator \
  --set tracing.endpoint=tempo.observability.svc:4317 \
  --set tracing.serviceName=valkey-operator
```

`tracing.endpoint` が空のときは OTel は非アクティブのままで、no-op tracer
が使用され、**オーバーヘッドはゼロ** となる。

### Kustomize / 素の manifest 利用者

operator Deployment の env に以下を追加する:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "tempo.observability.svc:4317"  # Tempo / Jaeger / Honeycomb 互換
  - name: OTEL_SERVICE_NAME
    value: "valkey-operator"
  # 任意: OTEL_RESOURCE_ATTRIBUTES=env=prod,team=cache
```

## Trace 階層 — 22 span

operator は 5 つの controller にまたがる **22 種類の span** を emit する。
件数と source 位置は live のコードベース (`internal/observability/tracing.go`
の helper `StartReconcileSpan` / `StartCallSpan`) と突き合わせ済み。

### Reconcile root span (5 件)

| Span | Source | 用途 |
|---|---|---|
| `Valkey/Reconcile` | `internal/controller/valkey_controller.go` | Valkey CR の reconcile loop |
| `ValkeyCluster/Reconcile` | `internal/controller/valkeycluster_controller.go` | ValkeyCluster CR の reconcile loop |
| `ValkeyBackup/Reconcile` | `internal/controller/valkeybackup_controller.go` | ValkeyBackup CR の reconcile loop |
| `ValkeyRestore/Reconcile` | `internal/controller/valkeyrestore_controller.go` | ValkeyRestore CR の reconcile loop |
| `ValkeyBackupTarget/Reconcile` | `internal/controller/valkeybackuptarget_controller.go` | ValkeyBackupTarget CR の reconcile loop |

reconcile root には必ず `k8s.namespace` / `k8s.name` / `k8s.kind` 属性
(`semconv` 準拠) が付与される。

### Replication failover (3 件)

| Span | Source | 用途 |
|---|---|---|
| `Failover/INFO_replication` | `internal/controller/failover.go` | 各 replica から `master_repl_offset` を収集 |
| `Failover/PromoteToPrimary` | `internal/controller/failover.go` | 選出された replica に対し `REPLICAOF NO ONE` を発行 |
| `Failover/EnsureReplicaOf_all` | `internal/controller/failover.go` | 生存している replica を新 primary に再接続 |

ADR-0017 (replication failover — offset が最大の replica を昇格)。

### Backup pipeline (4 件)

| Span | Source | 用途 |
|---|---|---|
| `ValkeyBackup/TriggerBGSAVE` | `internal/controller/valkeybackup_controller.go` | primary に対し `BGSAVE` を発行 |
| `ValkeyBackup/LASTSAVE` | `internal/controller/valkeybackup_controller.go` | snapshot timestamp が進むまで `LASTSAVE` を polling |
| `ValkeyBackup/Copying` | `internal/controller/valkeybackup_controller.go` | Job ベースの PVC コピー (ADR-0015 init-container source) |
| `ValkeyBackup/Uploading` | `internal/controller/valkeybackup_controller.go` | S3 / GCS / Azure Blob への upload Job (ADR-0016 + ADR-0023) |

### Restore pipeline (5 件)

| Span | Source | 用途 |
|---|---|---|
| `ValkeyRestore/Mounting` | `internal/controller/valkeyrestore_controller.go` | PVC / 外部 source からのダウンロード |
| `ValkeyRestore/EnsureTargetRefSource` | `internal/controller/valkeyrestore_controller.go` | `Spec.Source.TargetRef` 向けのダウンロード Job を起動 |
| `ValkeyRestore/Restoring` | `internal/controller/valkeyrestore_controller.go` | STS の init-container を patch し rolling restart |
| `ValkeyRestore/Verifying` | `internal/controller/valkeyrestore_controller.go` | STS を revert (init container を除去) + `paused` annotation 解除 |
| `ValkeyRestore/VerifyDataPlane` | `internal/controller/valkeyrestore_controller.go` | restore 後の `INFO keyspace` (`Status.RestoredKeys` を populate) |

### Cluster lifecycle (4 件)

| Span | Source | 用途 |
|---|---|---|
| `ValkeyCluster/EnsureClusterMeet` | `internal/controller/valkeycluster_controller.go` | `CLUSTER MEET` + `ADDSLOTS` + `REPLICATE` の bootstrap |
| `ValkeyCluster/CreateCluster` | `internal/controller/valkeycluster_controller.go` | `vk.CreateCluster` (`EnsureClusterMeet` の入れ子) |
| `ValkeyCluster/QueryAnyNode` | `internal/controller/valkeycluster_controller.go` | `INFO` + `CLUSTER NODES` の polling |
| `ValkeyCluster/GracefulTeardown` | `internal/controller/valkeycluster_controller.go` | Finalizer 起点の `CLUSTER FORGET` |

### Backup target の reachability (1 件)

| Span | Source | 用途 |
|---|---|---|
| `ValkeyBackupTarget/BucketExists` | `internal/controller/valkeybackuptarget_controller.go` | S3 / GCS / Azure への到達確認 ping (`Status.Phase=Reachable` を立てる) |

## 実装

tracer は `internal/observability/tracing.go` で配線されている:

```go
const TracerName = "github.com/keiailab/valkey-operator"

// Reconcile root span.
func StartReconcileSpan(ctx context.Context, kind, namespace, name string) (context.Context, trace.Span) {
    ctx, span := otel.Tracer(TracerName).Start(ctx, kind+"/Reconcile")
    span.SetAttributes(
        attribute.String("k8s.namespace", namespace),
        attribute.String("k8s.name", name),
        attribute.String("k8s.kind", kind),
    )
    return ctx, span
}

// Child span (外部呼び出し / 重い計算).
func StartCallSpan(ctx context.Context, name string) (context.Context, trace.Span) {
    return otel.Tracer(TracerName).Start(ctx, name)
}
```

呼び出し側はいずれも `defer span.End()` を入れている。

## エンドツーエンド検証

```bash
# ローカル kind + Jaeger all-in-one
helm install jaeger jaegertracing/jaeger-all-in-one

# OTel を有効にした operator
helm upgrade valkey-operator charts/valkey-operator \
  --set tracing.endpoint=jaeger:4317

# reconcile を発火させ Jaeger UI で確認
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl port-forward svc/jaeger-query 16686:16686
open http://localhost:16686
```

## 運用上の使い方

- **レイテンシ p95 / p99** — span 単位の duration が phase レベルの SLO
  を駆動する。reconcile root span のオーバーヘッド目標: **5 ms 以下**、
  trace export 遅延 (Batcher のデフォルト): **10 s 以下**。
- **エラーレート** — `span.RecordError` がクリティカルパスの失敗を分類する。
- **階層** — trace UI は reconcile loop 配下の child span の時間分布を可視化し、
  ボトルネックを浮き彫りにする。
- **サンプリング** — dev で 100 %、prod で 1 % (`OTEL_TRACES_SAMPLER_ARG`
  で調整可能)。

## 既知の制約

- **gRPC のみ** (`otlptracegrpc`)。HTTP exporter は未配線 — 別 ADR で扱う。
- **TLS / OAuth 認証なし** — exporter には未実装で、これも別 ADR の対象。
- **アプリケーションレベル metric** は本 tracer の責務外。
  controller-runtime の built-in metric と operator 側の
  `valkey_cluster_*` Prometheus exposition が該当する
  ([`operations/metrics-glossary.md`](../operations/metrics-glossary.md)
  を参照)。

## 参考

- [ADR-0025](../kb/adr/0025-otel-tracer-provider-optional.md) — OTel
  tracer provider を任意化し、未設定時はゼロオーバーヘッドの no-op に倒す
- `internal/observability/tracing.go` — `SetupTracing`,
  `StartReconcileSpan`, `StartCallSpan`
- OpenTelemetry Go SDK — <https://github.com/open-telemetry/opentelemetry-go>
