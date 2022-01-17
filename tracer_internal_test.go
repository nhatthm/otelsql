package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/codes"
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
