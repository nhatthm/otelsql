package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStmt_Exec(t *testing.T) {
	t.Parallel()

	expectedValues := []driver.Value{"foobar", 42}

	parent := struct {
		stmtCloseFunc
		stmtNumInputFunc
		stmtExecFunc
		stmtQueryFunc
	}{
		stmtExecFunc: func(actualValues []driver.Value) (driver.Result, error) {
			assert.Equal(t, expectedValues, actualValues)

			return nil, errors.New("exec error")
		},
	}

	stmt := wrapStmt(parent, stmtConfig{})

	result, actual := stmt.Exec(expectedValues) // nolint: staticcheck
	expected := errors.New("exec error")

	assert.Nil(t, result)
	assert.Equal(t, expected, actual)
}

func TestStmt_Query(t *testing.T) {
	t.Parallel()

	expectedValues := []driver.Value{"foobar", 42}

	parent := struct {
		stmtCloseFunc
		stmtNumInputFunc
		stmtExecFunc
		stmtQueryFunc
	}{
		stmtQueryFunc: func(actualValues []driver.Value) (driver.Rows, error) {
			assert.Equal(t, expectedValues, actualValues)

			return nil, errors.New("query error")
		},
	}

	stmt := wrapStmt(parent, stmtConfig{})

	rows, actual := stmt.Query(expectedValues) // nolint: staticcheck
	expected := errors.New("query error")

	assert.Nil(t, rows)
	assert.Equal(t, expected, actual)
}

