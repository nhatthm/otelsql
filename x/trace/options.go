package trace

import (
	"context"
	"database/sql/driver"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/nhatthm/otelsql"
)

// TransformAndTraceQueryWithoutArgs transforms the query using a given function and adds that to the span without arguments.
//
//    trace.TransformAndTraceQueryWithoutArgs(strings.TrimSpace)
func TransformAndTraceQueryWithoutArgs(transform func(query string) string) otelsql.DriverOption {
	return otelsql.TraceQuery(func(ctx context.Context, query string, args []driver.NamedValue) []attribute.KeyValue {
		return []attribute.KeyValue{
			semconv.DBStatementKey.String(transform(query)),
		}
	})
}
