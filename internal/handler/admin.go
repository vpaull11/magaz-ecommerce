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

	"github.com/go-chi/chi/v5"
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
	catSvc     *service.CategoryService
	attrRepo   *repository.AttrRepository
	uploadsDir string
}

func NewAdminHandler(
	base *Base,
	productSvc *service.ProductService,
	orderSvc *service.OrderService,
	userRepo *repository.UserRepository,
	catSvc *service.CategoryService,
	attrRepo *repository.AttrRepository,
	uploadsDir string,
) *AdminHandler {
	return &AdminHandler{
		Base:       base,
		productSvc: productSvc,
		orderSvc:   orderSvc,
		userRepo:   userRepo,
		catSvc:     catSvc,
		attrRepo:   attrRepo,
		uploadsDir: uploadsDir,
	}
}

// ─── Categories ───────────────────────────────────────────────────────────────

// GET /admin/categories
func (h *AdminHandler) Categories(w http.ResponseWriter, r *http.Request) {
	// One DB call — flat list sorted by sort_order, name
	flat, _ := h.catSvc.GetFlat()

	// Build lookup by ID
	byID := make(map[int64]*models.Category, len(flat))
	for _, c := range flat {
		byID[c.ID] = c
	}

	// Parent name map for table display
	parentNames := make(map[int64]string, len(flat))
	for _, c := range flat {
		if c.ParentID != nil {
			if p, ok := byID[*c.ParentID]; ok {
				parentNames[c.ID] = p.Name
			}
		} else {
			parentNames[c.ID] = "—"
		}
	}

	// Build ordered list: each root followed immediately by its children
	// (children map: parentID → []*Category)
	children := make(map[int64][]*models.Category)
	var roots []*models.Category
	for _, c := range flat {
		if c.ParentID == nil {
			roots = append(roots, c)
		} else {
			children[*c.ParentID] = append(children[*c.ParentID], c)
		}
	}
	ordered := make([]*models.Category, 0, len(flat))
	for _, root := range roots {
		ordered = append(ordered, root)
		ordered = append(ordered, children[root.ID]...)
	}

	errMsg := r.URL.Query().Get("err")
	h.Render(w, r, "admin_categories.html", map[string]any{
		"Flat":        ordered,
		"ParentNames": parentNames,
		"Error":       errMsg,
	})
}


// POST /admin/categories/new
func (h *AdminHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	seoTitle := strings.TrimSpace(r.FormValue("seo_title"))
	seoDesc := strings.TrimSpace(r.FormValue("seo_description"))
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	var parentID *int64
	if pid, err := strconv.ParseInt(r.FormValue("parent_id"), 10, 64); err == nil && pid > 0 {
		parentID = &pid
	}
	if _, err := h.catSvc.Create(name, slug, parentID, sortOrder, seoTitle, seoDesc); err != nil {
		http.Redirect(w, r, "/admin/categories?err="+err.Error(), http.StatusFound)
		return
	}
	h.Flash(w, r, "Категория создана", "success")
	http.Redirect(w, r, "/admin/categories", http.StatusFound)
}

// POST /admin/categories/{id}/edit
func (h *AdminHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	seoTitle := strings.TrimSpace(r.FormValue("seo_title"))
	seoDesc := strings.TrimSpace(r.FormValue("seo_description"))
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	var parentID *int64
	if pid, err := strconv.ParseInt(r.FormValue("parent_id"), 10, 64); err == nil && pid > 0 {
		parentID = &pid
	}
	if err := h.catSvc.Update(id, name, slug, parentID, sortOrder, seoTitle, seoDesc); err != nil {
		h.Flash(w, r, err.Error(), "error")
	} else {
		h.Flash(w, r, "Категория обновлена", "success")
	}
	http.Redirect(w, r, "/admin/categories", http.StatusFound)
}

// POST /admin/categories/{id}/delete
func (h *AdminHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.catSvc.Delete(id); err != nil {
		h.Flash(w, r, err.Error(), "error")
	} else {
		h.Flash(w, r, "Категория удалена", "success")
	}
	http.Redirect(w, r, "/admin/categories", http.StatusFound)
}

// ─── Attribute Definitions ────────────────────────────────────────────────────

// GET /admin/categories/{id}/attrs
func (h *AdminHandler) CategoryAttrs(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	cat, err := h.catSvc.FindByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defs, _ := h.catSvc.ListDefs(id)
	h.Render(w, r, "admin_attrs.html", map[string]any{
		"Category": cat,
		"Defs":     defs,
	})
}

