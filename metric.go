package otelsql

const (
	dbSQLClientLatencyMs = "db.sql.client.latency"
	dbSQLClientCalls     = "db.sql.client.calls"

	dbSQLConnectionsOpen           = "db.sql.connections.open"
	dbSQLConnectionsIdle           = "db.sql.connections.idle"
	dbSQLConnectionsActive         = "db.sql.connections.active"
	dbSQLConnectionsWaitCount      = "db.sql.connections.wait_count"
	dbSQLConnectionsWaitDuration   = "db.sql.connections.wait_duration"
	dbSQLConnectionsIdleClosed     = "db.sql.connections.idle_closed"
	dbSQLConnectionsLifetimeClosed = "db.sql.connections.lifetime_closed"
)
