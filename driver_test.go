package otelsql_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/nhatthm/otelsql"
	"github.com/nhatthm/otelsql/internal/test/oteltest"
	"github.com/nhatthm/otelsql/internal/test/sqlmock"
)

func TestRegister_UnknownDriver(t *testing.T) {
	t.Parallel()

	drv, err := otelsql.Register("unknown")

	assert.Empty(t, drv)

	expected := errors.New(`sql: unknown driver "unknown" (forgotten import?)`)

	assert.Equal(t, expected, err)
}

func TestRegister_MaxSlots(t *testing.T) {
	t.Parallel()

	numSlots := 100

	sql.Register("max-slots", struct {
		driver.Driver
	}{})

	for i := 0; i < numSlots; i++ {
		driverName, err := otelsql.Register("max-slots")

		assert.NotEmpty(t, driverName)
		assert.NoError(t, err)
	}

	driverName, err := otelsql.Register("max-slots")

	expected := errors.New("unable to register driver, all slots have been taken")

	assert.Empty(t, driverName)
	assert.Equal(t, expected, err)
}

func TestWrap_DriverContext_Driver(t *testing.T) {
	t.Parallel()

	parent := struct {
		driver.Driver
		driver.DriverContext
	}{
		DriverContext: driverOpenConnectorFunc(func(name string) (driver.Connector, error) {
			return nil, nil
		}),
	}

	drv := otelsql.Wrap(parent).(driver.DriverContext) // nolint: errcheck

	connector, err := drv.OpenConnector("")

	assert.NoError(t, err)

	assert.NotNil(t, connector.Driver())
}

func TestWrap_DriverContext_OpenConnectorError(t *testing.T) {
	t.Parallel()

	parent := struct {
		driver.Driver
		driver.DriverContext
	}{
		DriverContext: driverOpenConnectorFunc(func(name string) (driver.Connector, error) {
			return nil, errors.New("open connector error")
		}),
	}

	drv := otelsql.Wrap(parent).(driver.DriverContext) // nolint: errcheck

	connector, err := drv.OpenConnector("")
	expectedError := errors.New("open connector error")

	assert.Nil(t, connector)
	assert.Equal(t, expectedError, err)
}

func TestWrap_DriverContext_ConnectError(t *testing.T) {
	t.Parallel()

	parent := struct {
		driver.Driver
		driver.DriverContext
	}{
		DriverContext: driverOpenConnectorFunc(func(name string) (driver.Connector, error) {
			return struct {
				driverDriverFunc
				driverConnectFunc
			}{
				driverConnectFunc: func(ctx context.Context) (driver.Conn, error) {
					return nil, errors.New("connect error")
				},
			}, nil
		}),
	}

	drv := otelsql.Wrap(parent).(driver.DriverContext) // nolint: errcheck
	connector, err := drv.OpenConnector("")

	assert.NoError(t, err)

	conn, err := connector.Connect(context.Background())
	expectedError := errors.New("connect error")

	assert.Nil(t, conn)
	assert.Equal(t, expectedError, err)
}

func TestWrap_DriverContext_CloseBeforeOpenConnector(t *testing.T) {
	t.Parallel()

	parent := struct {
		driver.Driver
		driver.DriverContext
	}{
		DriverContext: driverOpenConnectorFunc(func(name string) (driver.Connector, error) {
			return struct {
				driverDriverFunc
				driverConnectFunc
				driverCloseFunc
			}{}, nil
		}),
	}

	drv, ok := otelsql.Wrap(parent).(struct {
		driver.Driver
		driver.DriverContext
	})
	require.True(t, ok, "unexpected driver implementation")

	c, ok := drv.Driver.(io.Closer)
	require.True(t, ok, "driver must implement io.Closer")

	err := c.Close()
	assert.NoError(t, err)
}

func TestWrap_DriverContext_CloseError(t *testing.T) {
	t.Parallel()

	parent := struct {
		driver.Driver
		driver.DriverContext
	}{
		DriverContext: driverOpenConnectorFunc(func(name string) (driver.Connector, error) {
			return struct {
				driverDriverFunc
				driverConnectFunc
				driverCloseFunc
			}{
				driverConnectFunc: func(ctx context.Context) (driver.Conn, error) {
					return nil, nil
				},
				driverCloseFunc: func() error {
					return errors.New("close error")
				},
			}, nil
		}),
	}

	drv := otelsql.Wrap(parent).(driver.DriverContext) // nolint: errcheck
	connector, err := drv.OpenConnector("")

	assert.NoError(t, err)

	err = connector.(io.Closer).Close()
	expectedError := errors.New("close error")

	assert.Equal(t, expectedError, err)
}

func Test_Open_Error(t *testing.T) {
	t.Parallel()

	parent := driverOpenFunc(func(string) (driver.Conn, error) {
		return nil, errors.New("open error")
	})

	drv := otelsql.Wrap(parent)

	conn, err := drv.Open("")
	expectedError := errors.New("open error")

	assert.Nil(t, conn)
	assert.Equal(t, expectedError, err)
}

