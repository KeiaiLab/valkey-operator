# ADR-0025: OTEL Tracer Provider — Optional, OTLP gRPC Exporter

- Date: 2026-05-06
- Status: Accepted
- Authors: @phil

## Context

Plan §3 Track F 의 첫 step. controller-runtime 가 noop tracer 기본 사용
— operator 의 reconcile path / 외부 호출 (Valkey / S3 / Job) 의 distributed
tracing 부재.

요구사항:
- *Optional* — OTEL endpoint 부재 시 noop (zero overhead)
- OTLP gRPC exporter (kube-prometheus-stack / Tempo / Jaeger 호환)
- env-driven 설정 (`OTEL_EXPORTER_OTLP_ENDPOINT` 등 표준 OpenTelemetry env)
- controller-runtime 의 reconcile span 자동 + 우리 함수 의 manual span 추가
  점

## Decision

**Optional OTEL tracer provider 통합** — `internal/observability` 패키지.

API:
```go
// SetupTracing — OTEL_EXPORTER_OTLP_ENDPOINT env 가 set 되어 있으면 OTLP
// gRPC exporter + sdktrace.TracerProvider 등록. 미설정 시 noop.
//
// Shutdown 함수 반환 — caller (cmd/main.go) 가 defer 로 호출.
func SetupTracing(ctx context.Context) (shutdown func(context.Context) error, err error)
```

cmd/main.go 통합:
```go
shutdownTracing, err := observability.SetupTracing(ctx)
if err != nil {
    setupLog.Error(err, "failed to setup tracing")
}
defer shutdownTracing(ctx)
```

Env 표준 (OpenTelemetry 표준):
- `OTEL_EXPORTER_OTLP_ENDPOINT` — gRPC endpoint (e.g. `tempo:4317`)
- `OTEL_SERVICE_NAME` — service name (default `valkey-operator`)
- `OTEL_RESOURCE_ATTRIBUTES` — additional resource attributes

## Consequences

긍정:
- **Zero overhead default** — endpoint 부재 시 noop tracer.
- **표준 OTEL env** — 사용자 친화 + Tempo/Jaeger/Honeycomb 호환.
- **otel SDK 이미 indirect 의존성** (사용자 cycle 7 commit c05b251 의
  v1.43.0 업데이트). 직접 import 로 활성화만 추가.
- **Reconcile path 자동 span** (controller-runtime 의 internal tracer).

부정:
- **OTLP gRPC 만** — HTTP exporter 미지원 (옵션 추후 ADR).
- **Authentication** (TLS / token) 미지원 — endpoint 평문 가정. 운영 배포
  시 별개 ADR 또는 OTEL Collector sidecar 패턴.
- **Additional manual span** 은 caller 책임 — 본 ADR 은 *infrastructure*
  만, 실제 instrumentation 은 reconcile path 별 별개 commit.

## Alternatives Considered

1. **Mandatory OTEL** (옵션 거절): 항상 활성화 + endpoint 강제. 거절: 개발/
   PoC 환경에 부담.
2. **Stdout / file exporter**: 디버깅 용. 거절: production 부적합 (volume
   처리 어려움). 추후 dev mode 별개 ADR.
3. **Jaeger native client** (legacy): OpenTelemetry 가 표준이므로 거절.

## Action Items

- [ ] AI-001: `internal/observability/tracing.go` — SetupTracing + shutdown.
- [ ] AI-002: `cmd/main.go` 통합.
- [ ] AI-003: 단위 테스트 — env 미설정 시 noop, 설정 시 OTLP setup.
- [ ] AI-004: README 운영 가이드 — OTEL endpoint 설정 절차.
- [ ] AI-005: 추후 — reconcile path 별 manual span (tracer.Start(ctx, ...)).

Refs: Plan §3 Track F, HANDOFF.md cycle 9 §10, 사용자 cycle 7 commit
c05b251 (otel SDK v1.43.0 CVE 패치).
