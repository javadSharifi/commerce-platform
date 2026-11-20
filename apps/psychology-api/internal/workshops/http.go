package workshops

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/domain"
	"psychology/api/internal/payments"
	"psychology/api/internal/platform/tenantctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PaymentInitiator interface {
	InitiateWorkshopPayment(ctx context.Context, p payments.InitiateWorkshopPaymentParams) (*payments.InitiatePaymentResult, error)
}

type Handler struct {
	service        *Service
	domainResolver *domain.Resolver
	authService    *auth.Service
	paymentService PaymentInitiator
}

func (h *Handler) SetPaymentService(ps PaymentInitiator) {
	h.paymentService = ps
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
	// Public Routes (Resolved by Host Domain)
	r.Get("/public/workshops", h.handleListPublicWorkshops)
	r.Get("/public/workshops/{id}", h.handleGetPublicWorkshopDetail)
	r.Post("/public/workshops/{id}/register", h.handleRegisterPublicWorkshop)
	r.Get("/public/workshops/registrations/{id}", h.handleGetPublicRegistrationStatus)

	// Admin Routes (Authenticated with Session and Tenant Membership)
	r.Group(func(admin chi.Router) {
		admin.Use(auth.Authenticate(h.authService))
		admin.Use(auth.RequireTenantMembership(h.authService))

		admin.Get("/admin/workshops", h.handleListAdminWorkshops)
		admin.Get("/admin/workshops/{id}", h.handleGetAdminWorkshopDetail)

		// Receptionist or Owner: View & manage attendees
		admin.Group(func(reception chi.Router) {
			reception.Use(auth.RequireRole("owner", "receptionist"))
			reception.Get("/admin/workshops/{id}/registrations", h.handleListAdminRegistrations)

			reception.Group(func(mutating chi.Router) {
				mutating.Use(auth.CSRFProtection())
				mutating.Post("/admin/workshops/{id}/registrations/{reg_id}/cancel", h.handleCancelAdminRegistration)
			})
		})

		// Content Manager or Owner: Create, Edit, Publish, Close
		admin.Group(func(mutating chi.Router) {
			mutating.Use(auth.RequireRole("owner", "content_manager"))
			mutating.Use(auth.CSRFProtection())

			mutating.Post("/admin/workshops", h.handleCreateAdminWorkshop)
			mutating.Put("/admin/workshops/{id}", h.handleUpdateAdminWorkshop)
			mutating.Post("/admin/workshops/{id}/publish", h.handlePublishAdminWorkshop)
			mutating.Post("/admin/workshops/{id}/close", h.handleCloseAdminWorkshop)
		})

		// Owner only: Workshop cancellation
		admin.Group(func(ownerOnly chi.Router) {
			ownerOnly.Use(auth.RequireRole("owner"))
			ownerOnly.Use(auth.CSRFProtection())

			ownerOnly.Post("/admin/workshops/{id}/cancel", h.handleCancelAdminWorkshop)
		})
	})
}

// ----------------------------------------------------
// Public Handlers
// ----------------------------------------------------

