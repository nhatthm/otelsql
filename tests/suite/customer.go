package suite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/swaggest/assertjson"
	"go.nhat.io/clock"

	"github.com/nhatthm/otelsql/tests/suite/customer"
)

// CustomerRepositoryConstructor is constructor to create a new repository using DatabaseContext.
type CustomerRepositoryConstructor func(db DatabaseExecer, c clock.Clock) customer.Repository

type customerTests struct {
	databaseDriver string
	databaseDSN    string

	db         *sql.DB
	runner     DatabaseContext
	construct  CustomerRepositoryConstructor
	repository customer.Repository
	clock      clock.Clock

	usePreparer bool
	lastResult  interface{}
	lastError   error
}

func (t *customerTests) constructRepository() {
	t.repository = t.construct(newDatabaseExecer(t.runner, t.usePreparer), t.clock)
}

func (t *customerTests) RegisterContext(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		db, err := openDB(t.databaseDriver, t.databaseDSN)
		if err != nil {
			return ctx, err
		}

		t.db = db
		t.runner = db
		t.usePreparer = false

		t.constructRepository()

		return ctx, err
	})

	sc.After(func(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
		if t.db != nil {
			_ = t.db.Close() // nolint: errcheck
		}

		return ctx, nil
	})

	sc.Step(`I do not use database preparer`, func() error {
		t.usePreparer = false

		t.constructRepository()

		return nil
	})

	sc.Step(`I use database preparer`, func() error {
		t.usePreparer = true

		t.constructRepository()

		return nil
	})

	sc.Step(`I create a new customer:`, t.createCustomer)
	sc.Step(`I update a customer by id "([^"]+)":`, t.updateCustomerByID)
	sc.Step(`I find a customer by id "([^"]+)"`, t.findCustomerByID)
	sc.Step(`I delete a customer by id "([^"]+)"`, t.deleteCustomerByID)
	sc.Step(`I find all customers`, t.findAllCustomers)

	sc.Step(`I got a customer:`, t.haveCustomer)
	sc.Step(`I got these customers:`, t.haveCustomers)
	sc.Step(`I got an error$`, t.haveAnError)
	sc.Step(`I got an error "([^"]+)"`, t.haveAnErrorWithMessage)
	sc.Step(`I got an error:`, t.haveAnErrorWithMessageInDocString)
	sc.Step(`I got no error`, t.haveNoError)

	sc.Step(`I start a new transaction`, t.startTransaction)
	sc.Step(`I commit the transaction`, t.commitTransaction)
	sc.Step(`I rollback the transaction`, t.rollbackTransaction)
}

func (t *customerTests) findCustomerByID(id int) error {
	t.lastResult, t.lastError = t.repository.Find(context.Background(), id)

	return nil
}

func (t *customerTests) findAllCustomers() error {
	t.lastResult, t.lastError = t.repository.FindAll(context.Background())

	return nil
}

func (t *customerTests) createCustomer(data *godog.DocString) error {
	c := customer.Customer{}

	if err := json.Unmarshal([]byte(data.Content), &c); err != nil {
		return fmt.Errorf("could not unmarshal customer data: %w", err)
	}

	t.lastError = t.repository.Create(context.Background(), c)
	t.lastResult = nil

	return nil
}

func (t *customerTests) updateCustomerByID(id int64, data *godog.DocString) error {
	c := customer.Customer{}

	if err := json.Unmarshal([]byte(data.Content), &c); err != nil {
		return fmt.Errorf("could not unmarshal customer data: %w", err)
	}

	c.ID = id

	t.lastError = t.repository.Update(context.Background(), c)
	t.lastResult = nil

	return nil
}

func (t *customerTests) deleteCustomerByID(id int) error {
	t.lastError = t.repository.Delete(context.Background(), id)
	t.lastResult = nil

	return nil
}

func (t *customerTests) haveCustomer(expected *godog.DocString) error {
	c, ok := t.lastResult.(*customer.Customer)
	if !ok {
		if t.lastError != nil {
			return fmt.Errorf("could not get customer: %w", t.lastError)
		}

		return fmt.Errorf("got no customer")
	}

	actual, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("could not marshal customer: %w", err)
	}

	return assertjson.FailNotEqual([]byte(expected.Content), actual)
}

func (t *customerTests) haveCustomers(expected *godog.DocString) error {
	customers, ok := t.lastResult.([]customer.Customer)
	if !ok {
		if t.lastError != nil {
			return fmt.Errorf("could not get customers: %w", t.lastError)
		}

		return fmt.Errorf("got no customer")
	}

	actual, err := json.Marshal(customers)
	if err != nil {
		return fmt.Errorf("could not marshal customers: %w", err)
	}

	return assertjson.FailNotEqual([]byte(expected.Content), actual)
}

func (t *customerTests) haveNoError() error {
	if t.lastError != nil {
		return fmt.Errorf("expect no error, got: %w", t.lastError)
	}

	return nil
}

func (t *customerTests) haveAnError() error {
	if t.lastError == nil {
		return fmt.Errorf("expect an error, got nothing")
	}

	return nil
}

func (t *customerTests) haveAnErrorWithMessageInDocString(err *godog.DocString) error {
	return t.haveAnErrorWithMessage(err.Content)
}

func (t *customerTests) haveAnErrorWithMessage(err string) error {
	if t.lastError == nil {
		return fmt.Errorf("there is no error, expected: %s", err)
	}

	if err != t.lastError.Error() {
		return fmt.Errorf("got error: %s, expected: %s", t.lastError.Error(), err) // nolint: errorlint
	}

	return nil
}

func (t *customerTests) startTransaction() error {
	tx, err := t.db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("could not start a new transaction: %w", err)
	}

	t.runner = &txContext{Tx: tx}

	t.constructRepository()

	return nil
}

func (t *customerTests) commitTransaction() error {
	tx, ok := t.runner.(*txContext)
	if !ok {
		return fmt.Errorf("no transaction to commit")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}

	t.runner = t.db

	t.constructRepository()

	return nil
}

func (t *customerTests) rollbackTransaction() error {
	tx, ok := t.runner.(*txContext)
	if !ok {
		return fmt.Errorf("no transaction to rollback")
	}

	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("could not rollback transaction: %w", err)
	}

	t.runner = t.db

	t.constructRepository()

	return nil
}

func newCustomerTests(
	databaseDriver, databaseDSN string,
	constructor CustomerRepositoryConstructor,
	clock clock.Clock,
) *customerTests {
	return &customerTests{
		databaseDriver: databaseDriver,
		databaseDSN:    databaseDSN,
		construct:      constructor,
		clock:          clock,
	}
}
