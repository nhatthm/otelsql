package otelsql

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
	dbSQLConnectionsIdleTimeClosed = "db.sql.connections.idle_time_closed"
	dbSQLConnectionsLifetimeClosed = "db.sql.connections.lifetime_closed"
)

// RecordStats records database statistics for provided sql.DB at the provided interval.
func RecordStats(db *sql.DB, opts ...StatsOption) error {
	o := statsOptions{
		meterProvider:              otel.GetMeterProvider(),
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

		openConnections   metric.Int64ObservableGauge
		idleConnections   metric.Int64ObservableGauge
		activeConnections metric.Int64ObservableGauge
		waitCount         metric.Int64ObservableCounter
		waitDuration      metric.Float64ObservableCounter
		idleClosed        metric.Int64ObservableCounter
		idleTimeClosed    metric.Int64ObservableCounter
		lifetimeClosed    metric.Int64ObservableCounter

		dbStats     sql.DBStats
		lastDBStats time.Time

		// lock prevents a race between batch observer and instrument registration.
		lock sync.Mutex
	)

	lock.Lock()
	defer lock.Unlock()

	openConnections, err = meter.Int64ObservableGauge(
		dbSQLConnectionsOpen,
		metric.WithUnit(unitDimensionless),
		metric.WithDescription("Count of open connections in the pool"),
	)
	handleErr(err)

	idleConnections, err = meter.Int64ObservableGauge(
		dbSQLConnectionsIdle,
		metric.WithUnit(unitDimensionless),
		metric.WithDescription("Count of idle connections in the pool"),
	)
	handleErr(err)

	activeConnections, err = meter.Int64ObservableGauge(
		dbSQLConnectionsActive,
		metric.WithUnit(unitDimensionless),
		metric.WithDescription("Count of active connections in the pool"),
	)
	handleErr(err)

	waitCount, err = meter.Int64ObservableCounter(
		dbSQLConnectionsWaitCount,
		metric.WithUnit(unitDimensionless),
		metric.WithDescription("The total number of connections waited for"),
	)
	handleErr(err)

	waitDuration, err = meter.Float64ObservableCounter(
		dbSQLConnectionsWaitDuration,
		metric.WithUnit(unitMilliseconds),
		metric.WithDescription("The total time blocked waiting for a new connection"),
	)
	handleErr(err)

	idleClosed, err = meter.Int64ObservableCounter(
		dbSQLConnectionsIdleClosed,
		metric.WithUnit(unitDimensionless),
		metric.WithDescription("The total number of connections closed due to SetMaxIdleConns"),
	)
	handleErr(err)

	idleTimeClosed, err = meter.Int64ObservableCounter(
		dbSQLConnectionsIdleTimeClosed,
		metric.WithUnit(unitDimensionless),
		metric.WithDescription("The total number of connections closed due to SetConnMaxIdleTime"),
	)
	handleErr(err)

	lifetimeClosed, err = meter.Int64ObservableCounter(
		dbSQLConnectionsLifetimeClosed,
		metric.WithUnit(unitDimensionless),
		metric.WithDescription("The total number of connections closed due to SetConnMaxLifetime"),
	)
	handleErr(err)

	_, err = meter.RegisterCallback(func(_ context.Context, obs metric.Observer) error {
		lock.Lock()
		defer lock.Unlock()

		now := time.Now()
		if now.Sub(lastDBStats) >= minimumReadDBStatsInterval {
			dbStats = db.Stats()
			lastDBStats = now
		}

		obs.ObserveInt64(openConnections, int64(dbStats.OpenConnections), metric.WithAttributes(attrs...))
		obs.ObserveInt64(idleConnections, int64(dbStats.Idle), metric.WithAttributes(attrs...))
		obs.ObserveInt64(activeConnections, int64(dbStats.InUse), metric.WithAttributes(attrs...))
		obs.ObserveInt64(waitCount, dbStats.WaitCount, metric.WithAttributes(attrs...))
		obs.ObserveFloat64(waitDuration, float64(dbStats.WaitDuration.Nanoseconds())/1e6, metric.WithAttributes(attrs...))
		obs.ObserveInt64(idleClosed, dbStats.MaxIdleClosed, metric.WithAttributes(attrs...))
		obs.ObserveInt64(idleTimeClosed, dbStats.MaxIdleTimeClosed, metric.WithAttributes(attrs...))
		obs.ObserveInt64(lifetimeClosed, dbStats.MaxLifetimeClosed, metric.WithAttributes(attrs...))

		return nil
	},
		openConnections,
		idleConnections,
		activeConnections,
		waitCount,
		waitDuration,
		idleClosed,
		idleTimeClosed,
		lifetimeClosed,
	)

	return err
}
