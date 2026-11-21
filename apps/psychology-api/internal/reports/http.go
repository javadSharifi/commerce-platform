package reports

import (
	"encoding/json"
	"net/http"

	"psychology/api/internal/auth"
	"psychology/api/internal/domain"
	"psychology/api/internal/platform/tenantctx"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service        *Service
	domainResolver *domain.Resolver
	authService    *auth.Service
}

func NewHandler(s *Service, dr *domain.Resolver, as *auth.Service) *Handler {
	return &Handler{
		service:        s,
		domainResolver: dr,
		authService:    as,
	}
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) MountRoutes(r chi.Router) {
	r.Group(func(admin chi.Router) {
		if h.authService != nil {
			admin.Use(auth.Authenticate(h.authService))
			admin.Use(auth.RequireTenantMembership(h.authService))
		}

		admin.Group(func(owner chi.Router) {
			if h.authService != nil {
				owner.Use(auth.RequireRole("owner"))
			}

			owner.Get("/admin/reports/metrics", h.handleGetTenantMetrics)
		})
	})
}

func (h *Handler) handleGetTenantMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "MISSING_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	metrics, err := h.service.GetTenantMetrics(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در استخراج شاخص‌های کلینیک.")
		return
	}

	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, metrics)
}
