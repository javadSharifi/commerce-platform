package media

import (
	"encoding/json"
	"errors"
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
	// Public streaming endpoints (Host-resolved)
	r.Get("/public/media/{id}/stream-url", h.handleGetPublicStreamURL)
	r.Get("/public/media/{id}/stream", h.handleGetPublicStreamURL)
	r.Get("/public/media/{id}/image", h.handleGetPublicImage)

	// Admin media management (Owner and Content Manager)
	if h.authService != nil {
		r.Group(func(admin chi.Router) {
			admin.Use(auth.Authenticate(h.authService))
			admin.Use(auth.RequireTenantMembership(h.authService))
			admin.Use(auth.RequireRole("owner", "content_manager"))

			admin.Get("/admin/media", h.handleListMedia)

			admin.Group(func(mutating chi.Router) {
				mutating.Use(auth.CSRFProtection())

				mutating.Post("/admin/media/upload-url", h.handleRequestUploadURL)
				mutating.Post("/admin/media/finalize", h.handleFinalizeUpload)
			})
		})
	}
}

// ---------------- Public Stream Handler ----------------

func (h *Handler) handleGetPublicStreamURL(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	rawID := chi.URLParam(r, "id")
	assetID, err := uuid.Parse(rawID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه رسانه نامعتبر است.")
		return
	}

	res, err := h.service.GetPublicStreamURL(r.Context(), tenantID, assetID)
	if err != nil {
		switch {
		case errors.Is(err, ErrMediaNotFound):
			writeJSONError(w, http.StatusNotFound, "MEDIA_NOT_FOUND", "فایل رسانه‌ای یافت نشد.")
		case errors.Is(err, ErrMediaNotVerified):
			writeJSONError(w, http.StatusForbidden, "MEDIA_NOT_VERIFIED", "فایل رسانه‌ای هنوز تأیید نشده است.")
		case errors.Is(err, ErrMediaNotPublic):
			writeJSONError(w, http.StatusForbidden, "MEDIA_NOT_PUBLIC", "این فایل عمومی نیست و نیاز به مجوز دسترسی دارد.")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در تولید پیوند پخش.")
		}
		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) handleGetPublicImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	assetID, idErr := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || idErr != nil {
		http.NotFound(w, r)
		return
	}
	location, err := h.service.GetPublicImageURL(r.Context(), tenantID, assetID)
	switch {
	case err == nil:
		http.Redirect(w, r, location, http.StatusFound)
	case errors.Is(err, ErrMediaNotFound), errors.Is(err, ErrMediaNotVerified),
		errors.Is(err, ErrMediaNotPublic), errors.Is(err, ErrInvalidMimeType),
		errors.Is(err, ErrFileSizeExceeded), errors.Is(err, ErrInvalidFileSize):
		http.NotFound(w, r)
	default:
		writeJSONError(w, http.StatusBadGateway, "IMAGE_UNAVAILABLE", "تصویر موقتاً در دسترس نیست.")
	}
}

// ---------------- Admin Handlers ----------------

type requestUploadURLReq struct {
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	AccessClass  string `json:"access_class"`
}

func (h *Handler) handleRequestUploadURL(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده است.")
		return
	}

	var req requestUploadURLReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "فرمت داده‌های ارسالی نامعتبر است.")
		return
	}

	res, err := h.service.RequestStagingUpload(r.Context(), tenantID, user.ID, RequestUploadInput{
		OriginalName: req.OriginalName,
		MimeType:     req.MimeType,
		SizeBytes:    req.SizeBytes,
		AccessClass:  req.AccessClass,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrQuotaExceeded):
			writeJSONError(w, http.StatusForbidden, "QUOTA_EXCEEDED", "سهمیه کلینیک پر شده است (سقف ۱۰ گیگابایت).")
		case errors.Is(err, ErrInvalidAccessClass):
			writeJSONError(w, http.StatusBadRequest, "INVALID_ACCESS_CLASS", "کلاس دسترسی باید public یا course_private باشد.")
		case errors.Is(err, ErrInvalidMimeType):
			writeJSONError(w, http.StatusBadRequest, "INVALID_MIME_TYPE", "فرمت فایل مجاز نیست.")
		case errors.Is(err, ErrFileSizeExceeded):
			writeJSONError(w, http.StatusBadRequest, "FILE_TOO_LARGE", "حجم فایل بیش از حد مجاز برای این دسته است.")
		case errors.Is(err, ErrInvalidFileSize):
			writeJSONError(w, http.StatusBadRequest, "INVALID_FILE_SIZE", "حجم فایل باید بزرگتر از صفر باشد.")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در صدور مجوز آپلود.")
		}
		return
	}

	writeJSON(w, http.StatusCreated, res)
}

type finalizeUploadReq struct {
	UploadID uuid.UUID `json:"upload_id"`
}

func (h *Handler) handleFinalizeUpload(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده است.")
		return
	}

	var req finalizeUploadReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "فرمت داده‌های ارسالی نامعتبر است.")
		return
	}

	res, err := h.service.FinalizeUpload(r.Context(), tenantID, user.ID, FinalizeUploadInput{
		UploadID: req.UploadID,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrUploadNotFound):
			writeJSONError(w, http.StatusNotFound, "UPLOAD_NOT_FOUND", "شناسه آپلود یافت نشد.")
		case errors.Is(err, ErrUploadExpired):
			writeJSONError(w, http.StatusBadRequest, "UPLOAD_EXPIRED", "مهلت آپلود به پایان رسیده است.")
		case errors.Is(err, ErrUploadNotReserved):
			writeJSONError(w, http.StatusBadRequest, "UPLOAD_ALREADY_COMPLETED", "این آپلود قبلاً نهایی شده است.")
		case errors.Is(err, ErrStorageSizeMismatch):
			writeJSONError(w, http.StatusBadRequest, "SIZE_MISMATCH", "حجم فایل آپلود شده بیش از حد مجاز است.")
		case errors.Is(err, ErrStorageMimeMismatch):
			writeJSONError(w, http.StatusBadRequest, "MIME_MISMATCH", "نوع فایل آپلود شده مطابقت ندارد.")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در نهایی‌سازی فایل.")
		}
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) handleListMedia(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	res, err := h.service.ListMedia(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت فهرست رسانه‌ها.")
		return
	}

	writeJSON(w, http.StatusOK, res)
}
