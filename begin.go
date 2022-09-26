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
type beginFuncMiddleware = middleware[beginFunc]

// beginFunc is a callback for beginFunc.
type beginFunc func(ctx context.Context, opts driver.TxOptions) (driver.Tx, error)

// nopBegin pings nothing.
func nopBegin(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
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