func Test_OpenConnector_Connect(t *testing.T) {
	t.Parallel()

	sql.Register("open-connector", struct {
		driver.Driver
		driver.DriverContext
	}{
		DriverContext: sqlmock.DriverContext(func(m sqlmock.Sqlmock) {
			m.ExpectPing()
		}),
	})

	driverName, err := otelsql.Register("open-connector")

	require.NoError(t, err)

	db, err := sql.Open(driverName, "")

	require.NoError(t, err)

	err = db.Ping()

	assert.NoError(t, err)
}

func Test_Ping(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedPingMetricOK()
	expectedTraceWithoutParent := expectedPingTrace(noParentSpanIDs())
	expectedTraceWithSampleParent := expectedPingTrace(sampleParentSpanIDs())

	testCases := []struct {
		scenario      string
		context       context.Context
		suiteOptions  []oteltest.SuiteOption
		driverOptions []otelsql.DriverOption
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectPing()
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
						otelsql.TracePing(),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					err = db.PingContext(tc.context)
					require.NoError(t, err)
				})
		})
	}
}

func Test_Ping_Error(t *testing.T) {
	t.Parallel()

	const pingErr testError = "ping error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectPing().
				WillReturnError(pingErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.TracePing(), otelsql.AllowRoot())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			err = db.PingContext(context.Background())

			assert.Equal(t, pingErr, err)
		})
}

func Test_ExecContext(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedExecMetricOK()
	expectedTraceNoQueryWithoutParent := expectedExecTraceNoQuery(noParentSpanIDs())
	expectedTraceNoQueryWithSampleParent := expectedExecTraceNoQuery(sampleParentSpanIDs())
	expectedTraceWithQuery := expectedExecTraceWithQuery(sampleParentSpanIDs())
	expectedTraceWithQueryArgs := expectedExecTraceWithQueryArgs(sampleParentSpanIDs())

	testCases := []struct {
		scenario        string
		context         context.Context
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithSampleParent),
			},
		},
		{
			scenario: "with parent and trace query without args",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithQuery),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceQueryWithoutArgs(),
			},
		},
		{
			scenario: "with parent and trace query and args",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithQueryArgs),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceQueryWithArgs(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectExec(`DELETE FROM data WHERE country = $1`).
						WithArgs("US").
						WillReturnResult(sqlmock.NewResult(0, 10))
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					result, err := db.ExecContext(tc.context, `DELETE FROM data WHERE country = $1`, "US")

					require.NoError(t, err)

					affectedRows, err := result.RowsAffected()

					require.Equal(t, int64(10), affectedRows)
					require.NoError(t, err)
				})
		})
	}
}

func Test_ExecContext_TraceRowsAffected(t *testing.T) {
	t.Parallel()

	expectedTraceWithoutParent := expectedExecTraceWithAffectedRows(noParentSpanIDs())
	expectedTraceWithSampleParent := expectedExecTraceWithAffectedRows(sampleParentSpanIDs())

	testCases := []struct {
		scenario      string
		context       context.Context
		suiteOptions  []oteltest.SuiteOption
		driverOptions []otelsql.DriverOption
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
				oteltest.TracesMatch(assertFirstSpanIsRoot),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
				oteltest.TracesMatch(assertSpansHaveSameRoot),
			},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
				oteltest.TracesMatch(assertSpansHaveSameRoot),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectExec(`DELETE FROM data WHERE country = $1`).
						WithArgs("US").
						WillReturnResult(sqlmock.NewResult(0, 10))
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
						otelsql.TraceRowsAffected(),
					)

					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					result, err := db.ExecContext(tc.context, `DELETE FROM data WHERE country = $1`, "US")

					require.NoError(t, err)

					affectedRows, err := result.RowsAffected()

					require.Equal(t, int64(10), affectedRows)
					require.NoError(t, err)
				})
		})
	}
}

func Test_ExecContext_TraceLastInsertID(t *testing.T) {
	t.Parallel()

	expectedTraceWithoutParent := expectedExecTraceWithLastInsertID(noParentSpanIDs())
	expectedTraceWithSampleParent := expectedExecTraceWithLastInsertID(sampleParentSpanIDs())

	testCases := []struct {
		scenario      string
		context       context.Context
		suiteOptions  []oteltest.SuiteOption
		driverOptions []otelsql.DriverOption
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertFirstSpanIsRoot),
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectExec(`INSERT INTO data VALUES ($1)`).
						WithArgs("US").
						WillReturnResult(sqlmock.NewResult(1, 0))
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
						otelsql.TraceLastInsertID(),
					)

					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					result, err := db.ExecContext(tc.context, `INSERT INTO data VALUES ($1)`, "US")

					require.NoError(t, err)

					id, err := result.LastInsertId()

					require.Equal(t, int64(1), id)
					require.NoError(t, err)
				})
		})
	}
}

func Test_ExecContext_Error(t *testing.T) {
	t.Parallel()

	const execErr testError = "exec error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectExec(`INSERT INTO data VALUES ($1)`).
				WithArgs("US").
				WillReturnError(execErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot(), otelsql.TraceLastInsertID())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			result, err := db.ExecContext(context.Background(), `INSERT INTO data VALUES ($1)`, "US")

			assert.Nil(t, result)
			assert.Equal(t, execErr, err)
		})
}

