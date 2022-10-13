package otelsql

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/metric/unit"
)

// defaultMinimumReadDBStatsInterval is the default minimum interval between calls to db.Stats().
const defaultMinimumReadDBStatsInterval = time.Second

const (
	dbSQLConnectionsOpen           = "db.sql.connections.open"
	dbSQLConnectionsIdle           = "db.sql.connections.idle"
	dbSQLConnectionsActive         = "db.sql.connections.active"
	dbSQLConnectionsWaitCount      = "db.sql.connections.wait_count"
	dbSQLConnectionsWaitDuration   = "db.sql.connections.wait_duration"
	dbSQLConnectionsIdleClosed     = "db.sql.connections.idle_closed"
	dbSQLConnectionsLifetimeClosed = "db.sql.connections.lifetime_closed"
)

// RecordStats records database statistics for provided sql.DB at the provided interval.
func RecordStats(db *sql.DB, opts ...StatsOption) error {
	o := statsOptions{
		meterProvider:              global.MeterProvider(),
		minimumReadDBStatsInterval: defaultMinimumReadDBStatsInterval,
	}

	for _, opt := range opts {
		opt.applyStatsOptions(&o)
	}

	meter := o.meterProvider.Meter(instrumentationName)

	return recordStats(meter, db, o.minimumReadDBStatsInterval, o.defaultAttributes...)
}

// nolint: funlen
func recordStats(
	meter metric.Meter,
	db *sql.DB,
	minimumReadDBStatsInterval time.Duration,
	attrs ...attribute.KeyValue,
) error {
	var (
		err error

		openConnections   asyncint64.Gauge
		idleConnections   asyncint64.Gauge
		activeConnections asyncint64.Gauge
		waitCount         asyncint64.Gauge
		waitDuration      asyncfloat64.Gauge
		idleClosed        asyncint64.Gauge
		lifetimeClosed    asyncint64.Gauge

		dbStats     sql.DBStats
		lastDBStats time.Time

		// lock prevents a race between batch observer and instrument registration.
		lock sync.Mutex
	)

	lock.Lock()
	defer lock.Unlock()

	openConnections, err = meter.AsyncInt64().Gauge(
		dbSQLConnectionsOpen,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Count of open connections in the pool"),
	)
	otel.Handle(err)

	idleConnections, err = meter.AsyncInt64().Gauge(
		dbSQLConnectionsIdle,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Count of idle connections in the pool"),
	)
	otel.Handle(err)

	activeConnections, err = meter.AsyncInt64().Gauge(
		dbSQLConnectionsActive,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Count of active connections in the pool"),
	)
	otel.Handle(err)

	waitCount, err = meter.AsyncInt64().Gauge(
		dbSQLConnectionsWaitCount,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("The total number of connections waited for"),
	)
	otel.Handle(err)

	waitDuration, err = meter.AsyncFloat64().Gauge(
		dbSQLConnectionsWaitDuration,
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("The total time blocked waiting for a new connection"),
	)
	otel.Handle(err)

	idleClosed, err = meter.AsyncInt64().Gauge(
		dbSQLConnectionsIdleClosed,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("The total number of connections closed due to SetMaxIdleConns"),
	)
	otel.Handle(err)

	lifetimeClosed, err = meter.AsyncInt64().Gauge(
		dbSQLConnectionsLifetimeClosed,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("The total number of connections closed due to SetConnMaxLifetime"),
	)
	otel.Handle(err)

	return meter.RegisterCallback([]instrument.Asynchronous{
		openConnections,
		idleConnections,
		activeConnections,
		waitCount,
		waitDuration,
		idleClosed,
		lifetimeClosed,
	}, func(ctx context.Context) {
		lock.Lock()
		defer lock.Unlock()

		now := time.Now()
		if now.Sub(lastDBStats) >= minimumReadDBStatsInterval {
			dbStats = db.Stats()
			lastDBStats = now
		}

		openConnections.Observe(ctx, int64(dbStats.OpenConnections), attrs...)
		idleConnections.Observe(ctx, int64(dbStats.Idle), attrs...)
		activeConnections.Observe(ctx, int64(dbStats.InUse), attrs...)
		waitCount.Observe(ctx, dbStats.WaitCount, attrs...)
		waitDuration.Observe(ctx, float64(dbStats.WaitDuration.Nanoseconds())/1e6, attrs...)
		idleClosed.Observe(ctx, dbStats.MaxIdleClosed, attrs...)
		lifetimeClosed.Observe(ctx, dbStats.MaxLifetimeClosed, attrs...)
	})
}
