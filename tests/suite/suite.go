package suite

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/cucumber/godog"
	"github.com/docker/go-connections/nat"
	"github.com/godogx/clocksteps"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file" // Default migration source.
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

type logger func(format string, args ...interface{})

// Suite is a test suite.
type Suite interface {
	Run(tb testing.TB)
}

type suite struct {
	containerRequests []testcontainers.ContainerRequest

	migrationSource string
	migrationDSN    string

	featureFilesLocation string

	databaseDriver            string
	databaseDSN               string
	databasePlaceholderFormat squirrel.PlaceholderFormat

	customerRepositoryConstructor CustomerRepositoryConstructor
}

func (s suite) startContainer(runID string, log logger, request testcontainers.ContainerRequest) (testcontainers.Container, map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	requestedName := request.Name
	request.Name = fmt.Sprintf("otelsqltest_%s_%s", request.Name, runID)

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: request,
		Started:          true,
	})
	if err != nil {
		log("could not start container %q: %s", requestedName, err.Error())

		return nil, nil, err
	}

	ports, _, err := nat.ParsePortSpecs(request.ExposedPorts)
	if err != nil {
		log("could not parse container %q ports: %s", requestedName, err.Error())

		return c, nil, err
	}

	envs := make(map[string]string, len(ports)*2)

	if len(ports) > 0 {
		ip, err := c.Host(ctx)
		if err != nil {
			log("could not get container %q ip: %s", requestedName, err.Error())

			return c, nil, err
		}

		for p := range ports {
			publicPort, err := c.MappedPort(ctx, p)
			if err != nil {
				return c, nil, err
			}

			_ = os.Setenv(envVarName(requestedName, p.Port(), "HOST"), ip)                // nolint: errcheck
			_ = os.Setenv(envVarName(requestedName, p.Port(), "PORT"), publicPort.Port()) // nolint: errcheck
		}
	}

	return c, envs, nil
}

func (s suite) startContainers(runID string, log logger, requests ...testcontainers.ContainerRequest) ([]testcontainers.Container, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		err error
	)

	wg.Add(len(requests))

	containers := make([]testcontainers.Container, 0, len(requests))

	for _, r := range requests {
		go func(r testcontainers.ContainerRequest) {
			defer wg.Done()

			c, _, startErr := s.startContainer(runID, log, r)

			mu.Lock()
			defer mu.Unlock()

			if c != nil {
				containers = append(containers, c)
			}

			if startErr != nil {
				err = startErr
			}
		}(r)
	}

	wg.Wait()

	return containers, err
}

func (s suite) stopContainers(tb testing.TB, containers ...testcontainers.Container) {
	var wg sync.WaitGroup

	wg.Add(len(containers))

	for _, c := range containers {
		go func(c testcontainers.Container) {
			defer wg.Done()

			if err := c.Terminate(context.Background()); err != nil {
				tb.Logf("could not terminate container: %s", err.Error())
			}
		}(c)
	}

	wg.Wait()
}

func (s suite) runMigrations(source, databaseDSN string, log logger) error {
	if source == "" {
		return nil
	}

	m, err := migrate.New(source, databaseDSN)
	if err != nil {
		log("could not init migrations: %s", err.Error())

		return err
	}

	err = m.Up()
	if err != nil {
		log("could not run migrations: %s", err.Error())
	}

	return err
}

func (s suite) getFeatureFiles(location string, log func(format string, args ...interface{})) ([]string, error) {
	entries, err := os.ReadDir(location)
	if err != nil {
		log("could not read feature files location: %s", err.Error())

		return nil, err
	}

	result := make([]string, 0)

	for _, f := range entries {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".feature") {
			continue
		}

		result = append(result, filepath.Join(location, f.Name()))
	}

	return result, nil
}

func (s suite) runTests(tb testing.TB, sc suiteContext) error {
	tb.Helper()

	db, err := openDBxWithoutInstrumentation(sc.databaseDriver, sc.databaseDSN)
	if err != nil {
		tb.Logf("could not init database with sqlx: %s", err.Error())

		return err
	}

	out := bytes.NewBuffer(nil)

	clock := clocksteps.New()
	otelsqlTests := newObservabilityTests()
	customerTests := newCustomerTests(sc.databaseDriver, sc.databaseDSN, sc.customerRepositoryConstructor, clock)
	dbm := makeDBManager(db, sc.databasePlaceholderFormat)

	suite := godog.TestSuite{
		Name: "Integration",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			clock.RegisterContext(sc)
			dbm.RegisterSteps(sc)
			otelsqlTests.RegisterContext(sc)
			customerTests.RegisterContext(sc)
		},
		Options: &godog.Options{
			Format:    "pretty",
			Strict:    true,
			Output:    out,
			Randomize: time.Now().UTC().UnixNano(),
			Paths:     sc.featureFiles,
		},
	}

	// Run the suite.
	if status := suite.Run(); status != 0 {
		tb.Fatal(out.String())
	}

	return nil
}

func (s suite) start(tb testing.TB) (suiteContext, error) {
	var (
		sc  suiteContext
		err error
	)

	// Start containers.
	sc.containers, err = s.startContainers(randomString(8), tb.Logf, s.containerRequests...)
	if err != nil {
		return sc, err
	}

	// Setup.
	sc.migrationDSN = os.ExpandEnv(s.migrationDSN)
	sc.databaseDriver = s.databaseDriver
	sc.databaseDSN = os.ExpandEnv(s.databaseDSN)
	sc.databasePlaceholderFormat = s.databasePlaceholderFormat
	sc.customerRepositoryConstructor = s.customerRepositoryConstructor

	if sc.migrationDSN == "" {
		sc.migrationDSN = sc.databaseDSN
	}

	if err := s.runMigrations(s.migrationSource, sc.migrationDSN, tb.Logf); err != nil {
		return sc, err
	}

	sc.featureFiles, err = s.getFeatureFiles(s.featureFilesLocation, tb.Logf)
	if err != nil {
		return sc, err
	}

	if err := s.runTests(tb, sc); err != nil {
		return sc, err
	}

	return sc, nil
}

func (s suite) stop(tb testing.TB, sc suiteContext) suiteContext {
	defer s.stopContainers(tb, sc.containers...)

	return sc
}

func (s suite) Run(tb testing.TB) {
	tb.Helper()

	sc, err := s.start(tb)
	defer s.stop(tb, sc)

	require.NoError(tb, err)
}

// New creates a new test suite.
func New(opts ...Option) Suite {
	s := suite{
		databasePlaceholderFormat: squirrel.Question,
	}

	for _, opt := range opts {
		opt(&s)
	}

	return s
}

// Run creates a new test suite and run it.
func Run(tb testing.TB, opts ...Option) {
	tb.Helper()

	New(opts...).Run(tb)
}
