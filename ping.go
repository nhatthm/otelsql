package otelsql

import (
	"context"
)

const (
	metricMethodPing = "go.sql.ping"
	traceMethodPing  = "ping"
)

// pingFuncMiddleware is a type for pingFunc middleware.
type pingFuncMiddleware func(next pingFunc) pingFunc

// pingFunc is a callback for pingFunc.
type pingFunc func(ctx context.Context) error

// noOpPing pings nothing.
func noOpPing(_ context.Context) error {
	return nil
}

// chainPingFuncMiddlewares builds a pingFunc composed of an inline middleware stack and the end pinger in the order they are
// passed.
func chainPingFuncMiddlewares(middlewares []pingFuncMiddleware, pinger pingFunc) pingFunc {
	// Return ahead of time if there are not any middlewares for the chain.
	if len(middlewares) == 0 {
		return pinger
	}

	// Wrap the end pinger with the middleware chain.
	h := middlewares[len(middlewares)-1](pinger)

	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
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