func (h *Handler) handleListPublicWorkshops(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		if errors.Is(err, domain.ErrDomainNotFound) {
			writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_FOUND", "دامنه در سیستم ثبت نشده است.")
			return
		}
		if errors.Is(err, domain.ErrDomainNotVerified) {
			writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_VERIFIED", "دامنه هنوز تأیید نشده است.")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "INVALID_HOST", "دامنه نامعتبر است.")
		return
	}

	items, err := h.service.ListPublishedWorkshops(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت لیست کارگاه‌ها.")
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetPublicWorkshopDetail(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_FOUND", "دامنه یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه کارگاه نامعتبر است.")
		return
	}

	detail, err := h.service.GetPublicWorkshopDetail(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrWorkshopNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "کارگاه مورد نظر یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت اطلاعات کارگاه.")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

type registerWorkshopReq struct {
	GuestName  string `json:"guest_name"`
	GuestPhone string `json:"guest_phone"`
}

type registerWorkshopResp struct {
	RegistrationID string     `json:"registration_id"`
	Status         string     `json:"status"`
	LookupToken    string     `json:"lookup_token"`
	PaymentURL     *string    `json:"payment_url,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

func (h *Handler) handleRegisterPublicWorkshop(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	workshopID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه کارگاه نامعتبر است.")
		return
	}

	var req registerWorkshopReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "فرمت داده‌های ارسالی نامعتبر است.")
		return
	}

	callerToken := r.Header.Get("X-Guest-Token")
	if callerToken == "" {
		callerToken = r.URL.Query().Get("token")
	}

	result, err := h.service.RegisterGuest(r.Context(), tenantID, workshopID, req.GuestName, req.GuestPhone, callerToken, time.Now())
	if err != nil {
		if errors.Is(err, ErrWorkshopNotFound) || errors.Is(err, ErrWorkshopNotPublished) {
			writeJSONError(w, http.StatusNotFound, "WORKSHOP_NOT_FOUND", "کارگاه مورد نظر یافت نشد یا ثبت‌نام آن فعال نیست.")
			return
		}
		if errors.Is(err, ErrRegistrationDeadlinePassed) {
			writeJSONError(w, http.StatusBadRequest, "DEADLINE_PASSED", "مهلت ثبت‌نام در این کارگاه به پایان رسیده است.")
			return
		}
		if errors.Is(err, ErrCapacityFull) {
			writeJSONError(w, http.StatusConflict, "CAPACITY_FULL", "ظرفیت کارگاه تکمیل شده است (۰ صندلی موجود).")
			return
		}
		if errors.Is(err, ErrDuplicateActiveRegistration) {
			writeJSONError(w, http.StatusConflict, "DUPLICATE_REGISTRATION", "یک ثبت‌نام فعال با این شماره تماس از قبل وجود دارد.")
			return
		}
		if errors.Is(err, ErrInvalidPhone) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_PHONE", "فرمت شماره موبایل نامعتبر است (مثال: 09123456789).")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در ثبت‌نام کارگاه.")
		return
	}

	var paymentURL *string
	if h.paymentService != nil && !result.IsFree {
		initRes, err := h.paymentService.InitiateWorkshopPayment(r.Context(), payments.InitiateWorkshopPaymentParams{
			TenantID:       tenantID,
			RegistrationID: result.RegistrationID,
			IdempotencyKey: "reg_" + result.RegistrationID.String(),
			CallbackURL:    "https://" + r.Host + "/callbacks/payments/zarinpal",
		})
		if err == nil {
			paymentURL = &initRes.PaymentURL
		}
	}

	resp := registerWorkshopResp{
		RegistrationID: result.RegistrationID.String(),
		Status:         result.Status,
		LookupToken:    result.LookupToken,
		PaymentURL:     paymentURL,
		ExpiresAt:      result.ExpiresAt,
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) handleGetPublicRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "DOMAIN_NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	regID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه ثبت‌نام نامعتبر است.")
		return
	}

	lookupToken := r.URL.Query().Get("lookup_token")
	if lookupToken == "" {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "ثبت‌نام یافت نشد.")
		return
	}

	status, err := h.service.GetRegistrationStatus(r.Context(), tenantID, regID, lookupToken)
	if err != nil {
		if errors.Is(err, ErrRegistrationNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "اطلاعات ثبت‌نام یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت وضعیت ثبت‌نام.")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// ----------------------------------------------------
// Admin Handlers
// ----------------------------------------------------

func (h *Handler) handleListAdminWorkshops(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	var statusPtr *string
	if statusQ := r.URL.Query().Get("status"); statusQ != "" {
		statusPtr = &statusQ
	}

	items, err := h.service.ListAdminWorkshops(r.Context(), tenantID, statusPtr)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت لیست کارگاه‌ها.")
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetAdminWorkshopDetail(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	detail, err := h.service.GetAdminWorkshopDetail(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrWorkshopNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "کارگاه یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت اطلاعات کارگاه.")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

type createWorkshopReq struct {
	Title                string    `json:"title"`
	Slug                 string    `json:"slug"`
	Description          string    `json:"description"`
	InstructorName       string    `json:"instructor_name"`
	StartsAt             time.Time `json:"starts_at"`
	EndsAt               time.Time `json:"ends_at"`
	Location             string    `json:"location"`
	RegistrationDeadline time.Time `json:"registration_deadline"`
	Capacity             int       `json:"capacity"`
	PriceRials           int64     `json:"price_rials"`
}

func (h *Handler) handleCreateAdminWorkshop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	var req createWorkshopReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "فرمت داده نامعتبر است.")
		return
	}

	if req.Title == "" || req.Slug == "" || req.InstructorName == "" || req.Location == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "تمامی فیلدهای الزامی باید پر شوند.")
		return
	}

	ws, err := h.service.CreateWorkshop(r.Context(), tenantID, CreateWorkshopParams{
		Title:                req.Title,
		Slug:                 req.Slug,
		Description:          req.Description,
		InstructorName:       req.InstructorName,
		StartsAt:             req.StartsAt,
		EndsAt:               req.EndsAt,
		Location:             req.Location,
		RegistrationDeadline: req.RegistrationDeadline,
		Capacity:             req.Capacity,
		PriceRials:           req.PriceRials,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidDates) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_DATES", "تاریخ‌ها نامعتبرند: پایان باید بعد از شروع باشد و مهلت ثبت‌نام قبل از شروع.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در ایجاد کارگاه.")
		return
	}

	detail, err := h.service.GetAdminWorkshopDetail(r.Context(), tenantID, ws.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت جزئیات کارگاه ایجادشده.")
		return
	}

	writeJSON(w, http.StatusCreated, detail)
}

type updateWorkshopReq struct {
	Title                *string    `json:"title"`
	Slug                 *string    `json:"slug"`
	Description          *string    `json:"description"`
	InstructorName       *string    `json:"instructor_name"`
	StartsAt             *time.Time `json:"starts_at"`
	EndsAt               *time.Time `json:"ends_at"`
	Location             *string    `json:"location"`
	RegistrationDeadline *time.Time `json:"registration_deadline"`
	Capacity             *int       `json:"capacity"`
	PriceRials           *int64     `json:"price_rials"`
}

func (h *Handler) handleUpdateAdminWorkshop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	var req updateWorkshopReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "فرمت داده نامعتبر است.")
		return
	}

	detail, err := h.service.UpdateWorkshop(r.Context(), tenantID, id, UpdateWorkshopParams{
		Title:                req.Title,
		Slug:                 req.Slug,
		Description:          req.Description,
		InstructorName:       req.InstructorName,
		StartsAt:             req.StartsAt,
		EndsAt:               req.EndsAt,
		Location:             req.Location,
		RegistrationDeadline: req.RegistrationDeadline,
		Capacity:             req.Capacity,
		PriceRials:           req.PriceRials,
	})
	if err != nil {
		if errors.Is(err, ErrWorkshopNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "کارگاه یافت نشد.")
			return
		}
		if errors.Is(err, ErrCannotModifyActiveWorkshop) {
			writeJSONError(w, http.StatusConflict, "CANNOT_MODIFY_ACTIVE", "پس از ثبت‌نام قطعی، تغییر مبلغ، تاریخ‌ها یا مکان ممنوع است. لطفاً کارگاه را لغو و کارگاه جدید بسازید.")
			return
		}
		if errors.Is(err, ErrCannotReduceCapacityBelowActive) {
			writeJSONError(w, http.StatusBadRequest, "CAPACITY_BELOW_ACTIVE", "امکان کاهش ظرفیت به کمتر از ثبت‌نام‌های جاری وجود ندارد.")
			return
		}
		if errors.Is(err, ErrInvalidDates) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_DATES", "ترتیب تاریخ‌ها نامعتبر است.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در ویرایش کارگاه.")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) handlePublishAdminWorkshop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	detail, err := h.service.PublishWorkshop(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrWorkshopNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "کارگاه یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در انتشار کارگاه.")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) handleCloseAdminWorkshop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	detail, err := h.service.CloseWorkshop(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrWorkshopNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "کارگاه یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در بستن ثبت‌نام کارگاه.")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) handleCancelAdminWorkshop(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	detail, err := h.service.CancelWorkshop(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrWorkshopNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "کارگاه یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در لغو کارگاه.")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) handleListAdminRegistrations(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	workshopID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	items, err := h.service.ListAdminRegistrations(r.Context(), tenantID, workshopID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت لیست شرکت‌کنندگان.")
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleCancelAdminRegistration(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	workshopID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_WORKSHOP_ID", "شناسه کارگاه نامعتبر است.")
		return
	}

	regID, err := uuid.Parse(chi.URLParam(r, "reg_id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REGISTRATION_ID", "شناسه ثبت‌نام نامعتبر است.")
		return
	}

	if err := h.service.CancelRegistrationByAdmin(r.Context(), tenantID, workshopID, regID); err != nil {
		if errors.Is(err, ErrWorkshopNotFound) || errors.Is(err, ErrRegistrationNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "کارگاه یا ثبت‌نام یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در لغو ثبت‌نام.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "ثبت‌نام با موفقیت لغو شد.",
	})
}
