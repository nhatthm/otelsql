package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/codes"
)

func TestTraceAll(t *testing.T) {
	t.Parallel()

	o := driverOptions{}

	TraceAll().applyDriverOptions(&o)

	assert.True(t, o.trace.AllowRoot)
	assert.True(t, o.trace.Ping)
	assert.True(t, o.trace.RowsNext)
	assert.True(t, o.trace.RowsClose)
	assert.True(t, o.trace.RowsAffected)
	assert.True(t, o.trace.LastInsertID)

	var (
		ctx    = context.Background()
		query  = "SELECT * FROM data WHERE country = $1"
		values = []driver.NamedValue{{
			Ordinal: 1,
			Value:   "US",
		}}
	)

	expected := traceQueryWithArgs(ctx, query, values)
	actual := o.trace.queryTracer(ctx, query, values)

	assert.Equal(t, expected, actual)
}

func TestDisableErrSkip(t *testing.T) {
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

			o := driverOptions{}

			DisableErrSkip().applyDriverOptions(&o)

			code, description := o.trace.errorToSpanStatus(tc.error)

			assert.Equal(t, tc.expectedCode, code)
			assert.Equal(t, tc.expectedDescription, description)
		})
	}
}
