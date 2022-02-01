package suite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
	"github.com/godogx/dbsteps"
	"github.com/jmoiron/sqlx"

	"github.com/nhatthm/otelsql"
	"github.com/nhatthm/otelsql/tests/suite/customer"
)

// DatabaseContext is a set of PreparerContext, ExecerContext, QueryerContext.
type DatabaseContext interface {
	PreparerContext
	ExecerContext
	QueryerContext
}

// PreparerContext creates a prepared statement for later queries or executions.
type PreparerContext interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// ExecerContext executes a query without returning any rows.
type ExecerContext interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// QueryerContext executes a query that returns rows, typically a SELECT.
type QueryerContext interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func openDB(driver, dsn string) (*sql.DB, error) {
	driver, err := otelsql.Register(driver,
		otelsql.TraceAll(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not register otelsql: %w", err)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("could not init db: %w", err)
	}

	if err := otelsql.RecordStats(db, otelsql.WithMinimumReadDBStatsInterval(50*time.Millisecond)); err != nil {
		return nil, fmt.Errorf("could not record db stats: %w", err)
	}

	return db, nil
}

func openDBxWithoutInstrumentation(driver, dsn string) (*sqlx.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("could not init db: %w", err)
	}

	return sqlx.NewDb(db, driver), nil
}

func makeDBManager(db *sqlx.DB, placeholderFormat squirrel.PlaceholderFormat) *dbsteps.Manager {
	dbm := dbsteps.NewManager()

	storage := sqluct.NewStorage(db)
	storage.Mapper = &sqluct.Mapper{}
	storage.Format = placeholderFormat

	dbm.Instances = map[string]dbsteps.Instance{
		"default": {
			Storage: storage,
			Tables: map[string]interface{}{
				"customer": new(customer.Customer),
			},
		},
	}

	return dbm
}

// DatabaseExecer executes query depends on what it has.
type DatabaseExecer interface {
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) (*sql.Row, error)
}

type databaseExecer struct {
	db DatabaseContext

	usePreparer bool
}

func (e *databaseExecer) closeStmt(stmt *sql.Stmt) {
	if tx, ok := e.db.(*txContext); ok {
		tx.statements = append(tx.statements, stmt)

		return
	}

	_ = stmt.Close() // nolint: errcheck
}

// Exec executes a query.
func (e *databaseExecer) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if !e.usePreparer {
		return e.db.ExecContext(ctx, query, args...)
	}

	p, err := e.db.PrepareContext(ctx, query) // nolint: sqlclosecheck
	if err != nil {
		return nil, err
	}

	defer e.closeStmt(p)

	return p.ExecContext(ctx, args...)
}

// Query executes a query and returns rows.
func (e *databaseExecer) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if !e.usePreparer {
		return e.db.QueryContext(ctx, query, args...)
	}

	p, err := e.db.PrepareContext(ctx, query) // nolint: sqlclosecheck
	if err != nil {
		return nil, err
	}

	defer e.closeStmt(p)

	return p.QueryContext(ctx, args...)
}

// QueryRow executes a query and returns a row.
func (e *databaseExecer) QueryRow(ctx context.Context, query string, args ...interface{}) (*sql.Row, error) {
	if !e.usePreparer {
		return e.db.QueryRowContext(ctx, query, args...), nil
	}

	p, err := e.db.PrepareContext(ctx, query) // nolint: sqlclosecheck
	if err != nil {
		return nil, err
	}

	defer e.closeStmt(p)

	return p.QueryRowContext(ctx, args...), nil
}

func newDatabaseExecer(db DatabaseContext, usePreparer bool) DatabaseExecer {
	return &databaseExecer{
		db:          db,
		usePreparer: usePreparer,
	}
}

type txContext struct {
	*sql.Tx

	statements []*sql.Stmt
}

func (t *txContext) closeStmts() {
	for _, s := range t.statements {
		_ = s.Close() // nolint: errcheck
	}
}

func (t *txContext) Commit() (err error) {
	err = t.Tx.Commit()

	defer t.closeStmts()

	return
}

func (t *txContext) Rollback() (err error) {
	err = t.Tx.Rollback()

	defer t.closeStmts()

	return
}
