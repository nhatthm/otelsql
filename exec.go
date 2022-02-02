package otelsql

import (
	"context"
	"database/sql/driver"
)

const (
	metricMethodExec = "go.sql.exec"
	traceMethodExec  = "exec"
)

type execContextFuncMiddleware func(next execContextFunc) execContextFunc

type execContextFunc func(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error)

// nopExecContext executes nothing.
func nopExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return nil, nil
}

// skippedExecContext always returns driver.ErrSkip.
func skippedExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return nil, driver.ErrSkip
}

// chainExecContextFuncMiddlewares builds a execContextFunc composed of an inline middleware stack and the end pinger in the order they are passed.
func chainExecContextFuncMiddlewares(middlewares []execContextFuncMiddleware, exec execContextFunc) execContextFunc {
	// Return ahead of time if there are not any middlewares for the chain.
	if len(middlewares) == 0 {
		return exec
	}

	// Wrap the end execer with the middleware chain.
	h := middlewares[len(middlewares)-1](exec)

	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
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
	middlewares := make([]execContextFuncMiddleware, 0, 3)

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