func Test_QueryContext(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedQueryMetricOK()
	expectedTraceNoQueryWithoutParent := expectedQueryTraceNoQuery(noParentSpanIDs())
	expectedTraceNoQueryWithSampleParent := expectedQueryTraceNoQuery(sampleParentSpanIDs())
	expectedTraceWithQuery := expectedQueryTraceWithQuery(sampleParentSpanIDs())
	expectedTraceWithQueryArgs := expectedQueryTraceWithQueryArgs(sampleParentSpanIDs())

	testCases := []struct {
		scenario        string
		context         context.Context
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithSampleParent),
			},
		},
		{
			scenario: "with parent and trace query without args",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithQuery),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceQueryWithoutArgs(),
			},
		},
		{
			scenario: "with parent and trace query and args",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithQueryArgs),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceQueryWithArgs(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectQuery(`SELECT * FROM data WHERE country = $1`).
						WithArgs("US").
						WillReturnRows(
							sqlmock.NewRows([]string{"country", "name"}).
								AddRow("US", "John"),
						)
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					result, err := db.QueryContext(tc.context, `SELECT * FROM data WHERE country = $1`, "US")

					require.NotNil(t, result)
					require.NoError(t, err)

					defer result.Close() // nolint: errcheck
				})
		})
	}
}

func Test_QueryContext_TraceRows(t *testing.T) {
	t.Parallel()

	expectedTraceRowsCloseWithoutParent := expectedQueryTraceWithRowsClose(noParentSpanIDs())
	expectedTraceRowsCloseWithSampleParent := expectedQueryTraceWithRowsClose(sampleParentSpanIDs())
	expectedTraceRowsNextWithoutParent := expectedQueryTraceWithRowsNext(noParentSpanIDs())
	expectedTraceRowsNextWithSampleParent := expectedQueryTraceWithRowsNext(sampleParentSpanIDs())
	expectedTraceRowsNextAndCloseWithoutParent := expectedQueryTraceWithRowsNextAndClose(noParentSpanIDs())
	expectedTraceRowsNextAndCloseWithSampleParent := expectedQueryTraceWithRowsNextAndClose(sampleParentSpanIDs())

	testCases := []struct {
		scenario        string
		context         context.Context
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		// ----------------------------------------
		// Only Rows Next
		// ----------------------------------------
		{
			scenario: "rows next / no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsNext(),
			},
		},
		{
			scenario: "rows next / no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertFirstSpanIsRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsNext(),
			},
		},
		{
			scenario: "rows next / with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsNext(),
			},
		},
		{
			scenario: "rows next / with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsNext(),
			},
		},
		// ----------------------------------------
		// Only Rows Close
		// ----------------------------------------
		{
			scenario: "rows close / no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertFirstSpanIsRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsCloseWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsCloseWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsCloseWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsClose(),
			},
		},
		// ----------------------------------------
		// Rows Next + Rows Close
		// ----------------------------------------
		{
			scenario: "rows close / no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsNext(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertFirstSpanIsRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextAndCloseWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsNext(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextAndCloseWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsNext(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextAndCloseWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsNext(),
				otelsql.TraceRowsClose(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectQuery(`SELECT * FROM data WHERE country = $1`).
						WithArgs("US").
						WillReturnRows(
							sqlmock.NewRows([]string{"country", "name"}).
								AddRow("US", "John"),
						)
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					rows, err := db.QueryContext(tc.context, `SELECT * FROM data WHERE country = $1`, "US")

					require.NotNil(t, rows)
					require.NoError(t, err)

					defer rows.Close() // nolint: errcheck

					actual := make([]dataRow, 0)

					for rows.Next() {
						r := dataRow{}

						err := rows.Scan(&r.Country, &r.Name)
						require.NoError(t, err)

						actual = append(actual, r)
					}

					expected := []dataRow{
						{Country: "US", Name: "John"},
					}

					assert.Equal(t, expected, actual)
				})
		})
	}
}

func Test_QueryContext_Error(t *testing.T) {
	t.Parallel()

	const queryErr testError = "query error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`SELECT * FROM data WHERE country = $1`).
				WithArgs("US").
				WillReturnError(queryErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot(), otelsql.TraceRowsClose())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			result, err := db.QueryContext(context.Background(), `SELECT * FROM data WHERE country = $1`, "US")

			assert.Nil(t, result)
			assert.Equal(t, queryErr, err)
		})
}

func Test_Begin_Error(t *testing.T) {
	t.Parallel()

	const beginErr testError = "begin error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectBegin().
				WillReturnError(beginErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			tx, err := db.Begin()

			assert.Nil(t, tx)
			assert.Equal(t, beginErr, err)
		})
}

func Test_Begin_Commit(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedBeginCommitMetricOK()
	expectedTraceWithoutParent := expectedBeginCommitTrace(noParentSpanIDs())

	testCases := []struct {
		scenario        string
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		{
			scenario: "no parent and not allow root",
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesMatch(assertFirstSpanIsRoot),
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectCommit()
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					tx, err := db.Begin()

					require.NotNil(t, tx)
					require.NoError(t, err)

					err = tx.Commit()

					require.NoError(t, err)
				})
		})
	}
}

func Test_Begin_Commit_Error(t *testing.T) {
	t.Parallel()

	const commitErr testError = "commit error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectCommit().
				WillReturnError(commitErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			tx, err := db.Begin()

			require.NotNil(t, tx)
			require.NoError(t, err)

			err = tx.Commit()

			require.Equal(t, commitErr, err)
		})
}

