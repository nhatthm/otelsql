package oteltest

import (
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
)

type errorMeterProvider struct {
	embedded.MeterProvider

	Error error
}

// NewMeterProviderWithError returns a new [metric.MeterProvider] that always
// returns the given error.
func NewMeterProviderWithError(e error) metric.MeterProvider {
	return &errorMeterProvider{
		Error: e,
	}
}

func (e *errorMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return &errorMeter{
		Error: e.Error,
	}
}

type errorMeter struct {
	embedded.Meter

	Error error
}

func (e *errorMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, e.Error
}

func (e *errorMeter) Int64UpDownCounter(string, ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	return nil, e.Error
}

func (e *errorMeter) Int64Histogram(string, ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	return nil, e.Error
}

func (e *errorMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, e.Error
}

func (e *errorMeter) Int64ObservableCounter(string, ...metric.Int64ObservableCounterOption) (metric.Int64ObservableCounter, error) {
	return nil, e.Error
}

func (e *errorMeter) Int64ObservableUpDownCounter(string, ...metric.Int64ObservableUpDownCounterOption) (metric.Int64ObservableUpDownCounter, error) {
	return nil, e.Error
}

func (e *errorMeter) Int64ObservableGauge(string, ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	return nil, e.Error
}

func (e *errorMeter) Float64Counter(string, ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	return nil, e.Error
}

func (e *errorMeter) Float64UpDownCounter(string, ...metric.Float64UpDownCounterOption) (metric.Float64UpDownCounter, error) {
	return nil, e.Error
}

func (e *errorMeter) Float64Histogram(string, ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return nil, e.Error
}

func (e *errorMeter) Float64Gauge(string, ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	return nil, e.Error
}

func (e *errorMeter) Float64ObservableCounter(string, ...metric.Float64ObservableCounterOption) (metric.Float64ObservableCounter, error) {
	return nil, e.Error
}

func (e *errorMeter) Float64ObservableUpDownCounter(string, ...metric.Float64ObservableUpDownCounterOption) (metric.Float64ObservableUpDownCounter, error) {
	return nil, e.Error
}

func (e *errorMeter) Float64ObservableGauge(string, ...metric.Float64ObservableGaugeOption) (metric.Float64ObservableGauge, error) {
	return nil, e.Error
}

func (e *errorMeter) RegisterCallback(metric.Callback, ...metric.Observable) (metric.Registration, error) {
	return nil, e.Error
}
