package repository

import (
	"database/sql"
	"errors"

	"magaz/internal/models"
)

type CategoryRepository struct{ db *sql.DB }

func NewCategoryRepository(db *sql.DB) *CategoryRepository { return &CategoryRepository{db: db} }

func (r *CategoryRepository) List() ([]*models.Category, error) {
	rows, err := r.db.Query(`SELECT id, name, slug FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cats []*models.Category
	for rows.Next() {
		c := &models.Category{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (r *CategoryRepository) FindBySlug(slug string) (*models.Category, error) {
	c := &models.Category{}
	err := r.db.QueryRow(
		`SELECT id, name, slug FROM categories WHERE slug = $1`, slug,
	).Scan(&c.ID, &c.Name, &c.Slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}
