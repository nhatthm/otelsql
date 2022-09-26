package otelsql

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	"go.nhat.io/otelsql/internal/test/oteltest"
)

func TestChainTxFuncMiddlewares_NoMiddleware(t *testing.T) {
	t.Parallel()

	f := chainTxFuncMiddlewares(nil, nopTxFunc)

	err := f()

	assert.NoError(t, err)
}

func TestChainTxFuncMiddlewares(t *testing.T) {
	t.Parallel()

	stack := make([]string, 0)

	pushTxFunc := func(s string) txFunc {
		return func() error {
			stack = append(stack, s)

			return nil
		}
	}

	pushTxFuncMiddleware := func(s string) txFuncMiddleware {
		return func(next txFunc) txFunc {
			return func() error {
				stack = append(stack, s)

				return next()
			}
		}
	}

	f := chainTxFuncMiddlewares(
		[]txFuncMiddleware{
			pushTxFuncMiddleware("outer"),
			pushTxFuncMiddleware("inner"),
		},
		pushTxFunc("end"),
	)
	err := f()

	assert.NoError(t, err)

	expected := []string{"outer", "inner", "end"}

	assert.Equal(t, expected, stack)
}

func TestTxStats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		beginner txFunc
		expected string
	}{
		{
			scenario: "error",
			beginner: func() error {
				return errors.New("error")
			},
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=tx_test,db.instance=test,db.operation=go.sql.commit,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=tx_test,db.instance=test,db.operation=go.sql.commit,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": "<ignore-diff>",
					"Count": 1
				}
			]`,
		},
		{
			scenario: "no error",
			beginner: nopTxFunc,
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=tx_test,db.instance=test,db.operation=go.sql.commit,db.sql.status=OK,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=tx_test,db.instance=test,db.operation=go.sql.commit,db.sql.status=OK,db.system=other_sql}",
					"Sum": "<ignore-diff>",
					"Count": 1,
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
					meter := s.MeterProvider().Meter("tx_test")

					histogram, err := meter.SyncFloat64().Histogram(dbSQLClientLatencyMs)
					require.NoError(t, err)

					count, err := meter.SyncInt64().Counter(dbSQLClientCalls)
					require.NoError(t, err)

					r := newMethodRecorder(histogram.Record, count.Add,
						semconv.DBSystemOtherSQL,
						dbInstance.String("test"),
					)

					f := chainTxFuncMiddlewares([]txFuncMiddleware{
						txStats(context.Background(), r, metricMethodCommit),
					}, tc.beginner)

					_ = f() // nolint: errcheck
				})
		})
	}
}
