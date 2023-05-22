package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"

	xattr "go.nhat.io/otelsql/attribute"
)

type spanNameFormatter func(ctx context.Context, op string) string

type errorToSpanStatus func(err error) (codes.Code, string)

type queryTracer func(ctx context.Context, query string, args []driver.NamedValue) []attribute.KeyValue

// methodTracer traces a sql method.
type methodTracer interface {
	// ShouldTrace checks whether it should trace a method and the given context has a parent span.
	ShouldTrace(ctx context.Context) (bool, bool)
	MustTrace(ctx context.Context, method string, labels ...attribute.KeyValue) (context.Context, func(err error, attrs ...attribute.KeyValue))
	Trace(ctx context.Context, method string, labels ...attribute.KeyValue) (context.Context, func(err error, attrs ...attribute.KeyValue))
}

type methodTracerImpl struct {
	tracer trace.Tracer

	formatSpanName spanNameFormatter
	errorToStatus  func(err error) (codes.Code, string)
	allowRoot      bool
	attributes     []attribute.KeyValue
}

func (t *methodTracerImpl) ShouldTrace(ctx context.Context) (bool, bool) {
	hasSpan := trace.SpanContextFromContext(ctx).IsValid()

	return t.allowRoot || hasSpan, hasSpan
}

func (t *methodTracerImpl) Trace(ctx context.Context, method string, labels ...attribute.KeyValue) (context.Context, func(err error, attrs ...attribute.KeyValue)) {
	shouldTrace, hasParentSpan := t.ShouldTrace(ctx)

	if !shouldTrace {
		return ctx, func(_ error, _ ...attribute.KeyValue) {}
	}

	newCtx, end := t.MustTrace(ctx, method, labels...)

	if !hasParentSpan {
		ctx = newCtx
	}

	return ctx, end
}

func (t *methodTracerImpl) MustTrace(ctx context.Context, method string, labels ...attribute.KeyValue) (context.Context, func(err error, attrs ...attribute.KeyValue)) {
	ctx, span := t.tracer.Start(ctx, t.formatSpanName(ctx, method),
		trace.WithTimestamp(time.Now()),
		trace.WithSpanKind(trace.SpanKindClient),
	)

	attrs := make([]attribute.KeyValue, 0, len(t.attributes)+len(labels)+1)

	attrs = append(attrs, t.attributes...)
	attrs = append(attrs, labels...)
	attrs = append(attrs, semconv.DBOperationKey.String(method))

	return ctx, func(err error, labels ...attribute.KeyValue) {
		code, desc := t.errorToStatus(err)

		attrs = append(attrs, labels...)

		span.SetAttributes(attrs...)
		span.SetStatus(code, desc)

		if code == codes.Error {
			span.RecordError(err)
		}

		span.End(
			trace.WithTimestamp(time.Now()),
		)
	}
}

func newMethodTracer(tracer trace.Tracer, opts ...func(t *methodTracerImpl)) *methodTracerImpl {
	t := &methodTracerImpl{
		tracer:         tracer,
		formatSpanName: formatSpanName,
		errorToStatus:  spanStatusFromError,
	}

	for _, o := range opts {
		o(t)
	}

	return t
}

func tracerOrNil(t methodTracer, shouldTrace bool) methodTracer {
	if shouldTrace {
		return t
	}

	return nil
}

func traceWithAllowRoot(allow bool) func(t *methodTracerImpl) {
	return func(t *methodTracerImpl) {
		t.allowRoot = allow
	}
}

func traceWithDefaultAttributes(attrs ...attribute.KeyValue) func(t *methodTracerImpl) {
	return func(t *methodTracerImpl) {
		t.attributes = append(t.attributes, attrs...)
	}
}

func traceWithSpanNameFormatter(f spanNameFormatter) func(t *methodTracerImpl) {
	return func(t *methodTracerImpl) {
		t.formatSpanName = f
	}
}

func traceWithErrorToSpanStatus(f errorToSpanStatus) func(t *methodTracerImpl) {
	return func(t *methodTracerImpl) {
		t.errorToStatus = f
	}
}

func formatSpanName(_ context.Context, method string) string {
	var sb strings.Builder

	sb.Grow(len(method) + 4)
	sb.WriteString("sql:")
	sb.WriteString(method)

	return sb.String()
}

func spanStatusFromError(err error) (codes.Code, string) {
	if err == nil {
		return codes.Ok, ""
	}

	return codes.Error, err.Error()
}

func spanStatusFromErrorIgnoreErrSkip(err error) (codes.Code, string) {
	if err == nil || errors.Is(err, driver.ErrSkip) {
		return codes.Ok, ""
	}

	return codes.Error, err.Error()
}

func traceNoQuery(context.Context, string, []driver.NamedValue) []attribute.KeyValue {
	return nil
}

func traceQueryWithoutArgs(_ context.Context, sql string, _ []driver.NamedValue) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.DBStatementKey.String(sql),
	}
}

func traceQueryWithArgs(_ context.Context, sql string, args []driver.NamedValue) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 1+len(args))
	attrs = append(attrs, semconv.DBStatementKey.String(sql))

	for _, arg := range args {
		attrs = append(attrs, xattr.FromNamedValue(arg))
	}

	return attrs
}
