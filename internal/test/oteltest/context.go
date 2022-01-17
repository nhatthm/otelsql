package oteltest

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// BackgroundWithSpanContext creates a new context.Background with trace id and span id.
func BackgroundWithSpanContext(traceID trace.TraceID, spanID trace.SpanID) context.Context {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})

	return trace.ContextWithSpanContext(context.Background(), sc)
}
