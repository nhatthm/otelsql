package postgres

import (
	"os"
	"testing"

	"github.com/Masterminds/squirrel"
	_ "github.com/jackc/pgx/v4/stdlib" // Database driver
	_ "github.com/lib/pq"              // Database driver
	"github.com/nhatthm/testcontainers-go-extra"
	pg "github.com/nhatthm/testcontainers-go-registry/database/postgres"

	"github.com/nhatthm/otelsql/tests/suite"
)

const (
	defaultVersion = "12-alpine"
	defaultDriver  = "pgx"

	databaseName     = "otelsql"
	databaseUsername = "otelsql"
	databasePassword = "OneWrapperToTraceThemAll"
)

func TestIntegration(t *testing.T) {
	suite.Run(t,
		suite.WithTestContainerRequests(
			pg.Request(databaseName, databaseUsername, databasePassword,
				pg.RunMigrations("file://./resources/migrations/"),
				testcontainers.WithImageTag(imageTag()),
			),
		),
		suite.WithDatabaseDriver(databaseDriver()),
		suite.WithDatabaseDSN(pg.DSN(databaseName, databaseUsername, databasePassword)),
		suite.WithDatabasePlaceholderFormat(squirrel.Dollar),
		suite.WithFeatureFilesLocation("../features"),
		suite.WithCustomerRepositoryConstructor(newRepository()),
	)
}

func imageTag() string {
	v := os.Getenv("POSTGRES_VERSION")
	if v == "" {
		return defaultVersion
	}

	return v
}

func databaseDriver() string {
	v := os.Getenv("POSTGRES_DRIVER")
	if v == "" {
		return defaultDriver
	}

	return v
}
