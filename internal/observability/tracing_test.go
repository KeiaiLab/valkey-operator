/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// OTEL tracer provider 단위 테스트. ADR-0025.
package observability

import (
	"context"
	"testing"
)

func TestSetupTracing_noopWhenEndpointUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdown, err := SetupTracing(t.Context())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if shutdown == nil {
		t.Fatalf("shutdown should not be nil (must be no-op)")
	}
	// no-op shutdown 호출 — 에러 없어야.
	if err := shutdown(t.Context()); err != nil {
		t.Fatalf("noop shutdown returned error: %v", err)
	}
}

func TestSetupTracing_invalidEndpoint(t *testing.T) {
	// gRPC client 자체는 *lazy* — endpoint 도달 못 해도 setup 시점에 fail 안 함.
	// 본 테스트는 단순 *no-op 도 아니고 panic 도 아님* 보장.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "invalid-host-no-such-domain:4317")
	t.Setenv("OTEL_SERVICE_NAME", "test-svc")
	shutdown, err := SetupTracing(t.Context())
	if err != nil {
		// otlptrace.New 가 즉시 fail 하면 이 path. 둘 다 OK 의도.
		t.Logf("SetupTracing returned err (acceptable for invalid endpoint): %v", err)
	}
	if shutdown == nil {
		t.Fatalf("shutdown should always be non-nil (even on err)")
	}
	// shutdown 호출 — 에러는 무시 (invalid endpoint 라 timeout 가능).
	ctx, cancel := context.WithTimeout(t.Context(), 0)
	cancel()
	_ = shutdown(ctx)
}

func TestNoopShutdown_returnsNil(t *testing.T) {
	if err := noopShutdown(t.Context()); err != nil {
		t.Fatalf("noopShutdown should return nil, got %v", err)
	}
}

func TestStartReconcileSpan_returnsValidContextAndSpan(t *testing.T) {
	// Tracer 미설치 (env 부재 → noop) 환경에서도 panic 없이 span 반환.
	ctx, span := StartReconcileSpan(t.Context(), "Valkey", "ns", "name")
	defer span.End()
	if ctx == nil {
		t.Fatal("ctx must be non-nil")
	}
	if span == nil {
		t.Fatal("span must be non-nil")
	}
	// SetAttributes 호출 — noop tracer 라도 panic 없음 보장.
	span.SetAttributes()
}

func TestStartCallSpan_returnsValidContextAndSpan(t *testing.T) {
	ctx, span := StartCallSpan(t.Context(), "test/operation")
	defer span.End()
	if ctx == nil || span == nil {
		t.Fatal("ctx + span must be non-nil")
	}
}

func TestStartCallSpan_recordError(t *testing.T) {
	// noop tracer 에서도 RecordError 가 panic 없이 동작.
	_, span := StartCallSpan(t.Context(), "test/op_with_error")
	defer span.End()
	span.RecordError(noopError{})
}

type noopError struct{}

func (noopError) Error() string { return "noop test error" }
