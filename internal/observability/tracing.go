/*
Copyright 2026 Keiailab.

Package observability — OTEL tracer provider setup. ADR-0025.

Optional — OTEL_EXPORTER_OTLP_ENDPOINT env 부재 시 noop tracer.
*/

package observability

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

const (
	envOTLPEndpoint       = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envServiceName        = "OTEL_SERVICE_NAME"
	defaultServiceName    = "valkey-operator"
	envResourceAttributes = "OTEL_RESOURCE_ATTRIBUTES" // 표준 — sdk/resource 가 자동 인식
)

// SetupTracing — OTEL endpoint 가 env 에 set 되어 있으면 OTLP gRPC exporter
// + TracerProvider 등록. 미설정 시 noop (zero overhead, shutdown 도 no-op).
//
// 호출자 (cmd/main.go) 는 반환된 shutdown 을 defer 로 실행. 에러 시
// shutdown 가 nil 이 아닌 *no-op* 함수 반환 — defer 안전.
func SetupTracing(ctx context.Context) (shutdown func(context.Context) error, err error) {
	endpoint := os.Getenv(envOTLPEndpoint)
	if endpoint == "" {
		// noop — 명시적 endpoint 부재. zero overhead.
		return noopShutdown, nil
	}

	serviceName := os.Getenv(envServiceName)
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	exporter, err := otlptrace.New(
		ctx,
		otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(), // TLS 는 추후 ADR.
		),
	)
	if err != nil {
		return noopShutdown, fmt.Errorf("otlptrace.New: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return noopShutdown, fmt.Errorf("resource.Merge: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func noopShutdown(_ context.Context) error { return nil }
