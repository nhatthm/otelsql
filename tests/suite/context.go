package suite

import (
	"github.com/Masterminds/squirrel"
	"go.nhat.io/testcontainers-extra"
)

type suiteContext struct {
	containers []testcontainers.Container

	featureFiles []string

	databaseDriver            string
	databaseDSN               string
	databasePlaceholderFormat squirrel.PlaceholderFormat

	customerRepositoryConstructor CustomerRepositoryConstructor
}