func Test_Begin_Rollback(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedBeginRollbackMetricOK()
	expectedTraceWithoutParent := expectedBeginRollbackTrace(noParentSpanIDs())

	testCases := []struct {
		scenario        string
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		{
			scenario: "no parent and not allow root",
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesMatch(assertFirstSpanIsRoot),
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectRollback()
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					tx, err := db.Begin()

					require.NotNil(t, tx)
					require.NoError(t, err)

					err = tx.Rollback()

					require.NoError(t, err)
				})
		})
	}
}

func Test_Begin_Rollback_Error(t *testing.T) {
	t.Parallel()

	const rollbackErr testError = "rollback error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectRollback().
				WillReturnError(rollbackErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			tx, err := db.Begin()

			require.NotNil(t, tx)
			require.NoError(t, err)

			err = tx.Rollback()

			require.Equal(t, rollbackErr, err)
		})
}

func Test_BeginTx_Commit(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedBeginCommitMetricOK()
	expectedTraceWithoutParent := expectedBeginCommitTrace(noParentSpanIDs())
	expectedTraceWithSampleParent := expectedBeginCommitTrace(sampleParentSpanIDs())

	testCases := []struct {
		scenario        string
		context         context.Context
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesMatch(assertFirstSpanIsRoot),
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectCommit()
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					tx, err := db.BeginTx(tc.context, &sql.TxOptions{})

					require.NotNil(t, tx)
					require.NoError(t, err)

					err = tx.Commit()

					require.NoError(t, err)
				})
		})
	}
}

func Test_BeginTx_Commit_Error(t *testing.T) {
	t.Parallel()

	const commitErr testError = "commit error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectCommit().
				WillReturnError(commitErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})

			require.NotNil(t, tx)
			require.NoError(t, err)

			err = tx.Commit()

			require.Equal(t, commitErr, err)
		})
}

func Test_BeginTx_Rollback(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedBeginRollbackMetricOK()
	expectedTraceWithoutParent := expectedBeginRollbackTrace(noParentSpanIDs())
	expectedTraceWithSampleParent := expectedBeginRollbackTrace(sampleParentSpanIDs())

	testCases := []struct {
		scenario        string
		context         context.Context
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesMatch(assertFirstSpanIsRoot),
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					m.ExpectBegin()
					m.ExpectRollback()
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					tx, err := db.BeginTx(tc.context, &sql.TxOptions{})

					require.NotNil(t, tx)
					require.NoError(t, err)

					err = tx.Rollback()

					require.NoError(t, err)
				})
		})
	}
}

func Test_BeginTx_Rollback_Error(t *testing.T) {
	t.Parallel()

	const rollbackErr testError = "rollback error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectRollback().
				WillReturnError(rollbackErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})

			require.NotNil(t, tx)
			require.NoError(t, err)

			err = tx.Rollback()

			require.Equal(t, rollbackErr, err)
		})
}

func Test_PrepareContext_Error(t *testing.T) {
	t.Parallel()

	const prepareError testError = "prepare error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectPrepare(`DELETE FROM data WHERE country = $1`).
				WillReturnError(prepareError)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			stmt, err := db.PrepareContext(context.Background(), `DELETE FROM data WHERE country = $1`)

			require.Nil(t, stmt)
			assert.Equal(t, prepareError, err)
		})
}

func Test_PrepareContext_ExecContext(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedPrepareContextExecContextMetricOK()
	expectedTraceNoQueryWithoutParent := expectedPrepareContextExecContextTraceNoQuery(noParentSpanIDs())
	expectedTraceNoQueryWithSampleParent := expectedPrepareContextExecContextTraceNoQuery(sampleParentSpanIDs())
	expectedTraceWithQuery := expectedPrepareContextExecContextTraceWithQuery(sampleParentSpanIDs())
	expectedTraceWithQueryArgs := expectedPrepareContextExecContextTraceWithQueryArgs(sampleParentSpanIDs())

	testCases := []struct {
		scenario        string
		context         context.Context
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithSampleParent),
			},
		},
		{
			scenario: "with parent and trace query without args",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithQuery),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceQueryWithoutArgs(),
			},
		},
		{
			scenario: "with parent and trace query and args",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithQueryArgs),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceQueryWithArgs(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					stmt := m.ExpectPrepare(`DELETE FROM data WHERE country = $1`).
						WillBeClosed()

					stmt.ExpectExec().
						WithArgs("US").
						WillReturnResult(sqlmock.NewResult(0, 10))
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					stmt, err := db.PrepareContext(tc.context, `DELETE FROM data WHERE country = $1`)

					require.NotNil(t, stmt)
					require.NoError(t, err)

					defer stmt.Close() // nolint: errcheck

					result, err := stmt.ExecContext(tc.context, "US")

					require.NoError(t, err)

					affectedRows, err := result.RowsAffected()

					require.Equal(t, int64(10), affectedRows)
					require.NoError(t, err)
				})
		})
	}
}

