package oteltest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkTrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"

	testassert "github.com/nhatthm/otelsql/internal/test/assert"
	"github.com/nhatthm/otelsql/internal/test/sqlmock"
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

	tracerProvider   *sdkTrace.TracerProvider
	metricController *controller.Controller

	sqlMocker sqlmock.Sqlmocker
}

// TracerProvider provides access to instrumentation Tracers.
func (s *suiteContext) TracerProvider() trace.TracerProvider {
	return s.tracerProvider
}

// MeterProvider supports named Meter instances.
func (s *suiteContext) MeterProvider() metric.MeterProvider {
	return s.metricController
}

// DatabaseDSN returns a database dsn to the sqlmock instance.
func (s *suiteContext) DatabaseDSN() string {
	return s.sqlMocker(s.test)
}

// SuiteOption setups the test suite.
type SuiteOption func(c *suiteConfig)

type suiteConfig struct {
	tracerProvider   *sdkTrace.TracerProvider
	metricController *controller.Controller

	assertTracesFuncs  []testassert.Func
	assertMetricsFuncs []testassert.Func

	sqlMocks []func(m sqlmock.Sqlmock)
}

type suite struct {
	tracerProvider   *sdkTrace.TracerProvider
	metricController *controller.Controller

	assertTraces  testassert.Func
	assertMetrics testassert.Func

	outMetrics fmt.Stringer
	outTraces  fmt.Stringer

	sqlMocker sqlmock.Sqlmocker
}

func (s *suite) start(tb testing.TB) *suiteContext {
	tb.Helper()

	handleErr(s.metricController.Start(context.Background()))

	return &suiteContext{
		test:             tb,
		tracerProvider:   s.tracerProvider,
		metricController: s.metricController,
		sqlMocker:        s.sqlMocker,
	}
}

func (s *suite) end() {
	_ = s.metricController.Stop(context.Background()) // nolint: errcheck
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

	if cfg.metricController == nil {
		metricExporter, out := mustNewMetricExporter()

		cfg.metricController = newController(controller.WithExporter(metricExporter))
		outMetrics = out
	}

	if cfg.tracerProvider == nil {
		traceExporter, out := mustNewTraceExporter()

		tp := sdkTrace.NewTracerProvider(
			sdkTrace.WithSampler(sdkTrace.AlwaysSample()),
			sdkTrace.WithSyncer(traceExporter),
			sdkTrace.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL, semconv.ServiceNameKey.String("oteltest"),
			)),
		)

		cfg.tracerProvider = tp
		outTraces = out
	}

	s := &suite{
		metricController: cfg.metricController,
		tracerProvider:   cfg.tracerProvider,

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

func newMetricExporter() (*stdoutmetric.Exporter, fmt.Stringer, error) {
	out := new(bytes.Buffer)

	e, err := stdoutmetric.New(
		stdoutmetric.WithPrettyPrint(),
		stdoutmetric.WithoutTimestamps(),
		stdoutmetric.WithWriter(out),
	)
	if err != nil {
		return nil, nil, err
	}

	return e, out, nil
}

func mustNewMetricExporter() (*stdoutmetric.Exporter, fmt.Stringer) {
	e, out, err := newMetricExporter()
	handleErr(err)

	return e, out
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

func newController(opts ...controller.Option) *controller.Controller {
	opts = append(opts,
		controller.WithResource(resource.NewSchemaless(
			semconv.ServiceNameKey.String("otelsql"),
		)),
		controller.WithCollectPeriod(time.Hour),
	)

	return controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
		opts...,
	)
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
