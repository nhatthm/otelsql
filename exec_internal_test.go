package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	"go.nhat.io/otelsql/internal/test/oteltest"
)

func BenchmarkExecStats(b *testing.B) {
	meter := metric.NewNoopMeter()

	histogram, err := meter.Float64Histogram("latency_ms")
	require.NoError(b, err)

	count, err := meter.Int64Counter("calls")
	require.NoError(b, err)

	r := newMethodRecorder(histogram.Record, count.Add,
		semconv.DBSystemOtherSQL,
		dbInstance.String("test"),
	)

	exec := chainMiddlewares([]execContextFuncMiddleware{
		execStats(r, metricMethodExec),
	}, nopExecContext)

	for i := 0; i < b.N; i++ {
		_, _ = exec(context.Background(), "", nil) // nolint: errcheck
	}
}

func TestNopExecContext(t *testing.T) {
	t.Parallel()

	result, err := nopExecContext(context.Background(), "", nil)

	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestSkippedExecContext(t *testing.T) {
	t.Parallel()

	result, err := skippedExecContext(context.Background(), "", nil)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, driver.ErrSkip)
}

func TestChainExecContextFuncMiddlewares_NoMiddleware(t *testing.T) {
	t.Parallel()

	exec := chainMiddlewares(nil, nopExecContext)

	result, err := exec(context.Background(), "", nil)

	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestChainExecContextFuncMiddlewares(t *testing.T) {
	t.Parallel()

	stack := make([]string, 0)

	pushExecContext := func(s string) execContextFunc {
		return func(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
			stack = append(stack, s)

			return nil, nil
		}
	}

	pushExecContextMiddleware := func(s string) execContextFuncMiddleware {
		return func(next execContextFunc) execContextFunc {
			return func(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
				stack = append(stack, s)

				return next(ctx, query, args)
			}
		}
	}

	f := chainMiddlewares(
		[]execContextFuncMiddleware{
			pushExecContextMiddleware("outer"),
			pushExecContextMiddleware("inner"),
		},
		pushExecContext("end"),
	)
	result, err := f(context.Background(), "", nil)

	assert.Nil(t, result)
	assert.NoError(t, err)

	expected := []string{"outer", "inner", "end"}

	assert.Equal(t, expected, stack)
}

func TestExecStats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		execer   execContextFunc
		expected string
	}{
		{
			scenario: "error",
			execer: func(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
				return nil, errors.New("error")
			},
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=exec_test,db.instance=test,db.operation=go.sql.exec,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=exec_test,db.instance=test,db.operation=go.sql.exec,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": "<ignore-diff>",
					"Count": 1
				}
			]`,
		},
		{
			scenario: "no error",
			execer:   nopExecContext,
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=exec_test,db.instance=test,db.operation=go.sql.exec,db.sql.status=OK,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=exec_test,db.instance=test,db.operation=go.sql.exec,db.sql.status=OK,db.system=other_sql}",
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
					meter := s.MeterProvider().Meter("exec_test")

					histogram, err := meter.Float64Histogram(dbSQLClientLatencyMs)
					require.NoError(t, err)

					count, err := meter.Int64Counter(dbSQLClientCalls)
					require.NoError(t, err)

					r := newMethodRecorder(histogram.Record, count.Add,
						semconv.DBSystemOtherSQL,
						dbInstance.String("test"),
					)

					exec := chainMiddlewares([]execContextFuncMiddleware{
						execStats(r, metricMethodExec),
					}, tc.execer)

					_, _ = exec(context.Background(), "", nil) // nolint: errcheck
				})
		})
	}
}