func Test_PrepareContext_ExecContext_Error(t *testing.T) {
	t.Parallel()

	const execErr testError = "exec error"

	oteltest.New(
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			stmt := m.ExpectPrepare(`DELETE FROM data WHERE country = $1`).
				WillBeClosed()

			stmt.ExpectExec().
				WithArgs("US").
				WillReturnError(execErr)
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(), otelsql.AllowRoot(), otelsql.TraceLastInsertID())
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			stmt, err := db.PrepareContext(context.Background(), `DELETE FROM data WHERE country = $1`)

			require.NotNil(t, stmt)
			require.NoError(t, err)

			defer stmt.Close() // nolint: errcheck

			result, err := stmt.ExecContext(context.Background(), "US")

			assert.Nil(t, result)
			assert.Equal(t, execErr, err)
		})
}

func Test_PrepareContext_ExecContext_TraceRowsAffected(t *testing.T) {
	t.Parallel()

	expectedTraceWithoutParent := expectedPrepareContextExecContextTraceWithAffectedRows(noParentSpanIDs())
	expectedTraceWithSampleParent := expectedPrepareContextExecContextTraceWithAffectedRows(sampleParentSpanIDs())

	testCases := []struct {
		scenario      string
		context       context.Context
		suiteOptions  []oteltest.SuiteOption
		driverOptions []otelsql.DriverOption
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
				oteltest.TracesMatch(assertFirstSpanIsRootAndSecondSpanIsAnotherRoot),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
				oteltest.TracesMatch(assertSpansHaveSameRoot),
			},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
				oteltest.TracesMatch(assertSpansHaveSameRoot),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					stmt := m.ExpectPrepare(`DELETE FROM data WHERE country = $1`).
						WillBeClosed()

					stmt.ExpectExec().
						WithArgs("US").
						WillReturnResult(sqlmock.NewResult(0, 10))
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
						otelsql.TraceRowsAffected(),
					)

					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					stmt, err := db.PrepareContext(tc.context, `DELETE FROM data WHERE country = $1`)

					require.NotNil(t, stmt)
					require.NoError(t, err)

					defer stmt.Close() // nolint: errcheck

					result, err := stmt.ExecContext(tc.context, "US")

					require.NoError(t, err)

					affectedRows, err := result.RowsAffected()

					require.Equal(t, int64(10), affectedRows)
					require.NoError(t, err)
				})
		})
	}
}

func Test_PrepareContext_ExecContext_TraceLastInsertID(t *testing.T) {
	t.Parallel()

	expectedTraceWithoutParent := expectedPrepareContextExecContextTraceWithLastInsertID(noParentSpanIDs())
	expectedTraceWithSampleParent := expectedPrepareContextExecContextTraceWithLastInsertID(sampleParentSpanIDs())

	testCases := []struct {
		scenario      string
		context       context.Context
		suiteOptions  []oteltest.SuiteOption
		driverOptions []otelsql.DriverOption
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertFirstSpanIsRootAndSecondSpanIsAnotherRoot),
				oteltest.TracesEqualJSON(expectedTraceWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					stmt := m.ExpectPrepare(`INSERT INTO data VALUES ($1)`).
						WillBeClosed()

					stmt.ExpectExec().
						WithArgs("US").
						WillReturnResult(sqlmock.NewResult(1, 0))
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
						otelsql.TraceLastInsertID(),
					)

					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					stmt, err := db.PrepareContext(tc.context, `INSERT INTO data VALUES ($1)`)

					require.NotNil(t, stmt)
					require.NoError(t, err)

					defer stmt.Close() // nolint: errcheck

					result, err := stmt.ExecContext(tc.context, "US")

					require.NoError(t, err)

					id, err := result.LastInsertId()

					require.Equal(t, int64(1), id)
					require.NoError(t, err)
				})
		})
	}
}

func Test_PrepareContext_QueryContext(t *testing.T) {
	t.Parallel()

	expectedMetrics := expectedPrepareContextQueryContextMetricOK()
	expectedTraceNoQueryWithoutParent := expectedPrepareContextQueryContextTraceNoQuery(noParentSpanIDs())
	expectedTraceNoQueryWithSampleParent := expectedPrepareContextQueryContextTraceNoQuery(sampleParentSpanIDs())
	expectedTraceWithQuery := expectedPrepareContextQueryContextTraceWithQuery(sampleParentSpanIDs())
	expectedTraceWithQueryArgs := expectedPrepareContextQueryContextTraceWithQueryArgs(sampleParentSpanIDs())

	testCases := []struct {
		scenario        string
		context         context.Context
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		{
			scenario: "no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEmpty(),
			},
		},
		{
			scenario: "no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{otelsql.AllowRoot()},
		},
		{
			scenario: "with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceNoQueryWithSampleParent),
			},
		},
		{
			scenario: "with parent and trace query without args",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithQuery),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceQueryWithoutArgs(),
			},
		},
		{
			scenario: "with parent and trace query and args",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.MetricsEqualJSON(expectedMetrics),
				oteltest.TracesEqualJSON(expectedTraceWithQueryArgs),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceQueryWithArgs(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					stmt := m.ExpectPrepare(`SELECT * FROM data WHERE country = $1`).
						WillBeClosed()

					stmt.ExpectQuery().
						WithArgs("US").
						WillReturnRows(
							sqlmock.NewRows([]string{"country", "name"}).
								AddRow("US", "John"),
						)
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					stmt, err := db.PrepareContext(tc.context, `SELECT * FROM data WHERE country = $1`)

					require.NotNil(t, stmt)
					require.NoError(t, err)

					defer stmt.Close() // nolint: errcheck

					result, err := stmt.QueryContext(tc.context, "US")

					require.NotNil(t, result)
					require.NoError(t, err)

					defer result.Close() // nolint: errcheck
				})
		})
	}
}

