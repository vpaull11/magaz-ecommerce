package middleware

import (
	"context"
	"net/http"

	"magaz/internal/models"
	"magaz/internal/service"

	"github.com/gorilla/sessions"
)

type contextKey string

const UserContextKey contextKey = "user"

// LoadUser reads the session and attaches the user to the request context.
// Does NOT redirect — use RequireAuth for that.
func LoadUser(store *sessions.CookieStore, authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, _ := store.Get(r, "session")
			id, ok := sess.Values["user_id"].(int64)
			if ok && id > 0 {
				u, err := authSvc.GetByID(id)
				if err == nil {
					r = r.WithContext(context.WithValue(r.Context(), UserContextKey, u))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth redirects to /auth/login if no user in context.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromCtx(r) == nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin requires role == admin, otherwise 403.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromCtx(r)
		if u == nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		if !u.IsAdmin() {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromCtx extracts the user from the request context (may be nil).
func UserFromCtx(r *http.Request) *models.User {
	u, _ := r.Context().Value(UserContextKey).(*models.User)
	return u
}
