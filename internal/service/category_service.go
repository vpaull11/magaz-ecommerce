package service

import (
	"errors"
	"strings"
	"sync"
	"unicode"

	"magaz/internal/models"
	"magaz/internal/repository"
)

type CategoryService struct {
	cats *repository.CategoryRepository
	attr *repository.AttrRepository

	mu          sync.RWMutex
	cachedFlat  []*models.Category
	cachedTree  []*models.Category
	cacheLoaded bool
}

func NewCategoryService(cats *repository.CategoryRepository, attr *repository.AttrRepository) *CategoryService {
	return &CategoryService{cats: cats, attr: attr}
}

// loadCache refreshes the cache from the database (under write lock).
func (s *CategoryService) loadCache() {
	flat, err := s.cats.List()
	if err != nil {
		return
	}
	tree, err := s.cats.ListTree()
	if err != nil {
		return
	}
	s.cachedFlat = flat
	s.cachedTree = repository.BuildTree(tree)
	s.cacheLoaded = true
}

// invalidate reloads cache under write lock; called after mutations.
func (s *CategoryService) invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadCache()
}

// GetTree returns root categories with nested Children (cached).
func (s *CategoryService) GetTree() ([]*models.Category, error) {
	s.mu.RLock()
	if s.cacheLoaded {
		defer s.mu.RUnlock()
		return s.cachedTree, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	if !s.cacheLoaded {
		s.loadCache()
	}
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cachedTree, nil
}

func (s *CategoryService) GetFlat() ([]*models.Category, error) {
	s.mu.RLock()
	if s.cacheLoaded {
		defer s.mu.RUnlock()
		return s.cachedFlat, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	if !s.cacheLoaded {
		s.loadCache()
	}
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cachedFlat, nil
}

func (s *CategoryService) FindByID(id int64) (*models.Category, error) {
	return s.cats.FindByID(id)
}

// Create creates a new category, generating a slug if not provided.
func (s *CategoryService) Create(name, slug string, parentID *int64, sortOrder int, seoTitle, seoDesc string) (*models.Category, error) {
	if slug == "" {
		slug = slugify(name)
	}
	c := &models.Category{
		Name:           name,
		Slug:           slug,
		ParentID:       parentID,
		SortOrder:      sortOrder,
		SeoTitle:       seoTitle,
		SeoDescription: seoDesc,
	}
	err := s.cats.Create(c)
	if err == nil {
		s.invalidate()
	}
	return c, err
}

func (s *CategoryService) Update(id int64, name, slug string, parentID *int64, sortOrder int, seoTitle, seoDesc string) error {
	c, err := s.cats.FindByID(id)
	if err != nil {
		return err
	}
	if slug == "" {
		slug = slugify(name)
	}
	c.Name = name
	c.Slug = slug
	c.ParentID = parentID
	c.SortOrder = sortOrder
	c.SeoTitle = seoTitle
	c.SeoDescription = seoDesc
	err = s.cats.Update(c)
	if err == nil {
		s.invalidate()
	}
	return err
}

// Delete enforces business rules before deleting.
func (s *CategoryService) Delete(id int64) error {
	if has, _ := s.cats.HasProducts(id); has {
		return errors.New("нельзя удалить категорию, в которой есть товары")
	}
	err := s.cats.Delete(id) // children are moved to root inside repo tx
	if err == nil {
		s.invalidate()
	}
	return err
}

// ─── AttrDef management ───────────────────────────────────────────────────────

func (s *CategoryService) ListDefs(categoryID int64) ([]*models.AttrDef, error) {
	return s.attr.ListDefs(categoryID)
}

func (s *CategoryService) CreateDef(categoryID int64, name, valueType string, sortOrder int) (*models.AttrDef, error) {
	d := &models.AttrDef{
		CategoryID: categoryID,
		Name:       name,
		Slug:       slugify(name),
		ValueType:  valueType,
		SortOrder:  sortOrder,
	}
	return d, s.attr.CreateDef(d)
}

func (s *CategoryService) DeleteDef(id int64) error {
	return s.attr.DeleteDef(id)
}

// ─── AttrValue management ─────────────────────────────────────────────────────

func (s *CategoryService) GetValuesForProduct(productID int64) ([]models.AttrValue, error) {
	return s.attr.GetValuesForProduct(productID)
}

// SaveProductAttrs saves all attribute values for a product in bulk (map defID → raw string value).
func (s *CategoryService) SaveProductAttrs(productID int64, vals map[string]string) error {
	for defIDStr, rawVal := range vals {
		var defID int64
		if _, err := parseID(defIDStr, &defID); err != nil {
			continue
		}
		def, err := s.attr.FindDef(defID)
		if err != nil {
			continue
		}
		rawVal = strings.TrimSpace(rawVal)
		if rawVal == "" {
			_ = s.attr.DeleteValue(productID, defID)
			continue
		}
		if def.ValueType == "number" {
			var num float64
			if _, err := parseFloat(rawVal, &num); err == nil {
				_ = s.attr.SetValue(productID, defID, nil, &num)
			}
		} else {
			_ = s.attr.SetValue(productID, defID, &rawVal, nil)
		}
	}
	return nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func slugify(s string) string {
	// Transliterate basic cyrillic + lowercase + replace non-alnum with -
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	// Replace multiple dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return slug
}

func parseID(s string, out *int64) (bool, error) {
	s = strings.TrimSpace(s)
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, errors.New("not a number")
		}
		n = n*10 + int64(c-'0')
	}
	*out = n
	return true, nil
}

func parseFloat(s string, out *float64) (bool, error) {
	s = strings.TrimSpace(s)
	var n float64
	var dec bool
	var decDiv float64 = 1
	neg := false
	i := 0
	if len(s) > 0 && s[0] == '-' {
		neg = true
		i = 1
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == ',' {
			dec = true
			continue
		}
		if c < '0' || c > '9' {
			return false, errors.New("not a number")
		}
		if dec {
			decDiv *= 10
			n += float64(c-'0') / decDiv
		} else {
			n = n*10 + float64(c-'0')
		}
	}
	if neg {
		n = -n
	}
	*out = n
	return true, nil
}
