package otelsql

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	sqlattribute "github.com/nhatthm/otelsql/attribute"
)

const (
	traceMethodRowsNext  = "rows_next"
	traceMethodRowsClose = "rows_close"
)

var _ driver.Rows = (*rows)(nil)

// withRowsColumnTypeScanType is the same as the driver.RowsColumnTypeScanType interface except it omits the driver.Rows embedded interface.
// If the original driver.Rows implementation wrapped by ocsql supports RowsColumnTypeScanType we enable the original method implementation in the returned
// driver.Rows from wrapRows by doing a composition with ocRows.
type withRowsColumnTypeScanType interface {
	ColumnTypeScanType(index int) reflect.Type
}

type rowsNextFunc func(dest []driver.Value) (err error)

type rowsCloseFunc func() error

type rowsColumnFunc func() []string

type rows struct {
	hasNextResultSetFunc           func() bool
	nextResultSetFunc              func() error
	columnTypeDatabaseTypeNameFunc func(index int) string
	columnTypeLengthFunc           func(index int) (length int64, ok bool)
	columnTypeNullableFunc         func(index int) (nullable, ok bool)
	columnTypePrecisionScaleFunc   func(index int) (precision, scale int64, ok bool)
	columnsFunc                    rowsColumnFunc
	closeFunc                      rowsCloseFunc
	nextFunc                       rowsNextFunc
}

func (r rows) HasNextResultSet() bool {
	return r.hasNextResultSetFunc()
}

func (r rows) NextResultSet() error {
	return r.nextResultSetFunc()
}

func (r rows) ColumnTypeDatabaseTypeName(index int) string {
	return r.columnTypeDatabaseTypeNameFunc(index)
}

func (r rows) ColumnTypeLength(index int) (length int64, ok bool) {
	return r.columnTypeLengthFunc(index)
}

func (r rows) ColumnTypeNullable(index int) (nullable, ok bool) {
	return r.columnTypeNullableFunc(index)
}

func (r rows) ColumnTypePrecisionScale(index int) (precision, scale int64, ok bool) {
	return r.columnTypePrecisionScaleFunc(index)
}

func (r rows) Columns() []string {
	return r.columnsFunc()
}

func (r rows) Close() error {
	return r.closeFunc()
}

func (r rows) Next(dest []driver.Value) (err error) {
	return r.nextFunc(dest)
}

// nolint: cyclop
func wrapRows(ctx context.Context, parent driver.Rows, t methodTracer, traceRowsNext bool, traceRowsClose bool) driver.Rows {
	if !traceRowsNext && !traceRowsClose {
		return parent
	}

	ctx = trace.ContextWithSpanContext(context.Background(), trace.SpanContextFromContext(ctx))

	r := rows{
		columnsFunc:                    parent.Columns,
		closeFunc:                      parent.Close,
		nextFunc:                       parent.Next,
		hasNextResultSetFunc:           func() bool { return false },
		nextResultSetFunc:              func() error { return io.EOF },
		columnTypeDatabaseTypeNameFunc: func(_ int) string { return "" },
		columnTypeLengthFunc:           func(_ int) (int64, bool) { return 0, false },
		columnTypeNullableFunc:         func(_ int) (bool, bool) { return false, false },
		columnTypePrecisionScaleFunc:   func(_ int) (int64, int64, bool) { return 0, 0, false },
	}

	if traceRowsClose {
		successCount, successTotalTime, nextFunc := rowsNextCount(r.nextFunc)
		r.nextFunc, r.closeFunc = nextFunc, rowsCloseTrace(ctx, t, successCount, successTotalTime, parent.Close)
	}

	if traceRowsNext {
		r.nextFunc = rowsNextTrace(ctx, t, r.nextFunc)
	}

	if v, ok := parent.(driver.RowsNextResultSet); ok {
		r.hasNextResultSetFunc = v.HasNextResultSet
		r.nextResultSetFunc = v.NextResultSet
	}

	if v, ok := parent.(driver.RowsColumnTypeDatabaseTypeName); ok {
		r.columnTypeDatabaseTypeNameFunc = v.ColumnTypeDatabaseTypeName
	}

	if v, ok := parent.(driver.RowsColumnTypeLength); ok {
		r.columnTypeLengthFunc = v.ColumnTypeLength
	}

	if v, ok := parent.(driver.RowsColumnTypeNullable); ok {
		r.columnTypeNullableFunc = v.ColumnTypeNullable
	}

	if v, ok := parent.(driver.RowsColumnTypePrecisionScale); ok {
		r.columnTypePrecisionScaleFunc = v.ColumnTypePrecisionScale
	}

	if ts, ok := parent.(withRowsColumnTypeScanType); ok {
		return struct {
			rows
			withRowsColumnTypeScanType
		}{r, ts}
	}

	return r
}

func rowsNextTrace(ctx context.Context, t methodTracer, f rowsNextFunc) rowsNextFunc {
	return func(dest []driver.Value) (err error) {
		_, end := t.MustTrace(ctx, traceMethodRowsNext)

		defer func() {
			if errors.Is(err, io.EOF) {
				end(nil)
			} else {
				end(err)
			}
		}()

		return f(dest)
	}
}

func rowsNextCount(f rowsNextFunc) (*int64, *time.Duration, rowsNextFunc) {
	var (
		successCount     int64
		successTotalTime time.Duration
	)

	return &successCount, &successTotalTime,
		func(values []driver.Value) (err error) {
			startTime := time.Now()

			if err = f(values); err == nil {
				successCount++

				successTotalTime += time.Since(startTime)
			}

			return
		}
}

func rowsCloseTrace(ctx context.Context, t methodTracer, count *int64, totalTime *time.Duration, f rowsCloseFunc) rowsCloseFunc {
	return func() (err error) {
		_, end := t.MustTrace(ctx, traceMethodRowsClose)

		defer func() {
			end(err, rowsCloseAttributes(*count, *totalTime)...)
		}()

		return f()
	}
}

func rowsCloseAttributes(count int64, totalTime time.Duration) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 2)

	attrs = append(attrs, dbSQLRowsNextSuccessCount.Int64(count))

	if count < 1 {
		return attrs
	}

	attrs = append(attrs, sqlattribute.KeyValueDuration(dbSQLRowsNextLatencyAvg, time.Duration(int64(totalTime)/count)))

	return attrs
}
