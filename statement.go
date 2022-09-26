package otelsql

import (
	"context"
	"database/sql/driver"
)

const (
	metricMethodStmtExec  = "go.sql.stmt.exec"
	traceMethodStmtExec   = "exec"
	metricMethodStmtQuery = "go.sql.stmt.query"
	traceMethodStmtQuery  = "query"
)

// Deprecated: Drivers should implement NamedValueChecker.
type columnConverter interface {
	ColumnConverter(idx int) driver.ValueConverter
}

type stmt struct {
	stmtQuery string

	exec        execContextFunc
	execContext execContextFunc

	query        queryContextFunc
	queryContext queryContextFunc

	close    func() error
	numInput func() int
}

func (s stmt) Exec(args []driver.Value) (res driver.Result, err error) {
	return s.exec(context.Background(), s.stmtQuery, valuesToNamedValues(args))
}

func (s stmt) Close() error {
	return s.close()
}

func (s stmt) NumInput() int {
	return s.numInput()
}

func (s stmt) Query(args []driver.Value) (rows driver.Rows, err error) {
	return s.query(context.Background(), s.stmtQuery, valuesToNamedValues(args))
}

func (s stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (res driver.Result, err error) {
	return s.execContext(ctx, s.stmtQuery, args)
}

func (s stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (rows driver.Rows, err error) {
	return s.queryContext(ctx, s.stmtQuery, args)
}

type stmtConfig struct {
	query string

	execFuncMiddlewares         []execContextFuncMiddleware
	execContextFuncMiddlewares  []execContextFuncMiddleware
	queryFuncMiddlewares        []queryContextFuncMiddleware
	queryContextFuncMiddlewares []queryContextFuncMiddleware
}

// nolint: cyclop,funlen,gocyclo
func wrapStmt(parent driver.Stmt, cfg stmtConfig) driver.Stmt {
	s := makeStmt(parent, cfg)

	var (
		_, hasExeCtx    = parent.(driver.StmtExecContext)
		_, hasQryCtx    = parent.(driver.StmtQueryContext)
		c, hasColConv   = parent.(columnConverter)
		n, hasNamValChk = parent.(driver.NamedValueChecker)
	)

	switch {
	default:
		// case !hasExeCtx && !hasQryCtx && !hasColConv && !hasNamValChk:
		return struct {
			driver.Stmt
		}{s}

	case !hasExeCtx && hasQryCtx && !hasColConv && !hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtQueryContext
		}{s, s}

	case hasExeCtx && !hasQryCtx && !hasColConv && !hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtExecContext
		}{s, s}

	case hasExeCtx && hasQryCtx && !hasColConv && !hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtExecContext
			driver.StmtQueryContext
		}{s, s, s}

	case !hasExeCtx && !hasQryCtx && hasColConv && !hasNamValChk:
		return struct {
			driver.Stmt
			columnConverter
		}{s, c}

	case !hasExeCtx && hasQryCtx && hasColConv && !hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtQueryContext
			columnConverter
		}{s, s, c}

	case hasExeCtx && !hasQryCtx && hasColConv && !hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtExecContext
			columnConverter
		}{s, s, c}

	case hasExeCtx && hasQryCtx && hasColConv && !hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtExecContext
			driver.StmtQueryContext
			columnConverter
		}{s, s, s, c}

	case !hasExeCtx && !hasQryCtx && !hasColConv && hasNamValChk:
		return struct {
			driver.Stmt
			driver.NamedValueChecker
		}{s, n}

	case !hasExeCtx && hasQryCtx && !hasColConv && hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtQueryContext
			driver.NamedValueChecker
		}{s, s, n}

	case hasExeCtx && !hasQryCtx && !hasColConv && hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtExecContext
			driver.NamedValueChecker
		}{s, s, n}

	case hasExeCtx && hasQryCtx && !hasColConv && hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtExecContext
			driver.StmtQueryContext
			driver.NamedValueChecker
		}{s, s, s, n}

	case !hasExeCtx && !hasQryCtx && hasColConv && hasNamValChk:
		return struct {
			driver.Stmt
			columnConverter
			driver.NamedValueChecker
		}{s, c, n}

	case !hasExeCtx && hasQryCtx && hasColConv && hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtQueryContext
			columnConverter
			driver.NamedValueChecker
		}{s, s, c, n}

	case hasExeCtx && !hasQryCtx && hasColConv && hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtExecContext
			columnConverter
			driver.NamedValueChecker
		}{s, s, c, n}

	case hasExeCtx && hasQryCtx && hasColConv && hasNamValChk:
		return struct {
			driver.Stmt
			driver.StmtExecContext
			driver.StmtQueryContext
			columnConverter
			driver.NamedValueChecker
		}{s, s, s, c, n}
	}
}

func makeStmt(parent driver.Stmt, cfg stmtConfig) stmt {
	return stmt{
		stmtQuery:    cfg.query,
		exec:         makeStmtExecFunc(parent, cfg.execFuncMiddlewares),
		execContext:  makeStmtExecContextFunc(parent, cfg.execContextFuncMiddlewares),
		query:        makeStmtQueryFunc(parent, cfg.queryFuncMiddlewares),
		queryContext: makeStmtQueryContextFunc(parent, cfg.queryContextFuncMiddlewares),
		close:        parent.Close,
		numInput:     parent.NumInput,
	}
}

func makeStmtExecFunc(parent driver.Stmt, execContextFuncMiddlewares []middleware[execContextFunc]) execContextFunc {
	return chainMiddlewares(execContextFuncMiddlewares, func(ctx context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
		return parent.Exec(namedValuesToValues(args)) // nolint: staticcheck
	})
}

func makeStmtExecContextFunc(parent driver.Stmt, execContextFuncMiddlewares []middleware[execContextFunc]) execContextFunc {
	execer, ok := parent.(driver.StmtExecContext)
	if !ok {
		return nopExecContext
	}

	return chainMiddlewares(execContextFuncMiddlewares, func(ctx context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
		return execer.ExecContext(ctx, args)
	})
}

func makeStmtQueryFunc(parent driver.Stmt, queryContextFuncMiddlewares []middleware[queryContextFunc]) queryContextFunc {
	return chainMiddlewares(queryContextFuncMiddlewares, func(ctx context.Context, _ string, args []driver.NamedValue) (driver.Rows, error) {
		return parent.Query(namedValuesToValues(args)) // nolint: staticcheck
	})
}

func makeStmtQueryContextFunc(parent driver.Stmt, queryContextFuncMiddlewares []middleware[queryContextFunc]) queryContextFunc {
	queryer, ok := parent.(driver.StmtQueryContext)
	if !ok {
		return nopQueryContext
	}

	return chainMiddlewares(queryContextFuncMiddlewares, func(ctx context.Context, _ string, args []driver.NamedValue) (driver.Rows, error) {
		return queryer.QueryContext(ctx, args)
	})
}