func TestWrapStmt(t *testing.T) {
	t.Parallel()

	var (
		expectedCloseError        = errors.New("close error")
		expectedNumInput          = 42
		expectedExecError         = errors.New("exec error")
		expectedExecContextError  = errors.New("exec context error")
		expectedQueryError        = errors.New("query error")
		expectedQueryContextError = errors.New("query context error")
		expectedConvertValueError = errors.New("convert value error")
		expectedCheckValueError   = errors.New("check value error")
	)

	parent := struct {
		stmtCloseFunc
		stmtNumInputFunc
		stmtExecFunc
		stmtQueryFunc
	}{
		stmtCloseFunc: func() error {
			return expectedCloseError
		},
		stmtNumInputFunc: func() int {
			return expectedNumInput
		},
		stmtExecFunc: func([]driver.Value) (driver.Result, error) {
			return nil, expectedExecError
		},
		stmtQueryFunc: func([]driver.Value) (driver.Rows, error) {
			return nil, expectedQueryError
		},
	}

	execContextFunc := stmtExecContextFunc(func(context.Context, []driver.NamedValue) (driver.Result, error) {
		return nil, expectedExecContextError
	})

	queryContextFunc := stmtQueryContextFunc(func(context.Context, []driver.NamedValue) (driver.Rows, error) {
		return nil, expectedQueryContextError
	})

	columnConverterFunc := stmtColumnConverterFunc(func(int) driver.ValueConverter {
		return valueConverterFunc(func(any) (driver.Value, error) {
			return nil, expectedConvertValueError
		})
	})

	namedValueCheckerFunc := stmtNamedValueChecker(func(*driver.NamedValue) error {
		return expectedCheckValueError
	})

	assertExecContextFunc := func(t *testing.T, stmt driver.Stmt) {
		t.Helper()

		_, err := stmt.(driver.StmtExecContext).ExecContext(context.Background(), nil)
		assert.Equal(t, expectedExecContextError, err)
	}

	assertQueryContextFunc := func(t *testing.T, stmt driver.Stmt) {
		t.Helper()

		_, err := stmt.(driver.StmtQueryContext).QueryContext(context.Background(), nil)
		assert.Equal(t, expectedQueryContextError, err)
	}

	assertColumnConverterFunc := func(t *testing.T, stmt driver.Stmt) {
		t.Helper()

		_, err := stmt.(driver.ColumnConverter).ColumnConverter(0).ConvertValue(nil) // nolint: staticcheck
		assert.Equal(t, expectedConvertValueError, err)
	}

	assertNamedValueCheckerFunc := func(t *testing.T, stmt driver.Stmt) {
		t.Helper()

		err := stmt.(driver.NamedValueChecker).CheckNamedValue(nil)
		assert.Equal(t, expectedCheckValueError, err)
	}

	testCases := []struct {
		scenario     string
		parent       driver.Stmt
		expectedType any
		assert       func(t *testing.T, stmt driver.Stmt)
	}{
		{
			scenario: "!hasExeCtx && !hasQryCtx && !hasColConv && !hasNamValChk",
			parent:   parent,
			expectedType: struct {
				driver.Stmt
			}{},
			assert: func(*testing.T, driver.Stmt) {},
		},
		{
			scenario: "!hasExeCtx && hasQryCtx && !hasColConv && !hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtQueryContext
			}{
				Stmt:             parent,
				StmtQueryContext: queryContextFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtQueryContext
			}{},
			assert: assertQueryContextFunc,
		},
		{
			scenario: "hasExeCtx && !hasQryCtx && !hasColConv && !hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtExecContext
			}{
				Stmt:            parent,
				StmtExecContext: execContextFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtExecContext
			}{},
			assert: assertExecContextFunc,
		},
		{
			scenario: "hasExeCtx && hasQryCtx && !hasColConv && !hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.StmtQueryContext
			}{
				Stmt:             parent,
				StmtExecContext:  execContextFunc,
				StmtQueryContext: queryContextFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.StmtQueryContext
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertExecContextFunc(t, stmt)
				assertQueryContextFunc(t, stmt)
			},
		},
		{
			scenario: "!hasExeCtx && !hasQryCtx && hasColConv && !hasNamValChk",
			parent: struct {
				driver.Stmt
				columnConverter
			}{
				Stmt:            parent,
				columnConverter: columnConverterFunc,
			},
			expectedType: struct {
				driver.Stmt
				columnConverter
			}{},
			assert: assertColumnConverterFunc,
		},
		{
			scenario: "!hasExeCtx && hasQryCtx && hasColConv && !hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtQueryContext
				columnConverter
			}{
				Stmt:             parent,
				StmtQueryContext: queryContextFunc,
				columnConverter:  columnConverterFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtQueryContext
				columnConverter
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertQueryContextFunc(t, stmt)
				assertColumnConverterFunc(t, stmt)
			},
		},
		{
			scenario: "hasExeCtx && !hasQryCtx && hasColConv && !hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtExecContext
				columnConverter
			}{
				Stmt:            parent,
				StmtExecContext: execContextFunc,
				columnConverter: columnConverterFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtExecContext
				columnConverter
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertExecContextFunc(t, stmt)
				assertColumnConverterFunc(t, stmt)
			},
		},
		{
			scenario: "hasExeCtx && hasQryCtx && hasColConv && !hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.StmtQueryContext
				columnConverter
			}{
				Stmt:             parent,
				StmtExecContext:  execContextFunc,
				StmtQueryContext: queryContextFunc,
				columnConverter:  columnConverterFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.StmtQueryContext
				columnConverter
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertExecContextFunc(t, stmt)
				assertQueryContextFunc(t, stmt)
				assertColumnConverterFunc(t, stmt)
			},
		},
		{
			scenario: "!hasExeCtx && !hasQryCtx && !hasColConv && hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.NamedValueChecker
			}{
				Stmt:              parent,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.NamedValueChecker
			}{},
			assert: assertNamedValueCheckerFunc,
		},
		{
			scenario: "!hasExeCtx && hasQryCtx && !hasColConv && hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtQueryContext
				driver.NamedValueChecker
			}{
				Stmt:              parent,
				StmtQueryContext:  queryContextFunc,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtQueryContext
				driver.NamedValueChecker
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertQueryContextFunc(t, stmt)
				assertNamedValueCheckerFunc(t, stmt)
			},
		},
		{
			scenario: "hasExeCtx && !hasQryCtx && !hasColConv && hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.NamedValueChecker
			}{
				Stmt:              parent,
				StmtExecContext:   execContextFunc,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.NamedValueChecker
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertExecContextFunc(t, stmt)
				assertNamedValueCheckerFunc(t, stmt)
			},
		},
		{
			scenario: "hasExeCtx && hasQryCtx && !hasColConv && hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.StmtQueryContext
				driver.NamedValueChecker
			}{
				Stmt:              parent,
				StmtExecContext:   execContextFunc,
				StmtQueryContext:  queryContextFunc,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.StmtQueryContext
				driver.NamedValueChecker
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertExecContextFunc(t, stmt)
				assertQueryContextFunc(t, stmt)
				assertNamedValueCheckerFunc(t, stmt)
			},
		},
		{
			scenario: "!hasExeCtx && !hasQryCtx && hasColConv && hasNamValChk",
			parent: struct {
				driver.Stmt
				columnConverter
				driver.NamedValueChecker
			}{
				Stmt:              parent,
				columnConverter:   columnConverterFunc,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				driver.Stmt
				columnConverter
				driver.NamedValueChecker
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertColumnConverterFunc(t, stmt)
				assertNamedValueCheckerFunc(t, stmt)
			},
		},
		{
			scenario: "!hasExeCtx && hasQryCtx && hasColConv && hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtQueryContext
				columnConverter
				driver.NamedValueChecker
			}{
				Stmt:              parent,
				StmtQueryContext:  queryContextFunc,
				columnConverter:   columnConverterFunc,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtQueryContext
				columnConverter
				driver.NamedValueChecker
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertQueryContextFunc(t, stmt)
				assertColumnConverterFunc(t, stmt)
				assertNamedValueCheckerFunc(t, stmt)
			},
		},
		{
			scenario: "hasExeCtx && !hasQryCtx && hasColConv && hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtExecContext
				columnConverter
				driver.NamedValueChecker
			}{
				Stmt:              parent,
				StmtExecContext:   execContextFunc,
				columnConverter:   columnConverterFunc,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtExecContext
				columnConverter
				driver.NamedValueChecker
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertExecContextFunc(t, stmt)
				assertColumnConverterFunc(t, stmt)
				assertNamedValueCheckerFunc(t, stmt)
			},
		},
		{
			scenario: "hasExeCtx && hasQryCtx && hasColConv && hasNamValChk",
			parent: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.StmtQueryContext
				columnConverter
				driver.NamedValueChecker
			}{
				Stmt:              parent,
				StmtExecContext:   execContextFunc,
				StmtQueryContext:  queryContextFunc,
				columnConverter:   columnConverterFunc,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				driver.Stmt
				driver.StmtExecContext
				driver.StmtQueryContext
				columnConverter
				driver.NamedValueChecker
			}{},
			assert: func(t *testing.T, stmt driver.Stmt) {
				t.Helper()

				assertExecContextFunc(t, stmt)
				assertQueryContextFunc(t, stmt)
				assertColumnConverterFunc(t, stmt)
				assertNamedValueCheckerFunc(t, stmt)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			stmt := wrapStmt(tc.parent, stmtConfig{})

			assert.IsType(t, tc.expectedType, stmt)

			// Basic asserts.
			err := stmt.Close()
			assert.Equal(t, expectedCloseError, err)

			numInput := stmt.NumInput()
			assert.Equal(t, expectedNumInput, numInput)

			_, err = stmt.Exec(nil) // nolint: staticcheck
			assert.Equal(t, expectedExecError, err)

			_, err = stmt.Query(nil) // nolint: staticcheck
			assert.Equal(t, expectedQueryError, err)

			// Extra asserts.
			tc.assert(t, stmt)
		})
	}
}

