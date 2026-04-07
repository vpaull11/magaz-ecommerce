package repository

import (
	"database/sql"

	"magaz/internal/models"
)

type SettingsRepository struct {
	db *sql.DB
}

func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// GetAll returns all site_settings rows as a map.
func (r *SettingsRepository) GetAll() (models.SiteSettings, error) {
	rows, err := r.db.Query(`SELECT key, value FROM site_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(models.SiteSettings)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

// Set upserts a single key-value pair.
func (r *SettingsRepository) Set(key, value string) error {
	_, err := r.db.Exec(
		`INSERT INTO site_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	return err
}
