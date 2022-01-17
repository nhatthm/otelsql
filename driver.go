package otelsql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strconv"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/unit"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

const _maxDriver = 100

const instrumentationName = "github.com/nhatthm/otelsql"

var regMu sync.Mutex

// Register initializes and registers our otelsql wrapped database driver identified by its driverName and using provided
// options. On success, it returns the generated driverName to use when calling sql.Open.
//
// It is possible to register multiple wrappers for the same database driver if needing different options for
// different connections.
func Register(driverName string, options ...DriverOption) (string, error) {
	return RegisterWithSource(driverName, "", options...)
}

// RegisterWithSource initializes and registers our otelsql wrapped database driver
// identified by its driverName, using provided options.
//
// source is useful if some drivers do not accept the empty string when opening the DB. On success, it returns the
// generated driverName to use when calling sql.Open.
//
// It is possible to register multiple wrappers for the same database driver if needing different options for
// different connections.
func RegisterWithSource(driverName string, source string, options ...DriverOption) (string, error) {
	// retrieve the driver implementation we need to wrap with instrumentation
	db, err := sql.Open(driverName, source)
	if err != nil {
		return "", err
	}

	dri := db.Driver()

	if err = db.Close(); err != nil {
		return "", err
	}

	regMu.Lock()
	defer regMu.Unlock()

	// Since we might want to register multiple otelsql drivers to have different options, but potentially the same
	// underlying database driver, we cycle through to find available driver names.
	driverName += "-otelsql-"

	for i := int64(0); i < _maxDriver; i++ {
		found := false
		regName := driverName + strconv.FormatInt(i, 10)

		for _, name := range sql.Drivers() {
			if name == regName {
				found = true
			}
		}

		if !found {
			sql.Register(regName, Wrap(dri, options...))

			return regName, nil
		}
	}

	return "", errors.New("unable to register driver, all slots have been taken")
}

// Wrap takes a SQL driver and wraps it with OpenCensus instrumentation.
func Wrap(d driver.Driver, opts ...DriverOption) driver.Driver {
	o := driverOptions{
		meterProvider:  global.GetMeterProvider(),
		tracerProvider: otel.GetTracerProvider(),
	}

	o.trace.spanNameFormatter = formatSpanName
	o.trace.errorToSpanStatus = spanStatusFromError
	o.trace.queryTracer = traceNoQuery

	for _, option := range opts {
		option.applyDriverOptions(&o)
	}

	return wrapDriver(d, o)
}

func wrapDriver(d driver.Driver, o driverOptions) driver.Driver {
	drv := otDriver{
		parent:     d,
		connConfig: newConnConfig(o),
	}

	if _, ok := d.(driver.DriverContext); ok {
		return struct {
			driver.Driver
			driver.DriverContext
		}{drv, drv}
	}

	return struct{ driver.Driver }{drv}
}

func newConnConfig(opts driverOptions) connConfig {
	meter := opts.meterProvider.Meter(instrumentationName)
	tracer := newMethodTracer(
		opts.tracerProvider.Tracer(instrumentationName,
			trace.WithInstrumentationVersion(Version()),
			trace.WithSchemaURL(semconv.SchemaURL),
		),
		traceWithAllowRoot(opts.trace.AllowRoot),
		traceWithDefaultAttributes(opts.defaultAttributes...),
		traceWithSpanNameFormatter(opts.trace.spanNameFormatter),
	)

	latencyMsHistogram, err := meter.NewFloat64Histogram(dbSQLClientLatencyMs,
		metric.WithUnit(unit.Milliseconds),
		metric.WithDescription(`The distribution of latencies of various calls in milliseconds`),
	)
	handleErr(err)

	callsCounter, err := meter.NewInt64Counter(dbSQLClientCalls,
		metric.WithUnit(unit.Dimensionless),
		metric.WithDescription(`The number of various calls of methods`),
	)
	handleErr(err)

	latencyRecorder := newMethodRecorder(latencyMsHistogram.Record, callsCounter.Add, opts.defaultAttributes...)

	return connConfig{
		pingFuncMiddlewares:         makePingFuncMiddlewares(latencyRecorder, tracerOrNil(tracer, opts.trace.Ping)),
		execContextFuncMiddlewares:  makeExecContextFuncMiddlewares(latencyRecorder, tracer, newExecConfig(opts, metricMethodExec, traceMethodExec)),
		queryContextFuncMiddlewares: makeQueryerContextMiddlewares(latencyRecorder, tracer, newQueryConfig(opts, metricMethodQuery, traceMethodQuery)),
		beginFuncMiddlewares:        makeBeginFuncMiddlewares(latencyRecorder, tracer),
		prepareFuncMiddlewares: makePrepareContextFuncMiddlewares(latencyRecorder, tracer, prepareConfig{
			traceQuery:                  opts.trace.queryTracer,
			execFuncMiddlewares:         makeExecContextFuncMiddlewares(latencyRecorder, tracerOrNil(tracer, opts.trace.AllowRoot), newExecConfig(opts, metricMethodStmtExec, traceMethodStmtExec)),
			execContextFuncMiddlewares:  makeExecContextFuncMiddlewares(latencyRecorder, tracer, newExecConfig(opts, metricMethodStmtExec, traceMethodStmtExec)),
			queryFuncMiddlewares:        makeQueryerContextMiddlewares(latencyRecorder, tracerOrNil(tracer, opts.trace.AllowRoot), newQueryConfig(opts, metricMethodStmtQuery, traceMethodStmtQuery)),
			queryContextFuncMiddlewares: makeQueryerContextMiddlewares(latencyRecorder, tracer, newQueryConfig(opts, metricMethodStmtQuery, traceMethodStmtQuery)),
		}),
	}
}

var _ driver.Driver = (*otDriver)(nil)

type otDriver struct {
	parent    driver.Driver
	connector driver.Connector
	closer    io.Closer

	connConfig connConfig
}

func (d otDriver) Open(name string) (driver.Conn, error) {
	c, err := d.parent.Open(name)
	if err != nil {
		return nil, err
	}

	return wrapConn(c, d.connConfig), nil
}

func (d otDriver) Close() error {
	return d.closer.Close()
}

func (d otDriver) OpenConnector(name string) (driver.Connector, error) {
	var err error

	d.connector, err = d.parent.(driver.DriverContext).OpenConnector(name)
	if err != nil {
		return nil, err
	}

	if c, ok := d.connector.(io.Closer); ok {
		d.closer = c
	}

	return d, err
}

func (d otDriver) Connect(ctx context.Context) (driver.Conn, error) {
	c, err := d.connector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return makeConn(c, d.connConfig), nil
}

func (d otDriver) Driver() driver.Driver {
	return d
}

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}
