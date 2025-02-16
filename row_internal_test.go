package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestWrapRows_NoWrap(t *testing.T) {
	t.Parallel()

	expectedError := errors.New("close error")

	parent := struct {
		rowsCloseFunc
		rowsNextFunc
		rowsColumnFunc
	}{
		rowsCloseFunc: func() error {
			return expectedError
		},
	}

	rows := wrapRows(context.Background(), parent, nil, false, false)

	assert.IsType(t, parent, rows)

	actual := rows.Close()

	assert.Equal(t, expectedError, actual)
}

func TestWrapRows_RowsNextResultSet(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario                   string
		parent                     driver.Rows
		expectedHasNextResultSet   bool
		expectedNextResultSetError error
	}{
		{
			scenario: "Rows",
			parent: struct {
				driver.Rows
			}{},
			expectedHasNextResultSet:   false,
			expectedNextResultSetError: io.EOF,
		},
		{
			scenario: "RowsNextResultSet",
			parent: struct {
				rowsNextFunc
				rowsColumnFunc
				rowsCloseFunc
				rowsHasNextResultSetFunc
				rowsNextResultSetFunc
			}{
				rowsHasNextResultSetFunc: func() bool {
					return true
				},
				rowsNextResultSetFunc: func() error {
					return errors.New("next error")
				},
			},
			expectedHasNextResultSet:   true,
			expectedNextResultSetError: errors.New("next error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			rows := wrapRows(context.Background(), tc.parent, nil, true, true)

			require.Implements(t, (*driver.RowsNextResultSet)(nil), rows)

			hasNextResultSet := rows.(driver.RowsNextResultSet).HasNextResultSet() //nolint: errcheck
			nextResultSetError := rows.(driver.RowsNextResultSet).NextResultSet()  //nolint: errcheck

			assert.Equal(t, tc.expectedHasNextResultSet, hasNextResultSet)
			assert.Equal(t, tc.expectedNextResultSetError, nextResultSetError)
		})
	}
}

func TestWrapRows_RowsColumnTypeDatabaseTypeName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		parent   driver.Rows
		expected string
	}{
		{
			scenario: "Rows",
			parent: struct {
				driver.Rows
			}{},
			expected: "",
		},
		{
			scenario: "RowsColumnTypeDatabaseTypeName",
			parent: struct {
				rowsNextFunc
				rowsColumnFunc
				rowsCloseFunc
				rowsColumnTypeDatabaseTypeNameFunc
			}{
				rowsColumnTypeDatabaseTypeNameFunc: func(int) string {
					return "foobar"
				},
			},
			expected: "foobar",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			rows := wrapRows(context.Background(), tc.parent, nil, true, true)

			require.Implements(t, (*driver.RowsColumnTypeDatabaseTypeName)(nil), rows)

			actual := rows.(driver.RowsColumnTypeDatabaseTypeName).ColumnTypeDatabaseTypeName(0) //nolint: errcheck

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestWrapRows_RowsColumnTypeLength(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario       string
		parent         driver.Rows
		expectedLength int64
		expectedOK     bool
	}{
		{
			scenario: "Rows",
			parent: struct {
				driver.Rows
			}{},
			expectedLength: 0,
			expectedOK:     false,
		},
		{
			scenario: "RowsColumnTypeLength",
			parent: struct {
				rowsNextFunc
				rowsColumnFunc
				rowsCloseFunc
				rowsColumnTypeLengthFunc
			}{
				rowsColumnTypeLengthFunc: func(int) (int64, bool) {
					return 42, true
				},
			},
			expectedLength: 42,
			expectedOK:     true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			rows := wrapRows(context.Background(), tc.parent, nil, true, true)

			require.Implements(t, (*driver.RowsColumnTypeLength)(nil), rows)

			length, ok := rows.(driver.RowsColumnTypeLength).ColumnTypeLength(0) //nolint: errcheck

			assert.Equal(t, tc.expectedLength, length)
			assert.Equal(t, tc.expectedOK, ok)
		})
	}
}

func TestWrapRows_RowsColumnTypeNullable(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario         string
		parent           driver.Rows
		expectedNullable bool
		expectedOK       bool
	}{
		{
			scenario: "Rows",
			parent: struct {
				driver.Rows
			}{},
			expectedNullable: false,
			expectedOK:       false,
		},
		{
			scenario: "RowsColumnTypeNullable",
			parent: struct {
				rowsNextFunc
				rowsColumnFunc
				rowsCloseFunc
				rowsColumnTypeNullableFunc
			}{
				rowsColumnTypeNullableFunc: func(int) (nullable, ok bool) {
					return true, true
				},
			},
			expectedNullable: true,
			expectedOK:       true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			rows := wrapRows(context.Background(), tc.parent, nil, true, true)

			require.Implements(t, (*driver.RowsColumnTypeNullable)(nil), rows)

			nullable, ok := rows.(driver.RowsColumnTypeNullable).ColumnTypeNullable(0) //nolint: errcheck

			assert.Equal(t, tc.expectedNullable, nullable)
			assert.Equal(t, tc.expectedOK, ok)
		})
	}
}

