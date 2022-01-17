package oteltest

import "go.opentelemetry.io/otel/trace"

// NilTraceID is an empty trace id.
var NilTraceID trace.TraceID

// SampleTraceID is a sample of trace id.
var SampleTraceID = MustParseTraceID("25239e8a2ad5562d561f2ecd6a9744de")

// MustParseTraceID parse a string to trace id.
func MustParseTraceID(s string) trace.TraceID {
	r, err := trace.TraceIDFromHex(s)
	handleErr(err)

	return r
}

// NilSpanID is an empty span id.
var NilSpanID trace.SpanID

// SampleSpanID is a sample of span id.
var SampleSpanID = MustParseSpanID("1d256548fd1a0dba")

// MustParseSpanID parse a string to span id.
func MustParseSpanID(s string) trace.SpanID {
	r, err := trace.SpanIDFromHex(s)
	handleErr(err)

	return r
}

// Span represents a span.
type Span struct {
	Name        string          `json:"Name"`
	SpanContext SpanContext     `json:"SpanContext"`
	Parent      SpanContext     `json:"Parent"`
	SpanKind    int             `json:"SpanKind"`
	Attributes  []SpanAttribute `json:"Attributes"`
}

// SpanContext represents a span context.
type SpanContext struct {
	TraceID string `json:"TraceID"`
	SpanID  string `json:"SpanID"`
}

// SpanAttribute represents a span attribute.
type SpanAttribute struct {
	Key   string `json:"Key"`
	Value struct {
		Type  string      `json:"Type"`
		Value interface{} `json:"Value"`
	} `json:"Value"`
}
