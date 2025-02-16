package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConn_Exec(t *testing.T) {
	t.Parallel()

	c := conn{}
	result, err := c.Exec("", nil)

	assert.Nil(t, result)
	assert.Error(t, err)
}

func TestConn_Query(t *testing.T) {
	t.Parallel()

	c := conn{}
	rows, err := c.Query("", nil)

	assert.Nil(t, rows)
	assert.Error(t, err)
}

func TestConn_Prepare(t *testing.T) {
	t.Parallel()

	c := conn{
		prepare: func(ctx context.Context, query string) (driver.Stmt, error) {
			assert.Equal(t, context.Background(), ctx)
			assert.Empty(t, query)

			return nil, errors.New("prepare context error")
		},
	}

	result, actual := c.Prepare("")
	expected := errors.New("prepare context error")

	assert.Nil(t, result)
	assert.Equal(t, expected, actual)
}

func TestConn_Begin(t *testing.T) {
	t.Parallel()

	c := conn{
		begin: func(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
			assert.Equal(t, context.Background(), ctx)
			assert.Equal(t, driver.TxOptions{}, opts)

			return nil, errors.New("begin context error")
		},
	}

	tx, actual := c.Begin()
	expected := errors.New("begin context error")

	assert.Nil(t, tx)
	assert.Equal(t, expected, actual)
}

func TestWrapConn(t *testing.T) {
	t.Parallel()

	var (
		expectedPrepareError      = errors.New("prepare error")
		expectedCloseError        = errors.New("close error")
		expectedBeginError        = errors.New("begin error")
		expectedCheckValueError   = errors.New("check value error")
		expectedResetSessionError = errors.New("reset session error")
	)

	parent := struct {
		connPrepareFunc
		connCloseFunc
		connBeginFunc
	}{
		connPrepareFunc: func(string) (driver.Stmt, error) {
			return nil, expectedPrepareError
		},
		connCloseFunc: func() error {
			return expectedCloseError
		},
		connBeginFunc: func() (driver.Tx, error) {
			return nil, expectedBeginError
		},
	}

	namedValueCheckerFunc := connNamedValueChecker(func(*driver.NamedValue) error {
		return expectedCheckValueError
	})

	sessionResetterFunc := connSessionResetter(func(context.Context) error {
		return expectedResetSessionError
	})

	assertNamedValueCheckerFunc := func(t *testing.T, conn driver.Conn) {
		t.Helper()

		err := conn.(driver.NamedValueChecker).CheckNamedValue(nil) //nolint: errcheck
		require.ErrorIs(t, err, expectedCheckValueError)
	}

	assertSessionResetterFunc := func(t *testing.T, conn driver.Conn) {
		t.Helper()

		err := conn.(driver.SessionResetter).ResetSession(context.Background()) //nolint: errcheck
		require.ErrorIs(t, err, expectedResetSessionError, err)
	}

	testCases := []struct {
		scenario     string
		parent       driver.Conn
		expectedType any
		assert       func(t *testing.T, conn driver.Conn)
	}{
		{
			scenario:     "!hasNameValueChecker && !hasSessionResetter",
			parent:       parent,
			expectedType: conn{},
			assert:       func(*testing.T, driver.Conn) {},
		},
		{
			scenario: "hasNameValueChecker && !hasSessionResetter",
			parent: struct {
				driver.Conn
				driver.NamedValueChecker
			}{
				Conn:              parent,
				NamedValueChecker: namedValueCheckerFunc,
			},
			expectedType: struct {
				conn
				driver.NamedValueChecker
			}{},
			assert: assertNamedValueCheckerFunc,
		},
		{
			scenario: "!hasNameValueChecker && hasSessionResetter",
			parent: struct {
				driver.Conn
				driver.SessionResetter
			}{
				Conn:            parent,
				SessionResetter: sessionResetterFunc,
			},
			expectedType: struct {
				conn
				driver.SessionResetter
			}{},
			assert: assertSessionResetterFunc,
		},
		{
			scenario: "hasNameValueChecker && hasSessionResetter",
			parent: struct {
				driver.Conn
				driver.NamedValueChecker
				driver.SessionResetter
			}{
				Conn:              parent,
				NamedValueChecker: namedValueCheckerFunc,
				SessionResetter:   sessionResetterFunc,
			},
			expectedType: struct {
				conn
				driver.NamedValueChecker
				driver.SessionResetter
			}{},
			assert: func(t *testing.T, conn driver.Conn) {
				t.Helper()

				assertNamedValueCheckerFunc(t, conn)
				assertSessionResetterFunc(t, conn)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			conn := wrapConn(tc.parent, connConfig{})

			assert.IsType(t, tc.expectedType, conn)

			// Basic asserts.
			err := conn.Close()
			assert.Equal(t, expectedCloseError, err)

			stmt, err := conn.Prepare("")

			assert.Nil(t, stmt)
			assert.Equal(t, expectedPrepareError, err)

			tx, err := conn.Begin() // nolint: staticcheck

			assert.Nil(t, tx)
			assert.Equal(t, expectedBeginError, err)

			// Extra asserts.
			tc.assert(t, conn)
		})
	}
}

type connPrepareFunc func(query string) (driver.Stmt, error)

func (f connPrepareFunc) Prepare(query string) (driver.Stmt, error) {
	return f(query)
}

type connCloseFunc func() error

func (f connCloseFunc) Close() error {
	return f()
}

type connBeginFunc func() (driver.Tx, error)

func (f connBeginFunc) Begin() (driver.Tx, error) {
	return f()
}

type connNamedValueChecker func(value *driver.NamedValue) error

func (f connNamedValueChecker) CheckNamedValue(value *driver.NamedValue) error {
	return f(value)
}

type connSessionResetter func(ctx context.Context) error

func (f connSessionResetter) ResetSession(ctx context.Context) error {
	return f(ctx)
}
