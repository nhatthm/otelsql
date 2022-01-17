package otelsql

import (
	"go.opentelemetry.io/otel/attribute"
)

const (
	// Type: string.
	// Required: No.
	dbInstance = attribute.Key("db.instance")

	// Type: string.
	// Required: No.
	dbSQLStatus = attribute.Key("db.sql.status")
	// Type: string.
	// Required: No.
	dbSQLError = attribute.Key("db.sql.error")
	// Type: int64.
	// Required: No.
	dbSQLRowsNextSuccessCount = attribute.Key("db.sql.rows_next.success_count")
	// Type: string.
	// Required: No.
	dbSQLRowsNextLatencyAvg = attribute.Key("db.sql.rows_next.latency_avg")
)

var (
	dbSQLStatusOK    = dbSQLStatus.String("OK")
	dbSQLStatusERROR = dbSQLStatus.String("ERROR")
)
