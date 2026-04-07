package repository

import (
	"database/sql"

	"magaz/internal/models"
)

type WishlistRepository struct{ db *sql.DB }

func NewWishlistRepository(db *sql.DB) *WishlistRepository {
	return &WishlistRepository{db: db}
}

// Toggle returns true if added, false if removed
func (r *WishlistRepository) Toggle(userID, productID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM wishlists WHERE user_id=$1 AND product_id=$2)`, userID, productID).Scan(&exists)
	if err != nil {
		return false, err
	}

	if exists {
		_, err = r.db.Exec(`DELETE FROM wishlists WHERE user_id=$1 AND product_id=$2`, userID, productID)
		return false, err
	} else {
		_, err = r.db.Exec(`INSERT INTO wishlists (user_id, product_id) VALUES ($1, $2)`, userID, productID)
		return true, err
	}
}

func (r *WishlistRepository) ListUserWishlist(userID int64) ([]*models.Product, error) {
	query := productSelectSQL + `
		JOIN wishlists w ON w.product_id = p.id
		WHERE w.user_id = $1 AND p.is_active = TRUE
		ORDER BY w.added_at DESC`
	
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *WishlistRepository) GetUserWishlistSet(userID int64) (map[int64]bool, error) {
	rows, err := r.db.Query(`SELECT product_id FROM wishlists WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	set := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			set[id] = true
		}
	}
	return set, nil
}
