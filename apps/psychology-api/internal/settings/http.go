package settings

import (
	"encoding/json"
	"net/http"

	"psychology/api/internal/auth"
	"psychology/api/internal/domain"
	"psychology/api/internal/platform/tenantctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

type updateSettingsRequest struct {
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Phone        string     `json:"phone"`
	Address      string     `json:"address"`
	PrimaryColor string     `json:"primary_color"`
	LogoMediaID  *uuid.UUID `json:"logo_media_id"`
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
	})
}

func (h *Handler) MountRoutes(r chi.Router) {
	// Public route: resolved from Host
	r.Get("/public/site", h.handleGetPublicSite)

	// Admin protected routes
	r.Group(func(admin chi.Router) {
		admin.Use(auth.Authenticate(h.authService))
		admin.Use(auth.RequireTenantMembership(h.authService))

		admin.Get("/admin/site", h.handleGetAdminSite)

		admin.Group(func(mutating chi.Router) {
			mutating.Use(auth.RequireRole("owner", "content_manager"))
			mutating.Use(auth.CSRFProtection())
			mutating.Put("/admin/site", h.handleUpdateAdminSite)
		})
	})
}

func (h *Handler) handleGetPublicSite(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		if err == domain.ErrDomainNotFound {
			writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_FOUND", "دامنه مورد نظر در سیستم ثبت نشده است.")
			return
		}
		if err == domain.ErrDomainNotVerified {
			writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_VERIFIED", "دامنه هنوز تأیید نشده است.")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "INVALID_HOST", "دامنه درخواست نامعتبر است.")
		return
	}

	profile, err := h.service.GetPublicProfile(r.Context(), tenantID)
	if err != nil {
		if err == ErrClinicNotFound {
			writeJSONError(w, http.StatusNotFound, "CLINIC_NOT_FOUND", "کلینیک مورد نظر یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت اطلاعات کلینیک.")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profile)
}

func (h *Handler) handleGetAdminSite(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	settings, err := h.service.GetSettings(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت تنظیمات کلینیک.")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(settings)
}

func (h *Handler) handleUpdateAdminSite(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	var req updateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	updated, err := h.service.UpdateSettings(r.Context(), tenantID, UpdateSettingsParams{
		Title:        req.Title,
		Description:  req.Description,
		Phone:        req.Phone,
		Address:      req.Address,
		PrimaryColor: req.PrimaryColor,
		LogoMediaID:  req.LogoMediaID,
	})
	if err != nil {
		if err == ErrEmptyTitle {
			writeJSONError(w, http.StatusBadRequest, "EMPTY_TITLE", "عنوان کلینیک الزامی است.")
			return
		}
		if err == ErrInvalidColor {
			writeJSONError(w, http.StatusBadRequest, "INVALID_COLOR", "فرمت رنگ باید کد هگز معتبر باشد (مانند #0284c7).")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در به‌روزرسانی تنظیمات.")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}
