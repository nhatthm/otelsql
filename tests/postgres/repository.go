package postgres

import (
	"context"

	"go.nhat.io/clock"

	"github.com/nhatthm/otelsql/tests/suite"
	"github.com/nhatthm/otelsql/tests/suite/customer"
)

type repository struct {
	db    suite.DatabaseExecer
	clock clock.Clock
}

func (r *repository) Find(ctx context.Context, id int) (*customer.Customer, error) {
	row, err := r.db.QueryRow(ctx,
		"SELECT * FROM customer WHERE id = $1 LIMIT 1",
		id,
	)
	if err != nil {
		return nil, err
	}

	c := &customer.Customer{}

	if err := row.Scan(&c.ID, &c.Country, &c.FirstName, &c.LastName, &c.Email, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *repository) FindAll(ctx context.Context) ([]customer.Customer, error) {
	rows, err := r.db.Query(ctx, "SELECT * FROM customer")
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = rows.Close() // nolint: errcheck
		_ = rows.Err()   // nolint: errcheck
	}()

	customers := make([]customer.Customer, 0)

	for rows.Next() {
		c := customer.Customer{}

		if err := rows.Scan(&c.ID, &c.Country, &c.FirstName, &c.LastName, &c.Email, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}

		customers = append(customers, c)
	}

	return customers, nil
}

func (r *repository) Create(ctx context.Context, user customer.Customer) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO customer VALUES ($1, $2, $3, $4, $5, $6, $7)",
		user.ID,
		user.Country,
		user.FirstName,
		user.LastName,
		user.Email,
		user.CreatedAt,
		user.UpdatedAt,
	)

	return err
}

func (r *repository) Update(ctx context.Context, user customer.Customer) error {
	_, err := r.db.Exec(ctx,
		"UPDATE customer SET country = $1, first_name = $2, last_name = $3, email = $4, created_at = $5, updated_at = $6 WHERE id = $7",
		user.Country,
		user.FirstName,
		user.LastName,
		user.Email,
		user.CreatedAt,
		r.clock.Now(),
		user.ID,
	)

	return err
}

func (r *repository) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM customer WHERE id = $1",
		id,
	)

	return err
}

func newRepository() suite.CustomerRepositoryConstructor {
	return func(db suite.DatabaseExecer, c clock.Clock) customer.Repository {
		return &repository{
			db:    db,
			clock: c,
		}
	}
}
