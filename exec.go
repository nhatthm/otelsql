package otelsql

import (
	"context"
	"database/sql/driver"
)

const (
	metricMethodExec = "go.sql.exec"
	traceMethodExec  = "exec"
)

type execContextFuncMiddleware = middleware[execContextFunc]

type execContextFunc func(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error)

// nopExecContext executes nothing.
func nopExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return nil, nil //nolint: nilnil
}

// skippedExecContext always returns driver.ErrSkip.
func skippedExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return nil, driver.ErrSkip
}

// execStats records metrics for exec.
func execStats(r methodRecorder, method string) execContextFuncMiddleware {
	return func(next execContextFunc) execContextFunc {
		return func(ctx context.Context, query string, args []driver.NamedValue) (result driver.Result, err error) {
			end := r.Record(ctx, method)

			defer func() {
				end(err)
			}()

			return next(ctx, query, args)
		}
	}
}

// execTrace creates a span for exec.
func execTrace(t methodTracer, traceQuery queryTracer, method string) execContextFuncMiddleware {
	return func(next execContextFunc) execContextFunc {
		return func(ctx context.Context, query string, args []driver.NamedValue) (result driver.Result, err error) {
			ctx = ContextWithQuery(ctx, query)
			ctx, end := t.Trace(ctx, method)

			defer func() {
				end(err, traceQuery(ctx, query, args)...)
			}()

			return next(ctx, query, args)
		}
	}
}

func execWrapResult(t methodTracer, traceLastInsertID bool, traceRowsAffected bool) execContextFuncMiddleware {
	return func(next execContextFunc) execContextFunc {
		return func(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
			result, err := next(ctx, query, args)
			if err != nil {
				return nil, err
			}

			shouldTrace, _ := t.ShouldTrace(ctx)

			return wrapResult(ctx, result, t, shouldTrace && traceLastInsertID, shouldTrace && traceRowsAffected), nil
		}
	}
}

func makeExecContextFuncMiddlewares(r methodRecorder, t methodTracer, cfg execConfig) []execContextFuncMiddleware {
	middlewares := make([]middleware[execContextFunc], 0, 3)

	middlewares = append(middlewares, execStats(r, cfg.metricMethod))

	if t == nil {
		return middlewares
	}

	middlewares = append(middlewares, execTrace(t, cfg.traceQuery, cfg.traceMethod))

	if cfg.traceLastInsertID || cfg.traceRowsAffected {
		middlewares = append(middlewares, execWrapResult(t, cfg.traceLastInsertID, cfg.traceRowsAffected))
	}

	return middlewares
}

type execConfig struct {
	metricMethod      string
	traceMethod       string
	traceQuery        queryTracer
	traceLastInsertID bool
	traceRowsAffected bool
}

func newExecConfig(opts driverOptions, metricMethod, traceMethod string) execConfig {
	return execConfig{
		metricMethod:      metricMethod,
		traceMethod:       traceMethod,
		traceQuery:        opts.trace.queryTracer,
		traceLastInsertID: opts.trace.LastInsertID,
		traceRowsAffected: opts.trace.RowsAffected,
	}
}
