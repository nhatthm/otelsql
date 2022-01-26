package suite

import (
	"github.com/Masterminds/squirrel"
	"github.com/testcontainers/testcontainers-go"
)

type Option func(*suite)

// WithTestContainerRequests appends container requests.
func WithTestContainerRequests(requests ...testcontainers.ContainerRequest) Option {
	return func(s *suite) {
		s.containerRequests = requests
	}
}

// WithMigrationSource sets migration source.
func WithMigrationSource(source string) Option {
	return func(s *suite) {
		s.migrationSource = source
	}
}

// WithDatabaseDriver sets the database driver.
func WithDatabaseDriver(driver string) Option {
	return func(s *suite) {
		s.databaseDriver = driver
	}
}

// WithDatabaseDSN sets the database dsn.
func WithDatabaseDSN(dsn string) Option {
	return func(s *suite) {
		s.databaseDSN = dsn
	}
}

// WithDatabasePlaceholderFormat sets the database placeholder format.
func WithDatabasePlaceholderFormat(format squirrel.PlaceholderFormat) Option {
	return func(s *suite) {
		s.databasePlaceholderFormat = format
	}
}

// WithFeatureFilesLocation sets the feature files location.
func WithFeatureFilesLocation(loc string) Option {
	return func(s *suite) {
		s.featureFilesLocation = loc
	}
}

// WithCustomerRepositoryConstructor sets the constructor.
func WithCustomerRepositoryConstructor(c CustomerRepositoryConstructor) Option {
	return func(s *suite) {
		s.customerRepositoryConstructor = c
	}
}
