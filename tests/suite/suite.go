package suite

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/cucumber/godog"
	"github.com/godogx/clocksteps"
	"github.com/stretchr/testify/require"
	"go.nhat.io/testcontainers-extra"
	_ "go.nhat.io/testcontainers-registry" // Let dependabot manage the update.
)

type logger func(format string, args ...interface{})

// Suite is a test suite.
type Suite interface {
	Run(tb testing.TB)
}

type suite struct {
	containerRequests []testcontainers.StartGenericContainerRequest

	featureFilesLocation string

	databaseDriver            string
	databaseDSN               string
	databasePlaceholderFormat squirrel.PlaceholderFormat

	customerRepositoryConstructor CustomerRepositoryConstructor
}

func (s suite) startContainers(runID string, log logger, requests ...testcontainers.StartGenericContainerRequest) ([]testcontainers.Container, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	for i := range requests {
		requests[i].Options = append(requests[i].Options,
			testcontainers.WithNamePrefix("otelsql"),
			testcontainers.WithNameSuffix(runID),
		)
	}

	containers, err := testcontainers.StartGenericContainers(context.Background(), requests...)
	if err != nil {
		log(err.Error())
	}

	return containers, err
}

func (s suite) stopContainers(tb testing.TB, containers ...testcontainers.Container) {
	tb.Helper()

	if err := testcontainers.StopGenericContainers(context.Background(), containers...); err != nil {
		tb.Log(err.Error())
	}
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
	tb.Helper()

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
	sc.databaseDriver = s.databaseDriver
	sc.databaseDSN = os.ExpandEnv(s.databaseDSN)
	sc.databasePlaceholderFormat = s.databasePlaceholderFormat
	sc.customerRepositoryConstructor = s.customerRepositoryConstructor

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
	tb.Helper()

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
