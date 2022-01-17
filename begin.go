package otelsql

import (
	"context"
	"database/sql/driver"
)

const (
	metricMethodBegin = "go.sql.begin"
	traceMethodBegin  = "begin_transaction"
)

// beginFuncMiddleware is a type for beginFunc middleware.
type beginFuncMiddleware func(next beginFunc) beginFunc

// beginFunc is a callback for beginFunc.
type beginFunc func(ctx context.Context, opts driver.TxOptions) (driver.Tx, error)

// noOpBegin pings nothing.
func noOpBegin(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	return nil, nil
}

func ensureBegin(conn driver.Conn) beginFunc {
	if b, ok := conn.(driver.ConnBeginTx); ok {
		return b.BeginTx
	}

	return func(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
		return conn.Begin() // nolint: staticcheck
	}
}

// chainBeginFuncMiddlewares builds a beginFunc composed of an inline middleware stack and the end beginner in the order they are passed.
func chainBeginFuncMiddlewares(middlewares []beginFuncMiddleware, begin beginFunc) beginFunc {
	// Return ahead of time if there are not any middlewares for the chain.
	if len(middlewares) == 0 {
		return begin
	}

	// Wrap the end beginner with the middleware chain.
	h := middlewares[len(middlewares)-1](begin)

	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}

// beginStats records begin stats.
func beginStats(r methodRecorder) beginFuncMiddleware {
	return func(next beginFunc) beginFunc {
		return func(ctx context.Context, opts driver.TxOptions) (result driver.Tx, err error) {
			end := r.Record(ctx, metricMethodBegin)

			defer func() {
				end(err)
			}()

			return next(ctx, opts)
		}
	}
}

// beginTrace traces begin.
func beginTrace(t methodTracer) beginFuncMiddleware {
	return func(next beginFunc) beginFunc {
		return func(ctx context.Context, opts driver.TxOptions) (result driver.Tx, err error) {
			ctx, end := t.Trace(ctx, traceMethodBegin)

			defer func() {
				end(err)
			}()

			return next(ctx, opts)
		}
	}
}

func beginWrapTx(r methodRecorder, t methodTracer) beginFuncMiddleware {
	return func(next beginFunc) beginFunc {
		return func(ctx context.Context, opts driver.TxOptions) (result driver.Tx, err error) {
			tx, err := next(ctx, opts)
			if err != nil {
				return nil, err
			}

			shouldTrace, _ := t.ShouldTrace(ctx)

			return wrapTx(ctx, tx, r, tracerOrNil(t, shouldTrace)), nil
		}
	}
}

func makeBeginFuncMiddlewares(r methodRecorder, t methodTracer) []beginFuncMiddleware {
	return []beginFuncMiddleware{
		beginStats(r), beginTrace(t), beginWrapTx(r, t),
	}
}
