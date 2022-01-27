package customer

import "time"

// Customer is a customer entity.
type Customer struct {
	ID        int64      `db:"id" json:"id"`
	Country   string     `db:"country" json:"country"`
	FirstName string     `db:"first_name" json:"first_name"`
	LastName  string     `db:"last_name" json:"last_name"`
	Email     string     `db:"email" json:"email"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`
}
