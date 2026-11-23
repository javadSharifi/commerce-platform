package courses

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/domain"
	"psychology/api/internal/payments"
	"psychology/api/internal/platform/tenantctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CoursePaymentService interface {
	InitiateCoursePayment(ctx context.Context, p payments.InitiateCoursePaymentParams) (*payments.InitiatePaymentResult, error)
}

type Handler struct {
	buyerService   *BuyerService
	adminService   *AdminCourseService
	domainResolver *domain.Resolver
	authService    *auth.Service
	paymentService CoursePaymentService
}

func NewHandler(buyerSvc *BuyerService, adminSvc *AdminCourseService, dr *domain.Resolver, as *auth.Service, ps ...CoursePaymentService) *Handler {
	h := &Handler{
		buyerService:   buyerSvc,
		adminService:   adminSvc,
		domainResolver: dr,
		authService:    as,
	}
	if len(ps) > 0 {
		h.paymentService = ps[0]
	}
	return h
}

func (h *Handler) SetPaymentService(ps CoursePaymentService) {
	h.paymentService = ps
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
	// Public Courses
	r.Get("/public/courses", h.handleListPublicCourses)
	r.Get("/public/courses/{slug}", h.handleGetPublicCourse)
	r.Get("/public/courses/{slug}/lessons/{lessonId}/preview-stream", h.handleGetPublicLessonPreviewStream)

	// Single-course checkout (authenticated buyer only)
	r.Group(func(chk chi.Router) {
		chk.Use(RequireBuyerAuth(h.buyerService, h.domainResolver))
		chk.Post("/public/courses/{slug}/checkout", h.handleCourseCheckout)
	})

	// Account buyer authentication
	r.Post("/account/auth/otp/send", h.handleSendBuyerOTP)
	r.Post("/account/auth/otp/verify", h.handleVerifyBuyerOTP)

	// Protected buyer routes
	r.Group(func(buyer chi.Router) {
		buyer.Use(RequireBuyerAuth(h.buyerService, h.domainResolver))

		buyer.Post("/account/auth/logout", h.handleBuyerLogout)
		buyer.Get("/account/auth/me", h.handleBuyerMe)
		buyer.Get("/account/courses", h.handleListMyCourses)
		buyer.Get("/account/courses/my", h.handleListMyCourses)
		buyer.Get("/account/courses/{courseId}/lessons/{lessonId}/stream", h.handleGetPrivateLessonStream)
	})

	// Admin Courses & Lessons routes
	r.Group(func(admin chi.Router) {
		if h.authService != nil {
			admin.Use(auth.Authenticate(h.authService))
			admin.Use(auth.RequireTenantMembership(h.authService))
		}

		admin.Group(func(ro chi.Router) {
			if h.authService != nil {
				ro.Use(auth.RequireRole("owner", "content_manager", "receptionist"))
			}

			ro.Get("/admin/courses", h.handleListAdminCourses)
			ro.Get("/admin/courses/{id}", h.handleGetAdminCourse)
			ro.Get("/admin/courses/{id}/lessons", h.handleListAdminCourseLessons)
			ro.Get("/admin/courses/{id}/lessons/{lessonId}", h.handleGetAdminCourseLesson)
			ro.Get("/admin/courses/{id}/entitlements", h.handleListAdminCourseEntitlements)
		})

		admin.Group(func(mut chi.Router) {
			if h.authService != nil {
				mut.Use(auth.RequireRole("owner", "content_manager"))
				mut.Use(auth.CSRFProtection())
			}

			mut.Post("/admin/courses", h.handleCreateAdminCourse)
			mut.Put("/admin/courses/{id}", h.handleUpdateAdminCourse)
			mut.Delete("/admin/courses/{id}", h.handleDeleteAdminCourse)
			mut.Post("/admin/courses/{id}/publish", h.handlePublishAdminCourse)
			mut.Post("/admin/courses/{id}/archive", h.handleArchiveAdminCourse)

			mut.Post("/admin/courses/{id}/lessons", h.handleCreateAdminCourseLesson)
			mut.Put("/admin/courses/{id}/lessons/{lessonId}", h.handleUpdateAdminCourseLesson)
			mut.Delete("/admin/courses/{id}/lessons/{lessonId}", h.handleDeleteAdminCourseLesson)
		})
	})
}

// ------------------------------------------------------------------------------------
// Public Course & Checkout Handlers
// ------------------------------------------------------------------------------------

