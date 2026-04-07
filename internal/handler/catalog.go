package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"magaz/internal/models"
	"magaz/internal/repository"
	"magaz/internal/service"

	"github.com/go-chi/chi/v5"
)

type CatalogHandler struct {
	*Base
	productSvc *service.ProductService
	catSvc     *service.CategoryService
	catRepo    *repository.CategoryRepository
}

func NewCatalogHandler(base *Base, productSvc *service.ProductService, catSvc *service.CategoryService, catRepo *repository.CategoryRepository) *CatalogHandler {
	return &CatalogHandler{Base: base, productSvc: productSvc, catSvc: catSvc, catRepo: catRepo}
}

// GET /
func (h *CatalogHandler) Index(w http.ResponseWriter, r *http.Request) {
	page, err := h.productSvc.List(service.ProductFilter{Page: 1, PerPage: 8})
	if err != nil {
		http.Error(w, "Ошибка загрузки товаров", http.StatusInternalServerError)
		return
	}
	h.Render(w, r, "index.html", page)
}

// GET /catalog
func (h *CatalogHandler) Catalog(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("category")
	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	sortBy := r.URL.Query().Get("sort") // "price_asc" | "price_desc"

	// Parse attr filters: ?attr_42=красный&attr_10=5:10
	attrFilters := make(map[int64]string)
	for key, vals := range r.URL.Query() {
		if strings.HasPrefix(key, "attr_") && len(vals) > 0 && vals[0] != "" {
			var defID int64
			if _, err := fmt.Sscanf(strings.TrimPrefix(key, "attr_"), "%d", &defID); err == nil {
				attrFilters[defID] = vals[0]
			}
		}
	}

	page, err := h.productSvc.List(service.ProductFilter{
		Category:    cat,
		Page:        pageNum,
		PerPage:     12,
		SortBy:      sortBy,
		AttrFilters: attrFilters,
	})
	if err != nil {
		http.Error(w, "Ошибка загрузки каталога", http.StatusInternalServerError)
		return
	}

	// Load attr defs for active category (for filter sidebar)
	var attrDefs []*models.AttrDef
	if cat != "" && h.catSvc != nil {
		if catObj, err2 := h.catRepo.FindBySlug(cat); err2 == nil {
			attrDefs, _ = h.catSvc.ListDefs(catObj.ID)
		}
	}

	type CatalogData struct {
		*service.ProductPage
		ActiveCategory string
		SortBy         string
		AttrDefs       []*models.AttrDef
		AttrFilters    map[int64]string
	}
	h.Render(w, r, "catalog.html", CatalogData{
		ProductPage:    page,
		ActiveCategory: cat,
		SortBy:         sortBy,
		AttrDefs:       attrDefs,
		AttrFilters:    attrFilters,
	})
}

// GET /catalog/{id}
func (h *CatalogHandler) Product(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, err := h.productSvc.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.Render(w, r, "product.html", p)
}

// GET /api/search?q=something
func (h *CatalogHandler) SearchAPI(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if len(query) < 2 {
		chttpJSON(w, []any{}) // Return empty
		return
	}
	
	// Since we don't have a specific search method in repo yet, we'll fetch all and filter in memory for simplicity.
	// In production, we'd add an ILIKE query.
	page, err := h.productSvc.List(service.ProductFilter{Page: 1, PerPage: 100})
	if err != nil {
		chttpJSON(w, []any{})
		return
	}
	
	var results []map[string]any
	for _, p := range page.Products {
		if containsIgnoreCase(p.Name, query) || containsIgnoreCase(p.Description, query) {
			results = append(results, map[string]any{
				"id": p.ID,
				"name": p.Name,
				"price": p.Price,
				"image_url": p.ImageURL,
			})
			if len(results) >= 5 {
				break
			}
		}
	}
	if results == nil {
		results = []map[string]any{}
	}
	
	chttpJSON(w, results)
}

func containsIgnoreCase(s, substr string) bool {
	return len(substr) > 0 && len(s) > 0 && (strings.Contains(strings.ToLower(s), strings.ToLower(substr)))
}

func chttpJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
