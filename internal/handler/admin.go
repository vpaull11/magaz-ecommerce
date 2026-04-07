package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"magaz/internal/middleware"
	"magaz/internal/models"
	"magaz/internal/repository"
	"magaz/internal/service"
)

type OrderCheckoutHandler struct {
	*Base
	orderSvc   *service.OrderService
	addressSvc *repository.AddressRepository
	cartSvc    *service.CartService
}

func NewOrderCheckoutHandler(
	base *Base,
	orderSvc *service.OrderService,
	addressRepo *repository.AddressRepository,
	cartSvc *service.CartService,
) *OrderCheckoutHandler {
	return &OrderCheckoutHandler{Base: base, orderSvc: orderSvc, addressSvc: addressRepo, cartSvc: cartSvc}
}

// POST /cart/checkout — place order
func (h *OrderCheckoutHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromCtx(r)

	addressID, err := strconv.ParseInt(r.FormValue("address_id"), 10, 64)
	if err != nil || addressID == 0 {
		h.Flash(w, r, "Выберите адрес доставки или добавьте новый", "error")
		http.Redirect(w, r, "/cart/checkout", http.StatusFound)
		return
	}

	cardNumber := strings.ReplaceAll(r.FormValue("card_number"), " ", "")
	expiry := r.FormValue("expiry")
	cvv := r.FormValue("cvv")

	if len(cardNumber) != 16 {
		h.Flash(w, r, "Неверный номер карты", "error")
		http.Redirect(w, r, "/cart/checkout", http.StatusFound)
		return
	}
	if cvv == "" || (len(cvv) < 3 || len(cvv) > 4) {
		h.Flash(w, r, "Неверный CVV", "error")
		http.Redirect(w, r, "/cart/checkout", http.StatusFound)
		return
	}

	order, err := h.orderSvc.Checkout(service.CheckoutInput{
		UserID:     u.ID,
		AddressID:  addressID,
		CardNumber: cardNumber,
		Expiry:     expiry,
		CVV:        cvv,
	})
	if err != nil {
		h.Flash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/cart/checkout", http.StatusFound)
		return
	}

	h.Flash(w, r, fmt.Sprintf("Заказ #%d успешно оплачен!", order.ID), "success")
	http.Redirect(w, r, fmt.Sprintf("/account/orders?id=%d", order.ID), http.StatusFound)
}

// ─── Admin handler ────────────────────────────────────────────────────────────

type AdminHandler struct {
	*Base
	productSvc *service.ProductService
	orderSvc   *service.OrderService
	userRepo   *repository.UserRepository
	uploadsDir string
}

func NewAdminHandler(
	base *Base,
	productSvc *service.ProductService,
	orderSvc *service.OrderService,
	userRepo *repository.UserRepository,
	uploadsDir string,
) *AdminHandler {
	return &AdminHandler{
		Base:       base,
		productSvc: productSvc,
		orderSvc:   orderSvc,
		userRepo:   userRepo,
		uploadsDir: uploadsDir,
	}
}

// GET /admin
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	orders, total, _ := h.orderSvc.ListAll(1, 5)
	h.Render(w, r, "admin_dashboard.html", map[string]any{"Orders": orders, "Total": total})
}

// GET /admin/products
func (h *AdminHandler) Products(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	products, total, err := h.productSvc.ListAdmin(page, 20)
	if err != nil {
		http.Error(w, "Ошибка", http.StatusInternalServerError)
		return
	}
	h.Render(w, r, "admin_products.html", map[string]any{"Products": products, "Total": total, "Page": page})
}

// GET /admin/products/new
func (h *AdminHandler) NewProductPage(w http.ResponseWriter, r *http.Request) {
	cats, _ := h.productSvc.Categories()
	h.Render(w, r, "admin_product_form.html", map[string]any{"Categories": cats, "Product": nil})
}

// POST /admin/products/new
func (h *AdminHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	imageURL, err := h.handleImageUpload(r)
	if err != nil {
		h.Flash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/admin/products/new", http.StatusFound)
		return
	}

	in := h.formToProductInput(r)
	if _, err := h.productSvc.Create(in, imageURL); err != nil {
		h.Flash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/admin/products/new", http.StatusFound)
		return
	}
	h.Flash(w, r, "Товар создан", "success")
	http.Redirect(w, r, "/admin/products", http.StatusFound)
}

// GET /admin/products/{id}/edit
func (h *AdminHandler) EditProductPage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	p, err := h.productSvc.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	cats, _ := h.productSvc.Categories()
	h.Render(w, r, "admin_product_form.html", map[string]any{"Categories": cats, "Product": p})
}

