package postgres

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"                      // Database driver
	_ "github.com/golang-migrate/migrate/v4/database/mysql" // Database driver
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/nhatthm/otelsql/tests/suite"
)

const (
	defaultVersion = "8"
	defaultImage   = "mysql"
	defaultDriver  = "mysql"

	databaseName     = "otelsql"
	databaseUsername = "otelsql"
	databasePassword = "OneWrapperToTraceThemAll"
)

var databaseDSN = fmt.Sprintf("%s:%s@tcp($MYSQL_3306_HOST:$MYSQL_3306_PORT)/%s?charset=utf8&parseTime=true", databaseUsername, databasePassword, databaseName)

func TestIntegration(t *testing.T) {
	suite.Run(t,
		suite.WithTestContainerRequests(
			testcontainers.ContainerRequest{
				Name:         "mysql",
				Image:        image(),
				ExposedPorts: []string{":3306"},
				Env: map[string]string{
					"LC_ALL":              "C.UTF-8",
					"MYSQL_DATABASE":      databaseName,
					"MYSQL_USER":          databaseUsername,
					"MYSQL_PASSWORD":      databasePassword,
					"MYSQL_ROOT_PASSWORD": databasePassword,
				},
				WaitingFor: wait.ForAll(
					waitForServer(),
					suite.WaitForDuration(30*time.Second),
					waitForServer(),
				).WithStartupTimeout(5 * time.Minute),
			},
		),
		suite.WithMigrationSource("file://./resources/migrations/"),
		suite.WithMigrationDSN(fmt.Sprintf("mysql://%s", databaseDSN)),
		suite.WithDatabaseDriver(defaultDriver),
		suite.WithDatabaseDSN(databaseDSN),
		suite.WithDatabasePlaceholderFormat(squirrel.Question),
		suite.WithFeatureFilesLocation("../features"),
		suite.WithCustomerRepositoryConstructor(newRepository()),
	)
}

func imageVersion() string {
	v := os.Getenv("MYSQL_VERSION")
	if v == "" {
		return defaultVersion
	}

	return v
}

func image() string {
	img := os.Getenv("MYSQL_DIST")
	if img == "" {
		return defaultImage
	}

	return fmt.Sprintf("%s:%s", img, imageVersion())
}

func waitForServer() wait.Strategy {
	return suite.WaitForCmd("mysqladmin", "ping", "-h", "localhost").
		WithRetries(12).
		WithExecTimeout(5 * time.Second).
		WithExecInterval(10 * time.Second)
}
