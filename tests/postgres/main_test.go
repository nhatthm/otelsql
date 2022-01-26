package postgres

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/jackc/pgx/v4/stdlib" // Database driver
	_ "github.com/lib/pq"              // Database driver
	"github.com/nhatthm/otelsql/tests/suite"
	"github.com/testcontainers/testcontainers-go"
)

const (
	defaultVersion = "12-alpine"
	defaultDriver  = "pgx"

	databaseImage    = "postgres"
	databaseName     = "otelsql"
	databaseUsername = "otelsql"
	databasePassword = "OneWrapperToTraceThemAll"
)

var databaseDSN = fmt.Sprintf("postgres://%s:%s@$POSTGRES_5432_HOST:$POSTGRES_5432_PORT/%s?sslmode=disable&client_encoding=UTF8", databaseUsername, databasePassword, databaseName)

func TestIntegration(t *testing.T) {
	suite.Run(t,
		suite.WithTestContainerRequests(
			testcontainers.ContainerRequest{
				Name:         "postgres",
				Image:        image(),
				ExposedPorts: []string{":5432"},
				Env: map[string]string{
					"LC_ALL":            "C.UTF-8",
					"POSTGRES_DB":       databaseName,
					"POSTGRES_USER":     databaseUsername,
					"POSTGRES_PASSWORD": databasePassword,
				},
				WaitingFor: suite.WaitForCmd("pg_isready").
					WithRetries(5).
					WithExecTimeout(5 * time.Second).
					WithExecInterval(10 * time.Second),
			},
		),
		suite.WithMigrationSource("file://./resources/migrations/"),
		suite.WithDatabaseDriver(databaseDriver()),
		suite.WithDatabaseDSN(databaseDSN),
		suite.WithDatabasePlaceholderFormat(squirrel.Dollar),
		suite.WithFeatureFilesLocation("../features"),
		suite.WithCustomerRepositoryConstructor(newRepository()),
	)
}

func imageVersion() string {
	v := os.Getenv("POSTGRES_VERSION")
	if v == "" {
		return defaultVersion
	}

	return v
}

func image() string {
	return fmt.Sprintf("%s:%s", databaseImage, imageVersion())
}

func databaseDriver() string {
	v := os.Getenv("POSTGRES_DRIVER")
	if v == "" {
		return defaultDriver
	}

	return v
}
