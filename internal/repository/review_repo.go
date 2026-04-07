package repository

import (
	"database/sql"
	"magaz/internal/models"
)

type ReviewRepository struct{ db *sql.DB }

func NewReviewRepository(db *sql.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) AddReview(rev *models.Review) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRow(
		`INSERT INTO reviews (user_id, product_id, rating, comment)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, product_id)
		 DO UPDATE SET rating = EXCLUDED.rating, comment = EXCLUDED.comment, created_at = CURRENT_TIMESTAMP
		 RETURNING id, created_at`,
		rev.UserID, rev.ProductID, rev.Rating, rev.Comment,
	).Scan(&rev.ID, &rev.CreatedAt)
	
	if err != nil {
		return err
	}

	// Update product cache (rating_avg, review_count)
	_, err = tx.Exec(`
		UPDATE products
		SET rating_avg = (SELECT ROUND(AVG(rating), 2) FROM reviews WHERE product_id = $1),
		    review_count = (SELECT COUNT(*) FROM reviews WHERE product_id = $1)
		WHERE id = $1`, rev.ProductID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *ReviewRepository) GetProductReviews(productID int64) ([]*models.Review, error) {
	rows, err := r.db.Query(`
		SELECT r.id, r.user_id, r.product_id, r.rating, r.comment, r.created_at, u.name as user_name
		FROM reviews r
		JOIN users u ON u.id = r.user_id
		WHERE r.product_id = $1
		ORDER BY r.created_at DESC
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []*models.Review
	for rows.Next() {
		rev := &models.Review{}
		if err := rows.Scan(&rev.ID, &rev.UserID, &rev.ProductID, &rev.Rating, &rev.Comment, &rev.CreatedAt, &rev.UserName); err != nil {
			return nil, err
		}
		reviews = append(reviews, rev)
	}
	return reviews, nil
}

func (r *ReviewRepository) GetUserReview(userID, productID int64) (*models.Review, error) {
	rev := &models.Review{}
	err := r.db.QueryRow(`
		SELECT id, user_id, product_id, rating, comment, created_at
		FROM reviews
		WHERE user_id = $1 AND product_id = $2
	`, userID, productID).Scan(&rev.ID, &rev.UserID, &rev.ProductID, &rev.Rating, &rev.Comment, &rev.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error here, just return nil
	}
	return rev, err
}
