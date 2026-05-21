# OpenTelemetry 통합 가이드 (한국어)

> English: [otel.md](otel.md) — canonical / 정본

> valkey-operator 의 OTel trace propagation 가이드. Controller reconcile
> span + OTLP gRPC exporter + Jaeger / Tempo / Honeycomb 백엔드.

ADR-0025 (OTel tracer provider optional). exporter 는 **선택 사항**으로,
`OTEL_EXPORTER_OTLP_ENDPOINT` 가 설정되지 않으면 operator 는 no-op tracer 를
사용하여 **추가 비용이 전혀 없다**. 설정 시 모든 reconcile loop 와 critical
path 호출이 OTLP gRPC span 으로 송출된다.

## 아키텍처

```
Reconciler (cmd/manager/main.go) ──► OTel SDK ──► OTLP Exporter ──► Collector ──► Jaeger / Tempo / Honeycomb
        │                                  │
        └── reconcile span (kind/Reconcile)
            ├── child span — external call (Failover/INFO_replication, ...)
            ├── child span — sub-phase (ValkeyBackup/Copying, ...)
            └── ...
```

## 활성화

### Helm chart 사용자

```sh
helm upgrade valkey-operator charts/valkey-operator \
  --set tracing.endpoint=tempo.observability.svc:4317 \
  --set tracing.serviceName=valkey-operator
```

`tracing.endpoint` 가 비어 있으면 OTel 은 비활성 상태로 유지된다 (no-op
tracer, **추가 비용 전무**).

### Kustomize / 정적 manifest 사용자

operator Deployment 의 env 에 다음을 추가한다:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "tempo.observability.svc:4317"  # Tempo / Jaeger / Honeycomb compatible
  - name: OTEL_SERVICE_NAME
    value: "valkey-operator"
  # 선택: OTEL_RESOURCE_ATTRIBUTES=env=prod,team=cache
