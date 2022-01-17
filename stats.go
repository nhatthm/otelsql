package otelsql

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/unit"
)

// defaultMinimumReadDBStatsInterval is the default minimum interval between calls to db.Stats().
const defaultMinimumReadDBStatsInterval = time.Second

// RecordStats records database statistics for provided sql.DB at the provided interval.
func RecordStats(db *sql.DB, opts ...StatsOption) error {
	o := statsOptions{
		meterProvider:              global.GetMeterProvider(),
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

		openConnections   metric.Int64GaugeObserver
		idleConnections   metric.Int64GaugeObserver
		activeConnections metric.Int64GaugeObserver
		waitCount         metric.Int64GaugeObserver
		waitDuration      metric.Float64GaugeObserver
		idleClosed        metric.Int64GaugeObserver
		lifetimeClosed    metric.Int64GaugeObserver

		dbStats     sql.DBStats
		lastDBStats time.Time

		// lock prevents a race between batch observer and instrument registration.
		lock sync.Mutex
	)

	lock.Lock()
	defer lock.Unlock()

	batchObserver := meter.NewBatchObserver(func(ctx context.Context, result metric.BatchObserverResult) {
		lock.Lock()
		defer lock.Unlock()

		now := time.Now()
		if now.Sub(lastDBStats) >= minimumReadDBStatsInterval {
			dbStats = db.Stats()
			lastDBStats = now
		}

		result.Observe(
			attrs,
			openConnections.Observation(int64(dbStats.OpenConnections)),
			idleConnections.Observation(int64(dbStats.Idle)),
			activeConnections.Observation(int64(dbStats.InUse)),
			waitCount.Observation(dbStats.WaitCount),
			waitDuration.Observation(float64(dbStats.WaitDuration.Nanoseconds())/1e6),
			idleClosed.Observation(dbStats.MaxIdleClosed),
			lifetimeClosed.Observation(dbStats.MaxLifetimeClosed),
		)
	})

	if openConnections, err = batchObserver.NewInt64GaugeObserver(
		dbSQLConnectionsOpen,
		metric.WithUnit(unit.Dimensionless),
		metric.WithDescription("Count of open connections in the pool"),
	); err != nil {
		return err
	}

	if idleConnections, err = batchObserver.NewInt64GaugeObserver(
		dbSQLConnectionsIdle,
		metric.WithUnit(unit.Dimensionless),
		metric.WithDescription("Count of idle connections in the pool"),
	); err != nil {
		return err
	}

	if activeConnections, err = batchObserver.NewInt64GaugeObserver(
		dbSQLConnectionsActive,
		metric.WithUnit(unit.Dimensionless),
		metric.WithDescription("Count of active connections in the pool"),
	); err != nil {
		return err
	}

	if waitCount, err = batchObserver.NewInt64GaugeObserver(
		dbSQLConnectionsWaitCount,
		metric.WithUnit(unit.Dimensionless),
		metric.WithDescription("The total number of connections waited for"),
	); err != nil {
		return err
	}

	if waitDuration, err = batchObserver.NewFloat64GaugeObserver(
		dbSQLConnectionsWaitDuration,
		metric.WithUnit(unit.Milliseconds),
		metric.WithDescription("The total time blocked waiting for a new connection"),
	); err != nil {
		return err
	}

	if idleClosed, err = batchObserver.NewInt64GaugeObserver(
		dbSQLConnectionsIdleClosed,
		metric.WithUnit(unit.Dimensionless),
		metric.WithDescription("The total number of connections closed due to SetMaxIdleConns"),
	); err != nil {
		return err
	}

	if lifetimeClosed, err = batchObserver.NewInt64GaugeObserver(
		dbSQLConnectionsLifetimeClosed,
		metric.WithUnit(unit.Dimensionless),
		metric.WithDescription("The total number of connections closed due to SetConnMaxLifetime"),
	); err != nil {
		return err
	}

	return nil
}