func Test_PrepareContext_QueryContext_TraceRows(t *testing.T) {
	t.Parallel()

	expectedTraceRowsCloseWithoutParent := expectedPrepareContextQueryContextTraceWithRowsClose(noParentSpanIDs())
	expectedTraceRowsCloseWithSampleParent := expectedPrepareContextQueryContextTraceWithRowsClose(sampleParentSpanIDs())
	expectedTraceRowsNextWithoutParent := expectedPrepareContextQueryContextTraceWithRowsNext(noParentSpanIDs())
	expectedTraceRowsNextWithSampleParent := expectedPrepareContextQueryContextTraceWithRowsNext(sampleParentSpanIDs())
	expectedTraceRowsNextAndCloseWithoutParent := expectedPrepareContextQueryContextTraceWithRowsNextAndClose(noParentSpanIDs())
	expectedTraceRowsNextAndCloseWithSampleParent := expectedPrepareContextQueryContextTraceWithRowsNextAndClose(sampleParentSpanIDs())

	testCases := []struct {
		scenario        string
		context         context.Context
		suiteOptions    []oteltest.SuiteOption
		driverOptions   []otelsql.DriverOption
		expectedMetrics string
		expectedTraces  string
	}{
		// ----------------------------------------
		// Only Rows Next
		// ----------------------------------------
		{
			scenario: "rows next / no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsNext(),
			},
		},
		{
			scenario: "rows next / no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertFirstSpanIsRootAndSecondSpanIsAnotherRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsNext(),
			},
		},
		{
			scenario: "rows next / with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsNext(),
			},
		},
		{
			scenario: "rows next / with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsNext(),
			},
		},
		// ----------------------------------------
		// Only Rows Close
		// ----------------------------------------
		{
			scenario: "rows close / no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertFirstSpanIsRootAndSecondSpanIsAnotherRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsCloseWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsCloseWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsCloseWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsClose(),
			},
		},
		// ----------------------------------------
		// Rows Next + Rows Close
		// ----------------------------------------
		{
			scenario: "rows close / no parent and not allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesEmpty(),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsNext(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / no parent and allow root",
			context:  context.Background(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertFirstSpanIsRootAndSecondSpanIsAnotherRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextAndCloseWithoutParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsNext(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / with parent and allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextAndCloseWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.AllowRoot(),
				otelsql.TraceRowsNext(),
				otelsql.TraceRowsClose(),
			},
		},
		{
			scenario: "rows close / with parent and not allow root",
			context:  contextWithSampleSpan(),
			suiteOptions: []oteltest.SuiteOption{
				oteltest.TracesMatch(assertSpansHaveSameRoot),
				oteltest.TracesEqualJSON(expectedTraceRowsNextAndCloseWithSampleParent),
			},
			driverOptions: []otelsql.DriverOption{
				otelsql.TraceRowsNext(),
				otelsql.TraceRowsClose(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			tc.suiteOptions = append(tc.suiteOptions,
				oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
					stmt := m.ExpectPrepare(`SELECT * FROM data WHERE country = $1`).
						WillBeClosed()

					stmt.ExpectQuery().
						WithArgs("US").
						WillReturnRows(
							sqlmock.NewRows([]string{"country", "name"}).
								AddRow("US", "John"),
						)
				}),
			)

			oteltest.New(tc.suiteOptions...).
				Run(t, func(sc oteltest.SuiteContext) {
					tc.driverOptions = append(tc.driverOptions,
						otelsql.WithMeterProvider(sc.MeterProvider()),
						otelsql.WithTracerProvider(sc.TracerProvider()),
					)
					db, err := newDB(sc.DatabaseDSN(), tc.driverOptions...)
					require.NoError(t, err)

					defer db.Close() // nolint: errcheck

					stmt, err := db.PrepareContext(tc.context, `SELECT * FROM data WHERE country = $1`)

					require.NotNil(t, stmt)
					require.NoError(t, err)

					defer stmt.Close() // nolint: errcheck

					rows, err := stmt.QueryContext(tc.context, "US")

					require.NotNil(t, rows)
					require.NoError(t, err)

					defer rows.Close() // nolint: errcheck

					actual := make([]dataRow, 0)

					for rows.Next() {
						r := dataRow{}

						err := rows.Scan(&r.Country, &r.Name)
						require.NoError(t, err)

						actual = append(actual, r)
					}

					expected := []dataRow{
						{Country: "US", Name: "John"},
					}

					assert.Equal(t, expected, actual)
				})
		})
	}
}

