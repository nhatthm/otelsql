package suite

import (
	"github.com/Masterminds/squirrel"
	"github.com/nhatthm/testcontainers-go-extra"
)

type suiteContext struct {
	containers []testcontainers.Container

	featureFiles []string

	databaseDriver            string
	databaseDSN               string
	databasePlaceholderFormat squirrel.PlaceholderFormat

	customerRepositoryConstructor CustomerRepositoryConstructor
}
