package handler

import (
	"net/http"
	"strconv"
	"strings"

	"magaz/internal/middleware"
	"magaz/internal/models"
	"magaz/internal/repository"
	"magaz/internal/service"
	"magaz/internal/validation"
)

type AccountHandler struct {
	*Base
	authSvc    *service.AuthService
	orderSvc   *service.OrderService
	addressSvc *repository.AddressRepository
}

func NewAccountHandler(
	base *Base,
	authSvc *service.AuthService,
	orderSvc *service.OrderService,
	addressRepo *repository.AddressRepository,
) *AccountHandler {
	return &AccountHandler{
		Base:       base,
		authSvc:    authSvc,
		orderSvc:   orderSvc,
		addressSvc: addressRepo,
	}
}

// GET /account
func (h *AccountHandler) Profile(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, "profile.html", nil)
}

// POST /account
func (h *AccountHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	name := strings.TrimSpace(r.FormValue("name"))
	if err := h.authSvc.UpdateName(u.ID, name); err != nil {
		h.Flash(w, r, err.Error(), "error")
	} else {
		h.Flash(w, r, "Профиль обновлён", "success")
	}
	http.Redirect(w, r, "/account", http.StatusFound)
}

// POST /account/password
func (h *AccountHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	
	oldPass := r.FormValue("old_password")
	newPass := r.FormValue("new_password")
	confirmPass := r.FormValue("confirm_password")

	if newPass != confirmPass {
		h.Flash(w, r, "Пароли не совпадают", "error")
		http.Redirect(w, r, "/account", http.StatusFound)
		return
	}

	if err := h.authSvc.ChangePassword(u.ID, oldPass, newPass); err != nil {
		h.Flash(w, r, err.Error(), "error")
	} else {
		h.Flash(w, r, "Пароль успешно изменён", "success")
	}

	http.Redirect(w, r, "/account", http.StatusFound)
}

// GET /account/addresses
func (h *AccountHandler) Addresses(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	addrs, err := h.addressSvc.List(u.ID)
	if err != nil {
		http.Error(w, "Ошибка загрузки адресов", http.StatusInternalServerError)
		return
	}
	h.Render(w, r, "addresses.html", addrs)
}

// POST /account/addresses
func (h *AccountHandler) AddAddress(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	label  := strings.TrimSpace(r.FormValue("label"))
	city   := strings.TrimSpace(r.FormValue("city"))
	street := strings.TrimSpace(r.FormValue("street"))
	zip    := strings.TrimSpace(r.FormValue("zip"))

	// Set default label
	if label == "" {
		label = "Домашний"
	}

	// Validate all fields
	if err := validation.ValidateAddress(label, city, street, zip); err != nil {
		h.Flash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/account/addresses", http.StatusFound)
		return
	}

	addr := &models.Address{
		UserID:    u.ID,
		Label:     label,
		City:      city,
		Street:    street,
		Zip:       zip,
		IsDefault: r.FormValue("is_default") == "on",
	}
	if err := h.addressSvc.Create(addr); err != nil {
		h.Flash(w, r, "Ошибка сохранения адреса", "error")
	} else {
		h.Flash(w, r, "Адрес добавлен", "success")
	}
	http.Redirect(w, r, "/account/addresses", http.StatusFound)
}

// POST /account/addresses/{id}/delete
func (h *AccountHandler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/account/addresses", http.StatusFound)
		return
	}
	if err := h.addressSvc.Delete(id, u.ID); err != nil {
		h.Flash(w, r, "Ошибка удаления адреса", "error")
	} else {
		h.Flash(w, r, "Адрес удалён", "success")
	}
	http.Redirect(w, r, "/account/addresses", http.StatusFound)
}

// GET /account/orders
func (h *AccountHandler) Orders(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	orders, err := h.orderSvc.ListByUser(u.ID)
	if err != nil {
		http.Error(w, "Ошибка загрузки заказов", http.StatusInternalServerError)
		return
	}
	h.Render(w, r, "orders.html", orders)
}

// GET /account/orders/{id}
func (h *AccountHandler) OrderDetail(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	order, err := h.orderSvc.GetForUser(id, u.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.Render(w, r, "order_detail.html", order)
}
