package suite

import (
	"github.com/Masterminds/squirrel"
	"github.com/testcontainers/testcontainers-go"
)

type suiteContext struct {
	containers                []testcontainers.Container
	featureFiles              []string
	migrationDSN              string
	databaseDriver            string
	databaseDSN               string
	databasePlaceholderFormat squirrel.PlaceholderFormat

	customerRepositoryConstructor CustomerRepositoryConstructor
}
