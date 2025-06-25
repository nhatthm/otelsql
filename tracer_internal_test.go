package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
)

func TestFormatSpanName(t *testing.T) {
	t.Parallel()

	actual := formatSpanName(context.Background(), "ping")
	expected := "sql:ping"

	assert.Equal(t, expected, actual)
}

func TestSpanStatusFromError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario            string
		error               error
		expectedCode        codes.Code
		expectedDescription string
	}{
		{
			scenario:            "no error",
			error:               nil,
			expectedCode:        codes.Ok,
			expectedDescription: "",
		},
		{
			scenario:            "no error",
			error:               errors.New("error"),
			expectedCode:        codes.Error,
			expectedDescription: "error",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			code, description := spanStatusFromError(tc.error)

			assert.Equal(t, tc.expectedCode, code)
			assert.Equal(t, tc.expectedDescription, description)
		})
	}
}

func TestSpanStatusFromErrorIgnoreErrSkip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario            string
		error               error
		expectedCode        codes.Code
		expectedDescription string
	}{
		{
			scenario:            "no error",
			error:               nil,
			expectedCode:        codes.Ok,
			expectedDescription: "",
		},
		{
			scenario:            "skip",
			error:               driver.ErrSkip,
			expectedCode:        codes.Ok,
			expectedDescription: "",
		},
		{
			scenario:            "no error",
			error:               errors.New("error"),
			expectedCode:        codes.Error,
			expectedDescription: "error",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			code, description := spanStatusFromErrorIgnoreErrSkip(tc.error)

			assert.Equal(t, tc.expectedCode, code)
			assert.Equal(t, tc.expectedDescription, description)
		})
	}
}

func TestMustTrace(t *testing.T) {
	tests := map[string]struct {
		method string
		labels []attribute.KeyValue

		endErr    error
		endLabels []attribute.KeyValue

		expectedStatus tracesdk.Status
		expectedLabels []attribute.KeyValue
	}{
		"records a span": {
			method: "ping",
			expectedLabels: []attribute.KeyValue{
				semconv.DBOperationKey.String("ping"),
			},
			expectedStatus: tracesdk.Status{
				Code:        codes.Ok,
				Description: "",
			},
		},
		"records a span with error": {
			method: "pong",
			endErr: errors.New("error"),
			expectedLabels: []attribute.KeyValue{
				semconv.DBOperationKey.String("pong"),
			},
			expectedStatus: tracesdk.Status{
				Code:        codes.Error,
				Description: "error",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			recorder := tracetest.NewSpanRecorder()

			mTracer := newMethodTracer(
				tracesdk.NewTracerProvider(
					tracesdk.WithSampler(tracesdk.AlwaysSample()),
					tracesdk.WithSpanProcessor(recorder),
				).Tracer(t.Name()),
			)

			newCtx, end := mTracer.MustTrace(ctx, tc.method, tc.labels...)
			assert.True(t, trace.SpanFromContext(newCtx).IsRecording())
			assert.Len(t, recorder.Started(), 1)

			end(tc.endErr, tc.endLabels...)
			assert.False(t, trace.SpanFromContext(newCtx).IsRecording())

			endedSpans := recorder.Ended()
			require.Len(t, endedSpans, 1)

			span := endedSpans[0]
			assert.Equal(t, tc.expectedLabels, span.Attributes())
			assert.Equal(t, tc.expectedStatus, span.Status())
		})
	}

	t.Run("record no span when not sampling", func(t *testing.T) {
		ctx := context.Background()
		recorder := tracetest.NewSpanRecorder()

		mTracer := newMethodTracer(
			tracesdk.NewTracerProvider(
				tracesdk.WithSampler(tracesdk.NeverSample()),
				tracesdk.WithSpanProcessor(recorder),
			).Tracer(t.Name()),
		)

		newCtx, end := mTracer.MustTrace(ctx, "ping")
		assert.False(t, trace.SpanFromContext(newCtx).IsRecording())
		assert.Len(t, recorder.Started(), 0)

		end(nil)
		assert.False(t, trace.SpanFromContext(newCtx).IsRecording())
		require.Len(t, recorder.Ended(), 0)
	})
}
