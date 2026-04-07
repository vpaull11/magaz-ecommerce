package repository

import (
	"database/sql"
	"strings"

	"magaz/internal/models"
)

type AttrRepository struct{ db *sql.DB }

func NewAttrRepository(db *sql.DB) *AttrRepository { return &AttrRepository{db: db} }

// ─── AttrDef CRUD ─────────────────────────────────────────────────────────────

func (r *AttrRepository) ListDefs(categoryID int64) ([]*models.AttrDef, error) {
	rows, err := r.db.Query(
		`SELECT id, category_id, name, slug, value_type, sort_order
		 FROM attr_defs WHERE category_id=$1 ORDER BY sort_order, name`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var defs []*models.AttrDef
	for rows.Next() {
		d := &models.AttrDef{}
		if err := rows.Scan(&d.ID, &d.CategoryID, &d.Name, &d.Slug, &d.ValueType, &d.SortOrder); err != nil {
			return nil, err
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

func (r *AttrRepository) CreateDef(d *models.AttrDef) error {
	return r.db.QueryRow(
		`INSERT INTO attr_defs (category_id, name, slug, value_type, sort_order)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		d.CategoryID, d.Name, d.Slug, d.ValueType, d.SortOrder,
	).Scan(&d.ID)
}

func (r *AttrRepository) UpdateDef(d *models.AttrDef) error {
	_, err := r.db.Exec(
		`UPDATE attr_defs SET name=$1, slug=$2, value_type=$3, sort_order=$4 WHERE id=$5`,
		d.Name, d.Slug, d.ValueType, d.SortOrder, d.ID,
	)
	return err
}

func (r *AttrRepository) DeleteDef(id int64) error {
	_, err := r.db.Exec(`DELETE FROM attr_defs WHERE id=$1`, id)
	return err
}

func (r *AttrRepository) FindDef(id int64) (*models.AttrDef, error) {
	d := &models.AttrDef{}
	err := r.db.QueryRow(
		`SELECT id, category_id, name, slug, value_type, sort_order FROM attr_defs WHERE id=$1`, id,
	).Scan(&d.ID, &d.CategoryID, &d.Name, &d.Slug, &d.ValueType, &d.SortOrder)
	return d, err
}

// ─── AttrValue CRUD ───────────────────────────────────────────────────────────

func (r *AttrRepository) GetValuesForProduct(productID int64) ([]models.AttrValue, error) {
	rows, err := r.db.Query(
		`SELECT av.id, av.product_id, av.attr_def_id, av.value_str, av.value_num,
		        ad.id, ad.category_id, ad.name, ad.slug, ad.value_type, ad.sort_order
		 FROM attr_values av
		 JOIN attr_defs ad ON ad.id = av.attr_def_id
		 WHERE av.product_id=$1
		 ORDER BY ad.sort_order, ad.name`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vals []models.AttrValue
	for rows.Next() {
		av := models.AttrValue{Def: &models.AttrDef{}}
		if err := rows.Scan(
			&av.ID, &av.ProductID, &av.AttrDefID, &av.ValueStr, &av.ValueNum,
			&av.Def.ID, &av.Def.CategoryID, &av.Def.Name, &av.Def.Slug, &av.Def.ValueType, &av.Def.SortOrder,
		); err != nil {
			return nil, err
		}
		vals = append(vals, av)
	}
	return vals, rows.Err()
}

// SetValue upserts a single attribute value for a product.
func (r *AttrRepository) SetValue(productID, attrDefID int64, valueStr *string, valueNum *float64) error {
	_, err := r.db.Exec(
		`INSERT INTO attr_values (product_id, attr_def_id, value_str, value_num)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (product_id, attr_def_id)
		 DO UPDATE SET value_str=EXCLUDED.value_str, value_num=EXCLUDED.value_num`,
		productID, attrDefID, valueStr, valueNum,
	)
	return err
}

func (r *AttrRepository) DeleteValue(productID, attrDefID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM attr_values WHERE product_id=$1 AND attr_def_id=$2`, productID, attrDefID,
	)
	return err
}

// ─── Filter helpers ───────────────────────────────────────────────────────────

// UniqueStringValues returns all distinct string values for an attr_def (for checkbox filters).
func (r *AttrRepository) UniqueStringValues(attrDefID int64) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT value_str FROM attr_values
		 WHERE attr_def_id=$1 AND value_str IS NOT NULL ORDER BY value_str`, attrDefID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v string
		rows.Scan(&v)
		vals = append(vals, v)
	}
	return vals, rows.Err()
}

// ProductIDsByAttrs returns product IDs that match all given attr filters.
// attrFilters: map[attrDefID]filterValue  (string match or "min:max" for numbers)
func (r *AttrRepository) ProductIDsByAttrs(attrFilters map[int64]string) ([]int64, error) {
	if len(attrFilters) == 0 {
		return nil, nil
	}
	// Build an intersect query: each filter must be satisfied
	var parts []string
	var args []any
	i := 1
	for defID, val := range attrFilters {
		if strings.Contains(val, ":") { // number range "min:max"
			parts2 := strings.SplitN(val, ":", 2)
			parts = append(parts, `SELECT product_id FROM attr_values
				WHERE attr_def_id=$`+itoa(i)+` AND value_num BETWEEN $`+itoa(i+1)+` AND $`+itoa(i+2))
			args = append(args, defID, parts2[0], parts2[1])
			i += 3
		} else {
			parts = append(parts, `SELECT product_id FROM attr_values
				WHERE attr_def_id=$`+itoa(i)+` AND value_str=$`+itoa(i+1))
			args = append(args, defID, val)
			i += 2
		}
	}
	q := strings.Join(parts, " INTERSECT ")
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func itoa(n int) string {
	return strings.TrimSpace(strings.Replace("  "+string(rune('0'+n%10)), "  ", "", 1))
}
