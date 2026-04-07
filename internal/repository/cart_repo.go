package repository

import (
	"database/sql"
	"errors"

	"magaz/internal/models"
)

type CartRepository struct{ db *sql.DB }

func NewCartRepository(db *sql.DB) *CartRepository { return &CartRepository{db: db} }

// Items returns cart items with joined product data for a user.
func (r *CartRepository) Items(userID int64) ([]*models.CartItem, error) {
	rows, err := r.db.Query(
		`SELECT ci.id, ci.user_id, ci.product_id, ci.quantity, ci.added_at,
		        p.id, p.name, p.description, p.price, p.stock,
		        p.image_url, p.category_id, COALESCE(c.name,'') as category_name,
		        p.is_active, p.created_at, p.updated_at
		 FROM cart_items ci
		 JOIN products p ON p.id = ci.product_id
		 LEFT JOIN categories c ON c.id = p.category_id
		 WHERE ci.user_id = $1
		 ORDER BY ci.added_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*models.CartItem
	for rows.Next() {
		ci := &models.CartItem{Product: &models.Product{}}
		if err := rows.Scan(
			&ci.ID, &ci.UserID, &ci.ProductID, &ci.Quantity, &ci.AddedAt,
			&ci.Product.ID, &ci.Product.Name, &ci.Product.Description,
			&ci.Product.Price, &ci.Product.Stock, &ci.Product.ImageURL,
			&ci.Product.CategoryID, &ci.Product.CategoryName,
			&ci.Product.IsActive, &ci.Product.CreatedAt, &ci.Product.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, ci)
	}
	return items, rows.Err()
}

func (r *CartRepository) Count(userID int64) (int, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COALESCE(SUM(quantity),0) FROM cart_items WHERE user_id=$1`, userID,
	).Scan(&n)
	return n, err
}

// Upsert adds a product or increments quantity if already in cart.
func (r *CartRepository) Upsert(userID, productID int64, qty int) error {
	_, err := r.db.Exec(
		`INSERT INTO cart_items (user_id, product_id, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, product_id)
		 DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity`,
		userID, productID, qty,
	)
	return err
}

func (r *CartRepository) Update(userID, productID int64, qty int) error {
	_, err := r.db.Exec(
		`UPDATE cart_items SET quantity=$1 WHERE user_id=$2 AND product_id=$3`,
		qty, userID, productID,
	)
	return err
}

func (r *CartRepository) Remove(userID, productID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM cart_items WHERE user_id=$1 AND product_id=$2`,
		userID, productID,
	)
	return err
}

func (r *CartRepository) Clear(tx *sql.Tx, userID int64) error {
	var err error
	if tx != nil {
		_, err = tx.Exec(`DELETE FROM cart_items WHERE user_id=$1`, userID)
	} else {
		_, err = r.db.Exec(`DELETE FROM cart_items WHERE user_id=$1`, userID)
	}
	return err
}

func (r *CartRepository) FindItem(userID, productID int64) (*models.CartItem, error) {
	ci := &models.CartItem{}
	err := r.db.QueryRow(
		`SELECT id, user_id, product_id, quantity, added_at
		 FROM cart_items WHERE user_id=$1 AND product_id=$2`,
		userID, productID,
	).Scan(&ci.ID, &ci.UserID, &ci.ProductID, &ci.Quantity, &ci.AddedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ci, err
}

func (r *CartRepository) DB() *sql.DB { return r.db }
