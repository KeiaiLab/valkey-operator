# OpenTelemetry Integration Guide

> valkey-operator OTel trace propagation guide. Controller reconcile
> spans + OTLP gRPC exporter + Jaeger / Tempo / Honeycomb backend.

ADR-0025 (OTel tracer provider optional). The exporter is **optional**:
when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, the operator uses a no-op
tracer with **zero overhead**. When set, every reconcile loop and every
critical-path call emits an OTLP gRPC span.

## Architecture

```
Reconciler (cmd/manager/main.go) ──► OTel SDK ──► OTLP Exporter ──► Collector ──► Jaeger / Tempo / Honeycomb
        │                                  │
        └── reconcile span (kind/Reconcile)
            ├── child span — external call (Failover/INFO_replication, ...)
            ├── child span — sub-phase (ValkeyBackup/Copying, ...)
            └── ...
```

## Activation

### Helm chart users

```sh
helm upgrade valkey-operator charts/valkey-operator \
  --set tracing.endpoint=tempo.observability.svc:4317 \
  --set tracing.serviceName=valkey-operator
```

When `tracing.endpoint` is empty, OTel stays inactive (no-op tracer,
**zero overhead**).

### Kustomize / direct manifest users

Add to the operator Deployment env:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "tempo.observability.svc:4317"  # Tempo / Jaeger / Honeycomb compatible
  - name: OTEL_SERVICE_NAME
    value: "valkey-operator"
  # Optional: OTEL_RESOURCE_ATTRIBUTES=env=prod,team=cache
```

## Trace hierarchy — 22 spans

The operator emits **22 distinct spans** across the five controllers.
Counts and source locations verified against the live codebase
(`internal/observability/tracing.go` helpers `StartReconcileSpan` /
`StartCallSpan`).

### Reconcile root spans (5)

| Span | Source | Purpose |
|---|---|---|
| `Valkey/Reconcile` | `internal/controller/valkey_controller.go` | Valkey CR reconcile loop |
| `ValkeyCluster/Reconcile` | `internal/controller/valkeycluster_controller.go` | ValkeyCluster CR reconcile loop |
| `ValkeyBackup/Reconcile` | `internal/controller/valkeybackup_controller.go` | ValkeyBackup CR reconcile loop |
| `ValkeyRestore/Reconcile` | `internal/controller/valkeyrestore_controller.go` | ValkeyRestore CR reconcile loop |
| `ValkeyBackupTarget/Reconcile` | `internal/controller/valkeybackuptarget_controller.go` | ValkeyBackupTarget CR reconcile loop |

Every reconcile root carries `k8s.namespace`, `k8s.name`, `k8s.kind`
attributes (`semconv` standard).

### Replication failover (3)

| Span | Source | Purpose |
|---|---|---|
| `Failover/INFO_replication` | `internal/controller/failover.go` | Collect `master_repl_offset` from every replica |
| `Failover/PromoteToPrimary` | `internal/controller/failover.go` | Issue `REPLICAOF NO ONE` against the elected replica |
| `Failover/EnsureReplicaOf_all` | `internal/controller/failover.go` | Reconnect surviving replicas to the new primary |

ADR-0017 (replication failover — replica with largest offset).

### Backup pipeline (4)

| Span | Source | Purpose |
|---|---|---|
| `ValkeyBackup/TriggerBGSAVE` | `internal/controller/valkeybackup_controller.go` | Issue `BGSAVE` against the primary |
| `ValkeyBackup/LASTSAVE` | `internal/controller/valkeybackup_controller.go` | Poll `LASTSAVE` until snapshot timestamp moves |
| `ValkeyBackup/Copying` | `internal/controller/valkeybackup_controller.go` | Job-based PVC copy (ADR-0015 init-container source) |
| `ValkeyBackup/Uploading` | `internal/controller/valkeybackup_controller.go` | S3 / GCS / Azure Blob upload Job (ADR-0016 + ADR-0023) |

### Restore pipeline (5)

| Span | Source | Purpose |
|---|---|---|
| `ValkeyRestore/Mounting` | `internal/controller/valkeyrestore_controller.go` | PVC / external source download |
| `ValkeyRestore/EnsureTargetRefSource` | `internal/controller/valkeyrestore_controller.go` | Download Job spawn for `Spec.Source.TargetRef` |
| `ValkeyRestore/Restoring` | `internal/controller/valkeyrestore_controller.go` | STS init-container patch + rolling restart |
| `ValkeyRestore/Verifying` | `internal/controller/valkeyrestore_controller.go` | STS revert (remove init container) + remove `paused` annotation |
| `ValkeyRestore/VerifyDataPlane` | `internal/controller/valkeyrestore_controller.go` | `INFO keyspace` post-restore (populates `Status.RestoredKeys`) |

### Cluster lifecycle (4)

| Span | Source | Purpose |
|---|---|---|
| `ValkeyCluster/EnsureClusterMeet` | `internal/controller/valkeycluster_controller.go` | `CLUSTER MEET` + `ADDSLOTS` + `REPLICATE` bootstrap |
| `ValkeyCluster/CreateCluster` | `internal/controller/valkeycluster_controller.go` | `vk.CreateCluster` (nested in `EnsureClusterMeet`) |
| `ValkeyCluster/QueryAnyNode` | `internal/controller/valkeycluster_controller.go` | `INFO` + `CLUSTER NODES` polling |
| `ValkeyCluster/GracefulTeardown` | `internal/controller/valkeycluster_controller.go` | Finalizer-driven `CLUSTER FORGET` |

### Backup target reachability (1)

| Span | Source | Purpose |
|---|---|---|
| `ValkeyBackupTarget/BucketExists` | `internal/controller/valkeybackuptarget_controller.go` | S3 / GCS / Azure reachability ping (sets `Status.Phase=Reachable`) |

## Implementation

Tracer wired in `internal/observability/tracing.go`:

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

// Child span (external call / expensive computation).
func StartCallSpan(ctx context.Context, name string) (context.Context, trace.Span) {
    return otel.Tracer(TracerName).Start(ctx, name)
}
```

