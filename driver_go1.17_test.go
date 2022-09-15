//go:build go1.17
// +build go1.17

package otelsql_test

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.nhat.io/otelsql"
)

func TestRegister_OpenError(t *testing.T) {
	t.Parallel()

	sql.Register("open-error", struct {
		driver.Driver
		driver.DriverContext
	}{
		DriverContext: driverOpenConnectorFunc(func(name string) (driver.Connector, error) {
			return struct {
				driverDriverFunc
				driverConnectFunc
				driverCloseFunc
			}{
				driverDriverFunc: func() driver.Driver {
					return nil
				},
				driverCloseFunc: func() error {
					return errors.New("close error")
				},
			}, nil
		}),
	})

	driverName, err := otelsql.Register("open-error")

	assert.Empty(t, driverName)

	expected := errors.New("close error")

	assert.Equal(t, expected, err)
}
