package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"

	"go.nhat.io/otelsql/internal/test/oteltest"
)

func BenchmarkQueryStats(b *testing.B) {
	meter := noop.NewMeterProvider().Meter("query_test")

	histogram, err := meter.Float64Histogram("latency_ms")
	require.NoError(b, err)

	count, err := meter.Int64Counter("calls")
	require.NoError(b, err)

	r := newMethodRecorder(histogram.Record, count.Add,
		semconv.DBSystemOtherSQL,
		dbInstance.String("test"),
	)

	query := chainMiddlewares([]queryContextFuncMiddleware{
		queryStats(r, metricMethodQuery),
	}, nopQueryContext)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = query(context.Background(), "", nil) // nolint: errcheck
	}
}

func TestNopQueryContext(t *testing.T) {
	t.Parallel()

	result, err := nopQueryContext(context.Background(), "", nil)

	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestSkippedQueryContext(t *testing.T) {
	t.Parallel()

	result, err := skippedQueryContext(context.Background(), "", nil)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, driver.ErrSkip)
}

func TestChainQueryContextFuncMiddlewares_NoMiddleware(t *testing.T) {
	t.Parallel()

	query := chainMiddlewares(nil, nopQueryContext)

	result, err := query(context.Background(), "", nil)

	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestChainQueryContextFuncMiddlewares(t *testing.T) {
	t.Parallel()

	stack := make([]string, 0)

	pushQueryContext := func(s string) queryContextFunc {
		return func(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
			stack = append(stack, s)

			return nil, nil
		}
	}

	pushQueryContextMiddleware := func(s string) queryContextFuncMiddleware {
		return func(next queryContextFunc) queryContextFunc {
			return func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
				stack = append(stack, s)

				return next(ctx, query, args)
			}
		}
	}

	query := chainMiddlewares(
		[]queryContextFuncMiddleware{
			pushQueryContextMiddleware("outer"),
			pushQueryContextMiddleware("inner"),
		},
		pushQueryContext("end"),
	)
	result, err := query(context.Background(), "", nil)

	assert.Nil(t, result)
	assert.NoError(t, err)

	expected := []string{"outer", "inner", "end"}

	assert.Equal(t, expected, stack)
}

func TestQueryStats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		query    queryContextFunc
		expected string
	}{
		{
			scenario: "error",
			query: func(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
				return nil, errors.New("error")
			},
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=query_test,db.instance=test,db.operation=go.sql.query,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=query_test,db.instance=test,db.operation=go.sql.query,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": "<ignore-diff>",
					"Count": 1
				}
			]`,
		},
		{
			scenario: "no error",
			query:    nopQueryContext,
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=query_test,db.instance=test,db.operation=go.sql.query,db.sql.status=OK,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=query_test,db.instance=test,db.operation=go.sql.query,db.sql.status=OK,db.system=other_sql}",
					"Sum": "<ignore-diff>",
					"Count": 1
				}
			]`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			oteltest.New(oteltest.MetricsEqualJSON(tc.expected)).
				Run(t, func(s oteltest.SuiteContext) {
					meter := s.MeterProvider().Meter("query_test")

					histogram, err := meter.Float64Histogram(dbSQLClientLatencyMs)
					require.NoError(t, err)

					count, err := meter.Int64Counter(dbSQLClientCalls)
					require.NoError(t, err)

					r := newMethodRecorder(histogram.Record, count.Add,
						semconv.DBSystemOtherSQL,
						dbInstance.String("test"),
					)

					query := chainMiddlewares([]queryContextFuncMiddleware{
						queryStats(r, metricMethodQuery),
					}, tc.query)

					_, _ = query(context.Background(), "", nil) // nolint: errcheck
				})
		})
	}
}