Every caller `defer`s `span.End()`.

## End-to-end verification

```bash
# Local kind + Jaeger all-in-one
helm install jaeger jaegertracing/jaeger-all-in-one

# Operator with OTel enabled
helm upgrade valkey-operator charts/valkey-operator \
  --set tracing.endpoint=jaeger:4317

# Trigger reconcile + view in Jaeger UI
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl port-forward svc/jaeger-query 16686:16686
open http://localhost:16686
```

## Operational use

- **Latency p95 / p99** — per-span duration drives phase-level SLOs.
  Reconcile root span overhead target: **≤ 5 ms**; trace export lag
  (Batcher default): **≤ 10 s**.
- **Error rate** — `span.RecordError` classifies critical-path failures.
- **Hierarchy** — the trace UI surfaces the child-span time
  distribution under each reconcile loop, exposing bottlenecks.
- **Sampling** — 100 % in dev, 1 % in prod (configurable via
  `OTEL_TRACES_SAMPLER_ARG`).

## Known limitations

- **gRPC only** (`otlptracegrpc`). HTTP exporter not wired —
  separate ADR.
- **No TLS / OAuth** authentication on the exporter — separate ADR.
- **Application-level metrics** are out of scope for the tracer.
  controller-runtime built-in metrics + the operator's
  `valkey_cluster_*` Prometheus exposition fill that role
  (see [`operations/metrics-glossary.md`](../operations/metrics-glossary.md)).

## Refs

- [ADR-0025](../kb/adr/0025-otel-tracer-provider-optional.md) — OTel
  tracer provider optional + zero-overhead no-op when unset
- `internal/observability/tracing.go` — `SetupTracing`,
  `StartReconcileSpan`, `StartCallSpan`
- OpenTelemetry Go SDK — <https://github.com/open-telemetry/opentelemetry-go>
