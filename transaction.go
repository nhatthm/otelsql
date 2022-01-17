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

type txFuncMiddleware func(next txFunc) txFunc

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
		commit:   chainTxFuncMiddlewares(makeTxFuncMiddlewares(ctx, r, t, metricMethodCommit, traceMethodCommit), parent.Commit),
		rollback: chainTxFuncMiddlewares(makeTxFuncMiddlewares(ctx, r, t, metricMethodRollback, traceMethodRollback), parent.Rollback),
	}
}

func noOpTxFunc() error {
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

// chainTxFuncMiddlewares builds a txFunc composed of an inline middleware stack and the end beginner in the order they are passed.
func chainTxFuncMiddlewares(middlewares []txFuncMiddleware, f txFunc) txFunc {
	// Return ahead of time if there are not any middlewares for the chain.
	if len(middlewares) == 0 {
		return f
	}

	// Wrap the end func with the middleware chain.
	h := middlewares[len(middlewares)-1](f)

	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}

func makeTxFuncMiddlewares(ctx context.Context, r methodRecorder, t methodTracer, metricMethod string, traceMethod string) []txFuncMiddleware {
	middlewares := make([]txFuncMiddleware, 0, 2)
	middlewares = append(middlewares, txStats(ctx, r, metricMethod))

	if t != nil {
		middlewares = append(middlewares, txTrace(ctx, t, traceMethod))
	}

	return middlewares
}
