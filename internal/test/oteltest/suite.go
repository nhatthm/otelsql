package oteltest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"

	testassert "go.nhat.io/otelsql/internal/test/assert"
	"go.nhat.io/otelsql/internal/test/sqlmock"
)

// Suite is a test suite.
type Suite interface {
	Run(t *testing.T, f func(sc SuiteContext))
}

// SuiteContext represents a test suite context.
type SuiteContext interface {
	TracerProvider() trace.TracerProvider
	MeterProvider() metric.MeterProvider
	DatabaseDSN() string
}

type suiteContext struct {
	test testing.TB

	tracerProvider *tracesdk.TracerProvider
	meterProvider  *metricsdk.MeterProvider

	sqlMocker sqlmock.Sqlmocker
}

// TracerProvider provides access to instrumentation Tracers.
func (s *suiteContext) TracerProvider() trace.TracerProvider {
	return s.tracerProvider
}

// MeterProvider supports named Meter instances.
func (s *suiteContext) MeterProvider() metric.MeterProvider {
	return s.meterProvider
}

// DatabaseDSN returns a database dsn to the sqlmock instance.
func (s *suiteContext) DatabaseDSN() string {
	return s.sqlMocker(s.test)
}

// SuiteOption setups the test suite.
type SuiteOption func(c *suiteConfig)

type suiteConfig struct {
	tracerProvider *tracesdk.TracerProvider
	meterProvider  *metricsdk.MeterProvider

	assertTracesFuncs  []testassert.Func
	assertMetricsFuncs []testassert.Func

	sqlMocks []func(m sqlmock.Sqlmock)
}

type suite struct {
	tracerProvider *tracesdk.TracerProvider
	meterProvider  *metricsdk.MeterProvider

	assertTraces  testassert.Func
	assertMetrics testassert.Func

	outMetrics fmt.Stringer
	outTraces  fmt.Stringer

	sqlMocker sqlmock.Sqlmocker
}

func (s *suite) start(tb testing.TB) *suiteContext {
	tb.Helper()

	// handleErr(s.meterProvider.Start(context.Background()))

	return &suiteContext{
		test:           tb,
		tracerProvider: s.tracerProvider,
		meterProvider:  s.meterProvider,
		sqlMocker:      s.sqlMocker,
	}
}

func (s *suite) end() {
	ctx := context.Background()

	_ = s.meterProvider.ForceFlush(ctx) // nolint: errcheck
	_ = s.meterProvider.Shutdown(ctx)   // nolint: errcheck
}

// Run creates a new test suite and run it.
func (s *suite) Run(t *testing.T, f func(sc SuiteContext)) {
	t.Helper()

	f(s.start(t))

	t.Cleanup(func() {
		s.end()

		metrics := fixMetricsOutput(s.outMetrics.String())
		traces := fixTracesOutput(s.outTraces.String())

		s.assertMetrics(t, metrics, "failed to assert metrics, actual:\n%s", metrics)
		s.assertTraces(t, traces, "failed to assert traces, actual:\n%s", traces)
	})
}

// New creates a new test suite.
func New(opts ...SuiteOption) Suite {
	cfg := suiteConfig{}

	for _, o := range opts {
		o(&cfg)
	}

	var (
		outMetrics fmt.Stringer = new(bytes.Buffer)
		outTraces  fmt.Stringer = new(bytes.Buffer)
	)

	if cfg.meterProvider == nil {
		r, out := mustNewMetricReader()

		cfg.meterProvider = newMeterProvider(metricsdk.WithReader(r))
		outMetrics = out
	}

	if cfg.tracerProvider == nil {
		traceExporter, out := mustNewTraceExporter()

		tp := tracesdk.NewTracerProvider(
			tracesdk.WithSampler(tracesdk.AlwaysSample()),
			tracesdk.WithSyncer(traceExporter),
			tracesdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL, semconv.ServiceNameKey.String("oteltest"),
			)),
		)

		cfg.tracerProvider = tp
		outTraces = out
	}

	s := &suite{
		meterProvider:  cfg.meterProvider,
		tracerProvider: cfg.tracerProvider,

		assertMetrics: chainAsserters(cfg.assertMetricsFuncs...),
		assertTraces:  chainAsserters(cfg.assertTracesFuncs...),

		outMetrics: outMetrics,
		outTraces:  outTraces,

		sqlMocker: sqlmock.Register(cfg.sqlMocks...),
	}

	return s
}

