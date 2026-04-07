package repository

import (
	"database/sql"
	"errors"

	"magaz/internal/models"
)

type AddressRepository struct{ db *sql.DB }

func NewAddressRepository(db *sql.DB) *AddressRepository { return &AddressRepository{db: db} }

func (r *AddressRepository) List(userID int64) ([]*models.Address, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, label, city, street, zip, is_default, created_at
		 FROM addresses WHERE user_id=$1 ORDER BY is_default DESC, created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var addrs []*models.Address
	for rows.Next() {
		a := &models.Address{}
		if err := rows.Scan(&a.ID, &a.UserID, &a.Label, &a.City,
			&a.Street, &a.Zip, &a.IsDefault, &a.CreatedAt); err != nil {
			return nil, err
		}
		addrs = append(addrs, a)
	}
	return addrs, rows.Err()
}

func (r *AddressRepository) FindByIDAndUser(id, userID int64) (*models.Address, error) {
	a := &models.Address{}
	err := r.db.QueryRow(
		`SELECT id, user_id, label, city, street, zip, is_default, created_at
		 FROM addresses WHERE id=$1 AND user_id=$2`, id, userID,
	).Scan(&a.ID, &a.UserID, &a.Label, &a.City, &a.Street, &a.Zip, &a.IsDefault, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (r *AddressRepository) Create(a *models.Address) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if a.IsDefault {
		if _, err := tx.Exec(
			`UPDATE addresses SET is_default=FALSE WHERE user_id=$1`, a.UserID,
		); err != nil {
			return err
		}
	}
	err = tx.QueryRow(
		`INSERT INTO addresses (user_id, label, city, street, zip, is_default)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at`,
		a.UserID, a.Label, a.City, a.Street, a.Zip, a.IsDefault,
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AddressRepository) Delete(id, userID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM addresses WHERE id=$1 AND user_id=$2`, id, userID,
	)
	return err
}

func (r *AddressRepository) Default(userID int64) (*models.Address, error) {
	a := &models.Address{}
	err := r.db.QueryRow(
		`SELECT id, user_id, label, city, street, zip, is_default, created_at
		 FROM addresses WHERE user_id=$1 ORDER BY is_default DESC, created_at LIMIT 1`,
		userID,
	).Scan(&a.ID, &a.UserID, &a.Label, &a.City, &a.Street, &a.Zip, &a.IsDefault, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}
