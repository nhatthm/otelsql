package otelsql

import (
	"context"
	"database/sql/driver"
)

const (
	metricMethodQuery = "go.sql.query"
	traceMethodQuery  = "query"
)

type queryContextFuncMiddleware = middleware[queryContextFunc]

type queryContextFunc func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error)

// nopQueryContext queries nothing.
func nopQueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return nil, nil
}

// skippedQueryContext always returns driver.ErrSkip.
func skippedQueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return nil, driver.ErrSkip
}

// queryStats records metrics for query.
func queryStats(r methodRecorder, method string) queryContextFuncMiddleware {
	return func(next queryContextFunc) queryContextFunc {
		return func(ctx context.Context, query string, args []driver.NamedValue) (result driver.Rows, err error) {
			end := r.Record(ctx, method)

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
				end(err, traceQuery(ctx, query, args)...)
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

	middlewares = append(middlewares,
		queryStats(r, cfg.metricMethod),
		queryTrace(t, cfg.traceQuery, cfg.traceMethod),
	)

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