func TestWrapRows_RowsColumnTypePrecisionScale(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario          string
		parent            driver.Rows
		expectedPrecision int64
		expectedScale     int64
		expectedOK        bool
	}{
		{
			scenario: "Rows",
			parent: struct {
				driver.Rows
			}{},
			expectedPrecision: 0,
			expectedScale:     0,
			expectedOK:        false,
		},
		{
			scenario: "RowsColumnTypePrecisionScale",
			parent: struct {
				rowsNextFunc
				rowsColumnFunc
				rowsCloseFunc
				rowsColumnTypePrecisionScaleFunc
			}{
				rowsColumnTypePrecisionScaleFunc: func(int) (precision, scale int64, ok bool) {
					return 10, 42, true
				},
			},
			expectedPrecision: 10,
			expectedScale:     42,
			expectedOK:        true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			rows := wrapRows(context.Background(), tc.parent, nil, true, true)

			require.Implements(t, (*driver.RowsColumnTypePrecisionScale)(nil), rows)

			precision, scale, ok := rows.(driver.RowsColumnTypePrecisionScale).ColumnTypePrecisionScale(0) //nolint: errcheck

			assert.Equal(t, tc.expectedPrecision, precision)
			assert.Equal(t, tc.expectedScale, scale)
			assert.Equal(t, tc.expectedOK, ok)
		})
	}
}

func TestWrapRows_RowsColumnTypeScanType(t *testing.T) {
	t.Parallel()

	parent := struct {
		rowsCloseFunc
		rowsNextFunc
		rowsColumnFunc
		rowsColumnTypeScanTypeFunc
	}{
		rowsColumnTypeScanTypeFunc: func(index int) reflect.Type {
			return reflect.TypeOf(index)
		},
	}

	rows := wrapRows(context.Background(), parent, nil, true, true)

	require.Implements(t, (*driver.RowsColumnTypeScanType)(nil), rows)

	actual := rows.(driver.RowsColumnTypeScanType).ColumnTypeScanType(0) //nolint: errcheck
	expected := reflect.TypeOf(0)

	assert.Equal(t, expected, actual)
}

func TestRowsCloseAttributes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario  string
		count     int64
		totalTime time.Duration
		expected  []attribute.KeyValue
	}{
		{
			scenario:  "now rows",
			totalTime: time.Second,
			expected: []attribute.KeyValue{
				dbSQLRowsNextSuccessCount.Int64(0),
			},
		},
		{
			scenario:  "has rows",
			count:     1,
			totalTime: 110 * time.Microsecond,
			expected: []attribute.KeyValue{
				dbSQLRowsNextSuccessCount.Int64(1),
				dbSQLRowsNextLatencyAvg.String("110us"),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			actual := rowsCloseAttributes(tc.count, tc.totalTime)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func (f rowsColumnFunc) Columns() []string {
	return f()
}

func (f rowsCloseFunc) Close() error {
	return f()
}

func (f rowsNextFunc) Next(dest []driver.Value) (err error) {
	return f(dest)
}

type rowsColumnTypeDatabaseTypeNameFunc func(index int) string

func (f rowsColumnTypeDatabaseTypeNameFunc) ColumnTypeDatabaseTypeName(index int) string {
	return f(index)
}

type rowsColumnTypeLengthFunc func(index int) (int64, bool)

func (f rowsColumnTypeLengthFunc) ColumnTypeLength(index int) (int64, bool) {
	return f(index)
}

type rowsColumnTypeNullableFunc func(index int) (nullable, ok bool)

func (f rowsColumnTypeNullableFunc) ColumnTypeNullable(index int) (nullable, ok bool) {
	return f(index)
}

type rowsColumnTypePrecisionScaleFunc func(index int) (precision, scale int64, ok bool)

func (f rowsColumnTypePrecisionScaleFunc) ColumnTypePrecisionScale(index int) (precision, scale int64, ok bool) {
	return f(index)
}

type rowsColumnTypeScanTypeFunc func(index int) reflect.Type

func (f rowsColumnTypeScanTypeFunc) ColumnTypeScanType(index int) reflect.Type {
	return f(index)
}

type rowsHasNextResultSetFunc func() bool

func (f rowsHasNextResultSetFunc) HasNextResultSet() bool {
	return f()
}

type rowsNextResultSetFunc func() error

func (f rowsNextResultSetFunc) NextResultSet() error {
	return f()
}