```

## Trace 계층 — 22 span

operator 는 5개 controller 에 걸쳐 **총 22개의 distinct span** 을 송출한다.
개수와 소스 위치는 실제 코드베이스에 대해 검증되었다
(`internal/observability/tracing.go` 의 helper `StartReconcileSpan` /
`StartCallSpan`).

### Reconcile root span (5개)

| Span | 위치 | 용도 |
|---|---|---|
| `Valkey/Reconcile` | `internal/controller/valkey_controller.go` | Valkey CR 의 reconcile loop |
| `ValkeyCluster/Reconcile` | `internal/controller/valkeycluster_controller.go` | ValkeyCluster CR 의 reconcile loop |
| `ValkeyBackup/Reconcile` | `internal/controller/valkeybackup_controller.go` | ValkeyBackup CR 의 reconcile loop |
| `ValkeyRestore/Reconcile` | `internal/controller/valkeyrestore_controller.go` | ValkeyRestore CR 의 reconcile loop |
| `ValkeyBackupTarget/Reconcile` | `internal/controller/valkeybackuptarget_controller.go` | ValkeyBackupTarget CR 의 reconcile loop |

모든 reconcile root 는 `k8s.namespace`, `k8s.name`, `k8s.kind` 속성을 갖는다
(`semconv` 표준).

### Replication failover (3개)

| Span | 위치 | 용도 |
|---|---|---|
| `Failover/INFO_replication` | `internal/controller/failover.go` | 모든 replica 로부터 `master_repl_offset` 수집 |
| `Failover/PromoteToPrimary` | `internal/controller/failover.go` | 선출된 replica 에 `REPLICAOF NO ONE` 발행 |
| `Failover/EnsureReplicaOf_all` | `internal/controller/failover.go` | 살아남은 replica 를 새 primary 로 재연결 |

ADR-0017 (replication failover — offset 이 가장 큰 replica 선출).

### Backup 파이프라인 (4개)

| Span | 위치 | 용도 |
|---|---|---|
| `ValkeyBackup/TriggerBGSAVE` | `internal/controller/valkeybackup_controller.go` | primary 에 `BGSAVE` 발행 |
| `ValkeyBackup/LASTSAVE` | `internal/controller/valkeybackup_controller.go` | snapshot timestamp 가 진행될 때까지 `LASTSAVE` 폴링 |
| `ValkeyBackup/Copying` | `internal/controller/valkeybackup_controller.go` | Job 기반 PVC 복사 (ADR-0015 init-container source) |
| `ValkeyBackup/Uploading` | `internal/controller/valkeybackup_controller.go` | S3 / GCS / Azure Blob 업로드 Job (ADR-0016 + ADR-0023) |

### Restore 파이프라인 (5개)

| Span | 위치 | 용도 |
|---|---|---|
| `ValkeyRestore/Mounting` | `internal/controller/valkeyrestore_controller.go` | PVC / 외부 소스 다운로드 |
| `ValkeyRestore/EnsureTargetRefSource` | `internal/controller/valkeyrestore_controller.go` | `Spec.Source.TargetRef` 를 위한 다운로드 Job 생성 |
| `ValkeyRestore/Restoring` | `internal/controller/valkeyrestore_controller.go` | STS init-container patch + rolling restart |
| `ValkeyRestore/Verifying` | `internal/controller/valkeyrestore_controller.go` | STS 원복 (init container 제거) + `paused` annotation 제거 |
| `ValkeyRestore/VerifyDataPlane` | `internal/controller/valkeyrestore_controller.go` | restore 후 `INFO keyspace` (`Status.RestoredKeys` 채움) |

### Cluster 라이프사이클 (4개)

| Span | 위치 | 용도 |
|---|---|---|
| `ValkeyCluster/EnsureClusterMeet` | `internal/controller/valkeycluster_controller.go` | `CLUSTER MEET` + `ADDSLOTS` + `REPLICATE` 부트스트랩 |
| `ValkeyCluster/CreateCluster` | `internal/controller/valkeycluster_controller.go` | `vk.CreateCluster` (`EnsureClusterMeet` 내부에 중첩) |
| `ValkeyCluster/QueryAnyNode` | `internal/controller/valkeycluster_controller.go` | `INFO` + `CLUSTER NODES` 폴링 |
| `ValkeyCluster/GracefulTeardown` | `internal/controller/valkeycluster_controller.go` | finalizer 기반 `CLUSTER FORGET` |

### Backup target 도달성 (1개)

| Span | 위치 | 용도 |
|---|---|---|
| `ValkeyBackupTarget/BucketExists` | `internal/controller/valkeybackuptarget_controller.go` | S3 / GCS / Azure 도달성 ping (`Status.Phase=Reachable` 설정) |

## 구현

tracer 는 `internal/observability/tracing.go` 에서 연결된다:

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

모든 caller 는 `span.End()` 를 `defer` 한다.

## End-to-end 검증

```bash
# 로컬 kind + Jaeger all-in-one
helm install jaeger jaegertracing/jaeger-all-in-one

# OTel 활성화된 operator
helm upgrade valkey-operator charts/valkey-operator \
  --set tracing.endpoint=jaeger:4317

# reconcile trigger 후 Jaeger UI 에서 확인
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
kubectl port-forward svc/jaeger-query 16686:16686
open http://localhost:16686
```

## 운영 활용

- **Latency p95 / p99** — span 별 duration 으로 phase 수준 SLO 를 산출한다.
  Reconcile root span overhead 목표: **≤ 5 ms**, trace export lag (Batcher
  default): **≤ 10 s**.
- **에러율** — `span.RecordError` 가 critical path 실패를 분류한다.
- **계층 구조** — trace UI 에서 각 reconcile loop 하위 child span 의 시간
  분포가 한눈에 드러나며, 병목 지점이 노출된다.
- **샘플링** — dev 100 %, prod 1 % (`OTEL_TRACES_SAMPLER_ARG` 로 설정).

## 알려진 제약

- **gRPC 전용** (`otlptracegrpc`). HTTP exporter 미연결 — 별도 ADR.
- exporter 에 **TLS / OAuth 인증 없음** — 별도 ADR.
- **애플리케이션 수준 metric** 은 tracer 의 범위 밖이다.
  controller-runtime 의 내장 metric 과 operator 가 노출하는
  `valkey_cluster_*` Prometheus exposition 이 그 역할을 담당한다
  ([`operations/metrics-glossary.md`](../operations/metrics-glossary.md) 참조).

## 참고

- [ADR-0025](../kb/adr/0025-otel-tracer-provider-optional.md) — OTel
  tracer provider optional + 미설정 시 zero-overhead no-op
- `internal/observability/tracing.go` — `SetupTracing`,
  `StartReconcileSpan`, `StartCallSpan`
- OpenTelemetry Go SDK — <https://github.com/open-telemetry/opentelemetry-go>