type stmtCloseFunc func() error

func (f stmtCloseFunc) Close() error {
	return f()
}

type stmtNumInputFunc func() int

func (f stmtNumInputFunc) NumInput() int {
	return f()
}

type stmtExecFunc func(args []driver.Value) (driver.Result, error)

func (f stmtExecFunc) Exec(args []driver.Value) (driver.Result, error) {
	return f(args)
}

type stmtQueryFunc func(args []driver.Value) (driver.Rows, error)

func (f stmtQueryFunc) Query(args []driver.Value) (driver.Rows, error) {
	return f(args)
}

type stmtExecContextFunc func(ctx context.Context, args []driver.NamedValue) (driver.Result, error)

func (f stmtExecContextFunc) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return f(ctx, args)
}

type stmtQueryContextFunc func(ctx context.Context, args []driver.NamedValue) (driver.Rows, error)

func (f stmtQueryContextFunc) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return f(ctx, args)
}

type stmtColumnConverterFunc func(idx int) driver.ValueConverter

func (f stmtColumnConverterFunc) ColumnConverter(idx int) driver.ValueConverter {
	return f(idx)
}

type stmtNamedValueChecker func(value *driver.NamedValue) error

func (f stmtNamedValueChecker) CheckNamedValue(value *driver.NamedValue) error {
	return f(value)
}

type valueConverterFunc func(v any) (driver.Value, error)

func (f valueConverterFunc) ConvertValue(v any) (driver.Value, error) {
	return f(v)
}
