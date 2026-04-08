package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"magaz/internal/config"
	"magaz/internal/db"

	"github.com/brianvoe/gofakeit/v6"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Load DB config
	cfg := config.Load()
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		slog.Error("Ошибка подключения к БД", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	slog.Info("Начинается процесс генерации фейковых данных...")

	// 1. Clear existing data
	// CASCADE will also clear products, attr_defs, attr_values etc.
	slog.Info("Очистка таблиц (TRUNCATE)...")
	_, err = database.Exec("TRUNCATE TABLE categories CASCADE")
	if err != nil {
		slog.Error("Ошибка при выполнении TRUNCATE", "err", err)
		os.Exit(1)
	}

	gofakeit.Seed(0) // Random seed

	// 2. Insert main categories
	cats := []struct {
		Name string
		Slug string
	}{
		{"Электроника", "electronics"},
		{"Одежда", "clothing"},
		{"Книги", "books"},
		{"Дом и сад", "home-garden"},
		{"Спорт", "sport"},
	}

	var catIDs []int64
	for _, c := range cats {
		var id int64
		err := database.QueryRow(
			"INSERT INTO categories (name, slug) VALUES ($1, $2) RETURNING id",
			c.Name, c.Slug,
		).Scan(&id)
		if err != nil {
			slog.Error("Не удалось создать категорию", "err", err)
			continue
		}
		catIDs = append(catIDs, id)

		// Create 2 attribute defs for each category
		attrDefs := []string{"Цвет", "Бренд"}
		if c.Slug == "electronics" {
			attrDefs = []string{"Память", "Цвет"}
		} else if c.Slug == "books" {
			attrDefs = []string{"Обложка", "Язык"}
		}

		for _, ad := range attrDefs {
			slug := "attr_" + strings.ToLower(gofakeit.Word())
			_ = database.QueryRow(
				"INSERT INTO attr_defs (category_id, name, slug) VALUES ($1, $2, $3) RETURNING id",
				id, ad, slug,
			).Scan()
		}
	}

	// 3. Fetch all attr_defs to link them to products
	rows, err := database.Query("SELECT id, category_id, name FROM attr_defs")
	if err == nil {
		defer rows.Close()
	}
	
	type AttrDef struct{
		ID int64
		CatID int64
		Name string
	}
	var defs []AttrDef
	for rows != nil && rows.Next() {
		var d AttrDef
		rows.Scan(&d.ID, &d.CatID, &d.Name)
		defs = append(defs, d)
	}

	// 4. Generate 100 random products
	slog.Info("Генерация 100 товаров...")
	for i := 0; i < 100; i++ {
		name := gofakeit.ProductName()
		desc := gofakeit.Paragraph(1, 3, 5, "\n")
		price := gofakeit.Price(100.0, 50000.0)
		stock := gofakeit.Number(0, 100)
		img := fmt.Sprintf("https://picsum.photos/seed/%d/800/600", gofakeit.Number(1, 10000))
		catID := catIDs[gofakeit.Number(0, len(catIDs)-1)] // Random category

		var prodID int64
		err = database.QueryRow(
			`INSERT INTO products (name, description, price, stock, image_url, category_id, is_active)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			name, desc, price, stock, img, catID, true,
		).Scan(&prodID)

		if err != nil {
			slog.Error("Не удалось создать товар", "err", err)
			continue
		}

		// Insert attributes if this product's category has them
		for _, d := range defs {
			if d.CatID == catID {
				valStr := gofakeit.Color()
				if d.Name == "Память" {
					valStr = fmt.Sprintf("%d GB", gofakeit.RandomInt([]int{64, 128, 256, 512, 1024}))
				} else if d.Name == "Язык" {
					valStr = gofakeit.Language()
				} else if d.Name == "Бренд" {
					valStr = gofakeit.Company()
				}

				database.Exec("INSERT INTO attr_values (product_id, attr_def_id, value_str) VALUES ($1, $2, $3)",
					prodID, d.ID, valStr)
			}
		}
	}

	slog.Info("Готово! Каталог успешно заполнен тестовыми данными.")
}
