package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"magaz/internal/models"
)

type ProductRepository struct{ db *sql.DB }

func NewProductRepository(db *sql.DB) *ProductRepository { return &ProductRepository{db: db} }

const productSelectSQL = `
	SELECT p.id, p.name, p.description, p.price, p.stock,
	       p.image_url, p.category_id, COALESCE(c.name,'') as category_name,
	       p.is_active, p.rating_avg, p.review_count,
	       p.created_at, p.updated_at,
	       COALESCE(p.seo_title,'') as seo_title, COALESCE(p.seo_description,'') as seo_description
	FROM products p
	LEFT JOIN categories c ON c.id = p.category_id`

func scanProduct(row interface{ Scan(...any) error }) (*models.Product, error) {
	p := &models.Product{}
	return p, row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock,
		&p.ImageURL, &p.CategoryID, &p.CategoryName,
		&p.IsActive, &p.RatingAvg, &p.ReviewCount,
		&p.CreatedAt, &p.UpdatedAt,
		&p.SeoTitle, &p.SeoDescription,
	)
}

func (r *ProductRepository) List(categoryIDs []int64, limit, offset int, sortBy string, attrFilters map[int64]string) ([]*models.Product, int, error) {
	var (
		args  []any
		where string
	)
	if len(categoryIDs) == 1 {
		// Single category — fast path
		where = " WHERE p.is_active = TRUE AND p.category_id = $1"
		args = []any{categoryIDs[0], limit, offset}
	} else if len(categoryIDs) > 1 {
		// Multiple categories (parent + children) — use IN (...)
		placeholders := make([]string, len(categoryIDs))
		for i, id := range categoryIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args = append(args, id)
		}
		where = fmt.Sprintf(" WHERE p.is_active = TRUE AND p.category_id IN (%s)", strings.Join(placeholders, ","))
		args = append(args, limit, offset)
	} else {
		// No category filter
		where = " WHERE p.is_active = TRUE"
		args = []any{limit, offset}
	}

	// Attribute filter — narrow by product IDs
	if len(attrFilters) > 0 {
		// Build intersect subquery inline
		var parts []string
		argIdx := len(args) - 1 // after existing args (before limit/offset)
		// We need to insert ID filter before limit/offset args
		limitVal := args[len(args)-2]
		offsetVal := args[len(args)-1]
		args = args[:len(args)-2]
		argIdx = len(args)

		for defID, val := range attrFilters {
			argIdx++
			argIdx2 := argIdx + 1
			if len(val) > 0 && strings.Contains(val, ":") {
				parts2 := strings.SplitN(val, ":", 2)
				parts = append(parts, fmt.Sprintf(
					"SELECT product_id FROM attr_values WHERE attr_def_id=$%d AND value_num BETWEEN $%d AND $%d",
					argIdx, argIdx2, argIdx2+1))
				args = append(args, defID, parts2[0], parts2[1])
				argIdx += 2
			} else {
				parts = append(parts, fmt.Sprintf(
					"SELECT product_id FROM attr_values WHERE attr_def_id=$%d AND value_str=$%d",
					argIdx, argIdx2))
				args = append(args, defID, val)
				argIdx++
			}
		}
		idSubq := strings.Join(parts, " INTERSECT ")
		where += fmt.Sprintf(" AND p.id IN (%s)", idSubq)
		args = append(args, limitVal, offsetVal)
	}

	// count
	countSQL := "SELECT COUNT(*) FROM products p LEFT JOIN categories c ON c.id = p.category_id" + where
	countArgs := args[:len(args)-2]
	var total int
	if err := r.db.QueryRow(countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// order
	orderClause := " ORDER BY p.created_at DESC"
	switch sortBy {
	case "price_asc":
		orderClause = " ORDER BY p.price ASC"
	case "price_desc":
		orderClause = " ORDER BY p.price DESC"
	}

	limitArg := len(args) - 2
	offsetArg := len(args) - 1
	query := productSelectSQL + where + orderClause +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitArg+1, offsetArg+1)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var products []*models.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}


func (r *ProductRepository) ListAll(limit, offset int) ([]*models.Product, int, error) {
	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(
		productSelectSQL+` ORDER BY p.created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var products []*models.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}

func (r *ProductRepository) FindByID(id int64) (*models.Product, error) {
	p, err := scanProduct(r.db.QueryRow(productSelectSQL+" WHERE p.id = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *ProductRepository) Create(p *models.Product) error {
	return r.db.QueryRow(
		`INSERT INTO products (name, description, price, stock, image_url, category_id, is_active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, created_at, updated_at`,
		p.Name, p.Description, p.Price, p.Stock, p.ImageURL, p.CategoryID, p.IsActive,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *ProductRepository) Update(p *models.Product) error {
	_, err := r.db.Exec(
		`UPDATE products SET name=$1, description=$2, price=$3, stock=$4,
		 image_url=$5, category_id=$6, is_active=$7,
		 seo_title=$8, seo_description=$9, updated_at=NOW()
		 WHERE id=$10`,
		p.Name, p.Description, p.Price, p.Stock,
		p.ImageURL, p.CategoryID, p.IsActive,
		p.SeoTitle, p.SeoDescription, p.ID,
	)
	return err
}

func (r *ProductRepository) Delete(id int64) error {
	_, err := r.db.Exec(`UPDATE products SET is_active=FALSE WHERE id=$1`, id)
	return err
}

func (r *ProductRepository) DecrementStock(tx *sql.Tx, productID int64, qty int) error {
	res, err := tx.Exec(
		`UPDATE products SET stock = stock - $1
		 WHERE id = $2 AND stock >= $1`,
		qty, productID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("недостаточно товара на складе (id=%d)", productID)
	}
	return nil
}
