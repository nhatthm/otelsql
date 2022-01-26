package customer

import (
	"context"
)

// Repository handles user data.
type Repository interface {
	Finder
	Creator
	Updater
	Deleter
}

// Finder finds customers.
type Finder interface {
	Find(ctx context.Context, id int) (*Customer, error)
	FindAll(ctx context.Context) ([]Customer, error)
}

// Creator creates customers.
type Creator interface {
	Create(ctx context.Context, customer Customer) error
}

// Updater updates customers.
type Updater interface {
	Update(ctx context.Context, customer Customer) error
}

// Deleter deletes customers.
type Deleter interface {
	Delete(ctx context.Context, id int) error
}
