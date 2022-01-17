package sqlmock

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sqlmock interface.
type Sqlmock = sqlmock.Sqlmock

// New creates sqlmock database connection and a mock to manage expectations.
var New = sqlmock.New

// NewResult creates a new sql driver Result for Exec based query mocks.
var NewResult = sqlmock.NewResult

// NewRows allows Rows to be created from a sql driver.Value slice or from the CSV string and to be used as sql driver.Rows.
var NewRows = sqlmock.NewRows

// Sqlmocker mocks and returns a sqlmock instance.
type Sqlmocker func(t testing.TB) string

// Register creates a new sqlmock instance and returns the dsn to connect to it.
func Register(mocks ...func(m Sqlmock)) Sqlmocker {
	return func(tb testing.TB) string {
		tb.Helper()

		mockDB, m, err := sqlmock.New(
			sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
			sqlmock.MonitorPingsOption(true),
		)
		require.NoError(tb, err)

		for _, mock := range mocks {
			mock(m)
		}

		tb.Cleanup(func() {
			assert.NoError(tb, m.ExpectationsWereMet())

			// We do not care if closing mock fails.
			_ = mockDB.Close() // nolint: errcheck
		})

		return getDSN(m)
	}
}

func getDSN(m Sqlmock) string {
	return reflect.Indirect(reflect.ValueOf(m)).FieldByName("dsn").String()
}
