package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"magaz/internal/middleware"
	"magaz/internal/models"
	"magaz/internal/repository"
)

type ReviewHandler struct {
	*Base
	repo *repository.ReviewRepository
}

func NewReviewHandler(b *Base, repo *repository.ReviewRepository) *ReviewHandler {
	return &ReviewHandler{Base: b, repo: repo}
}

// POST /api/reviews/add
func (h *ReviewHandler) AddAPI(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	
	var input struct {
		ProductID int64  `json:"product_id"`
		Rating    int    `json:"rating"`
		Comment   string `json:"comment"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		idStr := r.FormValue("product_id")
		input.ProductID, _ = strconv.ParseInt(idStr, 10, 64)
		input.Rating, _ = strconv.Atoi(r.FormValue("rating"))
		input.Comment = r.FormValue("comment")
	}

	if input.ProductID == 0 || input.Rating < 1 || input.Rating > 5 {
		http.Error(w, "invalid request data", http.StatusBadRequest)
		return
	}

	rev := &models.Review{
		UserID:    u.ID,
		ProductID: input.ProductID,
		Rating:    input.Rating,
		Comment:   input.Comment,
	}

	if err := h.repo.AddReview(rev); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	})
}
