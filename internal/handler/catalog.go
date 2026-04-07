package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"magaz/internal/service"

	"github.com/go-chi/chi/v5"
)

type CatalogHandler struct {
	*Base
	productSvc *service.ProductService
}

func NewCatalogHandler(base *Base, productSvc *service.ProductService) *CatalogHandler {
	return &CatalogHandler{Base: base, productSvc: productSvc}
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

	page, err := h.productSvc.List(service.ProductFilter{
		Category: cat,
		Page:     pageNum,
		PerPage:  12,
	})
	if err != nil {
		http.Error(w, "Ошибка загрузки каталога", http.StatusInternalServerError)
		return
	}
	type CatalogData struct {
		*service.ProductPage
		ActiveCategory string
	}
	h.Render(w, r, "catalog.html", CatalogData{ProductPage: page, ActiveCategory: cat})
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
