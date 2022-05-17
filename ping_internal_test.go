package otelsql

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/nonrecording"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/nhatthm/otelsql/internal/test/oteltest"
)

func BenchmarkPingStats(b *testing.B) {
	meter := nonrecording.NewNoopMeter()

	histogram, err := meter.SyncFloat64().Histogram("latency_ms")
	require.NoError(b, err)

	count, err := meter.SyncInt64().Counter("calls")
	require.NoError(b, err)

	r := newMethodRecorder(histogram.Record, count.Add,
		semconv.DBSystemOtherSQL,
		dbInstance.String("test"),
	)

	ping := chainPingFuncMiddlewares([]pingFuncMiddleware{
		pingStats(r),
	}, nopPing)

	for i := 0; i < b.N; i++ {
		_ = ping(context.Background()) // nolint: errcheck
	}
}

func TestNopPing(t *testing.T) {
	t.Parallel()

	err := nopPing(context.Background())

	assert.NoError(t, err)
}

func TestChainPingFuncMiddlewares_NoMiddleware(t *testing.T) {
	t.Parallel()

	f := chainPingFuncMiddlewares(nil, nopPing)

	err := f(context.Background())

	assert.NoError(t, err)
}

func TestChainPingFuncMiddlewares(t *testing.T) {
	t.Parallel()

	stack := make([]string, 0)

	pushPingFunc := func(s string) pingFunc {
		return func(ctx context.Context) error {
			stack = append(stack, s)

			return nil
		}
	}

	pushPingFuncMiddleware := func(s string) pingFuncMiddleware {
		return func(next pingFunc) pingFunc {
			return func(ctx context.Context) error {
				stack = append(stack, s)

				return next(ctx)
			}
		}
	}

	ping := chainPingFuncMiddlewares(
		[]pingFuncMiddleware{
			pushPingFuncMiddleware("outer"),
			pushPingFuncMiddleware("inner"),
		},
		pushPingFunc("end"),
	)
	err := ping(context.Background())

	assert.NoError(t, err)

	expected := []string{"outer", "inner", "end"}

	assert.Equal(t, expected, stack)
}

func TestPingStats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		ping     pingFunc
		expected string
	}{
		{
			scenario: "error",
			ping: func(_ context.Context) error {
				return errors.New("error")
			},
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=ping_test,db.instance=test,db.operation=go.sql.ping,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=ping_test,db.instance=test,db.operation=go.sql.ping,db.sql.error=error,db.sql.status=ERROR,db.system=other_sql}",
					"Sum": "<ignore-diff>"
				}
			]`,
		},
		{
			scenario: "no error",
			ping:     nopPing,
			expected: `[
				{
					"Name": "db.sql.client.calls{service.name=otelsql,instrumentation.name=ping_test,db.instance=test,db.operation=go.sql.ping,db.sql.status=OK,db.system=other_sql}",
					"Sum": 1
				},
				{
					"Name": "db.sql.client.latency{service.name=otelsql,instrumentation.name=ping_test,db.instance=test,db.operation=go.sql.ping,db.sql.status=OK,db.system=other_sql}",
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
					meter := s.MeterProvider().Meter("ping_test")

					histogram, err := meter.SyncFloat64().Histogram(dbSQLClientLatencyMs)
					require.NoError(t, err)

					count, err := meter.SyncInt64().Counter(dbSQLClientCalls)
					require.NoError(t, err)

					r := newMethodRecorder(histogram.Record, count.Add,
						semconv.DBSystemOtherSQL,
						dbInstance.String("test"),
					)

					ping := chainPingFuncMiddlewares([]pingFuncMiddleware{
						pingStats(r),
					}, tc.ping)

					_ = ping(context.Background()) // nolint: errcheck
				})
		})
	}
}
