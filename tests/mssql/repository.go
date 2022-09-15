package mssql

import (
	"context"

	"go.nhat.io/clock"

	"go.nhat.io/otelsql/tests/suite"
	"go.nhat.io/otelsql/tests/suite/customer"
)

type repository struct {
	db    suite.DatabaseExecer
	clock clock.Clock
}

func (r *repository) Find(ctx context.Context, id int) (*customer.Customer, error) {
	row, err := r.db.QueryRow(ctx,
		"SELECT TOP 1 * FROM customer WHERE id = @p1",
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
		"INSERT INTO customer VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7)",
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
		"UPDATE customer SET country = @p1, first_name = @p2, last_name = @p3, email = @p4, created_at = @p5, updated_at = @p6 WHERE id = @p7",
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
		"DELETE FROM customer WHERE id = @p1",
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
