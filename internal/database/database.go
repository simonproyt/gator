package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
}

type Queries struct {
	db *sql.DB
}

func New(db *sql.DB) *Queries {
	return &Queries{db: db}
}

func (q *Queries) CreateUser(ctx context.Context, id uuid.UUID, createdAt, updatedAt time.Time, name string) (User, error) {
	var u User
	// Use RETURNING to get the inserted row
	row := q.db.QueryRowContext(ctx, `
        INSERT INTO users (id, created_at, updated_at, name)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at, updated_at, name
    `, id, createdAt, updatedAt, name)

	var idStr string
	if err := row.Scan(&idStr, &u.CreatedAt, &u.UpdatedAt, &u.Name); err != nil {
		return u, err
	}
	uid, err := uuid.Parse(idStr)
	if err != nil {
		return u, err
	}
	u.ID = uid
	return u, nil
}

func (q *Queries) GetUser(ctx context.Context, name string) (User, error) {
	var u User
	row := q.db.QueryRowContext(ctx, `
        SELECT id, created_at, updated_at, name FROM users WHERE name = $1
    `, name)
	var idStr string
	if err := row.Scan(&idStr, &u.CreatedAt, &u.UpdatedAt, &u.Name); err != nil {
		return u, err
	}
	uid, err := uuid.Parse(idStr)
	if err != nil {
		return u, err
	}
	u.ID = uid
	return u, nil
}
