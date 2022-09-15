package mssql

import (
	"os"
	"testing"

	"github.com/Masterminds/squirrel"
	"go.nhat.io/testcontainers-extra"
	"go.nhat.io/testcontainers-registry/mssql"

	"go.nhat.io/otelsql/tests/suite"
)

const (
	defaultVersion = "2019-latest"

	databaseName     = "otelsql"
	databasePassword = "OneWrapperToTraceThemAll!"
)

func TestIntegration(t *testing.T) {
	suite.Run(t,
		suite.WithTestContainerRequests(
			mssql.Request(databaseName, databasePassword,
				mssql.RunMigrations("file://./resources/migrations/"),
				testcontainers.WithImageTag(imageTag()),
			),
		),
		suite.WithDatabaseDriver("sqlserver"),
		suite.WithDatabaseDSN(mssql.DSN(databaseName, databasePassword)),
		suite.WithDatabasePlaceholderFormat(squirrel.AtP),
		suite.WithFeatureFilesLocation("../features"),
		suite.WithCustomerRepositoryConstructor(newRepository()),
	)
}

func imageTag() string {
	v := os.Getenv("MSSQL_VERSION")
	if v == "" {
		return defaultVersion
	}

	return v
}