func (h *Handler) handleListPublicCourses(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveTenantID(r, h.domainResolver)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}
	coursesList, err := h.adminService.ListPublishedCourses(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت لیست دوره‌ها.")
		return
	}
	writeJSON(w, http.StatusOK, coursesList)
}

func (h *Handler) handleGetPublicCourse(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveTenantID(r, h.domainResolver)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}
	slug := chi.URLParam(r, "slug")
	course, err := h.adminService.GetPublicCourseBySlug(r.Context(), tenantID, slug)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			writeJSONError(w, http.StatusNotFound, "COURSE_NOT_FOUND", "دوره آموزشی یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت اطلاعات دوره.")
		return
	}
	writeJSON(w, http.StatusOK, course)
}

func (h *Handler) handleGetPublicLessonPreviewStream(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveTenantID(r, h.domainResolver)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	slug := chi.URLParam(r, "slug")
	lessonIDStr := chi.URLParam(r, "lessonId")
	lessonID, err := uuid.Parse(lessonIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_LESSON_ID", "شناسه جلسه نامعتبر است.")
		return
	}

	out, err := h.buyerService.GetPublicLessonPreviewStreamURL(r.Context(), tenantID, slug, lessonID)
	if err != nil {
		if errors.Is(err, ErrLessonNotFreePreview) {
			writeJSONError(w, http.StatusForbidden, "PREVIEW_FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrCourseNotFound) || errors.Is(err, ErrLessonNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "دوره یا جلسه آموزشی یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در صدور مجوز پخش پیش‌نمایش.")
		return
	}

	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, out)
}

type CourseCheckoutRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	CallbackURL    string `json:"callback_url"`
}