func Test_Custom_Setup(t *testing.T) {
	t.Parallel()

	ctx := contextWithSampleSpan()

	expectedMetrics := expectedCustomMetricOK()
	expectedTraces := expectedCustomTrace(sampleParentSpanIDs())

	oteltest.New(
		oteltest.MetricsEqualJSON(expectedMetrics),
		oteltest.TracesEqualJSON(expectedTraces),
		oteltest.MockDatabase(func(m sqlmock.Sqlmock) {
			m.ExpectPing()
		}),
	).
		Run(t, func(sc oteltest.SuiteContext) {
			db, err := newDB(sc.DatabaseDSN(),
				otelsql.WithMeterProvider(sc.MeterProvider()),
				otelsql.WithTracerProvider(sc.TracerProvider()),
				otelsql.TracePing(),
				otelsql.WithDatabaseName("test"),
				otelsql.WithInstanceName("default"),
				otelsql.WithSystem(semconv.DBSystemPostgreSQL),
				otelsql.WithSpanNameFormatter(func(_ context.Context, op string) string {
					return fmt.Sprintf("custom:sql:%s", op)
				}),
			)
			require.NoError(t, err)

			defer db.Close() // nolint: errcheck

			err = db.PingContext(ctx)
			require.NoError(t, err)
		})
}

type driverOpenFunc func(name string) (driver.Conn, error)

func (f driverOpenFunc) Open(name string) (driver.Conn, error) {
	return f(name)
}

type driverOpenConnectorFunc func(name string) (driver.Connector, error)

func (f driverOpenConnectorFunc) OpenConnector(name string) (driver.Connector, error) {
	return f(name)
}

type driverCloseFunc func() error

func (f driverCloseFunc) Close() error {
	return f()
}

type driverConnectFunc func(context.Context) (driver.Conn, error)

func (f driverConnectFunc) Connect(ctx context.Context) (driver.Conn, error) {
	return f(ctx)
}

type driverDriverFunc func() driver.Driver

func (f driverDriverFunc) Driver() driver.Driver {
	return f()
}

type testError string

func (e testError) Error() string {
	return string(e)
}

type dataRow struct {
	Country string
	Name    string
}

func newDB(dsn string, opts ...otelsql.DriverOption) (*sql.DB, error) {
	driverName, err := otelsql.RegisterWithSource("sqlmock", dsn, opts...)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func mustNotFail(err error) {
	if err != nil {
		panic(err)
	}
}

func noParentSpanIDs() (trace.TraceID, trace.SpanID) {
	return oteltest.NilTraceID, oteltest.NilSpanID
}

func sampleParentSpanIDs() (trace.TraceID, trace.SpanID) {
	return oteltest.SampleTraceID, oteltest.SampleSpanID
}

func contextWithSampleSpan() context.Context {
	return oteltest.BackgroundWithSpanContext(sampleParentSpanIDs())
}

func assertSpanIsRoot(t assert.TestingT, span oteltest.Span, msgAndArgs ...interface{}) bool {
	return assert.Equal(t, span.Parent.TraceID, oteltest.NilTraceID.String(), msgAndArgs...) &&
		assert.Equal(t, span.Parent.SpanID, oteltest.NilSpanID.String(), msgAndArgs...)
}

func assertFirstSpanIsRoot(t assert.TestingT, actual []oteltest.Span) bool {
	if !assert.Greater(t, len(actual), 1, "expect more than 1 span") {
		return false
	}

	if !assertSpanIsRoot(t, actual[0], "first span is not root, trace id: %q, span id: %q", actual[0].Parent.TraceID, actual[0].Parent.SpanID) {
		return false
	}

	rootTraceID, rootSpanID := actual[0].SpanContext.TraceID, actual[0].SpanContext.SpanID
	result := true

	for i := 1; i < len(actual); i++ {
		traceID := actual[i].SpanContext.TraceID
		parentTraceID, parentSpanID := actual[i].Parent.TraceID, actual[i].Parent.SpanID

		result = result && assert.Equal(t, rootTraceID, traceID, "span #%d does not have the same trace id as root, expected: %q, got %q", i, rootTraceID, traceID)
		result = result && assert.Equal(t, rootTraceID, parentTraceID, "parent of span #%d does not have the same trace id as root, expected: %q, got %q", i, rootTraceID, parentTraceID)
		result = result && assert.Equal(t, rootSpanID, parentSpanID, "parent of span #%d does not have the same span id as root, expected: %q, got %q", i, rootSpanID, parentSpanID)
	}

	return result
}

func assertFirstSpanIsRootAndSecondSpanIsAnotherRoot(t assert.TestingT, actual []oteltest.Span) bool {
	if !assert.Greater(t, len(actual), 1, "expect at least 2 spans") {
		return false
	}

	// Check whether the first span is root.
	if !assertSpanIsRoot(t, actual[0], "first span is not root, trace id: %q, span id: %q", actual[0].Parent.TraceID, actual[0].Parent.SpanID) {
		return false
	}

	// Check whether the second span is root.
	if !assertSpanIsRoot(t, actual[1], "second span is not root, trace id: %q, span id: %q", actual[0].Parent.TraceID, actual[0].Parent.SpanID) {
		return false
	}

	secondRootTraceID, secondRootSpanID := actual[1].SpanContext.TraceID, actual[1].SpanContext.SpanID
	result := true

	for i := 2; i < len(actual); i++ {
		traceID := actual[i].SpanContext.TraceID
		parentTraceID, parentSpanID := actual[i].Parent.TraceID, actual[i].Parent.SpanID

		result = result && assert.Equal(t, secondRootTraceID, traceID, "span #%d does not have the same trace id as second root, expected: %q, got %q", i, secondRootTraceID, traceID)
		result = result && assert.Equal(t, secondRootTraceID, parentTraceID, "parent of span #%d does not have the same trace id as second root, expected: %q, got %q", i, secondRootTraceID, parentTraceID)
		result = result && assert.Equal(t, secondRootSpanID, parentSpanID, "parent of span #%d does not have the same span id as second root, expected: %q, got %q", i, secondRootSpanID, parentSpanID)
	}

	return result
}

func assertSpansHaveSameRoot(t assert.TestingT, actual []oteltest.Span) bool {
	if !assert.NotEmpty(t, actual, "expect at least 1 span") {
		return false
	}

	rootTraceID, rootSpanID := actual[0].Parent.TraceID, actual[0].Parent.SpanID
	result := true

	for i := 1; i < len(actual); i++ {
		parentTraceID, parentSpanID := actual[i].Parent.TraceID, actual[i].Parent.SpanID

		result = result && assert.Equal(t, rootTraceID, parentTraceID, "span #%d does not have the same parent trace id, expected: %q, got %q", i, rootTraceID, parentTraceID)
		result = result && assert.Equal(t, rootSpanID, parentSpanID, "span #%d does not have the same parent span id, expected: %q, got %q", i, rootSpanID, parentSpanID)
	}

	return result
}

func getFixture(file string, args ...interface{}) string {
	data, err := os.ReadFile(filepath.Clean(file))
	mustNotFail(err)

	return fmt.Sprintf(string(data), args...)
}

func expectedMetricsFromFile(file string, args ...interface{}) string { // nolint: unparam
	return getFixture("resources/fixtures/metrics/"+file, args...)
}

func expectedPingMetricOK() string {
	return expectedMetricsFromFile("ping_ok.json")
}

func expectedExecMetricOK() string {
	return expectedMetricsFromFile("exec_ok.json")
}

func expectedQueryMetricOK() string {
	return expectedMetricsFromFile("query_ok.json")
}

func expectedBeginCommitMetricOK() string {
	return expectedMetricsFromFile("begin_commit_ok.json")
}

func expectedBeginRollbackMetricOK() string {
	return expectedMetricsFromFile("begin_rollback_ok.json")
}

func expectedPrepareContextExecContextMetricOK() string {
	return expectedMetricsFromFile("prepare_context_exec_context_ok.json")
}

func expectedPrepareContextQueryContextMetricOK() string {
	return expectedMetricsFromFile("prepare_context_query_context_ok.json")
}

func expectedCustomMetricOK() string {
	return expectedMetricsFromFile("custom_ok.json")
}

func expectedTracesFromFile(file string, args ...interface{}) string {
	return getFixture("resources/fixtures/traces/"+file, args...)
}

func expectedPingTrace(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("ping.json", parentTraceID, parentSpanID)
}

func expectedExecTraceNoQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("exec_no_query.json", parentTraceID, parentSpanID)
}

func expectedExecTraceWithQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("exec_with_query.json", parentTraceID, parentSpanID)
}

func expectedExecTraceWithQueryArgs(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("exec_with_query_args.json", parentTraceID, parentSpanID)
}

func expectedExecTraceWithAffectedRows(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("exec_with_affected_rows.json", parentTraceID, parentSpanID)
}

func expectedExecTraceWithLastInsertID(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("exec_with_last_insert_id.json", parentTraceID, parentSpanID)
}

func expectedQueryTraceNoQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("query_no_query.json", parentTraceID, parentSpanID)
}

func expectedQueryTraceWithQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("query_with_query.json", parentTraceID, parentSpanID)
}

func expectedQueryTraceWithQueryArgs(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("query_with_query_args.json", parentTraceID, parentSpanID)
}

func expectedQueryTraceWithRowsNext(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("query_with_rows_next.json", parentTraceID, parentSpanID)
}

func expectedQueryTraceWithRowsClose(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("query_with_rows_close.json", parentTraceID, parentSpanID)
}

func expectedQueryTraceWithRowsNextAndClose(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("query_with_rows_next_close.json", parentTraceID, parentSpanID)
}

func expectedBeginCommitTrace(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("begin_commit.json", parentTraceID, parentSpanID)
}

func expectedBeginRollbackTrace(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("begin_rollback.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextExecContextTraceNoQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_exec_context_no_query.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextExecContextTraceWithQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_exec_context_with_query.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextExecContextTraceWithQueryArgs(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_exec_context_with_query_args.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextExecContextTraceWithAffectedRows(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_exec_context_with_affected_rows.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextExecContextTraceWithLastInsertID(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_exec_context_with_last_insert_id.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextQueryContextTraceNoQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_query_context_no_query.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextQueryContextTraceWithQuery(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_query_context_with_query.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextQueryContextTraceWithQueryArgs(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_query_context_with_query_args.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextQueryContextTraceWithRowsNext(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_query_context_with_rows_next.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextQueryContextTraceWithRowsClose(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_query_context_with_rows_close.json", parentTraceID, parentSpanID)
}

func expectedPrepareContextQueryContextTraceWithRowsNextAndClose(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("prepare_context_query_context_with_rows_next_close.json", parentTraceID, parentSpanID)
}

func expectedCustomTrace(parentTraceID trace.TraceID, parentSpanID trace.SpanID) string {
	return expectedTracesFromFile("custom.json", parentTraceID, parentSpanID)
}
