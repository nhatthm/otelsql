package otelsql

import (
	"context"
	"database/sql/driver"

	"go.opentelemetry.io/otel/attribute"
)

const (
	metricMethodQuery = "go.sql.query"
	traceMethodQuery  = "query"
)

type queryContextFuncMiddleware = middleware[queryContextFunc]

type queryContextFunc func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error)

// nopQueryContext queries nothing.
func nopQueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return nil, nil //nolint: nilnil
}

// skippedQueryContext always returns driver.ErrSkip.
func skippedQueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return nil, driver.ErrSkip
}

// add custome labels to metrics and traces
type ctxKey string

const (
	meterCtxKey  ctxKey = "meterCtxKey"
	tracerCtxKey ctxKey = "traceCtxKey"
)

func getLabels(ctx context.Context, key ctxKey) []attribute.KeyValue {
	labels, ok := ctx.Value(key).([]attribute.KeyValue)
	if !ok || labels == nil {
		labels = make([]attribute.KeyValue, 0)
	}
	return labels
}

func addLabels(ctx context.Context, key ctxKey, labels ...attribute.KeyValue) context.Context {
	new := append(getLabels(ctx, key), labels...)
	return context.WithValue(ctx, key, new)
}

func AddMeterLabels(ctx context.Context, labels ...attribute.KeyValue) context.Context {
	return addLabels(ctx, meterCtxKey, labels...)
}

func AddTracerLabels(ctx context.Context, labels ...attribute.KeyValue) context.Context {
	return addLabels(ctx, tracerCtxKey, labels...)
}

// queryStats records metrics for query.
func queryStats(r methodRecorder, method string) queryContextFuncMiddleware {
	return func(next queryContextFunc) queryContextFunc {
		return func(ctx context.Context, query string, args []driver.NamedValue) (result driver.Rows, err error) {
			labels := getLabels(ctx, meterCtxKey)
			end := r.Record(ctx, method, labels...)

			defer func() {
				end(err)
			}()

			result, err = next(ctx, query, args)

			return
		}
	}
}

// queryTrace creates a span for query.
func queryTrace(t methodTracer, traceQuery queryTracer, method string) queryContextFuncMiddleware {
	return func(next queryContextFunc) queryContextFunc {
		return func(ctx context.Context, query string, args []driver.NamedValue) (result driver.Rows, err error) {
			ctx = ContextWithQuery(ctx, query)
			ctx, end := t.Trace(ctx, method)

			defer func() {
				labels := append(
					getLabels(ctx, tracerCtxKey),
					traceQuery(ctx, query, args)...,
				)
				end(err, labels...)
			}()

			result, err = next(ctx, query, args)

			return
		}
	}
}

func queryWrapRows(t methodTracer, traceLastInsertID bool, traceRowsAffected bool) queryContextFuncMiddleware {
	return func(next queryContextFunc) queryContextFunc {
		return func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
			result, err := next(ctx, query, args)
			if err != nil {
				return nil, err
			}

			shouldTrace, _ := t.ShouldTrace(ctx)

			return wrapRows(ctx, result, t, shouldTrace && traceLastInsertID, shouldTrace && traceRowsAffected), nil
		}
	}
}

func makeQueryerContextMiddlewares(r methodRecorder, t methodTracer, cfg queryConfig) []queryContextFuncMiddleware {
	middlewares := make([]queryContextFuncMiddleware, 0, 3)

	middlewares = append(middlewares, queryStats(r, cfg.metricMethod))

	if t == nil {
		return middlewares
	}

	middlewares = append(middlewares, queryTrace(t, cfg.traceQuery, cfg.traceMethod))

	if cfg.traceRowsNext || cfg.traceRowsClose {
		middlewares = append(middlewares, queryWrapRows(t, cfg.traceRowsNext, cfg.traceRowsClose))
	}

	return middlewares
}

type queryConfig struct {
	metricMethod   string
	traceMethod    string
	traceQuery     queryTracer
	traceRowsNext  bool
	traceRowsClose bool
}

func newQueryConfig(opts driverOptions, metricMethod, traceMethod string) queryConfig {
	return queryConfig{
		metricMethod:   metricMethod,
		traceMethod:    traceMethod,
		traceQuery:     opts.trace.queryTracer,
		traceRowsNext:  opts.trace.RowsNext,
		traceRowsClose: opts.trace.RowsClose,
	}
}
