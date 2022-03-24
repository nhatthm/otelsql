package otelsql_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"

	"github.com/nhatthm/otelsql"
	"github.com/nhatthm/otelsql/internal/test/oteltest"
	"github.com/nhatthm/otelsql/internal/test/sqlmock"
)

const instrumentationName = "github.com/nhatthm/otelsql"

func TestRecordStats(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedStatsMetric()

	oteltest.New(
		oteltest.MetricsEqualJSON(expectedMetrics),
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectPing()
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN())
			require.NoError(t, err)

			err = otelsql.RecordStats(db,
				otelsql.WithMeterProvider(sc.MeterProvider()),
				otelsql.WithMinimumReadDBStatsInterval(100*time.Millisecond),
				otelsql.WithInstanceName("default"),
				otelsql.WithSystem(semconv.DBSystemPostgreSQL),
			)
			require.NoError(t, err)

			err = db.Ping()
			require.NoError(t, err)
		})
}

func TestRecordStats_Error(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		metric string
	}{
		{metric: "db.sql.connections.open"},
		{metric: "db.sql.connections.idle"},
		{metric: "db.sql.connections.active"},
		{metric: "db.sql.connections.wait_count"},
		{metric: "db.sql.connections.wait_duration"},
		{metric: "db.sql.connections.idle_closed"},
		{metric: "db.sql.connections.lifetime_closed"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.metric, func(t *testing.T) {
			t.Parallel()

			oteltest.New().
				Run(t, func(sc oteltest.SuiteContext) {
					db, err := newDB(sc.DatabaseDSN())
					require.NoError(t, err)

					_, err = sc.MeterProvider().Meter(instrumentationName).SyncInt64().Counter(tc.metric)
					require.NoError(t, err)

					err = otelsql.RecordStats(db,
						otelsql.WithMeterProvider(sc.MeterProvider()),
					)
					expected := fmt.Sprintf(`metric %s registered as Int64Kind CounterInstrumentKind: a metric was already registered by this name with another kind or number type`, tc.metric)

					assert.EqualError(t, err, expected)
				})
		})
	}
}

func expectedStatsMetric() string {
	return expectedMetricsFromFile("stats.json")
}
