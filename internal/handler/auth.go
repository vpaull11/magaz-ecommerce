package handler

import (
	"errors"
	"net/http"

	"magaz/internal/service"
)

type AuthHandler struct {
	*Base
	authSvc *service.AuthService
}

func NewAuthHandler(base *Base, authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{Base: base, authSvc: authSvc}
}

// GET /auth/register
func (h *AuthHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, "register.html", nil)
}

// POST /auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.Render(w, r, "register.html", map[string]string{"error": "Ошибка формы"})
		return
	}
	in := service.RegisterInput{
		Email:    r.FormValue("email"),
		Name:     r.FormValue("name"),
		Password: r.FormValue("password"),
	}
	confirm := r.FormValue("password_confirm")
	if in.Password != confirm {
		h.Render(w, r, "register.html", map[string]string{"error": "Пароли не совпадают", "email": in.Email, "name": in.Name})
		return
	}

	u, err := h.authSvc.Register(in)
	if err != nil {
		h.Render(w, r, "register.html", map[string]string{"error": err.Error(), "email": in.Email, "name": in.Name})
		return
	}

	if err := SaveUserSession(w, r, h.Store, u); err != nil {
		h.Render(w, r, "register.html", map[string]string{"error": "Ошибка сессии"})
		return
	}
	h.Flash(w, r, "Добро пожаловать, "+u.Name+"!", "success")
	Redirect(w, r, "/")
}

// GET /auth/login
func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, "login.html", nil)
}

// POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.Render(w, r, "login.html", map[string]string{"error": "Ошибка формы"})
		return
	}
	email := r.FormValue("email")
	password := r.FormValue("password")

	u, err := h.authSvc.Login(email, password)
	if err != nil {
		h.Render(w, r, "login.html", map[string]string{"error": err.Error(), "email": email})
		return
	}

	if err := SaveUserSession(w, r, h.Store, u); err != nil {
		h.Render(w, r, "login.html", map[string]string{"error": "Ошибка сессии"})
		return
	}
	h.Flash(w, r, "С возвращением, "+u.Name+"!", "success")
	Redirect(w, r, "/")
}

// POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	DestroySession(w, r, h.Store)
	Redirect(w, r, "/auth/login")
}

// GET /auth/forgot
func (h *AuthHandler) ForgotPage(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, "forgot.html", nil)
}

// POST /auth/forgot
func (h *AuthHandler) Forgot(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	if email == "" {
		h.Render(w, r, "forgot.html", map[string]string{"error": "Укажите email"})
		return
	}
	// Always return success to avoid email enumeration
	_ = h.authSvc.ForgotPassword(email)
	h.Render(w, r, "forgot.html", map[string]string{
		"success": "Если такой аккаунт существует, ссылка для сброса была отправлена (или записана в лог сервера).",
	})
}

// GET /auth/reset?token=xxx
func (h *AuthHandler) ResetPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.Flash(w, r, "Недействительная ссылка для сброса пароля", "error")
		Redirect(w, r, "/auth/forgot")
		return
	}
	h.Render(w, r, "reset.html", map[string]string{"token": token})
}

// POST /auth/reset
func (h *AuthHandler) Reset(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	password := r.FormValue("password")
	confirm := r.FormValue("password_confirm")

	if password != confirm {
		h.Render(w, r, "reset.html", map[string]string{"token": token, "error": "Пароли не совпадают"})
		return
	}

	err := h.authSvc.ResetPassword(token, password)
	if errors.Is(err, service.ErrInvalidToken) || errors.Is(err, service.ErrWeakPassword) {
		h.Render(w, r, "reset.html", map[string]string{"token": token, "error": err.Error()})
		return
	}
	if err != nil {
		h.Render(w, r, "reset.html", map[string]string{"token": token, "error": "Произошла ошибка. Попробуйте снова."})
		return
	}

	h.Flash(w, r, "Пароль успешно изменён. Войдите в аккаунт.", "success")
	Redirect(w, r, "/auth/login")
}