func (h *Handler) handleCourseCheckout(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveTenantID(r, h.domainResolver)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	buyer, ok := BuyerFromContext(r.Context())
	if !ok || buyer == nil {
		writeJSONError(w, http.StatusUnauthorized, "BUYER_UNAUTHORIZED", "برای خرید دوره ابتدا وارد حساب کاربری خود شوید.")
		return
	}

	slug := chi.URLParam(r, "slug")
	course, err := h.adminService.GetPublicCourseBySlug(r.Context(), tenantID, slug)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			writeJSONError(w, http.StatusNotFound, "COURSE_NOT_FOUND", "دوره آموزشی مورد نظر یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای داخلی سرور.")
		return
	}

	var req CourseCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "درخواست نامعتبر است.")
		return
	}

	if h.paymentService == nil {
		writeJSONError(w, http.StatusInternalServerError, "PAYMENT_UNAVAILABLE", "سرویس پرداخت فعال نیست.")
		return
	}

	idempKey := strings.TrimSpace(req.IdempotencyKey)
	if idempKey == "" {
		idempKey = "chk_course_" + uuid.NewString()
	}

	callbackURL := strings.TrimSpace(req.CallbackURL)
	if callbackURL == "" {
		callbackURL = "https://" + r.Host + "/callbacks/payments/zarinpal"
	}

	initRes, err := h.paymentService.InitiateCoursePayment(r.Context(), payments.InitiateCoursePaymentParams{
		TenantID:       tenantID,
		BuyerID:        buyer.ID,
		CourseID:       course.ID,
		IdempotencyKey: idempKey,
		CallbackURL:    callbackURL,
		Now:            time.Now(),
	})
	if err != nil {
		if errors.Is(err, payments.ErrCourseAlreadyEntitled) {
			writeJSONError(w, http.StatusConflict, "ALREADY_ENTITLED", err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, "CHECKOUT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, initRes)
}

// ------------------------------------------------------------------------------------
// Buyer HTTP Handlers
// ------------------------------------------------------------------------------------

type SendOTPRequestBody struct {
	Phone string `json:"phone"`
}

type VerifyOTPRequestBody struct {
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	FullName string `json:"full_name,omitempty"`
}

func (h *Handler) handleSendBuyerOTP(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveTenantID(r, h.domainResolver)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	var req SendOTPRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "درخواست نامعتبر است.")
		return
	}

	res, err := h.buyerService.SendOTP(r.Context(), tenantID, req.Phone)
	if err != nil {
		if err == ErrInvalidPhone {
			writeJSONError(w, http.StatusBadRequest, "INVALID_PHONE", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در ارسال کد تایید.")
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) handleVerifyBuyerOTP(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveTenantID(r, h.domainResolver)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	var req VerifyOTPRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "درخواست نامعتبر است.")
		return
	}

	res, err := h.buyerService.VerifyOTP(r.Context(), tenantID, req.Phone, req.Code, req.FullName)
	if err != nil {
		switch err {
		case ErrInvalidPhone, ErrOTPInvalid:
			writeJSONError(w, http.StatusBadRequest, "INVALID_CODE", err.Error())
		case ErrOTPNotFoundOrExpired:
			writeJSONError(w, http.StatusBadRequest, "EXPIRED_CODE", err.Error())
		case ErrOTPExceededAttempts:
			writeJSONError(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", err.Error())
		case ErrBuyerBlocked:
			writeJSONError(w, http.StatusForbidden, "BUYER_BLOCKED", err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای ورود یا ثبت نام.")
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "buyer_session_token",
		Value:    res.SessionToken,
		Path:     "/",
		Expires:  res.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) handleBuyerLogout(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenantctx.ID(r.Context())
	token := ExtractBearerToken(r)

	_ = h.buyerService.Logout(r.Context(), tenantID, token)

	http.SetCookie(w, &http.Cookie{
		Name:     "buyer_session_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "خروج با موفقیت انجام شد.",
	})
}

func (h *Handler) handleBuyerMe(w http.ResponseWriter, r *http.Request) {
	buyer, ok := BuyerFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "نشست کاربری یافت نشد.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        buyer.ID.String(),
		"phone":     buyer.Phone,
		"full_name": buyer.FullName,
		"status":    buyer.Status,
	})
}

func (h *Handler) handleListMyCourses(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenantctx.ID(r.Context())
	buyer, ok := BuyerFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "نشست کاربری یافت نشد.")
		return
	}

	myList, err := h.buyerService.GetMyCourses(r.Context(), tenantID, buyer.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت لیست دوره‌ها.")
		return
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	writeJSON(w, http.StatusOK, myList)
}

func (h *Handler) handleGetPrivateLessonStream(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveTenantID(r, h.domainResolver)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "TENANT_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	buyer, ok := BuyerFromContext(r.Context())
	if !ok || buyer == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "لطفاً ابتدا وارد حساب کاربری خود شوید.")
		return
	}

	courseIDStr := chi.URLParam(r, "courseId")
	courseID, err := uuid.Parse(courseIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_COURSE_ID", "شناسه دوره نامعتبر است.")
		return
	}

	lessonIDStr := chi.URLParam(r, "lessonId")
	lessonID, err := uuid.Parse(lessonIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_LESSON_ID", "شناسه جلسه نامعتبر است.")
		return
	}

	out, err := h.buyerService.GetPrivateLessonStreamURL(r.Context(), tenantID, buyer.ID, courseID, lessonID)
	if err != nil {
		if errors.Is(err, ErrNotPurchased) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrEntitlementRevoked) {
			writeJSONError(w, http.StatusForbidden, "ENTITLEMENT_REVOKED", err.Error())
			return
		}
		if errors.Is(err, ErrCourseNotPublished) {
			writeJSONError(w, http.StatusForbidden, "COURSE_NOT_PUBLISHED", err.Error())
			return
		}
		if errors.Is(err, ErrCourseNotFound) || errors.Is(err, ErrLessonNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "دوره یا جلسه آموزشی یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در صدور مجوز پخش جلسه.")
		return
	}

	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, out)
}

// ------------------------------------------------------------------------------------
// Admin Courses & Lessons HTTP Handlers
// ------------------------------------------------------------------------------------

func (h *Handler) handleListAdminCourses(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	statusFilter := r.URL.Query().Get("status")
	list, err := h.adminService.ListCourses(r.Context(), tenantID, statusFilter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) handleGetAdminCourse(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}

	course, err := h.adminService.GetCourse(r.Context(), tenantID, courseID)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			writeJSONError(w, http.StatusNotFound, "COURSE_NOT_FOUND", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, course)
}

func (h *Handler) handleCreateAdminCourse(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	var in CreateCourseInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "بدنه درخواست نامعتبر است.")
		return
	}

	c, err := h.adminService.CreateCourse(r.Context(), tenantID, in)
	if err != nil {
		if errors.Is(err, ErrInvalidCoursePrice) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_PRICE", err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) handleUpdateAdminCourse(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}

	var in UpdateCourseInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "بدنه درخواست نامعتبر است.")
		return
	}

	c, err := h.adminService.UpdateCourse(r.Context(), tenantID, courseID, in)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			writeJSONError(w, http.StatusNotFound, "COURSE_NOT_FOUND", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidCoursePrice) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_PRICE", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) handleDeleteAdminCourse(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}

	if err := h.adminService.DeleteCourse(r.Context(), tenantID, courseID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "دوره آموزشی با موفقیت حذف شد."})
}

