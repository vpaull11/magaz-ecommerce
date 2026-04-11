package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"magaz/internal/middleware"
	"magaz/internal/repository"
	"magaz/internal/service"
)

type CartHandler struct {
	*Base
	cartSvc    *service.CartService
	addressSvc *repository.AddressRepository
}

func NewCartHandler(base *Base, cartSvc *service.CartService, addressRepo *repository.AddressRepository) *CartHandler {
	return &CartHandler{Base: base, cartSvc: cartSvc, addressSvc: addressRepo}
}

// GET /cart
func (h *CartHandler) CartPage(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	summary, err := h.cartSvc.Get(u.ID)
	if err != nil {
		http.Error(w, "Ошибка корзины", http.StatusInternalServerError)
		return
	}
	h.Render(w, r, "cart.html", summary)
}

// POST /cart/add
func (h *CartHandler) Add(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	productID, _ := strconv.ParseInt(r.FormValue("product_id"), 10, 64)
	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	if qty <= 0 {
		qty = 1
	}
	if err := h.cartSvc.Add(u.ID, productID, qty); err != nil {
		h.Flash(w, r, err.Error(), "error")
	} else {
		h.Flash(w, r, "Товар добавлен в корзину", "success")
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/catalog"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}

// POST /api/cart/add
func (h *CartHandler) AddAPI(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	
	var input struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		idStr := r.FormValue("product_id")
		input.ProductID, _ = strconv.ParseInt(idStr, 10, 64)
		input.Quantity, _ = strconv.Atoi(r.FormValue("quantity"))
	}
	if input.Quantity <= 0 {
		input.Quantity = 1
	}

	if err := h.cartSvc.Add(u.ID, input.ProductID, input.Quantity); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	summary, err := h.cartSvc.Get(u.ID)
	if err != nil {
		slog.Error("cart: get summary after add failed", "err", err, "user_id", u.ID)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"cart_count": summary.Count,
		"cart_total": summary.Total,
	})
}

// POST /api/cart/update
func (h *CartHandler) UpdateAPI(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	var req struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.cartSvc.Update(u.ID, req.ProductID, req.Quantity); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}
	summary, err := h.cartSvc.Get(u.ID)
	if err != nil {
		slog.Error("cart: get summary after update failed", "err", err, "user_id", u.ID)
	}
	json.NewEncoder(w).Encode(map[string]any{"success": true, "cart_count": summary.Count, "cart_total": summary.Total})
}

// POST /api/cart/remove
func (h *CartHandler) RemoveAPI(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	var req struct {
		ProductID int64 `json:"product_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.cartSvc.Remove(u.ID, req.ProductID); err != nil {
		slog.Error("cart: remove failed", "err", err, "user_id", u.ID, "product_id", req.ProductID)
	}
	summary, err := h.cartSvc.Get(u.ID)
	if err != nil {
		slog.Error("cart: get summary after remove failed", "err", err, "user_id", u.ID)
	}
	json.NewEncoder(w).Encode(map[string]any{"success": true, "cart_count": summary.Count, "cart_total": summary.Total})
}

// GET /api/cart
func (h *CartHandler) GetAPI(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	summary, err := h.cartSvc.Get(u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// POST /cart/update
func (h *CartHandler) Update(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	productID, _ := strconv.ParseInt(r.FormValue("product_id"), 10, 64)
	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	if err := h.cartSvc.Update(u.ID, productID, qty); err != nil {
		h.Flash(w, r, err.Error(), "error")
	}
	http.Redirect(w, r, "/cart", http.StatusFound)
}

// POST /cart/remove
func (h *CartHandler) Remove(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	productID, _ := strconv.ParseInt(r.FormValue("product_id"), 10, 64)
	_ = h.cartSvc.Remove(u.ID, productID)
	http.Redirect(w, r, "/cart", http.StatusFound)
}

// GET /cart/checkout
func (h *CartHandler) CheckoutPage(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	summary, err := h.cartSvc.Get(u.ID)
	if err != nil {
		http.Error(w, "Ошибка корзины", http.StatusInternalServerError)
		return
	}
	if len(summary.Items) == 0 {
		h.Flash(w, r, "Ваша корзина пуста", "info")
		http.Redirect(w, r, "/catalog", http.StatusFound)
		return
	}
	addrs, _ := h.addressSvc.List(u.ID)
	type CheckoutData struct {
		Summary   *service.CartSummary
		Addresses interface{}
	}
	h.Render(w, r, "checkout.html", CheckoutData{Summary: summary, Addresses: addrs})
}
