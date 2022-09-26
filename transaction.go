package otelsql

import (
	"context"
	"database/sql/driver"

	"go.opentelemetry.io/otel/trace"
)

const (
	metricMethodCommit   = "go.sql.commit"
	traceMethodCommit    = "commit"
	metricMethodRollback = "go.sql.rollback"
	traceMethodRollback  = "rollback"
)

var _ driver.Tx = (*tx)(nil)

type txFuncMiddleware = middleware[txFunc]

type txFunc func() error

type tx struct {
	commit   txFunc
	rollback txFunc
}

func (t tx) Commit() error {
	return t.commit()
}

func (t tx) Rollback() error {
	return t.rollback()
}

func wrapTx(ctx context.Context, parent driver.Tx, r methodRecorder, t methodTracer) driver.Tx {
	ctx = trace.ContextWithSpanContext(context.Background(), trace.SpanContextFromContext(ctx))

	return &tx{
		commit:   chainMiddlewares(makeTxFuncMiddlewares(ctx, r, t, metricMethodCommit, traceMethodCommit), parent.Commit),
		rollback: chainMiddlewares(makeTxFuncMiddlewares(ctx, r, t, metricMethodRollback, traceMethodRollback), parent.Rollback),
	}
}

func nopTxFunc() error {
	return nil
}

func txStats(ctx context.Context, r methodRecorder, method string) txFuncMiddleware {
	return func(next txFunc) txFunc {
		return func() (err error) {
			end := r.Record(ctx, method)

			defer func() {
				end(err)
			}()

			return next()
		}
	}
}

func txTrace(ctx context.Context, t methodTracer, method string) txFuncMiddleware {
	return func(next txFunc) txFunc {
		return func() (err error) {
			_, end := t.MustTrace(ctx, method)

			defer func() {
				end(err)
			}()

			return next()
		}
	}
}

func makeTxFuncMiddlewares(ctx context.Context, r methodRecorder, t methodTracer, metricMethod string, traceMethod string) []txFuncMiddleware {
	middlewares := make([]txFuncMiddleware, 0, 2)
	middlewares = append(middlewares, txStats(ctx, r, metricMethod))

	if t != nil {
		middlewares = append(middlewares, txTrace(ctx, t, traceMethod))
	}

	return middlewares
}