func (h *Handler) handlePublishAdminCourse(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}

	c, err := h.adminService.PublishCourse(r.Context(), tenantID, courseID)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			writeJSONError(w, http.StatusNotFound, "COURSE_NOT_FOUND", err.Error())
			return
		}
		if errors.Is(err, ErrCannotPublishWithoutLessons) {
			writeJSONError(w, http.StatusBadRequest, "CANNOT_PUBLISH_EMPTY", err.Error())
			return
		}
		if errors.Is(err, ErrCourseArchived) {
			writeJSONError(w, http.StatusBadRequest, "COURSE_ARCHIVED", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "PUBLISH_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) handleArchiveAdminCourse(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}

	c, err := h.adminService.ArchiveCourse(r.Context(), tenantID, courseID)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			writeJSONError(w, http.StatusNotFound, "COURSE_NOT_FOUND", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "ARCHIVE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) handleListAdminCourseLessons(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}

	lessons, err := h.adminService.ListLessons(r.Context(), tenantID, courseID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, lessons)
}

func (h *Handler) handleGetAdminCourseLesson(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}
	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonId"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه درس نامعتبر است.")
		return
	}

	l, err := h.adminService.GetLesson(r.Context(), tenantID, courseID, lessonID)
	if err != nil {
		if errors.Is(err, ErrLessonNotFound) {
			writeJSONError(w, http.StatusNotFound, "LESSON_NOT_FOUND", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, l)
}

func (h *Handler) handleCreateAdminCourseLesson(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}

	var in CreateLessonInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "بدنه درخواست نامعتبر است.")
		return
	}

	l, err := h.adminService.CreateLesson(r.Context(), tenantID, courseID, in)
	if err != nil {
		switch {
		case errors.Is(err, ErrCourseNotFound):
			writeJSONError(w, http.StatusNotFound, "COURSE_NOT_FOUND", err.Error())
		case errors.Is(err, ErrDisallowedMediaAccessClass):
			writeJSONError(w, http.StatusBadRequest, "DISALLOWED_MEDIA_ACCESS_CLASS", err.Error())
		case errors.Is(err, ErrMediaCrossTenantForbidden):
			writeJSONError(w, http.StatusForbidden, "CROSS_TENANT_FORBIDDEN", err.Error())
		case errors.Is(err, ErrMediaNotVerified):
			writeJSONError(w, http.StatusBadRequest, "MEDIA_NOT_VERIFIED", err.Error())
		default:
			writeJSONError(w, http.StatusBadRequest, "CREATE_LESSON_FAILED", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, l)
}

func (h *Handler) handleUpdateAdminCourseLesson(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}
	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonId"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه درس نامعتبر است.")
		return
	}

	var in UpdateLessonInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "بدنه درخواست نامعتبر است.")
		return
	}

	l, err := h.adminService.UpdateLesson(r.Context(), tenantID, courseID, lessonID, in)
	if err != nil {
		switch {
		case errors.Is(err, ErrLessonNotFound):
			writeJSONError(w, http.StatusNotFound, "LESSON_NOT_FOUND", err.Error())
		case errors.Is(err, ErrDisallowedMediaAccessClass):
			writeJSONError(w, http.StatusBadRequest, "DISALLOWED_MEDIA_ACCESS_CLASS", err.Error())
		case errors.Is(err, ErrMediaCrossTenantForbidden):
			writeJSONError(w, http.StatusForbidden, "CROSS_TENANT_FORBIDDEN", err.Error())
		default:
			writeJSONError(w, http.StatusBadRequest, "UPDATE_LESSON_FAILED", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, l)
}

func (h *Handler) handleDeleteAdminCourseLesson(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}
	lessonID, err := uuid.Parse(chi.URLParam(r, "lessonId"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه درس نامعتبر است.")
		return
	}

	if err := h.adminService.DeleteLesson(r.Context(), tenantID, courseID, lessonID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "جلسه آموزشی با موفقیت حذف شد."})
}

func (h *Handler) handleListAdminCourseEntitlements(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "TENANT_REQUIRED", "مستاجر نامشخص است.")
		return
	}

	courseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه دوره نامعتبر است.")
		return
	}

	list, err := h.adminService.ListEntitlements(r.Context(), tenantID, courseID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, list)
}
