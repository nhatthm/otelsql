package otelsql

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
)

// Labeler is a helper to add attributes to a context.
type Labeler struct {
	mu         sync.Mutex
	attributes []attribute.KeyValue
}

// Add attributes to a Labeler.
func (l *Labeler) Add(ls ...attribute.KeyValue) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.attributes = append(l.attributes, ls...)
}

// Get returns a copy of the attributes added to the Labeler.
func (l *Labeler) Get() []attribute.KeyValue {
	l.mu.Lock()
	defer l.mu.Unlock()

	ret := make([]attribute.KeyValue, len(l.attributes))
	copy(ret, l.attributes)

	return ret
}

const (
	labelerCtxMetrics = labelerContextKey("metrics")
	labelerCtxTrace   = labelerContextKey("trace")
)

type labelerContextKey string

// MetricsLabelsFromContext retrieves the labels from the provided context.
func MetricsLabelsFromContext(ctx context.Context) []attribute.KeyValue {
	l, _ := labelerFromContext(ctx, labelerCtxMetrics)

	return l.Get()
}

// ContextWithMetricsLabels returns a new context with the labels added to the Labeler.
func ContextWithMetricsLabels(ctx context.Context, labels ...attribute.KeyValue) context.Context {
	return contextWithLabels(ctx, labelerCtxMetrics, labels...)
}

// TraceLabelsFromContext retrieves the labels from the provided context.
func TraceLabelsFromContext(ctx context.Context) []attribute.KeyValue {
	l, _ := labelerFromContext(ctx, labelerCtxTrace)

	return l.Get()
}

// ContextWithTraceLabels returns a new context with the labels added to the Labeler.
func ContextWithTraceLabels(ctx context.Context, labels ...attribute.KeyValue) context.Context {
	return contextWithLabels(ctx, labelerCtxTrace, labels...)
}

// ContextWithTraceAndMetricsLabels returns a new context with the labels added to the Labeler.
func ContextWithTraceAndMetricsLabels(ctx context.Context, labels ...attribute.KeyValue) context.Context {
	ctx = ContextWithMetricsLabels(ctx, labels...)
	ctx = ContextWithTraceLabels(ctx, labels...)

	return ctx
}

func labelerFromContext(ctx context.Context, key labelerContextKey) (*Labeler, bool) { //nolint: unparam
	l, ok := ctx.Value(key).(*Labeler)
	if !ok {
		l = &Labeler{}
	}

	return l, ok
}

func contextWithLabels(ctx context.Context, key labelerContextKey, labels ...attribute.KeyValue) context.Context {
	l, _ := labelerFromContext(ctx, key)

	l.Add(labels...)

	return context.WithValue(ctx, key, l)
}
