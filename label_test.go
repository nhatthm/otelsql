package otelsql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"

	"go.nhat.io/otelsql"
)

var (
	label1 = attribute.String("key1", "value1")
	label2 = attribute.Int("key2", 2)
	label3 = attribute.Bool("key3", true)
)

func TestMetricsLabeler(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	labels := otelsql.MetricsLabelsFromContext(ctx)
	assert.Empty(t, labels)

	ctx = otelsql.ContextWithMetricsLabels(ctx, label1, label2)

	labels = otelsql.MetricsLabelsFromContext(ctx)
	assert.Equal(t, []attribute.KeyValue{label1, label2}, labels)

	ctx = otelsql.ContextWithMetricsLabels(ctx, label3)
	assert.Equal(t, []attribute.KeyValue{label1, label2, label3}, otelsql.MetricsLabelsFromContext(ctx))

	assert.Empty(t, otelsql.TraceLabelsFromContext(ctx))
}

func TestTraceLabeler(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	labels := otelsql.TraceLabelsFromContext(ctx)
	assert.Empty(t, labels)

	ctx = otelsql.ContextWithTraceLabels(ctx, label1, label2)

	labels = otelsql.TraceLabelsFromContext(ctx)
	assert.Equal(t, []attribute.KeyValue{label1, label2}, labels)

	ctx = otelsql.ContextWithTraceLabels(ctx, label3)
	assert.Equal(t, []attribute.KeyValue{label1, label2, label3}, otelsql.TraceLabelsFromContext(ctx))

	assert.Empty(t, otelsql.MetricsLabelsFromContext(ctx))
}

func TestMetricsAndTraceLabeler(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	labels := otelsql.MetricsLabelsFromContext(ctx)
	assert.Empty(t, labels)

	labels = otelsql.TraceLabelsFromContext(ctx)
	assert.Empty(t, labels)

	ctx = otelsql.ContextWithTraceAndMetricsLabels(ctx, label1, label2)

	assert.Equal(t, []attribute.KeyValue{label1, label2}, otelsql.MetricsLabelsFromContext(ctx))
	assert.Equal(t, []attribute.KeyValue{label1, label2}, otelsql.TraceLabelsFromContext(ctx))

	ctx = otelsql.ContextWithTraceAndMetricsLabels(ctx, label3)

	assert.Equal(t, []attribute.KeyValue{label1, label2, label3}, otelsql.MetricsLabelsFromContext(ctx))
	assert.Equal(t, []attribute.KeyValue{label1, label2, label3}, otelsql.TraceLabelsFromContext(ctx))
}
