package trace_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"github.com/nhatthm/otelsql"
	"github.com/nhatthm/otelsql/internal/test/oteltest"
	xtrace "github.com/nhatthm/otelsql/x/trace"
)

func Test_TransformAndTraceQueryWithoutArgs(t *testing.T) {
	t.Parallel()

	expectedTraceWithQuery := expectedExecTraceWithQuery(sampleParentSpanIDs())

	oteltest.New(
		oteltest.TracesEqualJSON(expectedTraceWithQuery),
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectExec(`    DELETE FROM data WHERE country = $1    `).
				WithArgs("US").
				WillReturnResult(sqlmock.NewResult(0, 10))
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(),
				otelsql.WithMeterProvider(sc.MeterProvider()),
				otelsql.WithTracerProvider(sc.TracerProvider()),
				xtrace.TransformAndTraceQueryWithoutArgs(strings.TrimSpace),
			)
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			result, err := db.ExecContext(contextWithSampleSpan(), `    DELETE FROM data WHERE country = $1    `, "US")

			require.NoError(t, err)

			affectedRows, err := result.RowsAffected()

			require.Equal(t, int64(10), affectedRows)
			require.NoError(t, err)
		})
}

func newDB(dsn string, opts ...otelsql.DriverOption) (*sql.DB, error) {
	driverName, err := otelsql.RegisterWithSource("sqlmock", dsn, opts...)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func contextWithSampleSpan() context.Context {
	return oteltest.BackgroundWithSpanContext(sampleParentSpanIDs())
}

func sampleParentSpanIDs() (trace.TraceID, trace.SpanID) {
	return oteltest.SampleTraceID, oteltest.SampleSpanID
}

func mustNotFail(err error) {
	if err != nil {
		panic(err)
	}
}

func getFixture(file string, args ...interface{}) string {
	data, err := os.ReadFile(filepath.Clean(file))
	mustNotFail(err)

	return fmt.Sprintf(string(data), args...)
}

func expectedTracesFromFile(file string, args ...interface{}) string {
	return getFixture("../../resources/fixtures/traces/"+file, args...)
}

func expectedExecTraceWithQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("exec_with_query.json", parentTraceID, parentSpanID)
}
