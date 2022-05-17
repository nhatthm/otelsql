package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/nonrecording"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/nhatthm/otelsql/internal/test/oteltest"
)

func BenchmarkPrepareStats(b *testing.B) {
	meter := nonrecording.NewNoopMeter()

	histogram, err := meter.SyncFloat64().Histogram("latency_ms")
	require.NoError(b, err)

	count, err := meter.SyncInt64().Counter("calls")
	require.NoError(b, err)

	r := newMethodRecorder(histogram.Record, count.Add,
		semconv.DBSystemOtherSQL,
		dbInstance.String("test"),
	)

	prepare := chainPrepareContextFuncMiddlewares([]prepareContextFuncMiddleware{
		prepareStats(r),
	}, nopPrepareContext)

	for i := 0; i < b.N; i++ {
		_, _ = prepare(context.Background(), "") // nolint: errcheck
	}
}

func TestNopPrepareContext(t *testing.T) {
	t.Parallel()

	result, err := nopPrepareContext(context.Background(), "")

	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestEnsurePrepareContext(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario      string
		conn          driver.Conn
		expectedTx    driver.Tx
		expectedError error
	}{
		{
			scenario:      "prepare",
			conn:          prepareTest{err: errors.New("prepare error")},
			expectedError: errors.New("prepare error"),
		},
		{
			scenario:      "prepare context",
			conn:          prepareTxTest{err: errors.New("prepare context error")},
			expectedError: errors.New("prepare context error"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			prepare := ensurePrepareContext(tc.conn)
			tx, err := prepare(context.Background(), "")

			assert.Equal(t, tc.expectedTx, tx)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestChainPrepareContextFuncMiddlewares_NoMiddleware(t *testing.T) {
	t.Parallel()

	prepare := chainPrepareContextFuncMiddlewares(nil, nopPrepareContext)

	result, err := prepare(context.Background(), "")

	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestChainPrepareContextFuncMiddlewares(t *testing.T) {
	t.Parallel()

	stack := make([]string, 0)

	pushPrepareContextFunc := func(s string) prepareContextFunc {
		return func(_ context.Context, _ string) (driver.Stmt, error) {
			stack = append(stack, s)

			return nil, nil
		}
	}

	pushPrepareContextFuncMiddleware := func(s string) prepareContextFuncMiddleware {
		return func(next prepareContextFunc) prepareContextFunc {
			return func(ctx context.Context, query string) (driver.Stmt, error) {
				stack = append(stack, s)

				return next(ctx, query)
			}
		}
	}

	prepare := chainPrepareContextFuncMiddlewares(
		[]prepareContextFuncMiddleware{
			pushPrepareContextFuncMiddleware("outer"),
			pushPrepareContextFuncMiddleware("inner"),
		},
		pushPrepareContextFunc("end"),
	)
	result, err := prepare(context.Background(), "")

	assert.Nil(t, result)
	assert.NoError(t, err)

	expected := []string{"outer", "inner", "end"}

	assert.Equal(t, expected, stack)
}

func TestPrepareStats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		prepare  prepareContextFunc
		expected string
	}{
		{
			scenario: "error",
			prepare: func(_ context.Context, _ string) (driver.Stmt, error) {
				return nil, errors.New("error")
			},
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=prepare_test,db.instance=test,db.operation=go.sql.prepare,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=prepare_test,db.instance=test,db.operation=go.sql.prepare,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": "<ignore-diff>"
				}
			]`,
		},
		{
			scenario: "no error",
			prepare:  nopPrepareContext,
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=prepare_test,db.instance=test,db.operation=go.sql.prepare,db.sql.status=OK,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=prepare_test,db.instance=test,db.operation=go.sql.prepare,db.sql.status=OK,db.system=other_sql}",
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
					meter := s.MeterProvider().Meter("prepare_test")

					histogram, err := meter.SyncFloat64().Histogram(dbSQLClientLatencyMs)
					require.NoError(t, err)

					count, err := meter.SyncInt64().Counter(dbSQLClientCalls)
					require.NoError(t, err)

					r := newMethodRecorder(histogram.Record, count.Add,
						semconv.DBSystemOtherSQL,
						dbInstance.String("test"),
					)

					prepare := chainPrepareContextFuncMiddlewares([]prepareContextFuncMiddleware{
						prepareStats(r),
					}, tc.prepare)

					_, _ = prepare(context.Background(), "") // nolint: errcheck
				})
		})
	}
}

type prepareTxTest struct {
	driver.Conn
	driver.ConnPrepareContext

	stmt driver.Stmt
	err  error
}

func (t prepareTxTest) PrepareContext(context.Context, string) (driver.Stmt, error) {
	return t.stmt, t.err
}

type prepareTest struct {
	driver.Conn

	stmt driver.Stmt
	err  error
}

func (t prepareTest) Prepare(string) (driver.Stmt, error) {
	return t.stmt, t.err
}
