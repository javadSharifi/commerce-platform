package settings

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"psychology/api/internal/auth"
	"psychology/api/internal/domain"
	"psychology/api/internal/platform/tenantctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service               *Service
	domainResolver        *domain.Resolver
	authService           *auth.Service
	appearanceOwnerUserID *uuid.UUID
}

func NewHandler(s *Service, dr *domain.Resolver, as *auth.Service) *Handler {
	return &Handler{
		service:        s,
		domainResolver: dr,
		authService:    as,
	}
}

func (h *Handler) SetAppearanceOwnerUserID(id uuid.UUID) {
	h.appearanceOwnerUserID = &id
}

func (h *Handler) getAppearanceOwnerUserID() (uuid.UUID, bool) {
	if h.appearanceOwnerUserID != nil {
		return *h.appearanceOwnerUserID, true
	}
	raw := strings.TrimSpace(os.Getenv("APPEARANCE_OWNER_USER_ID"))
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

type updateProfileRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Phone       string     `json:"phone"`
	Address     string     `json:"address"`
	LogoMediaID *uuid.UUID `json:"logo_media_id"`
}

type updateAppearanceRequest struct {
	TemplateID  string  `json:"template_id"`
	PaletteID   string  `json:"palette_id"`
	ThemeRadius *string `json:"theme_radius"`
	ThemeScale  *string `json:"theme_scale"`
}

type siteAppearanceResponse struct {
	TemplateID      string `json:"template_id"`
	TemplateVersion string `json:"template_version"`
	PaletteID       string `json:"palette_id"`
	ThemeRadius     string `json:"theme_radius"`
	ThemeScale      string `json:"theme_scale"`
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

		// Appearance routes: requires active membership + matching APPEARANCE_OWNER_USER_ID
		admin.Group(func(appRouter chi.Router) {
			appRouter.Use(h.requireAppearanceOwner())

			appRouter.Get("/admin/site/appearance", h.handleGetAdminAppearance)

			appRouter.Group(func(mutating chi.Router) {
				mutating.Use(auth.CSRFProtection())
				mutating.Put("/admin/site/appearance", h.handleUpdateAdminAppearance)
			})
		})
	})
}

func (h *Handler) requireAppearanceOwner() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده است.")
				return
			}

			ownerID, ok := h.getAppearanceOwnerUserID()
			if !ok {
				writeJSONError(w, http.StatusForbidden, "APPEARANCE_NOT_CONFIGURED", "دسترسی به مدیریت ظاهر در این محیط فعال نشده است.")
				return
			}

			if user.ID != ownerID {
				writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "شناسه کاربری شما مجاز به ویرایش یا مشاهده ظاهر سایت نیست.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
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

	ctx := tenantctx.With(r.Context(), tenantID)
	profile, err := h.service.GetPublicProfile(ctx, tenantID)
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

	w.Header().Set("Cache-Control", "private, no-store")
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

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "خطا در خواندن بدنه درخواست.")
		return
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &rawMap); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	// Strictly reject appearance fields from the content update endpoint (PSY-6.6.2 / PSY-D10)
	appearanceFields := []string{
		"template_id",
		"template_version",
		"palette_id",
		"theme_radius",
		"theme_scale",
		"primary_color",
	}
	for _, field := range appearanceFields {
		if _, exists := rawMap[field]; exists {
			writeJSONError(w, http.StatusBadRequest, "APPEARANCE_FIELDS_NOT_ALLOWED", "مسیر /v1/admin/site فقط فیلدهای محتوایی را می‌پذیرد. فیلدهای ظاهری باید از مسیر اختصاصی /v1/admin/site/appearance ارسال شوند.")
			return
		}
	}

	var req updateProfileRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	updated, err := h.service.UpdateProfile(r.Context(), tenantID, UpdateProfileParams{
		Title:       req.Title,
		Description: req.Description,
		Phone:       req.Phone,
		Address:     req.Address,
		LogoMediaID: req.LogoMediaID,
	})
	if err != nil {
		switch err {
		case ErrEmptyTitle:
			writeJSONError(w, http.StatusBadRequest, "EMPTY_TITLE", "عنوان کلینیک الزامی است.")
			return
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در به‌روزرسانی تنظیمات محتوایی.")
			return
		}
	}

	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (h *Handler) handleGetAdminAppearance(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	app, err := h.service.GetAppearance(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت ظاهر کلینیک.")
		return
	}

	res := siteAppearanceResponse{
		TemplateID:      app.TemplateID,
		TemplateVersion: app.TemplateVersion,
		PaletteID:       app.PaletteID,
		ThemeRadius:     app.ThemeRadius,
		ThemeScale:      app.ThemeScale,
	}

	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

func (h *Handler) handleUpdateAdminAppearance(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	var req updateAppearanceRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_FIELDS", "فرمت بدنه درخواست نامعتبر است یا فیلدهای خارج از قرارداد ارسال شده است.")
		return
	}

	tplID := strings.TrimSpace(req.TemplateID)
	if tplID == "" {
		writeJSONError(w, http.StatusBadRequest, "MISSING_TEMPLATE_ID", "شناسه قالب (template_id) الزامی است.")
		return
	}

	paletteID := strings.TrimSpace(req.PaletteID)
	if paletteID == "" {
		writeJSONError(w, http.StatusBadRequest, "MISSING_PALETTE_ID", "شناسه پالت (palette_id) الزامی است.")
		return
	}

	var radius, scale string
	if req.ThemeRadius != nil {
		radius = *req.ThemeRadius
	}
	if req.ThemeScale != nil {
		scale = *req.ThemeScale
	}

	updated, err := h.service.UpdateAppearance(r.Context(), tenantID, UpdateAppearanceParams{
		TemplateID:  tplID,
		PaletteID:   paletteID,
		ThemeRadius: radius,
		ThemeScale:  scale,
	})
	if err != nil {
		switch err {
		case ErrInvalidTemplateID:
			writeJSONError(w, http.StatusBadRequest, "INVALID_TEMPLATE_ID", "شناسه قالب نامعتبر است.")
			return
		case ErrInvalidPaletteID:
			writeJSONError(w, http.StatusBadRequest, "INVALID_PALETTE_ID", "شناسه پالت نامعتبر است.")
			return
		case ErrInvalidRadius:
			writeJSONError(w, http.StatusBadRequest, "INVALID_RADIUS", "شعاع گوشه‌ها باید یکی از مقادیر sm، md یا lg باشد.")
			return
		case ErrInvalidScale:
			writeJSONError(w, http.StatusBadRequest, "INVALID_SCALE", "مقیاس باید یکی از مقادیر sm، md یا lg باشد.")
			return
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در به‌روزرسانی ظاهر.")
			return
		}
	}

	res := siteAppearanceResponse{
		TemplateID:      updated.TemplateID,
		TemplateVersion: updated.TemplateVersion,
		PaletteID:       updated.PaletteID,
		ThemeRadius:     updated.ThemeRadius,
		ThemeScale:      updated.ThemeScale,
	}

	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}
