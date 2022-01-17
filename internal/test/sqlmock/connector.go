package sqlmock

import (
	"context"
	"database/sql/driver"

	"github.com/DATA-DOG/go-sqlmock"
)

// DriverContext creates a new driver.DriverContext.
func DriverContext(mocks ...func(Sqlmock)) driver.DriverContext {
	_, m, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
		sqlmock.MonitorPingsOption(true),
	)

	var lazyInit driver.Driver

	drv := struct {
		driver.Driver
		driver.DriverContext
	}{
		DriverContext: openConnectorFunc(func(name string) (driver.Connector, error) {
			if err != nil {
				return nil, err
			}

			return struct {
				driverFunc
				connectFunc
			}{
				connectFunc: func(ctx context.Context) (driver.Conn, error) {
					for _, mock := range mocks {
						mock(m)
					}

					return m.(driver.Conn), nil
				},
				driverFunc: func() driver.Driver {
					return lazyInit
				},
			}, nil
		}),
	}

	lazyInit = drv

	return drv
}

type openConnectorFunc func(name string) (driver.Connector, error)

func (f openConnectorFunc) OpenConnector(name string) (driver.Connector, error) {
	return f(name)
}

type connectFunc func(context.Context) (driver.Conn, error)

func (f connectFunc) Connect(ctx context.Context) (driver.Conn, error) {
	return f(ctx)
}

type driverFunc func() driver.Driver

func (f driverFunc) Driver() driver.Driver {
	return f()
}
