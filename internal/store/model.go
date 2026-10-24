package store

import "time"

type Store struct {
	ID        uint      `db:"id"`
	Code      string    `db:"code"`
	Name      string    `db:"name"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
}
