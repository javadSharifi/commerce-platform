package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"psychology/api/internal/platform/tenantctx"

	"github.com/google/uuid"
)

type userContextKey struct{}
type roleContextKey struct{}

func WithUser(ctx context.Context, user *PanelUser) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func UserFromContext(ctx context.Context) (*PanelUser, bool) {
	u, ok := ctx.Value(userContextKey{}).(*PanelUser)
	return u, ok && u != nil
}

func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleContextKey{}, role)
}

func RoleFromContext(ctx context.Context) (string, bool) {
	r, ok := ctx.Value(roleContextKey{}).(string)
	return r, ok && r != ""
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
	})
}

// Authenticate verifies the session cookie or Bearer token.
func Authenticate(service *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string

			// 1. Try Cookie
			if cookie, err := r.Cookie("session_token"); err == nil {
				token = cookie.Value
			}

			// 2. Try Authorization header
			if token == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					token = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if token == "" {
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "نشست احراز هویت الزامی است.")
				return
			}

			user, err := service.ValidateSession(r.Context(), token)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "نشست نامعتبر یا منقضی شده است.")
				return
			}

			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireTenantMembership ensures the authenticated user is a member of the selected tenant.
func RequireTenantMembership(service *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده است.")
				return
			}

			rawTenantID := r.Header.Get("X-Tenant-ID")
			if rawTenantID == "" {
				writeJSONError(w, http.StatusBadRequest, "MISSING_TENANT", "شناسه کلینیک (X-Tenant-ID) ارسال نشده است.")
				return
			}

			tenantID, err := uuid.Parse(rawTenantID)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_TENANT_ID", "شناسه کلینیک نامعتبر است.")
				return
			}

			membership, err := service.GetMembership(r.Context(), user.ID, tenantID)
			if err != nil {
				writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "شما عضو این کلینیک یا مطب نیستید.")
				return
			}

			ctx := tenantctx.With(r.Context(), tenantID)
			ctx = WithRole(ctx, membership.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks that the user's role in the current tenant is permitted.
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := RoleFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "نقش کاربری در این کلینیک مشخص نشده است.")
				return
			}

			permitted := false
			for _, allowed := range allowedRoles {
				if role == allowed {
					permitted = true
					break
				}
			}

			if !permitted {
				writeJSONError(w, http.StatusForbidden, "FORBIDDEN_ROLE", "نقش کاربری شما اجازه انجام این عملیات را ندارد.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CSRFProtection guards against cross-site request forgery for state-changing requests.
func CSRFProtection() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// 1. Sec-Fetch-Site check
			secFetchSite := r.Header.Get("Sec-Fetch-Site")
			if secFetchSite == "cross-site" {
				writeJSONError(w, http.StatusForbidden, "CSRF_DETECTED", "درخواست مشکوک به CSRF مسدود شد.")
				return
			}

			// 2. Custom header check (same-origin AJAX requests typically send X-Requested-With or Content-Type: application/json)
			contentType := r.Header.Get("Content-Type")
			if !strings.HasPrefix(contentType, "application/json") {
				writeJSONError(w, http.StatusUnsupportedMediaType, "INVALID_CONTENT_TYPE", "نوع محتوا باید application/json باشد.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
