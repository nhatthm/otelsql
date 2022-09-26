package otelsql

import (
	"context"
)

const (
	metricMethodPing = "go.sql.ping"
	traceMethodPing  = "ping"
)

// pingFuncMiddleware is a type for pingFunc middleware.
type pingFuncMiddleware = middleware[pingFunc]

// pingFunc is a callback for pingFunc.
type pingFunc func(ctx context.Context) error

// nopPing pings nothing.
func nopPing(_ context.Context) error {
	return nil
}

// pingStats records ping stats.
func pingStats(r methodRecorder) pingFuncMiddleware {
	return func(next pingFunc) pingFunc {
		return func(ctx context.Context) (err error) {
			end := r.Record(ctx, metricMethodPing)

			defer func() {
				end(err)
			}()

			return next(ctx)
		}
	}
}

// pingTrace traces ping.
func pingTrace(t methodTracer) pingFuncMiddleware {
	return func(next pingFunc) pingFunc {
		return func(ctx context.Context) (err error) {
			ctx, end := t.Trace(ctx, traceMethodPing)

			defer func() {
				end(err)
			}()

			return next(ctx)
		}
	}
}

func makePingFuncMiddlewares(r methodRecorder, t methodTracer) []pingFuncMiddleware {
	middlewares := make([]pingFuncMiddleware, 0, 2)
	middlewares = append(middlewares, pingStats(r))

	if t != nil {
		middlewares = append(middlewares, pingTrace(t))
	}

	return middlewares
}
