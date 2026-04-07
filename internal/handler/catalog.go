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

// GET /catalog, /catalog/{slug}, /catalog/{slug}/{page}
func (h *CatalogHandler) Catalog(w http.ResponseWriter, r *http.Request) {
	// Slug comes from URL path param OR query-param fallback (not used in new URLs)
	catSlug := chi.URLParam(r, "slug")
	// Page: from path param {page} or query param for sort/filter links
	pageNum, _ := strconv.Atoi(chi.URLParam(r, "page"))
	if pageNum == 0 {
		pageNum, _ = strconv.Atoi(r.URL.Query().Get("page"))
	}
	sortBy := r.URL.Query().Get("sort") // stays in query: ?sort=price_asc

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

	// Resolve category slug → IDs (parent + all children)
	var categoryIDs []int64
	var attrDefs []*models.AttrDef
	var activeCatName, catSeoTitle, catSeoDesc string
	if catSlug != "" {
		catObj, err2 := h.catRepo.FindBySlug(catSlug)
		if err2 == nil {
			activeCatName = catObj.Name
			catSeoTitle   = catObj.SeoTitle
			catSeoDesc    = catObj.SeoDescription
			// Collect the category itself + all its children from the flat list
			allCats, _ := h.catRepo.List()
			categoryIDs = append(categoryIDs, catObj.ID)
			for _, c := range allCats {
				if c.ParentID != nil && *c.ParentID == catObj.ID {
					categoryIDs = append(categoryIDs, c.ID)
				}
			}
			// Load attr defs for the selected category (for filter sidebar)
			if h.catSvc != nil {
				attrDefs, _ = h.catSvc.ListDefs(catObj.ID)
			}
		}
	}

	page, err := h.productSvc.List(service.ProductFilter{
		CategoryIDs: categoryIDs,
		Page:        pageNum,
		PerPage:     12,
		SortBy:      sortBy,
		AttrFilters: attrFilters,
	})
	if err != nil {
		http.Error(w, "Ошибка загрузки каталога", http.StatusInternalServerError)
		return
	}

	type CatalogData struct {
		*service.ProductPage
		ActiveCategory     string
		ActiveCategoryName string
		CategorySeoTitle   string
		CategorySeoDesc    string
		SortBy             string
		AttrDefs           []*models.AttrDef
		AttrFilters        map[int64]string
	}
	h.Render(w, r, "catalog.html", CatalogData{
		ProductPage:        page,
		ActiveCategory:     catSlug,
		ActiveCategoryName: activeCatName,
		CategorySeoTitle:   catSeoTitle,
		CategorySeoDesc:    catSeoDesc,
		SortBy:             sortBy,
		AttrDefs:           attrDefs,
		AttrFilters:        attrFilters,
	})
}

// GET /catalog/item/{id}
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
// GET /catalog/{id:[0-9]+} — 301 redirect to /catalog/item/{id}
func (h *CatalogHandler) ProductRedirect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	http.Redirect(w, r, "/catalog/item/"+id, http.StatusMovedPermanently)
}
