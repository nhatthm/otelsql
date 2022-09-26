package oteltest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type metricReader struct {
	exporter metricsdk.Exporter
	metricsdk.Reader
}

func (r *metricReader) Shutdown(ctx context.Context) error {
	metrics, err := r.Reader.Collect(ctx)
	if err != nil {
		return err
	}

	if err := r.exporter.Export(ctx, metrics); err != nil {
		return err
	}

	return r.Reader.Shutdown(ctx)
}

type metricData struct {
	Name  string `json:"Name"`
	Last  any    `json:"Last,omitempty"`
	Sum   any    `json:"Sum,omitempty"`
	Count any    `json:"Count,omitempty"`
}

type metricEncoder struct {
	*json.Encoder
}

func (e *metricEncoder) Encode(v any) error {
	resMetrics, ok := v.(metricdata.ResourceMetrics)
	if !ok {
		return e.Encoder.Encode(v)
	}

	metrics := make([]metricData, 0)

	for _, scopedMetrics := range resMetrics.ScopeMetrics {
		for _, scopedMetric := range scopedMetrics.Metrics {
			attrs := resMetrics.Resource.Attributes()
			attrs = append(attrs, attribute.String("instrumentation.name", scopedMetrics.Scope.Name))

			switch smd := scopedMetric.Data.(type) {
			case metricdata.Gauge[int64]:
				metrics = append(metrics, metricDataFromGauge(scopedMetric.Name, smd, attrs)...)

			case metricdata.Gauge[float64]:
				metrics = append(metrics, metricDataFromGauge(scopedMetric.Name, smd, attrs)...)

			case metricdata.Sum[int64]:
				metrics = append(metrics, metricDataFromSum(scopedMetric.Name, smd, attrs)...)

			case metricdata.Sum[float64]:
				metrics = append(metrics, metricDataFromSum(scopedMetric.Name, smd, attrs)...)

			case metricdata.Histogram:
				metrics = append(metrics, metricDataFromHistogram(scopedMetric.Name, smd, attrs)...)
			}
		}
	}

	return e.Encoder.Encode(metrics)
}

func metricDataFromGauge[N int64 | float64](name string, g metricdata.Gauge[N], attrs []attribute.KeyValue) []metricData {
	result := make([]metricData, 0, len(g.DataPoints))

	for _, dp := range g.DataPoints {
		result = append(result, metricData{
			Name: metricDataName(name, append(attrs, dp.Attributes.ToSlice()...)),
			Last: dp.Value,
		})
	}

	return result
}

func metricDataFromSum[N int64 | float64](name string, g metricdata.Sum[N], attrs []attribute.KeyValue) []metricData {
	result := make([]metricData, 0, len(g.DataPoints))

	for _, dp := range g.DataPoints {
		result = append(result, metricData{
			Name: metricDataName(name, append(attrs, dp.Attributes.ToSlice()...)),
			Sum:  dp.Value,
		})
	}

	return result
}

func metricDataFromHistogram(name string, g metricdata.Histogram, attrs []attribute.KeyValue) []metricData {
	result := make([]metricData, 0, len(g.DataPoints))

	for _, dp := range g.DataPoints {
		result = append(result, metricData{
			Name:  metricDataName(name, append(attrs, dp.Attributes.ToSlice()...)),
			Count: dp.Count,
			Sum:   dp.Sum,
		})
	}

	return result
}

func metricDataName(name string, attrs []attribute.KeyValue) string {
	labels := make([]string, len(attrs))

	for i, attr := range attrs {
		labels[i] = fmt.Sprintf("%s=%s", attr.Key, attr.Value.Emit())
	}

	return fmt.Sprintf("%s{%s}", name, strings.Join(labels, ","))
}
