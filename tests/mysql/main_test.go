package mysql

import (
	"os"
	"testing"

	"github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql" // Database driver
	"go.nhat.io/testcontainers-extra"
	"go.nhat.io/testcontainers-registry/mysql"

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

func TestIntegration(t *testing.T) {
	suite.Run(t,
		suite.WithTestContainerRequests(
			mysql.Request(databaseName, databaseUsername, databasePassword,
				mysql.RunMigrations("file://./resources/migrations/"),
				testcontainers.WithImageName(imageName()),
				testcontainers.WithImageTag(imageTag()),
			),
		),
		suite.WithDatabaseDriver(defaultDriver),
		suite.WithDatabaseDSN(mysql.DSN(databaseName, databaseUsername, databasePassword)),
		suite.WithDatabasePlaceholderFormat(squirrel.Question),
		suite.WithFeatureFilesLocation("../features"),
		suite.WithCustomerRepositoryConstructor(newRepository()),
	)
}

func imageTag() string {
	v := os.Getenv("MYSQL_VERSION")
	if v == "" {
		return defaultVersion
	}

	return v
}

func imageName() string {
	img := os.Getenv("MYSQL_DIST")
	if img == "" {
		return defaultImage
	}

	return img
}
