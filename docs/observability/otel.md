# OpenTelemetry Integration Guide

> valkey-operator OTel trace propagation 도입 가이드. controller reconcile span + OTLP exporter + Jaeger backend.

## Architecture

```
Reconciler (main.go) ──► OTel SDK ──► OTLP Exporter ──► Collector ──► Jaeger
        │                                  │
        └── reconcile span:
            valkey.reconcile  (Valkey/ValkeyCluster CR)
            ├── statefulset.apply  (child span)
            ├── service.apply
            ├── configmap.apply
            ├── secret.apply
            └── status.update
```

## C.6.1 Controller reconcile span instrumentation

Wire OTel SDK in `cmd/manager/main.go`:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func setupOTel(ctx context.Context, endpoint string) (func(), error) {
    exp, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil { return nil, err }
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(resource.NewSchemaless(
            semconv.ServiceNameKey.String("valkey-operator"),
        )),
    )
    otel.SetTracerProvider(tp)
    return func() { _ = tp.Shutdown(ctx) }, nil
}
```

In reconciler:

```go
tracer := otel.Tracer("valkey-operator")
ctx, span := tracer.Start(ctx, "valkey.reconcile",
    trace.WithAttributes(attribute.String("name", req.Name)),
)
defer span.End()
```

## C.6.2 OTLP exporter wiring

Flag in `cmd/manager/main.go`:

```go
flag.StringVar(&otelEndpoint, "otel-endpoint", "",
    "OTLP gRPC endpoint (host:4317). 미설정 시 OTel 비활성.")
```

Helm `values.yaml`:

```yaml
observability:
  otel:
    enabled: false
    endpoint: "opentelemetry-collector.observability:4317"
```

## C.6.3 Jaeger view e2e

```bash
# Local kind + Jaeger all-in-one
helm install jaeger jaegertracing/jaeger-all-in-one
# operator with OTel enabled
helm upgrade valkey-operator . --set observability.otel.enabled=true \
    --set observability.otel.endpoint=jaeger:4317

# Trigger reconcile (CR apply) + view in Jaeger UI
kubectl apply -f config/samples/valkey-instance.yaml
kubectl port-forward svc/jaeger-query 16686:16686
open http://localhost:16686
```

## SLO

- Reconcile span overhead: **≤ 5ms** per reconcile
- Trace export lag: **≤ 10s** (Batcher default)
- Sample rate: 100% in dev, 1% in prod (configurable via `OTEL_TRACES_SAMPLER_ARG`)

## Refs

- ROADMAP.md L161-163 (P-C.6.1 + C.6.2 + C.6.3)
- OpenTelemetry Go SDK: https://github.com/open-telemetry/opentelemetry-go
