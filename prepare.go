package otelsql

import (
	"context"
	"database/sql/driver"
)

const (
	metricMethodPrepare = "go.sql.prepare"
	traceMethodPrepare  = "prepare"
)

type prepareContextFuncMiddleware = middleware[prepareContextFunc]

type prepareContextFunc func(ctx context.Context, query string) (driver.Stmt, error)

// nopPrepareContext prepares nothing.
func nopPrepareContext(_ context.Context, _ string) (driver.Stmt, error) {
	return nil, nil
}

func ensurePrepareContext(conn driver.Conn) prepareContextFunc {
	if p, ok := conn.(driver.ConnPrepareContext); ok {
		return p.PrepareContext
	}

	return func(_ context.Context, query string) (driver.Stmt, error) {
		return conn.Prepare(query)
	}
}

// prepareStats records metrics for prepare.
func prepareStats(r methodRecorder) prepareContextFuncMiddleware {
	return func(next prepareContextFunc) prepareContextFunc {
		return func(ctx context.Context, query string) (stmt driver.Stmt, err error) {
			end := r.Record(ctx, metricMethodPrepare)

			defer func() {
				end(err)
			}()

			return next(ctx, query)
		}
	}
}

// prepareTrace creates a span for prepare.
func prepareTrace(t methodTracer, traceQuery queryTracer) prepareContextFuncMiddleware {
	return func(next prepareContextFunc) prepareContextFunc {
		return func(ctx context.Context, query string) (stmt driver.Stmt, err error) {
			ctx, end := t.Trace(ctx, traceMethodPrepare)

			defer func() {
				end(err, traceQuery(ctx, query, nil)...)
			}()

			return next(ctx, query)
		}
	}
}

func prepareWrapResult(
	execFuncMiddlewares []execContextFuncMiddleware,
	execContextFuncMiddlewares []execContextFuncMiddleware,
	queryFuncMiddlewares []queryContextFuncMiddleware,
	queryContextFuncMiddlewares []queryContextFuncMiddleware,
) prepareContextFuncMiddleware {
	return func(next prepareContextFunc) prepareContextFunc {
		return func(ctx context.Context, query string) (driver.Stmt, error) {
			stmt, err := next(ctx, query)
			if err != nil {
				return nil, err
			}

			return wrapStmt(stmt, stmtConfig{
				query:                       query,
				execFuncMiddlewares:         execFuncMiddlewares,
				queryContextFuncMiddlewares: queryContextFuncMiddlewares,
				execContextFuncMiddlewares:  execContextFuncMiddlewares,
				queryFuncMiddlewares:        queryFuncMiddlewares,
			}), nil
		}
	}
}

type prepareConfig struct {
	traceQuery queryTracer

	execFuncMiddlewares         []execContextFuncMiddleware
	execContextFuncMiddlewares  []execContextFuncMiddleware
	queryFuncMiddlewares        []queryContextFuncMiddleware
	queryContextFuncMiddlewares []queryContextFuncMiddleware
}

func makePrepareContextFuncMiddlewares(r methodRecorder, t methodTracer, cfg prepareConfig) []prepareContextFuncMiddleware {
	return []prepareContextFuncMiddleware{
		prepareStats(r),
		prepareTrace(t, cfg.traceQuery),
		prepareWrapResult(
			cfg.execFuncMiddlewares,
			cfg.execContextFuncMiddlewares,
			cfg.queryFuncMiddlewares,
			cfg.queryContextFuncMiddlewares,
		),
	}
}
