package suite

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	prom "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

type observabilityTests struct {
	promExporter *prometheus.Exporter
}

func (t *observabilityTests) RegisterContext(sc *godog.ScenarioContext) {
	// Reset prometheus exporter for every test.
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		promExporter, err := newPrometheusExporter()
		if err != nil {
			return ctx, fmt.Errorf("could not init prometheus exporter: %w", err)
		}

		t.promExporter = promExporter

		return ctx, nil
	})
}

func newObservabilityTests() *observabilityTests {
	return &observabilityTests{}
}

var defaultHistogramBoundaries = []float64{1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 50000, 100000}

func newPrometheusExporter() (*prometheus.Exporter, error) {
	ctrl := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(defaultHistogramBoundaries),
			),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
		controller.WithResource(resource.NewSchemaless(
			semconv.ServiceNameKey.String("otelsqltest"),
		)),
		controller.WithCollectPeriod(50*time.Millisecond),
	)

	e, err := prometheus.New(prometheus.Config{
		Registry: prom.NewRegistry(),
	}, ctrl)
	if err != nil {
		return nil, fmt.Errorf("failed to init prometheus exporter: %w", err)
	}

	global.SetMeterProvider(e.MeterProvider())

	return e, nil
}
