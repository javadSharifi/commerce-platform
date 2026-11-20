package blog

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
	// Public blog endpoints (resolved strictly by Host)
	r.Get("/public/blog", h.handleListPublicBlogPosts)
	r.Get("/public/blog/{slug}", h.handleGetPublicBlogPostBySlug)

	// Admin blog management (Owner and Content Manager)
	if h.authService != nil {
		r.Group(func(admin chi.Router) {
			admin.Use(auth.Authenticate(h.authService))
			admin.Use(auth.RequireTenantMembership(h.authService))
			admin.Use(auth.RequireRole("owner", "content_manager"))

			admin.Get("/admin/blog", h.handleListAdminBlogPosts)
			admin.Get("/admin/blog/{id}", h.handleGetAdminBlogPost)

			admin.Group(func(mutating chi.Router) {
				mutating.Use(auth.CSRFProtection())

				mutating.Post("/admin/blog", h.handleCreateAdminBlogPost)
				mutating.Put("/admin/blog/{id}", h.handleUpdateAdminBlogPost)
				mutating.Post("/admin/blog/{id}/publish", h.handlePublishAdminBlogPost)
				mutating.Post("/admin/blog/{id}/unpublish", h.handleUnpublishAdminBlogPost)
				mutating.Delete("/admin/blog/{id}", h.handleDeleteAdminBlogPost)
			})
		})
	}
}

// ---------------- Public Handlers ----------------

func (h *Handler) handleListPublicBlogPosts(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	posts, err := h.service.ListPublishedPosts(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت مقالات.")
		return
	}

	writeJSON(w, http.StatusOK, posts)
}

func (h *Handler) handleGetPublicBlogPostBySlug(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_SLUG", "نامک مقاله الزامی است.")
		return
	}

	post, err := h.service.GetPublishedPostBySlug(r.Context(), tenantID, slug)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "مقاله یافت نشد یا در وضعیت پیش‌نویس است.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت اطلاعات مقاله.")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

// ---------------- Admin Handlers ----------------

func (h *Handler) handleListAdminBlogPosts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	var statusPtr *string
	if statusQ := r.URL.Query().Get("status"); statusQ != "" {
		statusPtr = &statusQ
	}

	posts, err := h.service.ListAdminPosts(r.Context(), tenantID, statusPtr)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت مقالات پنل.")
		return
	}

	writeJSON(w, http.StatusOK, posts)
}

func (h *Handler) handleGetAdminBlogPost(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه مقاله نامعتبر است.")
		return
	}

	post, err := h.service.GetAdminPost(r.Context(), tenantID, postID)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "مقاله مورد نظر یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت مقاله.")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

type createBlogPostReq struct {
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	SeoDescription string         `json:"seo_description"`
	ContentBlocks  []ContentBlock `json:"content_blocks"`
	Status         string         `json:"status"`
}

func (h *Handler) handleCreateAdminBlogPost(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	var req createBlogPostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "فرمت داده‌های ارسالی نامعتبر است.")
		return
	}

	post, err := h.service.CreatePost(r.Context(), CreatePostParams{
		TenantID:       tenantID,
		Title:          req.Title,
		Slug:           req.Slug,
		SeoDescription: req.SeoDescription,
		ContentBlocks:  req.ContentBlocks,
		Status:         req.Status,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidTitle) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_TITLE", "عنوان مقاله الزامی است.")
			return
		}
		if errors.Is(err, ErrInvalidSlug) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_SLUG", "نامک (slug) مقاله فقط باید شامل حروف کوچک انگلیسی، ارقام و خط تیره باشد.")
			return
		}
		if errors.Is(err, ErrDuplicateSlug) {
			writeJSONError(w, http.StatusConflict, "DUPLICATE_SLUG", "این نامک قبلاً برای مقاله دیگری در این کلینیک ثبت شده است.")
			return
		}
		if errors.Is(err, ErrScriptInjection) {
			writeJSONError(w, http.StatusBadRequest, "SCRIPT_INJECTION_FORBIDDEN", "تزریق کدهای جاوااسکریپت و تگ‌های HTML ناامن در محتوا ممنوع است.")
			return
		}
		if errors.Is(err, ErrInvalidContentBlock) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_CONTENT_BLOCK", "ساختار بلوک‌های محتوا نامعتبر است.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در ایجاد مقاله.")
		return
	}

	writeJSON(w, http.StatusCreated, post)
}

type updateBlogPostReq struct {
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	SeoDescription string         `json:"seo_description"`
	ContentBlocks  []ContentBlock `json:"content_blocks"`
}

func (h *Handler) handleUpdateAdminBlogPost(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه مقاله نامعتبر است.")
		return
	}

	var req updateBlogPostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "فرمت داده‌های ارسالی نامعتبر است.")
		return
	}

	post, err := h.service.UpdatePost(r.Context(), UpdatePostParams{
		TenantID:       tenantID,
		ID:             postID,
		Title:          req.Title,
		Slug:           req.Slug,
		SeoDescription: req.SeoDescription,
		ContentBlocks:  req.ContentBlocks,
	})
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "مقاله یافت نشد.")
			return
		}
		if errors.Is(err, ErrInvalidTitle) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_TITLE", "عنوان مقاله الزامی است.")
			return
		}
		if errors.Is(err, ErrInvalidSlug) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_SLUG", "فرمت نامک مقاله نامعتبر است.")
			return
		}
		if errors.Is(err, ErrDuplicateSlug) {
			writeJSONError(w, http.StatusConflict, "DUPLICATE_SLUG", "این نامک قبلاً برای مقاله دیگری ثبت شده است.")
			return
		}
		if errors.Is(err, ErrScriptInjection) {
			writeJSONError(w, http.StatusBadRequest, "SCRIPT_INJECTION_FORBIDDEN", "تزریق تگ‌های اسکریپت یا HTML ناامن ممنوع است.")
			return
		}
		if errors.Is(err, ErrInvalidContentBlock) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_CONTENT_BLOCK", "ساختار بلوک‌های محتوا نامعتبر است.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در به‌روزرسانی مقاله.")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) handlePublishAdminBlogPost(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه مقاله نامعتبر است.")
		return
	}

	post, err := h.service.PublishPost(r.Context(), tenantID, postID)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "مقاله یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در انتشار مقاله.")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) handleUnpublishAdminBlogPost(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه مقاله نامعتبر است.")
		return
	}

	post, err := h.service.UnpublishPost(r.Context(), tenantID, postID)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "مقاله یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در لغو انتشار مقاله.")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) handleDeleteAdminBlogPost(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه مقاله نامعتبر است.")
		return
	}

	if err := h.service.DeletePost(r.Context(), tenantID, postID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در حذف مقاله.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "مقاله با موفقیت حذف گردید."})
}
