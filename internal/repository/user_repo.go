package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"magaz/internal/models"
)

var ErrNotFound = errors.New("record not found")

type UserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(u *models.User) error {
	return r.db.QueryRow(
		`INSERT INTO users (email, password_hash, name, role)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		u.Email, u.PasswordHash, u.Name, u.Role,
	).Scan(&u.ID, &u.CreatedAt)
}

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, email, password_hash, name, role,
		        reset_token, reset_expires, created_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.ResetToken, &u.ResetExpires, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepository) FindByID(id int64) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, email, password_hash, name, role,
		        reset_token, reset_expires, created_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.ResetToken, &u.ResetExpires, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepository) FindByResetToken(token string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, email, password_hash, name, role,
		        reset_token, reset_expires, created_at
		 FROM users WHERE reset_token = $1 AND reset_expires > NOW()`, token,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role,
		&u.ResetToken, &u.ResetExpires, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepository) SetResetToken(id int64, token string, expires time.Time) error {
	_, err := r.db.Exec(
		`UPDATE users SET reset_token = $1, reset_expires = $2 WHERE id = $3`,
		token, expires, id,
	)
	return err
}

func (r *UserRepository) UpdatePassword(id int64, hash string) error {
	_, err := r.db.Exec(
		`UPDATE users SET password_hash = $1, reset_token = NULL, reset_expires = NULL
		 WHERE id = $2`,
		hash, id,
	)
	return err
}

func (r *UserRepository) UpdateName(id int64, name string) error {
	_, err := r.db.Exec(`UPDATE users SET name = $1 WHERE id = $2`, name, id)
	return err
}

func (r *UserRepository) List(limit, offset int) ([]*models.User, int, error) {
	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(
		`SELECT id, email, name, role, created_at FROM users
		 ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}