// WithMetricsAsserters sets metrics asserter.
func WithMetricsAsserters(fs ...testassert.Func) SuiteOption {
	return func(c *suiteConfig) {
		c.assertMetricsFuncs = append(c.assertMetricsFuncs, fs...)
	}
}

// MetricsEqualJSON sets metrics asserter.
func MetricsEqualJSON(expect string) SuiteOption {
	return WithMetricsAsserters(testassert.EqualJSON(expect))
}

// MetricsEmpty sets metrics asserter.
func MetricsEmpty() SuiteOption {
	return WithMetricsAsserters(testassert.Empty())
}

// WithTracesAsserters sets traces asserter.
func WithTracesAsserters(fs ...testassert.Func) SuiteOption {
	return func(c *suiteConfig) {
		c.assertTracesFuncs = append(c.assertTracesFuncs, fs...)
	}
}

// TracesEqualJSON sets traces asserter.
func TracesEqualJSON(expect string) SuiteOption {
	return WithTracesAsserters(testassert.EqualJSON(expect))
}

// TracesEmpty sets traces asserter.
func TracesEmpty() SuiteOption {
	return WithTracesAsserters(testassert.Empty())
}

// TracesMatch asserts traces by a callback.
func TracesMatch(f func(t assert.TestingT, actual []Span) bool) SuiteOption {
	return WithTracesAsserters(matchTraces(f))
}

// MockDatabase sets sql mockers.
func MockDatabase(mocks ...func(m sqlmock.Sqlmock)) SuiteOption {
	return func(c *suiteConfig) {
		c.sqlMocks = append(c.sqlMocks, mocks...)
	}
}

func newMetricReader() (metricsdk.Reader, fmt.Stringer, error) {
	out := new(bytes.Buffer)

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")

	e, err := stdoutmetric.New(stdoutmetric.WithEncoder(&metricEncoder{Encoder: enc}))
	if err != nil {
		return nil, nil, err
	}

	r := &metricReader{
		exporter: e,
		Reader:   metricsdk.NewManualReader(),
	}

	return r, out, nil
}

func mustNewMetricReader() (metricsdk.Reader, fmt.Stringer) {
	r, out, err := newMetricReader()
	handleErr(err)

	return r, out
}

func newTraceExporter() (*stdouttrace.Exporter, fmt.Stringer, error) {
	out := new(bytes.Buffer)

	e, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
		stdouttrace.WithoutTimestamps(),
		stdouttrace.WithWriter(out),
	)
	if err != nil {
		return nil, nil, err
	}

	return e, out, nil
}

func mustNewTraceExporter() (*stdouttrace.Exporter, fmt.Stringer) {
	e, out, err := newTraceExporter()
	handleErr(err)

	return e, out
}

func newMeterProvider(opts ...metricsdk.Option) *metricsdk.MeterProvider {
	opts = append(opts,
		metricsdk.WithResource(resource.NewSchemaless(
			semconv.ServiceNameKey.String("otelsql"),
		)),
	)

	return metricsdk.NewMeterProvider(opts...)
}

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func fixMetricsOutput(out string) string {
	if out == "" {
		return out
	}

	metrics := make([]json.RawMessage, 0)

	err := json.Unmarshal([]byte(out), &metrics)
	handleErr(err)

	sort.Slice(metrics, func(i, j int) bool {
		return string(metrics[i]) < string(metrics[j])
	})

	data, err := json.MarshalIndent(metrics, "", "    ")
	handleErr(err)

	return string(data)
}

func fixTracesOutput(out string) string {
	if out == "" {
		return out
	}

	traces := make([]json.RawMessage, 0)
	out = fmt.Sprintf("[%s]", strings.ReplaceAll(out, "}\n{", "},\n{"))

	err := json.Unmarshal([]byte(out), &traces)
	handleErr(err)

	data, err := json.MarshalIndent(traces, "", "    ")
	handleErr(err)

	return string(data)
}

func chainAsserters(fs ...testassert.Func) testassert.Func {
	return func(t assert.TestingT, actual string, msgAndArgs ...interface{}) bool {
		for _, f := range fs {
			if !f(t, actual, msgAndArgs...) {
				return false
			}
		}

		return true
	}
}

func matchTraces(f func(t assert.TestingT, actual []Span) bool) testassert.Func {
	return func(t assert.TestingT, actual string, msgAndArgs ...interface{}) bool {
		var spans []Span

		if actual == "" {
			return f(t, spans)
		}

		spans = make([]Span, 0)

		err := json.Unmarshal([]byte(actual), &spans)
		handleErr(err)

		return f(t, spans)
	}
}
