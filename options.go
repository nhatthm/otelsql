package otelsql

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

// Option allows for managing otelsql configuration using functional options.
type Option interface {
	DriverOption
	StatsOption
}

// DriverOption allows for managing otelsql configuration using functional options.
type DriverOption interface {
	applyDriverOptions(o *driverOptions)
}

// StatsOption allows for managing otelsql configuration using functional options.
type StatsOption interface {
	applyStatsOptions(o *statsOptions)
}

// driverOptions holds configuration of our otelsql tracing middleware.
//
// By default, all options are set to false intentionally when creating a wrapped driver and provide the most sensible default with both performance and
// security in mind.
type driverOptions struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider

	trace TraceOptions

	// defaultAttributes will be set to each span and metrics as default.
	defaultAttributes []attribute.KeyValue
}

// TraceOptions are options to enable the creations of spans on sql calls.
type TraceOptions struct {
	spanNameFormatter spanNameFormatter
	errorToSpanStatus errorToSpanStatus
	queryTracer       queryTracer

	// AllowRoot, if set to true, will allow otelsql to create root spans in absence of existing spans or even context.
	//
	// Default is to not trace otelsql calls if no existing parent span is found in context or when using methods not taking context.
	AllowRoot bool

	// Ping, if set to true, will enable the creation of spans on Ping requests.
	Ping bool

	// RowsNext, if set to true, will enable the creation of spans on RowsNext calls. This can result in many spans.
	RowsNext bool

	// RowsClose, if set to true, will enable the creation of spans on RowsClose calls.
	RowsClose bool

	// RowsAffected, if set to true, will enable the creation of spans on RowsAffected calls.
	RowsAffected bool

	// LastInsertID, if set to true, will enable the creation of spans on LastInsertId calls.
	LastInsertID bool
}

// WithMeterProvider sets meter provider.
func WithMeterProvider(p metric.MeterProvider) Option {
	return struct {
		driverOptionFunc
		statsOptionFunc
	}{
		driverOptionFunc: func(o *driverOptions) {
			o.meterProvider = p
		},
		statsOptionFunc: func(o *statsOptions) {
			o.meterProvider = p
		},
	}
}

// WithTracerProvider sets tracer provider.
func WithTracerProvider(p trace.TracerProvider) DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.tracerProvider = p
	})
}

// WithInstanceName sets database instance name.
func WithInstanceName(instanceName string) Option {
	return WithDefaultAttributes(dbInstance.String(instanceName))
}

// WithSystem sets database system name.
// See: semconv.DBSystemKey.
func WithSystem(system attribute.KeyValue) Option {
	return WithDefaultAttributes(system)
}

// WithDatabaseName sets database name.
func WithDatabaseName(system string) Option {
	return WithDefaultAttributes(semconv.DBNameKey.String(system))
}

// WithDefaultAttributes will be set to each span as default.
func WithDefaultAttributes(attrs ...attribute.KeyValue) Option {
	return struct {
		driverOptionFunc
		statsOptionFunc
	}{
		driverOptionFunc: func(o *driverOptions) {
			o.defaultAttributes = append(o.defaultAttributes, attrs...)
		},
		statsOptionFunc: func(o *statsOptions) {
			o.defaultAttributes = append(o.defaultAttributes, attrs...)
		},
	}
}

// WithSpanNameFormatter sets tracer provider.
func WithSpanNameFormatter(f spanNameFormatter) DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.spanNameFormatter = f
	})
}

// ConvertErrorToSpanStatus sets a custom error converter.
func ConvertErrorToSpanStatus(f errorToSpanStatus) DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.errorToSpanStatus = f
	})
}

// DisableErrSkip suppresses driver.ErrSkip errors in spans if set to true.
func DisableErrSkip() DriverOption {
	return ConvertErrorToSpanStatus(spanStatusFromErrorIgnoreErrSkip)
}

// TraceQuery sets a custom function that will return a list of attributes to add to the spans with a given query and args.
//
// For example:
// 	otelsql.TraceQuery(func(sql string, args []driver.NamedValue) []attribute.KeyValue {
// 		attrs := make([]attribute.KeyValue, 0)
// 		attrs = append(attrs, semconv.DBStatementKey.String(sql))
//
// 		for _, arg := range args {
// 			if arg.Name != "password" {
// 				attrs = append(attrs, sqlattribute.FromNamedValue(arg))
// 			}
// 		}
//
// 		return attrs
// 	})
func TraceQuery(f queryTracer) DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.queryTracer = f
	})
}

// TraceQueryWithArgs will add to the spans the given sql query and all arguments.
func TraceQueryWithArgs() DriverOption {
	return TraceQuery(traceQueryWithArgs)
}

// TraceQueryWithoutArgs will add to the spans the given sql query without any arguments.
func TraceQueryWithoutArgs() DriverOption {
	return TraceQuery(traceQueryWithoutArgs)
}

// TraceAll enables the creation of spans on methods.
func TraceAll() DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.queryTracer = traceQueryWithArgs
		o.trace.AllowRoot = true
		o.trace.Ping = true
		o.trace.RowsNext = true
		o.trace.RowsClose = true
		o.trace.RowsAffected = true
		o.trace.LastInsertID = true
	})
}

// AllowRoot allows otelsql to create root spans in absence of existing spans or even context.
//
// Default is to not trace otelsql calls if no existing parent span is found in context or when using methods not taking context.
func AllowRoot() DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.AllowRoot = true
	})
}

// TracePing enables the creation of spans on Ping requests.
func TracePing() DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.Ping = true
	})
}

// TraceRowsNext enables the creation of spans on RowsNext calls. This can result in many spans.
func TraceRowsNext() DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.RowsNext = true
	})
}

// TraceRowsClose enables the creation of spans on RowsClose calls.
func TraceRowsClose() DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.RowsClose = true
	})
}

// TraceRowsAffected enables the creation of spans on RowsAffected calls.
func TraceRowsAffected() DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.RowsAffected = true
	})
}

// TraceLastInsertID enables the creation of spans on LastInsertId calls.
func TraceLastInsertID() DriverOption {
	return driverOptionFunc(func(o *driverOptions) {
		o.trace.LastInsertID = true
	})
}

// WithMinimumReadDBStatsInterval sets the minimum interval between calls to db.Stats(). Negative values are ignored.
func WithMinimumReadDBStatsInterval(interval time.Duration) StatsOption {
	return statsOptionFunc(func(o *statsOptions) {
		o.minimumReadDBStatsInterval = interval
	})
}

type statsOptions struct {
	// meterProvider sets the metric.MeterProvider. If nil, the global Provider will be used.
	meterProvider metric.MeterProvider

	// minimumReadDBStatsInterval sets the minimum interval between calls to db.Stats(). Negative values are ignored.
	minimumReadDBStatsInterval time.Duration

	// defaultAttributes will be set to each metrics as default.
	defaultAttributes []attribute.KeyValue
}

type driverOptionFunc func(o *driverOptions)

func (f driverOptionFunc) applyDriverOptions(o *driverOptions) {
	f(o)
}

type statsOptionFunc func(o *statsOptions)

func (f statsOptionFunc) applyStatsOptions(o *statsOptions) {
	f(o)
}
