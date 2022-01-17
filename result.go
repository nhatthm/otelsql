package otelsql

import (
	"context"
	"database/sql/driver"

	"go.opentelemetry.io/otel/trace"
)

const (
	traceMethodLastInsertID = "last_insert_id"
	traceMethodRowsAffected = "rows_affected"
)

type resultFunc func() (int64, error)

var _ driver.Result = (*result)(nil)

type result struct {
	lastInsertIDFunc resultFunc
	rowsAffectedFunc resultFunc
}

func (r result) LastInsertId() (int64, error) {
	return r.lastInsertIDFunc()
}

func (r result) RowsAffected() (int64, error) {
	return r.rowsAffectedFunc()
}

func wrapResult(ctx context.Context, parent driver.Result, t methodTracer, traceLastInsertID bool, traceRowsAffected bool) driver.Result {
	if !traceLastInsertID && !traceRowsAffected {
		return parent
	}

	ctx = trace.ContextWithSpanContext(context.Background(), trace.SpanContextFromContext(ctx))

	r := &result{
		lastInsertIDFunc: parent.LastInsertId,
		rowsAffectedFunc: parent.RowsAffected,
	}

	if traceLastInsertID {
		r.lastInsertIDFunc = resultTrace(ctx, t, traceMethodLastInsertID, parent.LastInsertId)
	}

	if traceRowsAffected {
		r.rowsAffectedFunc = resultTrace(ctx, t, traceMethodRowsAffected, parent.RowsAffected)
	}

	return r
}

func resultTrace(ctx context.Context, t methodTracer, method string, f resultFunc) resultFunc {
	return func() (result int64, err error) {
		_, end := t.MustTrace(ctx, method)

		defer func() {
			end(err)
		}()

		result, err = f()

		return
	}
}
