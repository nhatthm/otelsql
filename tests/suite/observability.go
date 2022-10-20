package suite

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	prom "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
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

// var defaultHistogramBoundaries = []float64{1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 50000, 100000}

func newPrometheusExporter() (*prometheus.Exporter, error) {
	e, err := prometheus.New(
		prometheus.WithRegisterer(prom.NewRegistry()),
	)
	if err != nil {
		return nil, fmt.Errorf("could not init prometheus exporter: %w", err)
	}

	provider := metricsdk.NewMeterProvider(
		metricsdk.WithReader(e),
		metricsdk.WithResource(resource.NewSchemaless(
			semconv.ServiceNameKey.String("otelsqltest"),
		)),
	)

	global.SetMeterProvider(provider)

	return e, nil
}
