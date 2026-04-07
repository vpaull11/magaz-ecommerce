package handler

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"

	"magaz/internal/middleware"
	"magaz/internal/models"
	"magaz/internal/service"

	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
)

// Base holds common dependencies shared by all handlers.
type Base struct {
	Store     *sessions.CookieStore
	CartSvc   *service.CartService
	templates map[string]*template.Template
}

// pageMap maps template name → relative path under tmplDir.
var pageMap = []struct{ name, path string }{
	{"index.html", "catalog/index.html"},
	{"catalog.html", "catalog/catalog.html"},
	{"product.html", "catalog/product.html"},
	{"register.html", "auth/register.html"},
	{"login.html", "auth/login.html"},
	{"forgot.html", "auth/forgot.html"},
	{"reset.html", "auth/reset.html"},
	{"cart.html", "cart/cart.html"},
	{"checkout.html", "cart/checkout.html"},
	{"profile.html", "account/profile.html"},
	{"addresses.html", "account/addresses.html"},
	{"orders.html", "account/orders.html"},
	{"order_detail.html", "account/order_detail.html"},
	{"admin_dashboard.html", "admin/admin_dashboard.html"},
	{"admin_products.html", "admin/admin_products.html"},
	{"admin_product_form.html", "admin/admin_product_form.html"},
	{"admin_orders.html", "admin/admin_orders.html"},
	{"admin_users.html", "admin/admin_users.html"},
}

func NewBase(store *sessions.CookieStore, cartSvc *service.CartService, tmplDir string) (*Base, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		// mulfi: price (float64) × quantity (int)
		"mulfi": func(a float64, b int) float64 { return a * float64(b) },
		"string": func(v any) string { return fmt.Sprint(v) },
		"statusLabel": func(s models.OrderStatus) string { return s.Label() },
		"addrFull": func(a *models.Address) string { return a.Full() },
		"seq": func(start, end int) []int {
			s := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
	}

	base := filepath.Join(tmplDir, "base.html")
	partials := filepath.Join(tmplDir, "_partials.html")

	templates := make(map[string]*template.Template, len(pageMap))
	for _, p := range pageMap {
		t, err := template.New("").Funcs(funcMap).ParseFiles(
			base,
			partials,
			filepath.Join(tmplDir, p.path),
		)
		if err != nil {
			return nil, fmt.Errorf("load template %q: %w", p.name, err)
		}
		templates[p.name] = t
	}

	return &Base{Store: store, CartSvc: cartSvc, templates: templates}, nil
}

// PageData is the top-level struct passed to every template.
type PageData struct {
	User      *models.User
	CartCount int
	CartTotal float64
	Flash     string
	FlashType string // "success" | "error" | "info"
	CSRF      template.HTML
	CSRFValue string
	Data      any
}

func (b *Base) render(w http.ResponseWriter, r *http.Request, tmplName string, data any, status int) {
	t, ok := b.templates[tmplName]
	if !ok {
		// Log internal template name but never expose it to the user.
		slog.Error("template not found", "template", tmplName)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}

	u := middleware.UserFromCtx(r)
	pd := PageData{
		User: u,
		CSRF: csrf.TemplateField(r),
		CSRFValue: csrf.Token(r),
		Data: data,
	}
	if u != nil {
		if summary, err := b.CartSvc.Get(u.ID); err == nil {
			pd.CartCount = summary.Count
			pd.CartTotal = summary.Total
		} else {
			pd.CartCount = b.CartSvc.Count(u.ID)
		}
	}

	// Consume flash from session
	sess, _ := b.Store.Get(r, "session")
	if f, ok := sess.Values["flash"].(string); ok && f != "" {
		pd.Flash = f
		pd.FlashType, _ = sess.Values["flash_type"].(string)
		delete(sess.Values, "flash")
		delete(sess.Values, "flash_type")
		sess.Save(r, w)
	}

	w.WriteHeader(status)
	// Always execute the "base.html" layout; page-specific {{define "content"}} overrides the block.
	if err := t.ExecuteTemplate(w, "base.html", pd); err != nil {
		// Header already written; log the internal error but don't expose details.
		slog.Error("template execute error", "template", tmplName, "err", err)
	}
}

func (b *Base) Render(w http.ResponseWriter, r *http.Request, name string, data any) {
	b.render(w, r, name, data, http.StatusOK)
}

func (b *Base) RenderStatus(w http.ResponseWriter, r *http.Request, name string, data any, status int) {
	b.render(w, r, name, data, status)
}

// Flash stores a one-time notification in the session.
func (b *Base) Flash(w http.ResponseWriter, r *http.Request, msg, kind string) {
	sess, _ := b.Store.Get(r, "session")
	sess.Values["flash"] = msg
	sess.Values["flash_type"] = kind
	sess.Save(r, w)
}

// Redirect is a helper for http.Redirect with 302.
func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusFound)
}

// SessionUserID reads the user ID from the gorilla session.
func SessionUserID(r *http.Request, store *sessions.CookieStore) (int64, bool) {
	sess, _ := store.Get(r, "session")
	id, ok := sess.Values["user_id"].(int64)
	return id, ok && id > 0
}

// SaveUserSession persists user ID and role in the session cookie.
// Cookie options (Secure, HttpOnly, SameSite) are inherited from store.Options
// which is configured at startup in main.go — this ensures the Secure flag
// is correctly set based on the runtime environment (dev vs production).
func SaveUserSession(w http.ResponseWriter, r *http.Request, store *sessions.CookieStore, u *models.User) error {
	sess, _ := store.Get(r, "session")
	sess.Values["user_id"] = u.ID
	sess.Values["user_role"] = string(u.Role)
	// Copy store-level options, then override MaxAge only.
	opts := *store.Options
	opts.MaxAge = 86400 * 30 // 30 days
	sess.Options = &opts
	return sess.Save(r, w)
}

// DestroySession clears the session cookie on logout.
func DestroySession(w http.ResponseWriter, r *http.Request, store *sessions.CookieStore) error {
	sess, _ := store.Get(r, "session")
	// Copy store options and set MaxAge=-1 to instruct browser to delete the cookie.
	opts := *store.Options
	opts.MaxAge = -1
	sess.Options = &opts
	return sess.Save(r, w)
}
