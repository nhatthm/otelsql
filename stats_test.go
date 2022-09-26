package otelsql_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	"go.nhat.io/otelsql"
	"go.nhat.io/otelsql/internal/test/oteltest"
	"go.nhat.io/otelsql/internal/test/sqlmock"
)

const instrumentationName = "go.nhat.io/otelsql"

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
		unit   unit.Unit
	}{
		{metric: "db.sql.connections.open", unit: unit.Dimensionless},
		{metric: "db.sql.connections.idle", unit: unit.Dimensionless},
		{metric: "db.sql.connections.active", unit: unit.Dimensionless},
		{metric: "db.sql.connections.wait_count", unit: unit.Dimensionless},
		{metric: "db.sql.connections.wait_duration", unit: unit.Milliseconds},
		{metric: "db.sql.connections.idle_closed", unit: unit.Dimensionless},
		{metric: "db.sql.connections.lifetime_closed", unit: unit.Dimensionless},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.metric, func(t *testing.T) {
			t.Parallel()

			oteltest.New().
				Run(t, func(sc oteltest.SuiteContext) {
					db, err := newDB(sc.DatabaseDSN())
					require.NoError(t, err)

					_, err = sc.MeterProvider().Meter(instrumentationName).AsyncInt64().UpDownCounter(tc.metric, instrument.WithUnit(tc.unit))
					require.NoError(t, err)

					err = otelsql.RecordStats(db,
						otelsql.WithMeterProvider(sc.MeterProvider()),
					)
					expected := fmt.Sprintf(`instrument already registered: name %s`, tc.metric)

					assert.ErrorContains(t, err, expected)
				})
		})
	}
}

func expectedStatsMetric() string {
	return expectedMetricsFromFile("stats.json")
}
