package service

import (
	"mime/multipart"
	"path/filepath"
	"strings"

	"magaz/internal/models"
	"magaz/internal/repository"
)

type ProductService struct {
	products   *repository.ProductRepository
	categories *repository.CategoryRepository
}

func NewProductService(
	products *repository.ProductRepository,
	categories *repository.CategoryRepository,
) *ProductService {
	return &ProductService{products: products, categories: categories}
}

type ProductFilter struct {
	Category    string
	Page        int
	PerPage     int
	SortBy      string            // "price_asc" | "price_desc" | "" (default)
	AttrFilters map[int64]string  // attrDefID -> value (string or "min:max")
}

type ProductPage struct {
	Products      []*models.Product
	Categories    []*models.Category   // flat list
	CategoryTree  []*models.Category   // hierarchical tree (root categories with Children)
	Total         int
	Page          int
	TotalPages    int
}

func (s *ProductService) List(f ProductFilter) (*ProductPage, error) {
	if f.PerPage <= 0 {
		f.PerPage = 12
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.PerPage

	products, total, err := s.products.List(f.Category, f.PerPage, offset, f.SortBy, f.AttrFilters)
	if err != nil {
		return nil, err
	}
	cats, err := s.categories.List()
	if err != nil {
		return nil, err
	}
	totalPages := (total + f.PerPage - 1) / f.PerPage
	if totalPages < 1 {
		totalPages = 1
	}
	return &ProductPage{
		Products:     products,
		Categories:   cats,
		CategoryTree: repository.BuildTree(cats),
		Total:        total,
		Page:         f.Page,
		TotalPages:   totalPages,
	}, nil
}

func (s *ProductService) ListAdmin(page, perPage int) ([]*models.Product, int, error) {
	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}
	return s.products.ListAll(perPage, (page-1)*perPage)
}

func (s *ProductService) GetByID(id int64) (*models.Product, error) {
	return s.products.FindByID(id)
}

func (s *ProductService) Categories() ([]*models.Category, error) {
	return s.categories.List()
}

type ProductInput struct {
	Name        string `validate:"required,min=2,max=255"`
	Description string
	Price       float64 `validate:"required,gt=0"`
	Stock       int     `validate:"min=0"`
	CategoryID  int64   `validate:"required"`
	IsActive    bool
}

func (s *ProductService) Create(in ProductInput, imageURL string) (*models.Product, error) {
	p := &models.Product{
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		Price:       in.Price,
		Stock:       in.Stock,
		ImageURL:    imageURL,
		CategoryID:  in.CategoryID,
		IsActive:    in.IsActive,
	}
	if err := s.products.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProductService) Update(id int64, in ProductInput, imageURL string) error {
	p, err := s.products.FindByID(id)
	if err != nil {
		return err
	}
	p.Name = strings.TrimSpace(in.Name)
	p.Description = in.Description
	p.Price = in.Price
	p.Stock = in.Stock
	p.CategoryID = in.CategoryID
	p.IsActive = in.IsActive
	if imageURL != "" {
		p.ImageURL = imageURL
	}
	return s.products.Update(p)
}

func (s *ProductService) Delete(id int64) error {
	return s.products.Delete(id)
}

// SafeFilename sanitises an uploaded file name to prevent path traversal.
func SafeFilename(header *multipart.FileHeader) string {
	name := filepath.Base(header.Filename)
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	return name
}

// AllowedImageExt returns true for jpeg/png/webp/gif.
func AllowedImageExt(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	}
	return false
}

// AllowedMIMEType returns true for trusted image MIME types detected via http.DetectContentType.
// This checks actual file content, not just the filename extension.
func AllowedMIMEType(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	}
	return false
}
