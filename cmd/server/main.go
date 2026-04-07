package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"magaz/internal/config"
	"magaz/internal/db"
	"magaz/internal/handler"
	"magaz/internal/middleware"
	"magaz/internal/repository"
	"magaz/internal/service"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	// ─── Database ──────────────────────────────────────────────────────────
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database, "migrations"); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}

	// ─── Repositories ──────────────────────────────────────────────────────
	userRepo    := repository.NewUserRepository(database)
	productRepo := repository.NewProductRepository(database)
	categoryRepo := repository.NewCategoryRepository(database)
	cartRepo     := repository.NewCartRepository(database)
	orderRepo    := repository.NewOrderRepository(database)
	addressRepo  := repository.NewAddressRepository(database)
	wishlistRepo := repository.NewWishlistRepository(database)
	reviewRepo   := repository.NewReviewRepository(database)
	attrRepo     := repository.NewAttrRepository(database)

	// ─── Services ──────────────────────────────────────────────────────────
	authSvc    := service.NewAuthService(userRepo)
	productSvc := service.NewProductService(productRepo, categoryRepo)
	cartSvc    := service.NewCartService(cartRepo, productRepo)
	catSvc     := service.NewCategoryService(categoryRepo, attrRepo)
	payClient  := service.NewPaymentClient(cfg.PaymentURL, cfg.PaymentSecret)
	orderSvc   := service.NewOrderService(orderRepo, productRepo, cartRepo, addressRepo, payClient, database)

	// ─── Session store ─────────────────────────────────────────────────────
	store := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	// Set secure defaults on the store; SaveUserSession inherits these.
	// Secure=true is disabled in dev mode (no HTTPS locally).
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   !cfg.IsDev(), // true in production (requires HTTPS)
	}

	// ─── Template renderer (base) ──────────────────────────────────────────
	base, err := handler.NewBase(store, cartSvc, "web/templates")
	if err != nil {
		slog.Error("template load failed", "err", err)
		os.Exit(1)
	}

	// ─── Handlers ──────────────────────────────────────────────────────────
	authH     := handler.NewAuthHandler(base, authSvc)
	catalogH  := handler.NewCatalogHandler(base, productSvc, catSvc, categoryRepo)
	wishlistH := handler.NewWishlistHandler(base, wishlistRepo)
	reviewH   := handler.NewReviewHandler(base, reviewRepo)
	cartH     := handler.NewCartHandler(base, cartSvc, addressRepo)
	accountH  := handler.NewAccountHandler(base, authSvc, orderSvc, addressRepo)
	checkoutH := handler.NewOrderCheckoutHandler(base, orderSvc, addressRepo, cartSvc)
	adminH    := handler.NewAdminHandler(base, productSvc, orderSvc, userRepo, catSvc, attrRepo, cfg.UploadsDir)

	// ─── Rate limiters ─────────────────────────────────────────────────────
	// globalLimiter: 300 req/min per IP — blocks floods across all routes
	globalLimiter := middleware.NewRateLimiter(300, time.Minute)
	// authLimiter: 10 req/min — strict limit on login/register endpoints
	authLimiter := middleware.NewRateLimiter(10, time.Minute)

	// ─── CSRF ──────────────────────────────────────────────────────────────
	csrfKey := []byte(cfg.CSRFSecret)
	if len(csrfKey) < 32 {
		slog.Error("CSRF_SECRET must be at least 32 bytes")
		os.Exit(1)
	}
	csrfKey = csrfKey[:32]
	csrfMiddleware := csrf.Protect(csrfKey,
		csrf.Secure(!cfg.IsDev()),
		csrf.SameSite(csrf.SameSiteStrictMode),
	)

	// ─── Router ────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.SecurityHeaders)   // OWASP security headers on every response
	r.Use(globalLimiter.Limit)          // DDoS: global rate limit
	r.Use(middleware.LoadUser(store, authSvc))
	r.Use(csrfMiddleware)

	// Static files — no body size limit needed (GET-only)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))

	// API routes (JSON responses, smaller body limit)
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.MaxBodySize(32 * 1024))
		r.Get("/search", catalogH.SearchAPI) // Live search
		
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Get("/cart", cartH.GetAPI)
			r.Post("/cart/add", cartH.AddAPI)
			r.Post("/cart/update", cartH.UpdateAPI)
			r.Post("/cart/remove", cartH.RemoveAPI)
			r.Post("/wishlist/toggle", wishlistH.ToggleAPI)
			r.Post("/reviews/add", reviewH.AddAPI)
		})
	})

	// Public read-only routes (small body limit — no forms)
	r.Group(func(r chi.Router) {
		r.Use(middleware.MaxBodySize(32 * 1024)) // 32 KB
		r.Get("/", catalogH.Index)
		r.Get("/catalog", catalogH.Catalog)
		r.Get("/catalog/{id}", catalogH.Product)
	})

	// Auth routes — rate-limited + small body (login/register forms only, no uploads)
	r.Route("/auth", func(r chi.Router) {
		r.Use(authLimiter.Limit)
		r.Use(middleware.MaxBodySize(64 * 1024)) // 64 KB — forms only
		r.Get("/register", authH.RegisterPage)
		r.Post("/register", authH.Register)
		r.Get("/login", authH.LoginPage)
		r.Post("/login", authH.Login)
		r.Post("/logout", authH.Logout)
		r.Get("/forgot", authH.ForgotPage)
		r.Post("/forgot", authH.Forgot)
		r.Get("/reset", authH.ResetPage)
		r.Post("/reset", authH.Reset)
	})

	// Protected user routes — medium body limit (form submissions, cart)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth)
		r.Use(middleware.MaxBodySize(256 * 1024)) // 256 KB
		r.Get("/cart", cartH.CartPage)
		r.Post("/cart/add", cartH.Add)
		r.Post("/cart/update", cartH.Update)
		r.Post("/cart/remove", cartH.Remove)
		r.Get("/cart/checkout", cartH.CheckoutPage)
		r.Post("/cart/checkout", checkoutH.Checkout)

		r.Get("/account", accountH.Profile)
		r.Post("/account", accountH.UpdateProfile)
		r.Get("/account/addresses", accountH.Addresses)
		r.Post("/account/addresses", accountH.AddAddress)
		r.Post("/account/addresses/delete", accountH.DeleteAddress)
		r.Get("/account/orders", accountH.Orders)
		r.Get("/account/orders/detail", accountH.OrderDetail)
		r.Get("/wishlist", wishlistH.WishlistPage)
	})

	// Admin routes — large body limit for image uploads
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAdmin)
		r.Use(middleware.MaxBodySize(10 * 1024 * 1024)) // 10 MB for product images
		r.Get("/admin", adminH.Dashboard)
		r.Get("/admin/products", adminH.Products)
		r.Get("/admin/products/new", adminH.NewProductPage)
		r.Post("/admin/products/new", adminH.CreateProduct)
		r.Get("/admin/products/edit", adminH.EditProductPage)
		r.Post("/admin/products/edit", adminH.UpdateProduct)
		r.Post("/admin/products/delete", adminH.DeleteProduct)
		r.Post("/admin/products/{id}/attrs", adminH.SaveProductAttrs)
		r.Get("/admin/orders", adminH.Orders)
		r.Post("/admin/orders/status", adminH.UpdateOrderStatus)
		r.Get("/admin/users", adminH.Users)
		// Categories
		r.Get("/admin/categories", adminH.Categories)
		r.Post("/admin/categories/new", adminH.CreateCategory)
		r.Post("/admin/categories/{id}/edit", adminH.UpdateCategory)
		r.Post("/admin/categories/{id}/delete", adminH.DeleteCategory)
		r.Get("/admin/categories/{id}/attrs", adminH.CategoryAttrs)
		r.Post("/admin/categories/{id}/attrs/new", adminH.CreateAttrDef)
		r.Post("/admin/attrs/{id}/delete", adminH.DeleteAttrDef)
	})

	// ─── HTTP Server ────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("main server starting", "addr", cfg.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
