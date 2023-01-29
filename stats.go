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

		openConnections   instrument.Int64ObservableGauge
		idleConnections   instrument.Int64ObservableGauge
		activeConnections instrument.Int64ObservableGauge
		waitCount         instrument.Int64ObservableGauge
		waitDuration      instrument.Float64ObservableGauge
		idleClosed        instrument.Int64ObservableGauge
		lifetimeClosed    instrument.Int64ObservableGauge

		dbStats     sql.DBStats
		lastDBStats time.Time

		// lock prevents a race between batch observer and instrument registration.
		lock sync.Mutex
	)

	lock.Lock()
	defer lock.Unlock()

	openConnections, err = meter.Int64ObservableGauge(
		dbSQLConnectionsOpen,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Count of open connections in the pool"),
	)
	otel.Handle(err)

	idleConnections, err = meter.Int64ObservableGauge(
		dbSQLConnectionsIdle,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Count of idle connections in the pool"),
	)
	otel.Handle(err)

	activeConnections, err = meter.Int64ObservableGauge(
		dbSQLConnectionsActive,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Count of active connections in the pool"),
	)
	otel.Handle(err)

	waitCount, err = meter.Int64ObservableGauge(
		dbSQLConnectionsWaitCount,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("The total number of connections waited for"),
	)
	otel.Handle(err)

	waitDuration, err = meter.Float64ObservableGauge(
		dbSQLConnectionsWaitDuration,
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("The total time blocked waiting for a new connection"),
	)
	otel.Handle(err)

	idleClosed, err = meter.Int64ObservableGauge(
		dbSQLConnectionsIdleClosed,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("The total number of connections closed due to SetMaxIdleConns"),
	)
	otel.Handle(err)

	lifetimeClosed, err = meter.Int64ObservableGauge(
		dbSQLConnectionsLifetimeClosed,
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("The total number of connections closed due to SetConnMaxLifetime"),
	)
	otel.Handle(err)

	_, err = meter.RegisterCallback(func(ctx context.Context, obs metric.Observer) error {
		lock.Lock()
		defer lock.Unlock()

		now := time.Now()
		if now.Sub(lastDBStats) >= minimumReadDBStatsInterval {
			dbStats = db.Stats()
			lastDBStats = now
		}

		obs.ObserveInt64(openConnections, int64(dbStats.OpenConnections), attrs...)
		obs.ObserveInt64(idleConnections, int64(dbStats.Idle), attrs...)
		obs.ObserveInt64(activeConnections, int64(dbStats.InUse), attrs...)
		obs.ObserveInt64(waitCount, dbStats.WaitCount, attrs...)
		obs.ObserveFloat64(waitDuration, float64(dbStats.WaitDuration.Nanoseconds())/1e6, attrs...)
		obs.ObserveInt64(idleClosed, dbStats.MaxIdleClosed, attrs...)
		obs.ObserveInt64(lifetimeClosed, dbStats.MaxLifetimeClosed, attrs...)

		return nil
	},
		openConnections,
		idleConnections,
		activeConnections,
		waitCount,
		waitDuration,
		idleClosed,
		lifetimeClosed,
	)

	return err
}
