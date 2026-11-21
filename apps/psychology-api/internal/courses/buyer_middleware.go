package courses

import (
	"context"
	"net/http"
	"strings"

	"psychology/api/internal/domain"
	"psychology/api/internal/platform/tenantctx"

	"github.com/google/uuid"
)

type buyerCtxKey struct{}

// WithBuyerContext injects the authenticated buyer account into the context.
func WithBuyerContext(ctx context.Context, buyer *BuyerAccount) context.Context {
	return context.WithValue(ctx, buyerCtxKey{}, buyer)
}

// BuyerFromContext extracts the authenticated buyer account from the context.
func BuyerFromContext(ctx context.Context) (*BuyerAccount, bool) {
	b, ok := ctx.Value(buyerCtxKey{}).(*BuyerAccount)
	return b, ok && b != nil
}

// ExtractBearerToken extracts token from 'Authorization: Bearer <token>' or fallback cookie.
func ExtractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if cookie, err := r.Cookie("buyer_session_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

// RequireBuyerAuth ensures a valid tenant-scoped buyer session is present.
func RequireBuyerAuth(svc *BuyerService, resolver *domain.Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ExtractBearerToken(r)
			if token == "" {
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "لطفاً ابتدا وارد حساب کاربری خود شوید.")
				return
			}

			// Resolve tenant ID
			tenantID, ok := tenantctx.ID(r.Context())
			if !ok {
				if resolver == nil {
					writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر مشخص نشده است.")
					return
				}
				resolved, err := resolver.Resolve(r.Context(), r.Host)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "دامنه کلینیک یافت نشد.")
					return
				}
				tenantID = resolved
			}

			buyer, err := svc.AuthenticateSession(r.Context(), tenantID, token)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
				return
			}

			ctx := WithBuyerContext(r.Context(), buyer)
			// Ensure tenantctx is also attached
			ctx = tenantctx.With(ctx, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveTenantID(r *http.Request, resolver *domain.Resolver) (uuid.UUID, error) {
	if tid, ok := tenantctx.ID(r.Context()); ok {
		return tid, nil
	}
	if resolver != nil {
		return resolver.Resolve(r.Context(), r.Host)
	}
	return uuid.Nil, domain.ErrDomainNotFound
}
