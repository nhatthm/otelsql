package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"

	"github.com/nhatthm/otelsql/internal/test/oteltest"
)

func BenchmarkBeginStats(b *testing.B) {
	meter := metric.NewNoopMeterProvider().Meter("")

	histogram, err := meter.NewFloat64Histogram("latency_ms")
	require.NoError(b, err)

	count, err := meter.NewInt64Counter("calls")
	require.NoError(b, err)

	r := newMethodRecorder(histogram.Record, count.Add,
		semconv.DBSystemOtherSQL,
		dbInstance.String("test"),
	)

	begin := chainBeginFuncMiddlewares([]beginFuncMiddleware{
		beginStats(r),
	}, noOpBegin)

	for i := 0; i < b.N; i++ {
		_, _ = begin(context.Background(), driver.TxOptions{}) // nolint: errcheck
	}
}

func TestNoOpBegin(t *testing.T) {
	t.Parallel()

	result, err := noOpBegin(context.Background(), driver.TxOptions{})

	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestEnsureBegin(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario      string
		conn          driver.Conn
		expectedTx    driver.Tx
		expectedError error
	}{
		{
			scenario:      "begin",
			conn:          beginTest{err: errors.New("begin error")},
			expectedError: errors.New("begin error"),
		},
		{
			scenario:      "begin context",
			conn:          beginTxTest{err: errors.New("begin context error")},
			expectedError: errors.New("begin context error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			begin := ensureBegin(tc.conn)
			tx, err := begin(context.Background(), driver.TxOptions{})

			assert.Equal(t, tc.expectedTx, tx)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestChainBeginFuncMiddlewares_NoMiddleware(t *testing.T) {
	t.Parallel()

	begin := chainBeginFuncMiddlewares(nil, noOpBegin)

	result, err := begin(context.Background(), driver.TxOptions{})

	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestChainBeginFuncMiddlewares(t *testing.T) {
	t.Parallel()

	stack := make([]string, 0)

	pushBeginFunc := func(s string) beginFunc {
		return func(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
			stack = append(stack, s)

			return nil, nil
		}
	}

	pushBeginFuncMiddleware := func(s string) beginFuncMiddleware {
		return func(next beginFunc) beginFunc {
			return func(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
				stack = append(stack, s)

				return next(ctx, opts)
			}
		}
	}

	begin := chainBeginFuncMiddlewares(
		[]beginFuncMiddleware{
			pushBeginFuncMiddleware("outer"),
			pushBeginFuncMiddleware("inner"),
		},
		pushBeginFunc("end"),
	)
	result, err := begin(context.Background(), driver.TxOptions{})

	assert.Nil(t, result)
	assert.NoError(t, err)

	expected := []string{"outer", "inner", "end"}

	assert.Equal(t, expected, stack)
}

func TestBeginStats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		begin    beginFunc
		expected string
	}{
		{
			scenario: "error",
			begin: func(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
				return nil, errors.New("error")
			},
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=begin_test,db.instance=test,db.operation=go.sql.begin,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=begin_test,db.instance=test,db.operation=go.sql.begin,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": "<ignore-diff>"
				}
			]`,
		},
		{
			scenario: "no error",
			begin:    noOpBegin,
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=begin_test,db.instance=test,db.operation=go.sql.begin,db.sql.status=OK,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=begin_test,db.instance=test,db.operation=go.sql.begin,db.sql.status=OK,db.system=other_sql}",
					"Sum": "<ignore-diff>"
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
					meter := s.MeterProvider().Meter("begin_test")

					histogram, err := meter.NewFloat64Histogram(dbSQLClientLatencyMs)
					require.NoError(t, err)

					count, err := meter.NewInt64Counter(dbSQLClientCalls)
					require.NoError(t, err)

					r := newMethodRecorder(histogram.Record, count.Add,
						semconv.DBSystemOtherSQL,
						dbInstance.String("test"),
					)

					begin := chainBeginFuncMiddlewares([]beginFuncMiddleware{
						beginStats(r),
					}, tc.begin)

					_, _ = begin(context.Background(), driver.TxOptions{}) // nolint: errcheck
				})
		})
	}
}

type beginTxTest struct {
	driver.Conn
	driver.ConnBeginTx

	tx  driver.Tx
	err error
}

func (t beginTxTest) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return t.tx, t.err
}

type beginTest struct {
	driver.Conn

	tx  driver.Tx
	err error
}

func (t beginTest) Begin() (driver.Tx, error) {
	return t.tx, t.err
}
