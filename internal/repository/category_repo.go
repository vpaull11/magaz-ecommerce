package repository

import (
	"database/sql"
	"errors"

	"magaz/internal/models"
)

type CategoryRepository struct{ db *sql.DB }

func NewCategoryRepository(db *sql.DB) *CategoryRepository { return &CategoryRepository{db: db} }

func (r *CategoryRepository) List() ([]*models.Category, error) {
	rows, err := r.db.Query(`SELECT id, name, slug, parent_id, sort_order FROM categories ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cats []*models.Category
	for rows.Next() {
		c := &models.Category{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.SortOrder); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// ListTree returns a flat list (all categories), use BuildTree to get hierarchy.
func (r *CategoryRepository) ListTree() ([]*models.Category, error) {
	return r.List()
}

// BuildTree converts a flat list into a parent→children hierarchy.
func BuildTree(cats []*models.Category) []*models.Category {
	byID := make(map[int64]*models.Category, len(cats))
	for _, c := range cats {
		byID[c.ID] = c
	}
	var roots []*models.Category
	for _, c := range cats {
		if c.ParentID == nil {
			roots = append(roots, c)
		} else {
			if parent, ok := byID[*c.ParentID]; ok {
				parent.Children = append(parent.Children, c)
			}
		}
	}
	return roots
}

func (r *CategoryRepository) FindBySlug(slug string) (*models.Category, error) {
	c := &models.Category{}
	err := r.db.QueryRow(
		`SELECT id, name, slug, parent_id, sort_order FROM categories WHERE slug = $1`, slug,
	).Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (r *CategoryRepository) FindByID(id int64) (*models.Category, error) {
	c := &models.Category{}
	err := r.db.QueryRow(
		`SELECT id, name, slug, parent_id, sort_order FROM categories WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (r *CategoryRepository) Create(c *models.Category) error {
	return r.db.QueryRow(
		`INSERT INTO categories (name, slug, parent_id, sort_order) VALUES ($1,$2,$3,$4) RETURNING id`,
		c.Name, c.Slug, c.ParentID, c.SortOrder,
	).Scan(&c.ID)
}

func (r *CategoryRepository) Update(c *models.Category) error {
	_, err := r.db.Exec(
		`UPDATE categories SET name=$1, slug=$2, parent_id=$3, sort_order=$4 WHERE id=$5`,
		c.Name, c.Slug, c.ParentID, c.SortOrder, c.ID,
	)
	return err
}

// HasProducts returns true if category has products (blocks deletion).
func (r *CategoryRepository) HasProducts(id int64) (bool, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM products WHERE category_id=$1`, id).Scan(&n)
	return n > 0, err
}

// HasChildren returns true if category has sub-categories.
func (r *CategoryRepository) HasChildren(id int64) (bool, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM categories WHERE parent_id=$1`, id).Scan(&n)
	return n > 0, err
}

// Delete moves children to root then deletes the category.
func (r *CategoryRepository) Delete(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Move children to root
	if _, err := tx.Exec(`UPDATE categories SET parent_id=NULL WHERE parent_id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM categories WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}