// POST /admin/categories/{id}/attrs/new
func (h *AdminHandler) CreateAttrDef(w http.ResponseWriter, r *http.Request) {
	catID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	name := strings.TrimSpace(r.FormValue("name"))
	valueType := r.FormValue("value_type")
	if valueType != "number" {
		valueType = "string"
	}
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	if _, err := h.catSvc.CreateDef(catID, name, valueType, sortOrder); err != nil {
		h.Flash(w, r, err.Error(), "error")
	} else {
		h.Flash(w, r, "Атрибут добавлен", "success")
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/categories/%d/attrs", catID), http.StatusFound)
}

// POST /admin/attrs/{id}/delete
func (h *AdminHandler) DeleteAttrDef(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	def, _ := h.attrRepo.FindDef(id)
	_ = h.catSvc.DeleteDef(id)
	h.Flash(w, r, "Атрибут удалён", "success")
	redirectID := int64(0)
	if def != nil {
		redirectID = def.CategoryID
	}
	if redirectID > 0 {
		http.Redirect(w, r, fmt.Sprintf("/admin/categories/%d/attrs", redirectID), http.StatusFound)
	} else {
		http.Redirect(w, r, "/admin/categories", http.StatusFound)
	}
}


// GET /admin
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	orders, total, _ := h.orderSvc.ListAll(1, 5)
	h.Render(w, r, "admin_dashboard.html", map[string]any{"Orders": orders, "Total": total})
}

// GET /admin/settings
func (h *AdminHandler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	settings := h.Settings.GetAll()
	h.Render(w, r, "admin_settings.html", settings)
}

// POST /admin/settings
func (h *AdminHandler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	keys := []string{"site_name", "site_description", "robots_txt"}
	for _, key := range keys {
		val := strings.TrimSpace(r.FormValue(key))
		// For simplicity, we just save them sequentially.
		_ = h.Settings.Set(key, val)
	}

	// Handle og_image upload
	if file, header, err := r.FormFile("og_image"); err == nil {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			filename := fmt.Sprintf("og_%d%s", time.Now().Unix(), ext)
			outPath := filepath.Join(h.uploadsDir, filename)
			if out, err2 := os.Create(outPath); err2 == nil {
				io.Copy(out, file)
				out.Close()
				_ = h.Settings.Set("og_image", "/uploads/"+filename)
			}
		}
		file.Close()
	}

	h.Flash(w, r, "Настройки успешно сохранены", "success")
	http.Redirect(w, r, "/admin/settings", http.StatusFound)
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
	h.Render(w, r, "admin_product_form.html", map[string]any{
		"Categories":   cats,
		"Product":      nil,
		"AttrDefs":     nil,
		"AttrValueMap": map[int64]string{},
	})
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
	p, err2 := h.productSvc.Create(in, imageURL)
	if err2 != nil {
		h.Flash(w, r, err2.Error(), "error")
		http.Redirect(w, r, "/admin/products/new", http.StatusFound)
		return
	}
	// Save attribute values
	attrVals := extractAttrValues(r)
	if len(attrVals) > 0 {
		_ = h.catSvc.SaveProductAttrs(p.ID, attrVals)
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
	// Load attr defs for the product's category
	attrDefs, _ := h.catSvc.ListDefs(p.CategoryID)
	// Load existing attr values and build a map defID -> display value
	attrValues, _ := h.catSvc.GetValuesForProduct(id)
	attrValueMap := make(map[int64]string)
	for _, av := range attrValues {
		attrValueMap[av.AttrDefID] = av.DisplayValue()
	}
	h.Render(w, r, "admin_product_form.html", map[string]any{
		"Categories":   cats,
		"Product":      p,
		"AttrDefs":     attrDefs,
		"AttrValueMap": attrValueMap,
	})
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
		// Save attribute values
		attrVals := extractAttrValues(r)
		if len(attrVals) > 0 {
			_ = h.catSvc.SaveProductAttrs(id, attrVals)
		}
		h.Flash(w, r, "Товар обновлён", "success")
	}
	http.Redirect(w, r, "/admin/products", http.StatusFound)
}

// POST /admin/products/{id}/attrs
func (h *AdminHandler) SaveProductAttrs(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	attrVals := extractAttrValues(r)
	_ = h.catSvc.SaveProductAttrs(id, attrVals)
	h.Flash(w, r, "Характеристики сохранены", "success")
	http.Redirect(w, r, fmt.Sprintf("/admin/products/edit?id=%d", id), http.StatusFound)
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
		Name:           strings.TrimSpace(r.FormValue("name")),
		Description:    r.FormValue("description"),
		Price:          price,
		Stock:          stock,
		CategoryID:     catID,
		IsActive:       r.FormValue("is_active") == "on",
		SeoTitle:       strings.TrimSpace(r.FormValue("seo_title")),
		SeoDescription: strings.TrimSpace(r.FormValue("seo_description")),
	}
}

// extractAttrValues pulls all form fields named "attr_<id>" and returns them as a map.
func extractAttrValues(r *http.Request) map[string]string {
	result := make(map[string]string)
	for key, vals := range r.Form {
		if strings.HasPrefix(key, "attr_") && len(vals) > 0 {
			result[strings.TrimPrefix(key, "attr_")] = vals[0]
		}
	}
	return result
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
