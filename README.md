# OpenTelemetry SQL database driver wrapper for Go

[![GitHub Releases](https://img.shields.io/github/v/release/nhatthm/otelsql)](https://github.com/nhatthm/otelsql/releases/latest)
[![Build Status](https://github.com/nhatthm/otelsql/actions/workflows/test-unit.yaml/badge.svg?branch=master)](https://github.com/nhatthm/otelsql/actions/workflows/test-unit.yaml)
[![codecov](https://codecov.io/gh/nhatthm/otelsql/branch/master/graph/badge.svg?token=eTdAgDE2vR)](https://codecov.io/gh/nhatthm/otelsql)
[![Go Report Card](https://goreportcard.com/badge/github.com/nhatthm/otelsql)](https://goreportcard.com/report/github.com/nhatthm/otelsql)
[![GoDevDoc](https://img.shields.io/badge/dev-doc-00ADD8?logo=go)](https://pkg.go.dev/github.com/nhatthm/otelsql)
[![Donate](https://img.shields.io/badge/Donate-PayPal-green.svg)](https://www.paypal.com/donate/?hosted_button_id=PJZSGJN57TDJY)

Add a OpenTelemetry wrapper to your existing database code to instrument the interactions with the database. The wrapper supports both traces and metrics.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Install](#install)
- [Usage](#usage)
    - [Options](#options)
- [Extras](#extras)
    - [Span Name Formatter](#span-name-formatter)
    - [Convert Error to Span Status](#convert-error-to-span-status)
    - [Trace Query](#trace-query)
    - [AllowRoot() and Span Context](#allowroot-and-span-context)
    - [`jmoiron/sqlx`](#jmoironsqlx)
- [Metrics](#metrics)
    - [Client](#client-metrics)
    - [Database Connection](#database-connection-metrics)
- [Traces](#traces)
- [Migration from `ocsql`](#migration-from-ocsql)
    - [Options](#options-1)
    - [Metrics](#metrics-1)
    - [Traces](#traces-1)
- [Compatibility](#compatibility)

## Prerequisites

- `Go >= 1.16`

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

## Install

```bash
go get github.com/nhatthm/otelsql
```

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

## Usage

To use `otelsql` with your application, register the `otelsql` wrapper by using `otelsql.Register(driverName string, opts ...otelsql.DriverOption)`. For
example:

```go
package example

import (
	"database/sql"

	"github.com/nhatthm/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

func openDB(dsn string) (*sql.DB, error) {
	// Register the otelsql wrapper for the provided postgres driver.
	driverName, err := otelsql.Register("postgres",
		otelsql.AllowRoot(),
		otelsql.TraceQueryWithoutArgs(),
		otelsql.TraceRowsClose(),
		otelsql.TraceRowsAffected(),
		otelsql.WithDatabaseName("my_database"),        // Optional.
		otelsql.WithSystem(semconv.DBSystemPostgreSQL), // Optional.
	)
	if err != nil {
		return nil, err
	}

	// Connect to a Postgres database using the postgres driver wrapper.
	return sql.Open(driverName, dsn)
}
```

The wrapper will automatically instrument the interactions with the database.

Optionally, you could record [database connection metrics](#database-connection-metrics) using the `otelsql.RecordStats()`. For example:

```go
package example

import (
	"database/sql"

	"github.com/nhatthm/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

func openDB(dsn string) (*sql.DB, error) {
	// Register the otelsql wrapper for the provided postgres driver.
	driverName, err := otelsql.Register("postgres",
		otelsql.AllowRoot(),
		otelsql.TraceQueryWithoutArgs(),
		otelsql.TraceRowsClose(),
		otelsql.TraceRowsAffected(),
		otelsql.WithDatabaseName("my_database"),        // Optional.
		otelsql.WithSystem(semconv.DBSystemPostgreSQL), // Optional.
	)
	if err != nil {
		return nil, err
	}

	// Connect to a Postgres database using the postgres driver wrapper.
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	if err := otelsql.RecordStats(db); err != nil {
		return nil, err
	}

	return db, nil
}
```

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Options

**Driver Options**

| Option                                         | Description                                                                                                                                                                                                                                                                                    |
|:-----------------------------------------------|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `WithMeterProvider(metric.MeterProvider)`      | Specify a meter provider                                                                                                                                                                                                                                                                       |
| `WithTracerProvider(trace.TracerProvider)`     | Specify a tracer provider                                                                                                                                                                                                                                                                      |
| `WithDefaultAttributes(...attribute.KeyValue)` | Add extra attributes for the recorded spans and metrics                                                                                                                                                                                                                                        |
| `WithInstanceName(string)`                     | Add an extra attribute for annotating the instance name                                                                                                                                                                                                                                        |
| `WithSystem(attribute.KeyValue)`               | Add an extra attribute for annotating the type of database server.<br/> The value is set by using the well-known identifiers in `semconv`. For example: `semconv.DBSystemPostgreSQL`. See [more](https://github.com/open-telemetry/opentelemetry-go/blob/main/semconv/v1.7.0/trace.go#L37-L43) |
| `WithDatabaseName(string)`                     | Add an extra attribute for annotating the database name                                                                                                                                                                                                                                        |
| `WithSpanNameFormatter(spanNameFormatter)`     | Set a custom [span name formatter](#span-name-formatter)                                                                                                                                                                                                                                       |
| `ConvertErrorToSpanStatus(errorToSpanStatus)`  | Set a custom [converter for span status](#convert-error-to-span-status)                                                                                                                                                                                                                        |
| `DisableErrSkip()`                             | `sql.ErrSkip` is considered as `OK` in span status                                                                                                                                                                                                                                             |
| `TraceQuery()`                                 | Set a custom function for [tracing query](#trace-query)                                                                                                                                                                                                                                        |
| `TraceQueryWithArgs()`                         | [Trace query](#trace-query) and all arguments                                                                                                                                                                                                                                                  |
| `TraceQueryWithoutArgs()`                      | [Trace query](#trace-query) without the arguments                                                                                                                                                                                                                                              |
| `AllowRoot()`                                  | Create root spans in absence of existing spans or even context                                                                                                                                                                                                                                 |
| `TracePing()`                                  | Enable the creation of spans on Ping requests                                                                                                                                                                                                                                                  |
| `TraceRowsNext()`                              | Enable the creation of spans on RowsNext calls. (This can result in many spans)                                                                                                                                                                                                                |
| `TraceRowsClose()`                             | Enable the creation of spans on RowsClose calls                                                                                                                                                                                                                                                |
| `TraceRowsAffected()`                          | Enable the creation of spans on RowsAffected calls                                                                                                                                                                                                                                             |
| `TraceLastInsertID()`                          | Enable the creation of spans on LastInsertId call                                                                                                                                                                                                                                              |
| `TraceAll()`                                   | Turn on all tracing options, including `AllowRoot()` and `TraceQueryWithArgs()`                                                                                                                                                                                                                |

**Record Stats Options**

| Option                                          | Description                                                                                                                                                                                                                                                                                    |
|:------------------------------------------------|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `WithMeterProvider(metric.MeterProvider)`       | Specify a meter provider                                                                                                                                                                                                                                                                       |
| `WithMinimumReadDBStatsInterval(time.Duration)` | The minimum interval between calls to db.Stats(). Negative values are ignored.                                                                                                                                                                                                                 |
| `WithDefaultAttributes(...attribute.KeyValue)`  | Add extra attributes for the recorded metrics                                                                                                                                                                                                                                                  |
| `WithInstanceName(string)`                      | Add an extra attribute for annotating the instance name                                                                                                                                                                                                                                        |
| `WithSystem(attribute.KeyValue)`                | Add an extra attribute for annotating the type of database server.<br/> The value is set by using the well-known identifiers in `semconv`. For example: `semconv.DBSystemPostgreSQL`. See [more](https://github.com/open-telemetry/opentelemetry-go/blob/main/semconv/v1.7.0/trace.go#L37-L43) |
| `WithDatabaseName(string)`                      | Add an extra attribute for annotating the database name                                                                                                                                                                                                                                        |

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

## Extras

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Span Name Formatter

By default, spans will be created with the `sql:METHOD` format, like `sql:exec` or `sql:query`. You could change this behavior by using
the `WithSpanNameFormatter()` option and set your own logic.

For example

```go
package example

import (
	"context"
	"database/sql"

	"github.com/nhatthm/otelsql"
)

func openDB(dsn string) (*sql.DB, error) {
	driverName, err := otelsql.Register("my-driver",
		otelsql.WithSpanNameFormatter(func(_ context.Context, op string) string {
			return "main-db:" + op
		}),
	)
	if err != nil {
		return nil, err
	}

	return sql.Open(driverName, dsn)
}
```

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Convert Error to Span Status

By default, all errors are considered as `ERROR` while setting span status, except `io.EOF` on RowsNext calls (which is `OK`). `otelsql` also provides an extra
option `DisableErrSkip()` if you want to ignore the `sql.ErrSkip`.

You can write your own conversion by using the `ConvertErrorToSpanStatus()` option. For example

```go
package example

import (
	"database/sql"
	"errors"

	"github.com/nhatthm/otelsql"
	"go.opentelemetry.io/otel/codes"
)

func openDB(dsn string) (*sql.DB, error) {
	driverName, err := otelsql.Register("my-driver",
		otelsql.ConvertErrorToSpanStatus(func(err error) (codes.Code, string) {
			if err == nil || errors.Is(err, ignoredError) {
				return codes.Ok, ""
			}

			return codes.Error, err.Error()
		}),
	)
	if err != nil {
		return nil, err
	}

	return sql.Open(driverName, dsn)
}
```

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Trace Query

By default, `otelsql` does not trace query and arguments. When you use these options:

- `TraceQueryWithArgs()`: Trace the query and all arguments.
- `TraceQueryWithoutArgs()`: Trace only the query, without the arguments.

The traced query will be set in the `semconv.DBStatementKey` attribute (`db.statement`) and the arguments are set as follows:

- `db.sql.args.NAME`: if the arguments are named.
- `db.sql.args.ORDINAL`: Otherwise.

Example #1:

```sql
SELECT *
FROM data
WHERE country = :country
```

The argument attribute will be `db.sql.args.country`

Example #2:

```sql
SELECT *
FROM data
WHERE country = $1
```

The argument attribute will be `db.sql.args.1`

You can change this behavior for your own purpose (like, redaction or stripping out sensitive information) by using the `TraceQuery()` option. For example:

```go
package example

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/nhatthm/otelsql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

func openDB(dsn string) (*sql.DB, error) {
	driverName, err := otelsql.Register("my-driver",
		otelsql.TraceQuery(func(_ context.Context, query string, args []driver.NamedValue) []attribute.KeyValue {
			attrs := make([]attribute.KeyValue, 0, 1+len(args))

			attrs = append(attrs, semconv.DBStatementKey.String(query))

			// Your redaction goes here.

			return attrs
		}),
	)
	if err != nil {
		return nil, err
	}

	return sql.Open(driverName, dsn)
}
```

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### AllowRoot() and Span Context

To fully take advantage of `otelsql`, all database calls should be made using the `*Context` methods. Failing to do so will result in many orphaned traces if
the `AllowRoot()` is used. By default, `AllowRoot()` is disabled and will result in `otelsql` not tracing the database calls if context or parent spans are
missing.

| Old              | New                     |
|:-----------------|:------------------------|
| `*DB.Begin`      | `*DB.BeginTx`           |
| `*DB.Exec`       | `*DB.ExecContext`       |
| `*DB.Ping`       | `*DB.PingContext`       |
| `*DB.Prepare`    | `*DB.PrepareContext`    |
| `*DB.Query`      | `*DB.QueryContext`      |
| `*DB.QueryRow`   | `*DB.QueryRowContext`   |
|||
| `*Stmt.Exec`     | `*Stmt.ExecContext`     |
| `*Stmt.Query`    | `*Stmt.QueryContext`    |
| `*Stmt.QueryRow` | `*Stmt.QueryRowContext` |
|||
| `*Tx.Exec`       | `*Tx.ExecContext`       |
| `*Tx.Prepare`    | `*Tx.PrepareContext`    |
| `*Tx.Query`      | `*Tx.QueryContext`      |
| `*Tx.QueryRow`   | `*Tx.QueryRowContext`   |

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### `jmoiron/sqlx`

If using the `jmoiron/sqlx` library with named queries you will need to use the `sqlx.NewDb` function to wrap an existing `*sql.DB` connection. Do not use the
`sqlx.Open` and `sqlx.Connect` methods. `jmoiron/sqlx` uses the driver name to figure out which database is being used. It uses this knowledge to convert named
queries to the correct bind type (dollar sign, question mark) if named queries are not supported natively by the database. Since `otelsql` creates a new driver
name it will not be recognized by `jmoiron/sqlx` and named queries will fail.

For example:

```go
package example

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/nhatthm/otelsql"
)

func openDB(dsn string) (*sql.DB, error) {
	driverName, err := otelsql.Register("my-driver",
		otelsql.AllowRoot(),
		otelsql.TraceQueryWithoutArgs(),
		otelsql.TraceRowsClose(),
		otelsql.TraceRowsAffected(),
	)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	return sqlx.NewDb(db, "my-driver"), nil
}
```

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

## Metrics

**Attributes** *(applies to all the metrics below)*

| Attribute       | Description             | Note                                                     |
|:----------------|:------------------------|:---------------------------------------------------------|
| `db_operation`  | The executed sql method | For example: `exec`, `query`, `prepare`                  |
| `db_sql_status` | The execution status    | `OK` if no error, otherwise `ERROR`                      |
| `db_sql_error`  | The error message       | When `status` is `ERROR`. The value is the error message |
| `db_instance`   | The instance name       | Only when using `WithInstanceName()` option              |
| `db_system`     | The system name         | Only when using `WithSystem()` option                    |
| `db_name`       | The database name       | Only when using `WithDatabaseName()` option              |

`WithDefaultAttributes(attrs ...attribute.KeyValue)` will also add the `attrs` to the recorded metrics.

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Client Metrics

| Metric                                                                                      | Description                         |
|:--------------------------------------------------------------------------------------------|:------------------------------------|
| `db_sql_client_calls{db_instance,db_operation,db_sql_status,db_system,db_name}`             | Number of Calls (Counter)           |
| `db_sql_client_latency_bucket{db_instance,db_operation,db_sql_status,db_system,db_name,le}` | Latency in milliseconds (Histogram) |
| `db_sql_client_latency_sum{db_instance,db_operation,db_sql_status,db_system,db_name}`       |                                     |
| `db_sql_client_latency_count{db_instance,db_operation,db_sql_status,db_system,db_name}`     |                                     |

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Database Connection Metrics

| Metric                                                              | Description                                                |
|:--------------------------------------------------------------------|:-----------------------------------------------------------|
| `db_sql_connections_active{db_instance,db_system,db_name}`          | Number of active connections                               |
| `db_sql_connections_idle{db_instance,db_system,db_name}`            | Number of idle connections                                 |
| `db_sql_connections_idle_closed{db_instance,db_system,db_name}`     | Total number of closed connections by `SetMaxIdleConns`    |
| `db_sql_connections_lifetime_closed{db_instance,db_system,db_name}` | Total number of closed connections by `SetConnMaxLifetime` |
| `db_sql_connections_open{db_instance,db_system,db_name}`            | Number of open connections                                 |
| `db_sql_connections_wait_count{db_instance,db_system,db_name}`      | Total number of connections waited for                     |
| `db_sql_connections_wait_duration{db_instance,db_system,db_name}`   | Total time blocked waiting for new connections             |

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

## Traces

| Operation               | Trace                                         |
|:------------------------|:----------------------------------------------|
| `*DB.BeginTx`           | Always                                        |
| `*DB.ExecContext`       | Always                                        |
| `*DB.PingContext`       | Disabled. Use `TracePing()` to enable         |
| `*DB.PrepareContext`    | Always                                        |
| `*DB.QueryContext`      | Always                                        |
| `*DB.QueryRowContext`   | Always                                        |
|||
| `*Stmt.ExecContext`     | Always                                        |
| `*Stmt.QueryContext`    | Always                                        |
| `*Stmt.QueryRowContext` | Always                                        |
|||
| `*Tx.ExecContext`       | Always                                        |
| `*Tx.PrepareContext`    | Always                                        |
| `*Tx.QueryContext`      | Always                                        |
| `*Tx.QueryRowContext`   | Always                                        |
|||
| `*Rows.Next`            | Disabled. Use `TraceRowsNext()` to enable     |
| `*Rows.Close`           | Disabled. Use `TraceRowsClose()` to enable    |
|||
| `*Result.LastInsertID`  | Disabled. Use `TraceLastInsertID()` to enable |
| `*Result.RowsAffected`  | Disabled. Use `TraceRowsAffected()` to enable |

`ExecContext`, `QueryContext`, `QueryRowContext`, `PrepareContext` are always traced without query args unless using `TraceQuery()`, `TraceQueryWithArgs()`, or `TraceQueryWithoutArgs()` option.

Using `WithDefaultAttributes(...attribute.KeyValue)` will add extra attributes to the recorded spans.

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

## Migration from `ocsql`

The migration is easy because the behaviors of `otelsql` are the same as `ocsql`, and all options are almost similar.

|                             | `ocsql`                                               | `otelsql`                                              |
|:----------------------------|:------------------------------------------------------|:-------------------------------------------------------|
| Register driver wrapper     | `Register(driverName string, options ...TraceOption)` | `Register(driverName string, options ...DriverOption)` |
| Records database statistics | `RecordStats(db *sql.DB, interval time.Duration)`     | `RecordStats(db *sql.DB, opts ...StatsOption)`         |

The `interval` in `RecordStats()` is replaced with `WithMinimumReadDBStatsInterval(time.Duration)` option.

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Options

| `ocsql`                                         | `otelsql`                                                             |
|:------------------------------------------------|:----------------------------------------------------------------------|
| `WithAllTraceOptions()`                         | `TraceAll()` <br/> <sub>`otelsql` always set to `true`</sub>          |
| `WithOptions(ocsql.TraceOptions)`               | *Dropped*                                                             |
| `WithAllowRoot(bool)`                           | `AllowRoot()` <br/> <sub>`otelsql` always set to `true`</sub>         |
| `WithPing(bool)`                                | `TracePing()` <br/> <sub>`otelsql` always set to `true`</sub>         |
| `WithRowsNext(bool)`                            | `TraceRowsNext()` <br/> <sub>`otelsql` always set to `true`</sub>     |
| `WithRowsClose(bool)`                           | `TraceRowsClose()` <br/> <sub>`otelsql` always set to `true`</sub>    |
| `WithRowsAffected(bool)`                        | `TraceRowsAffected()` <br/> <sub>`otelsql` always set to `true`</sub> |
| `WithLastInsertID(bool)`                        | `TraceLastInsertID()` <br/> <sub>`otelsql` always set to `true`</sub> |
| `WithQuery(bool)` <br/> `WithQueryParams(bool)` | `TraceQueryWithArgs()` <br/> `TraceQueryWithoutArgs()`                |
| `WithDefaultAttributes(...trace.Attribute)`     | `WithDefaultAttributes(...attribute.KeyValue)`                        |
| `WithDisableErrSkip(bool)`                      | `DisableErrSkip()`                                                    |
| `WithSampler(trace.Sampler)`                    | *Dropped*                                                             |
| `WithInstanceName(string)`                      | `WithInstanceName(string)`                                            |

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Metrics

**Attributes** *(applies to all the metrics below)*

| `ocsql`           | `otelsql`       | Note                                        |
|:------------------|:----------------|:--------------------------------------------|
| `go_sql_instance` | `db_instance`   | Only when using `WithInstanceName()` option |
| `go_sql_method`   | `db_operation`  |                                             |
| `go_sql_status`   | `db_sql_status` |                                             |
| n/a               | `db_system`     | Only when using `WithSystem()` option       |
| n/a               | `db_name`       | Only when using `WithDatabaseName()` option |

**Client Metrics**

| `ocsql`                                                                        | `otelsql`                                                                                   |
|:-------------------------------------------------------------------------------|:--------------------------------------------------------------------------------------------|
| `go_sql_client_calls{go_sql_instance,go_sql_method,go_sql_status}`             | `db_sql_client_calls{db_instance,db_operation,db_sql_status,db_system,db_name}`             |
| `go_sql_client_latency_bucket{go_sql_instance,go_sql_method,go_sql_status,le}` | `db_sql_client_latency_bucket{db_instance,db_operation,db_sql_status,db_system,db_name,le}` |
| `go_sql_client_latency_sum{go_sql_instance,go_sql_method,go_sql_status}`       | `db_sql_client_latency_sum{db_instance,db_operation,db_sql_status,db_system,db_name}`       |
| `go_sql_client_latency_count{go_sql_instance,go_sql_method,go_sql_status}`     | `db_sql_client_latency_count{db_instance,db_operation,db_sql_status,db_system,db_name}`     |

**Connection Metrics**

| `ocsql`                                                        | `otelsql`                                                           |
|:---------------------------------------------------------------|:--------------------------------------------------------------------|
| `go_sql_db_connections_active{go_sql_instance}`                | `db_sql_connections_active{db_instance,db_system,db_name}`          |
| `go_sql_db_connections_idle{go_sql_instance}`                  | `db_sql_connections_idle{db_instance,db_system,db_name}`            |
| `go_sql_db_connections_idle_closed_count{go_sql_instance}`     | `db_sql_connections_idle_closed{db_instance,db_system,db_name}`     |
| `go_sql_db_connections_lifetime_closed_count{go_sql_instance}` | `db_sql_connections_lifetime_closed{db_instance,db_system,db_name}` |
| `go_sql_db_connections_open{go_sql_instance}`                  | `db_sql_connections_open{db_instance,db_system,db_name}`            |
| `go_sql_db_connections_wait_count{go_sql_instance}`            | `db_sql_connections_wait_count{db_instance,db_system,db_name}`      |
| `go_sql_db_connections_wait_duration{go_sql_instance}`         | `db_sql_connections_wait_duration{db_instance,db_system,db_name}`   |

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Traces

The traces are almost identical with some minor changes:

1. Named arguments are not just recorder as `<NAME>` in the span. They are now `db.sql.args.<NAME>`.
2. `sql.query` is now `db.statement`.

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

## Compatibility

<table>
    <thead>
        <tr>
            <th colspan="2"></th>
            <th colspan="6">OS</th>
        </tr>
        <tr>
            <th rowspan="2">Driver</th>
            <th rowspan="2">Database</th>
            <th colspan="2">Ubuntu</th>
            <th colspan="2">MacOS</th>
            <th colspan="2">Windows</th>
        </tr>
        <tr>
            <th>go 1.16</th>
            <th>go 1.17</th>
            <th>go 1.16</th>
            <th>go 1.17</th>
            <th>go 1.16</th>
            <th>go 1.17</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td colspan="2">
                <code>DATA-DOG/go-sqlmock</code>
            </td>
            <td colspan="6" align="center">
                <a href="https://github.com/nhatthm/otelsql/actions/workflows/test-unit.yaml">
                    <img
                        src="https://github.com/nhatthm/otelsql/actions/workflows/test-unit.yaml/badge.svg?branch=master" alt="Build Status"
                        style="max-width: 100%;">
                </a>
            </td>
        </tr>
        <tr>
            <td colspan="2">
                <code>jmoiron/sqlx</code>
            </td>
            <td colspan="6" align="center">
                <img src="https://img.shields.io/badge/manual%20test-passing-brightgreen?labelColor=3F4750&logo=target&logoWidth=10&logoColor=959DA5&color=31C754" alt="Manually Tested">
            </td>
        </tr>
        <tr>
            <td>
                <code>jackc/pgx/stdlib</code>
            </td>
            <td>
                Postgres 12, 13, 14
            </td>
            <td colspan="6" align="center">
                <a href="https://github.com/nhatthm/otelsql/actions/workflows/test-compatibility-pgx.yaml">
                    <img
                        src="https://github.com/nhatthm/otelsql/actions/workflows/test-compatibility-pgx.yaml/badge.svg?branch=master" alt="Build Status"
                        style="max-width: 100%;">
                </a>
            </td>
        </tr>
        <tr>
            <td>
                <code>lib/pq</code>
            </td>
            <td>
                Postgres 12, 13, 14
            </td>
            <td colspan="6" align="center">
                <a href="https://github.com/nhatthm/otelsql/actions/workflows/test-compatibility-libpq.yaml">
                    <img
                        src="https://github.com/nhatthm/otelsql/actions/workflows/test-compatibility-libpq.yaml/badge.svg?branch=master" alt="Build Status"
                        style="max-width: 100%;">
                </a>
            </td>
        </tr>
        <tr>
            <td>
                <code>go-sql-driver/mysql</code>
            </td>
            <td>
                MySQL 8
            </td>
            <td colspan="6" align="center">
                <a href="https://github.com/nhatthm/otelsql/actions/workflows/test-compatibility-mysql.yaml">
                    <img
                        src="https://github.com/nhatthm/otelsql/actions/workflows/test-compatibility-mysql.yaml/badge.svg?branch=master" alt="Build Status"
                        style="max-width: 100%;">
                </a>
            </td>
        </tr>
    <tbody>
</table>

<sub>*If you don't see a driver in the list, it doesn't mean the wrapper is incompatible. it's just not tested. Let me know if it works with your driver*</sub>

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

## Donation

If this project help you reduce time to develop, you can give me a cup of coffee :)

[<sub><sup>[table of contents]</sup></sub>](#table-of-contents)

### Paypal donation

[![paypal](https://www.paypalobjects.com/en_US/i/btn/btn_donateCC_LG.gif)](https://www.paypal.com/donate/?hosted_button_id=PJZSGJN57TDJY)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;or scan this

<img src="https://user-images.githubusercontent.com/1154587/113494222-ad8cb200-94e6-11eb-9ef3-eb883ada222a.png" width="147px" />