// POST /admin/products/{id}/edit
func (h *AdminHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	imageURL, err := h.handleImageUpload(r)
	if err != nil {
		h.Flash(w, r, err.Error(), "error")
		http.Redirect(w, r, fmt.Sprintf("/admin/products/edit?id=%d", id), http.StatusFound)
		return
	}
	in := h.formToProductInput(r)
	if err := h.productSvc.Update(id, in, imageURL); err != nil {
		h.Flash(w, r, err.Error(), "error")
	} else {
		h.Flash(w, r, "Товар обновлён", "success")
	}
	http.Redirect(w, r, "/admin/products", http.StatusFound)
}

// POST /admin/products/{id}/delete
func (h *AdminHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err := h.productSvc.Delete(id); err != nil {
		h.Flash(w, r, "Ошибка удаления", "error")
	} else {
		h.Flash(w, r, "Товар деактивирован", "success")
	}
	http.Redirect(w, r, "/admin/products", http.StatusFound)
}

// GET /admin/orders
func (h *AdminHandler) Orders(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	orders, total, _ := h.orderSvc.ListAll(page, 20)
	h.Render(w, r, "admin_orders.html", map[string]any{"Orders": orders, "Total": total, "Page": page})
}

// POST /admin/orders/{id}/status
func (h *AdminHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	status := models.OrderStatus(r.FormValue("status"))

	// Whitelist — only allow valid transitions
	validStatuses := map[models.OrderStatus]bool{
		models.StatusPending:   true,
		models.StatusPaid:      true,
		models.StatusShipped:   true,
		models.StatusDone:      true,
		models.StatusCancelled: true,
	}
	if !validStatuses[status] {
		http.Error(w, "Недопустимый статус заказа", http.StatusBadRequest)
		return
	}

	if err := h.orderSvc.UpdateStatus(id, status); err != nil {
		h.Flash(w, r, "Ошибка обновления статуса", "error")
	} else {
		h.Flash(w, r, "Статус обновлён", "success")
	}
	http.Redirect(w, r, "/admin/orders", http.StatusFound)
}

// GET /admin/users
func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	users, total, _ := h.userRepo.List(20, (page-1)*20)
	h.Render(w, r, "admin_users.html", map[string]any{"Users": users, "Total": total, "Page": page})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func (h *AdminHandler) formToProductInput(r *http.Request) service.ProductInput {
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	stock, _ := strconv.Atoi(r.FormValue("stock"))
	catID, _ := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	return service.ProductInput{
		Name:        strings.TrimSpace(r.FormValue("name")),
		Description: r.FormValue("description"),
		Price:       price,
		Stock:       stock,
		CategoryID:  catID,
		IsActive:    r.FormValue("is_active") == "on",
	}
}

func (h *AdminHandler) handleImageUpload(r *http.Request) (string, error) {
	r.ParseMultipartForm(8 << 20) // up to 8 MB in memory, rest to temp files
	file, header, err := r.FormFile("image")
	if err != nil {
		// No new image uploaded — OK
		return "", nil
	}
	defer file.Close()

	// ── 1. Extension check (fast)
	if !service.AllowedImageExt(header.Filename) {
		return "", fmt.Errorf("допустимые форматы файлов: jpg, png, webp, gif")
	}

	// ── 2. Read first 512 bytes and verify actual MIME type
	//    This prevents attacks where .php/.html is renamed to .jpg
	sniff := make([]byte, 512)
	n, err := file.Read(sniff)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("ошибка чтения файла")
	}
	detectedMIME := http.DetectContentType(sniff[:n])
	if !service.AllowedMIMEType(detectedMIME) {
		return "", fmt.Errorf("файл не является изображением (обнаруженный тип: %s)", detectedMIME)
	}

	// ── 3. Safe filename + unique name
	safe := service.SafeFilename(header)
	unique := fmt.Sprintf("%d_%s", time.Now().UnixNano(), safe)
	dst := filepath.Join(h.uploadsDir, unique)

	// 0750: owner=rwx, group=r-x, other=none (not world-readable like 0755)
	if err := os.MkdirAll(h.uploadsDir, 0750); err != nil {
		return "", fmt.Errorf("ошибка создания папки uploads: %w", err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("ошибка сохранения файла: %w", err)
	}
	defer out.Close()

	// ── 4. Write the full file — include the already-read sniff bytes via MultiReader
	if _, err := io.Copy(out, io.MultiReader(bytes.NewReader(sniff[:n]), file)); err != nil {
		os.Remove(dst) // clean up partial file
		return "", fmt.Errorf("ошибка записи файла: %w", err)
	}

	return "/uploads/" + unique, nil
}
