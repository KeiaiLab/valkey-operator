/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerName — global tracer 이름. 모든 reconciler 가 동일.
const TracerName = "github.com/keiailab/valkey-operator"

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

// StartReconcileSpan — reconcile path 의 root span 시작. caller 가 defer
// span.End() 의무. attributes: k8s.namespace + k8s.name (semconv 표준).
//
// noop tracer (env 미설정) 시 zero overhead — span 객체 자체가 noop.
func StartReconcileSpan(
	ctx context.Context, kind, namespace, name string,
) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(TracerName).Start(ctx, kind+"/Reconcile")
	span.SetAttributes(
		attribute.String("k8s.namespace", namespace),
		attribute.String("k8s.name", name),
		attribute.String("k8s.kind", kind),
	)
	return ctx, span
}

// StartCallSpan — reconcile path *내부* 의 child span (외부 호출 / 비싼
// computation 등). caller 가 defer span.End() 의무.
//
// 사용 예: redis client 호출 (INFO replication / PromoteToPrimary), S3 client
// 호출 (FPut / FGet), Pod ready 검증 등. operation 별 latency + error 추적.
func StartCallSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer(TracerName).Start(ctx, name)
}
