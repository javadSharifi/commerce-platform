package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service      *Service
	cookieSecure bool
}

func isEnvCookieSecure() bool {
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		return v == "true" || v == "1"
	}
	env := strings.ToLower(os.Getenv("ENV"))
	if env == "production" || env == "prod" {
		return true
	}
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	if appEnv == "production" || appEnv == "prod" {
		return true
	}
	nodeEnv := strings.ToLower(os.Getenv("NODE_ENV"))
	if nodeEnv == "production" || nodeEnv == "prod" {
		return true
	}
	return false
}

func NewHandler(s *Service) *Handler {
	return &Handler{
		service:      s,
		cookieSecure: isEnvCookieSecure(),
	}
}

func (h *Handler) SetCookieSecure(secure bool) {
	h.cookieSecure = secure
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string    `json:"token"`
	User  PanelUser `json:"user"`
}

type meResponse struct {
	User        PanelUser    `json:"user"`
	Memberships []Membership `json:"memberships"`
}

func (h *Handler) MountRoutes(r chi.Router) {
	r.Post("/admin/auth/login", h.handleLogin)

	// Protected routes
	r.Group(func(protected chi.Router) {
		protected.Use(Authenticate(h.service))
		protected.Post("/admin/auth/logout", h.handleLogout)
		protected.Get("/admin/auth/me", h.handleMe)
	})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "فرمت درخواست نامعتبر است.")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "MISSING_FIELDS", "ایمیل و رمز عبور الزامی است.")
		return
	}

	sess, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if err == ErrInvalidCredentials {
			writeJSONError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "ایمیل یا رمز عبور اشتباه است.")
			return
		}
		if err == ErrUserInactive {
			writeJSONError(w, http.StatusForbidden, "ACCOUNT_INACTIVE", "حساب کاربری غیرفعال است.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای داخلی سرور رخ داد.")
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(loginResponse{
		Token: sess.Token,
		User:  sess.User,
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	var token string
	if cookie, err := r.Cookie("session_token"); err == nil {
		token = cookie.Value
	}
	if token != "" {
		_ = h.service.Logout(r.Context(), token)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "خروج با موفقیت انجام شد.",
	})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده است.")
		return
	}

	memberships, err := h.service.ListMemberships(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت لیست عضویت‌ها.")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(meResponse{
		User:        *user,
		Memberships: memberships,
	})
}
