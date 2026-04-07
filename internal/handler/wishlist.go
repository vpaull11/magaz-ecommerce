package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"magaz/internal/middleware"
	"magaz/internal/repository"
)

type WishlistHandler struct {
	*Base
	repo *repository.WishlistRepository
}

func NewWishlistHandler(b *Base, repo *repository.WishlistRepository) *WishlistHandler {
	return &WishlistHandler{Base: b, repo: repo}
}

// POST /api/wishlist/toggle
func (h *WishlistHandler) ToggleAPI(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	
	// Ensure we handle JSON
	var input struct {
		ProductID int64 `json:"product_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		idStr := r.FormValue("product_id")
		input.ProductID, _ = strconv.ParseInt(idStr, 10, 64)
	}

	if input.ProductID == 0 {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	added, err := h.repo.Toggle(u.ID, input.ProductID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"added": added,
	})
}

// GET /wishlist
func (h *WishlistHandler) WishlistPage(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	products, err := h.repo.ListUserWishlist(u.ID)
	if err != nil {
		products = nil
	}
	
	// Add CSRF purely so wishlist page buttons have it if needed
	h.Render(w, r, "wishlist.html", map[string]any{
		"Products": products,
	})
}
