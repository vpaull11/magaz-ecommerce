package payment

import (
	"database/sql"
	"errors"
	"fmt"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Save(tx *Transaction) error {
	_, err := r.db.Exec(
		`INSERT INTO transactions (id, order_id, amount, card_last4, status, message)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		tx.ID, tx.OrderID, tx.Amount, tx.CardLast4, tx.Status, tx.Message,
	)
	return err
}

func (r *Repository) FindByID(id string) (*Transaction, error) {
	tx := &Transaction{}
	err := r.db.QueryRow(
		`SELECT id, order_id, amount, card_last4, status, message, created_at
		 FROM transactions WHERE id=$1`, id,
	).Scan(&tx.ID, &tx.OrderID, &tx.Amount, &tx.CardLast4, &tx.Status, &tx.Message, &tx.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("transaction not found")
	}
	return tx, err
}
