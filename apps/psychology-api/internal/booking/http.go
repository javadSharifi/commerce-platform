package booking

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/booking/bookingdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	publicStore    *db.Store
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

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (h *Handler) MountRoutes(r chi.Router) {
	// Public routes (Resolved by Host Domain)
	r.Get("/public/therapists", h.handleListPublicTherapists)
	r.Get("/public/therapists/{id}", h.handleGetPublicTherapistDetail)
	r.Get("/public/therapists/{id}/slots", h.handleListPublicTherapistSlots)
	r.Post("/public/appointments", h.handleCreatePublicAppointment)
	r.Get("/public/appointments/{id}", h.handleGetPublicAppointmentStatus)
	r.Post("/public/appointments/{id}/cancel", h.handleCancelPublicAppointment)

	// Admin routes
	r.Group(func(admin chi.Router) {
		admin.Use(auth.Authenticate(h.authService))
		admin.Use(auth.RequireTenantMembership(h.authService))

		// Read therapists list & details (owner, receptionist, therapist)
		admin.Group(func(ro chi.Router) {
			ro.Use(auth.RequireRole("owner", "receptionist", "therapist"))
			ro.Get("/admin/therapists", h.handleListAdminTherapists)
			ro.Get("/admin/therapists/{id}", h.handleGetAdminTherapist)
			ro.Get("/admin/therapists/{id}/working-hours", h.handleGetAdminWorkingHours)
			ro.Get("/admin/therapists/{id}/exceptions", h.handleGetAdminScheduleExceptions)
			ro.Get("/admin/therapists/{id}/slots", h.handleListAdminAppointmentSlots)

			ro.Get("/admin/appointments", h.handleListAdminAppointments)
			ro.Get("/admin/appointments/{id}", h.handleGetAdminAppointment)
			ro.Get("/admin/appointments/{id}/history", h.handleGetAdminAppointmentHistory)
		})

		// Working hours, exceptions & appointments mutations (owner, receptionist, therapist)
		admin.Group(func(staff chi.Router) {
			staff.Use(auth.RequireRole("owner", "receptionist", "therapist"))
			staff.Use(auth.CSRFProtection())

			staff.Put("/admin/therapists/{id}/working-hours", h.handleUpdateAdminWorkingHours)
			staff.Post("/admin/therapists/{id}/exceptions", h.handleCreateAdminScheduleException)
			staff.Delete("/admin/therapists/{id}/exceptions/{exceptionId}", h.handleDeleteAdminScheduleException)
			staff.Post("/admin/therapists/{id}/slots/generate", h.handleGenerateAdminAppointmentSlots)

			staff.Post("/admin/appointments/{id}/confirm", h.handleConfirmAdminAppointment)
			staff.Post("/admin/appointments/{id}/cancel", h.handleCancelAdminAppointment)
			staff.Post("/admin/appointments/{id}/reschedule", h.handleRescheduleAdminAppointment)
		})

		// Owner only: Create, Update, Delete therapists
		admin.Group(func(ownerOnly chi.Router) {
			ownerOnly.Use(auth.RequireRole("owner"))
			ownerOnly.Use(auth.CSRFProtection())

			ownerOnly.Post("/admin/therapists", h.handleCreateAdminTherapist)
			ownerOnly.Put("/admin/therapists/{id}", h.handleUpdateAdminTherapist)
			ownerOnly.Delete("/admin/therapists/{id}", h.handleDeleteAdminTherapist)
		})
	})
}

// ----------------------------------------------------
// Public Handlers
// ----------------------------------------------------

func (h *Handler) handleListPublicTherapists(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	therapists, err := publicBooking(r.Context(), h, tenantID, func(svc *Service) ([]bookingdb.Therapist, error) {
		return svc.ListActiveTherapists(r.Context(), tenantID)
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت لیست درمانگران")
		return
	}

	resp := make([]map[string]any, 0, len(therapists))
	for _, th := range therapists {
		item := map[string]any{
			"id":                       fromPgUUID(th.ID).String(),
			"name":                     th.Name,
			"slug":                     th.Slug,
			"title":                    th.Title,
			"session_duration_minutes": th.SessionDurationMinutes,
		}
		if th.AvatarAssetID.Valid {
			item["avatar_asset_id"] = fromPgUUID(th.AvatarAssetID).String()
		}
		resp = append(resp, item)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleGetPublicTherapistDetail(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه درمانگر نامعتبر است.")
		return
	}

	th, err := publicBooking(r.Context(), h, tenantID, func(svc *Service) (*bookingdb.Therapist, error) {
		return svc.GetTherapistByID(r.Context(), tenantID, therapistID)
	})
	if err != nil || !th.IsActive {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "درمانگر یافت نشد.")
		return
	}

	resp := map[string]any{
		"id":                       fromPgUUID(th.ID).String(),
		"name":                     th.Name,
		"slug":                     th.Slug,
		"title":                    th.Title,
		"bio":                      th.Bio,
		"session_duration_minutes": th.SessionDurationMinutes,
	}
	if th.AvatarAssetID.Valid {
		resp["avatar_asset_id"] = fromPgUUID(th.AvatarAssetID).String()
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleListPublicTherapistSlots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه درمانگر نامعتبر است.")
		return
	}

	fromDate := time.Now().UTC()
	if q := r.URL.Query().Get("from_date"); q != "" {
		parsed, err := time.ParseInLocation("2006-01-02", q, h.service.tehranLoc)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_DATE", "تاریخ سانس نامعتبر است.")
			return
		}
		fromDate = parsed
	}

	slots, err := publicBooking(r.Context(), h, tenantID, func(svc *Service) ([]bookingdb.AppointmentSlot, error) {
		th, err := svc.GetTherapistByID(r.Context(), tenantID, therapistID)
		if err != nil {
			return nil, err
		}
		if !th.IsActive {
			return nil, ErrTherapistNotFound
		}
		return svc.ListAvailableSlots(r.Context(), tenantID, therapistID, fromDate)
	})
	if err != nil {
		if errors.Is(err, ErrTherapistNotFound) {
			writeJSONError(w, http.StatusNotFound, "THERAPIST_NOT_FOUND", "درمانگر یافت نشد.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت سانس‌های آزاد")
		return
	}

	resp := make([]map[string]any, 0, len(slots))
	for _, sl := range slots {
		resp = append(resp, map[string]any{
			"id":           fromPgUUID(sl.ID).String(),
			"therapist_id": fromPgUUID(sl.TherapistID).String(),
			"starts_at":    sl.StartsAt.Time.Format(time.RFC3339),
			"ends_at":      sl.EndsAt.Time.Format(time.RFC3339),
			"status":       sl.Status,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

type CreateAppointmentRequest struct {
	TherapistID string `json:"therapist_id"`
	SlotID      string `json:"slot_id"`
	GuestName   string `json:"guest_name"`
	GuestPhone  string `json:"guest_phone"`
}

func (h *Handler) handleCreatePublicAppointment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	var req CreateAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "فرمت درخواست نامعتبر است.")
		return
	}

	therapistID, err := uuid.Parse(req.TherapistID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_THERAPIST_ID", "شناسه درمانگر نامعتبر است.")
		return
	}

	slotID, err := uuid.Parse(req.SlotID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_SLOT_ID", "شناسه سانس نامعتبر است.")
		return
	}

	clientIP := getClientIP(r)

	res, err := publicBooking(r.Context(), h, tenantID, func(svc *Service) (*AppointmentResult, error) {
		return svc.CreatePublicAppointment(r.Context(), tenantID, CreatePublicAppointmentInput{
			TherapistID: therapistID,
			SlotID:      slotID,
			GuestName:   req.GuestName,
			GuestPhone:  req.GuestPhone,
			ClientIP:    clientIP,
		}, time.Now().UTC())
	})

	if err != nil {
		switch {
		case errors.Is(err, ErrRateLimitExceeded):
			w.Header().Set("Retry-After", "60")
			writeJSONError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "تعداد درخواست‌های شما بیش از حد مجاز است. لطفاً کمی بعد تلاش کنید.")
		case errors.Is(err, ErrSlotNotAvailable):
			writeJSONError(w, http.StatusConflict, "SLOT_UNAVAILABLE", "این سانس دیگر در دسترس نیست و رزرو شده است.")
		case errors.Is(err, ErrSlotInThePast):
			writeJSONError(w, http.StatusBadRequest, "SLOT_IN_PAST", "امکان رزرو سانس‌های گذشته وجود ندارد.")
		case errors.Is(err, ErrInvalidGuestName):
			writeJSONError(w, http.StatusBadRequest, "INVALID_GUEST_NAME", "نام مراجع الزامی است.")
		case errors.Is(err, ErrInvalidPhone):
			writeJSONError(w, http.StatusBadRequest, "INVALID_PHONE", "شماره موبایل وارد شده نامعتبر است.")
		case errors.Is(err, ErrTherapistNotFound), errors.Is(err, ErrTherapistInactive):
			writeJSONError(w, http.StatusNotFound, "THERAPIST_NOT_FOUND", "درمانگر یافت نشد.")
		case errors.Is(err, ErrSlotNotFound):
			writeJSONError(w, http.StatusNotFound, "SLOT_NOT_FOUND", "سانس نوبت‌دهی یافت نشد.")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ثبت نوبت موقتاً ممکن نیست.")
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: "appointment_lookup", Value: res.GuestLookupHash, Path: "/appointments/" + res.ID.String(),
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" ||
			strings.EqualFold(os.Getenv("APP_ENV"), "production"),
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                res.ID.String(),
		"therapist_id":      res.TherapistID.String(),
		"slot_id":           res.SlotID.String(),
		"guest_name":        res.GuestName,
		"guest_lookup_hash": res.GuestLookupHash,
		"status":            res.Status,
		"expires_at":        res.ExpiresAt.Format(time.RFC3339),
		"starts_at":         res.StartsAt.Format(time.RFC3339),
		"ends_at":           res.EndsAt.Format(time.RFC3339),
	})
}

func (h *Handler) handleGetPublicAppointmentStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	appointmentID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نوبت نامعتبر است.")
		return
	}

	lookupHash := strings.TrimSpace(r.Header.Get("X-Guest-Lookup-Hash"))
	if lookupHash == "" {
		lookupHash = strings.TrimSpace(r.URL.Query().Get("hash"))
	}
	if lookupHash == "" {
		lookupHash = strings.TrimSpace(r.URL.Query().Get("lookup_hash"))
	}
	if lookupHash == "" {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یافت نشد یا توکن پیگیری ارائه نشده است.")
		return
	}

	res, err := publicBooking(r.Context(), h, tenantID, func(svc *Service) (*AppointmentResult, error) {
		return svc.GetPublicAppointmentStatus(r.Context(), tenantID, appointmentID, lookupHash, time.Now().UTC())
	})
	if err != nil {
		if errors.Is(err, ErrAppointmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یافت نشد یا توکن پیگیری نامعتبر است.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت وضعیت نوبت.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                res.ID.String(),
		"therapist_id":      res.TherapistID.String(),
		"slot_id":           res.SlotID.String(),
		"guest_name":        res.GuestName,
		"guest_lookup_hash": res.GuestLookupHash,
		"status":            res.Status,
		"expires_at":        res.ExpiresAt.Format(time.RFC3339),
		"starts_at":         res.StartsAt.Format(time.RFC3339),
		"ends_at":           res.EndsAt.Format(time.RFC3339),
	})
}

type CancelAppointmentRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) handleCancelPublicAppointment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "دامنه کلینیک یافت نشد.")
		return
	}

	idStr := chi.URLParam(r, "id")
	appointmentID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نوبت نامعتبر است.")
		return
	}

	lookupHash := strings.TrimSpace(r.Header.Get("X-Guest-Lookup-Hash"))
	if lookupHash == "" {
		lookupHash = strings.TrimSpace(r.URL.Query().Get("hash"))
	}
	if lookupHash == "" {
		lookupHash = strings.TrimSpace(r.URL.Query().Get("lookup_hash"))
	}
	if lookupHash == "" {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یافت نشد یا توکن پیگیری ارائه نشده است.")
		return
	}

	var req CancelAppointmentRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	res, err := publicBooking(r.Context(), h, tenantID, func(svc *Service) (*AppointmentResult, error) {
		return svc.CancelPublicAppointment(r.Context(), tenantID, appointmentID, lookupHash, req.Reason, time.Now().UTC())
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrAppointmentNotFound):
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یافت نشد یا توکن پیگیری نامعتبر است.")
		case errors.Is(err, ErrCannotCancelPastSlot):
			writeJSONError(w, http.StatusBadRequest, "PAST_SLOT", "امکان لغو نوبت پس از آغاز زمان جلسه وجود ندارد.")
		case errors.Is(err, ErrCannotCancel):
			writeJSONError(w, http.StatusBadRequest, "CANNOT_CANCEL", "وضعیت این نوبت امکان لغو ندارد.")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "لغو نوبت موقتاً ممکن نیست.")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                res.ID.String(),
		"therapist_id":      res.TherapistID.String(),
		"slot_id":           res.SlotID.String(),
		"guest_name":        res.GuestName,
		"guest_lookup_hash": res.GuestLookupHash,
		"status":            res.Status,
		"expires_at":        res.ExpiresAt.Format(time.RFC3339),
		"starts_at":         res.StartsAt.Format(time.RFC3339),
		"ends_at":           res.EndsAt.Format(time.RFC3339),
	})
}

// ----------------------------------------------------
// Admin Handlers
// ----------------------------------------------------

func (h *Handler) handleListAdminTherapists(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	therapists, err := h.service.ListTherapists(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای سرور")
		return
	}

	resp := make([]map[string]any, 0, len(therapists))
	for _, th := range therapists {
		item := map[string]any{
			"id":                       fromPgUUID(th.ID).String(),
			"name":                     th.Name,
			"slug":                     th.Slug,
			"title":                    th.Title,
			"bio":                      th.Bio,
			"session_duration_minutes": th.SessionDurationMinutes,
			"is_active":                th.IsActive,
			"created_at":               th.CreatedAt.Time.Format(time.RFC3339),
			"updated_at":               th.UpdatedAt.Time.Format(time.RFC3339),
		}
		if th.UserID.Valid {
			item["user_id"] = fromPgUUID(th.UserID).String()
		}
		if th.AvatarAssetID.Valid {
			item["avatar_asset_id"] = fromPgUUID(th.AvatarAssetID).String()
		}
		resp = append(resp, item)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleGetAdminTherapist(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	th, err := h.service.GetTherapistByID(r.Context(), tenantID, therapistID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "درمانگر یافت نشد.")
		return
	}

	item := map[string]any{
		"id":                       fromPgUUID(th.ID).String(),
		"name":                     th.Name,
		"slug":                     th.Slug,
		"title":                    th.Title,
		"bio":                      th.Bio,
		"session_duration_minutes": th.SessionDurationMinutes,
		"is_active":                th.IsActive,
		"created_at":               th.CreatedAt.Time.Format(time.RFC3339),
		"updated_at":               th.UpdatedAt.Time.Format(time.RFC3339),
	}
	if th.UserID.Valid {
		item["user_id"] = fromPgUUID(th.UserID).String()
	}
	if th.AvatarAssetID.Valid {
		item["avatar_asset_id"] = fromPgUUID(th.AvatarAssetID).String()
	}

	writeJSON(w, http.StatusOK, item)
}

type CreateTherapistRequest struct {
	UserID                 *string `json:"user_id"`
	Name                   string  `json:"name"`
	Slug                   string  `json:"slug"`
	Title                  string  `json:"title"`
	Bio                    string  `json:"bio"`
	AvatarAssetID          *string `json:"avatar_asset_id"`
	SessionDurationMinutes int32   `json:"session_duration_minutes"`
	IsActive               *bool   `json:"is_active"`
}

func (h *Handler) handleCreateAdminTherapist(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	var req CreateTherapistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Slug) == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", "نام و اسلاگ درمانگر الزامی است.")
		return
	}

	var userID *uuid.UUID
	if req.UserID != nil && *req.UserID != "" {
		u, err := uuid.Parse(*req.UserID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "شناسه کاربر نامعتبر است.")
			return
		}
		userID = &u
	}

	var avatarID *uuid.UUID
	if req.AvatarAssetID != nil && *req.AvatarAssetID != "" {
		a, err := uuid.Parse(*req.AvatarAssetID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_AVATAR_ID", "شناسه تصویر آواتار نامعتبر است.")
			return
		}
		avatarID = &a
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	th, err := h.service.CreateTherapist(r.Context(), tenantID, CreateTherapistInput{
		UserID:                 userID,
		Name:                   req.Name,
		Slug:                   req.Slug,
		Title:                  req.Title,
		Bio:                    req.Bio,
		AvatarAssetID:          avatarID,
		SessionDurationMinutes: req.SessionDurationMinutes,
		IsActive:               isActive,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "ERROR", err.Error())
		return
	}

	item := map[string]any{
		"id":                       fromPgUUID(th.ID).String(),
		"name":                     th.Name,
		"slug":                     th.Slug,
		"title":                    th.Title,
		"bio":                      th.Bio,
		"session_duration_minutes": th.SessionDurationMinutes,
		"is_active":                th.IsActive,
		"created_at":               th.CreatedAt.Time.Format(time.RFC3339),
		"updated_at":               th.UpdatedAt.Time.Format(time.RFC3339),
	}
	if th.UserID.Valid {
		item["user_id"] = fromPgUUID(th.UserID).String()
	}
	if th.AvatarAssetID.Valid {
		item["avatar_asset_id"] = fromPgUUID(th.AvatarAssetID).String()
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) handleUpdateAdminTherapist(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	var req CreateTherapistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	var userID *uuid.UUID
	if req.UserID != nil && *req.UserID != "" {
		u, err := uuid.Parse(*req.UserID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "شناسه کاربر نامعتبر است.")
			return
		}
		userID = &u
	}

	var avatarID *uuid.UUID
	if req.AvatarAssetID != nil && *req.AvatarAssetID != "" {
		a, err := uuid.Parse(*req.AvatarAssetID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_AVATAR_ID", "شناسه تصویر آواتار نامعتبر است.")
			return
		}
		avatarID = &a
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	th, err := h.service.UpdateTherapist(r.Context(), tenantID, therapistID, UpdateTherapistInput{
		UserID:                 userID,
		Name:                   req.Name,
		Slug:                   req.Slug,
		Title:                  req.Title,
		Bio:                    req.Bio,
		AvatarAssetID:          avatarID,
		SessionDurationMinutes: req.SessionDurationMinutes,
		IsActive:               isActive,
	})
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "درمانگر یافت نشد.")
		return
	}

	item := map[string]any{
		"id":                       fromPgUUID(th.ID).String(),
		"name":                     th.Name,
		"slug":                     th.Slug,
		"title":                    th.Title,
		"bio":                      th.Bio,
		"session_duration_minutes": th.SessionDurationMinutes,
		"is_active":                th.IsActive,
		"created_at":               th.CreatedAt.Time.Format(time.RFC3339),
		"updated_at":               th.UpdatedAt.Time.Format(time.RFC3339),
	}
	if th.UserID.Valid {
		item["user_id"] = fromPgUUID(th.UserID).String()
	}
	if th.AvatarAssetID.Valid {
		item["avatar_asset_id"] = fromPgUUID(th.AvatarAssetID).String()
	}

	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) handleDeleteAdminTherapist(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	if err := h.service.DeleteTherapist(r.Context(), tenantID, therapistID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای حذف درمانگر")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "درمانگر با موفقیت حذف شد."})
}

// ----------------------------------------------------
// Admin Working Hours & Exceptions
// ----------------------------------------------------

func (h *Handler) handleGetAdminWorkingHours(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	hours, err := h.service.ListWorkingHours(r.Context(), tenantID, therapistID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت ساعات کاری")
		return
	}

	resp := make([]map[string]any, 0, len(hours))
	for _, wh := range hours {
		sh, sm, ss := parseMicrosecondsToTime(wh.StartTime.Microseconds)
		eh, em, es := parseMicrosecondsToTime(wh.EndTime.Microseconds)
		resp = append(resp, map[string]any{
			"id":           fromPgUUID(wh.ID).String(),
			"therapist_id": fromPgUUID(wh.TherapistID).String(),
			"day_of_week":  wh.DayOfWeek,
			"start_time":   formatTimeHMS(sh, sm, ss),
			"end_time":     formatTimeHMS(eh, em, es),
			"is_active":    wh.IsActive,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

type UpdateWorkingHoursRequest struct {
	Hours []struct {
		DayOfWeek int16  `json:"day_of_week"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		IsActive  bool   `json:"is_active"`
	} `json:"hours"`
}

func (h *Handler) handleUpdateAdminWorkingHours(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	var req UpdateWorkingHoursRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	inputs := make([]WorkingHourInput, 0, len(req.Hours))
	for _, hIn := range req.Hours {
		inputs = append(inputs, WorkingHourInput{
			DayOfWeek: hIn.DayOfWeek,
			StartTime: hIn.StartTime,
			EndTime:   hIn.EndTime,
			IsActive:  hIn.IsActive,
		})
	}

	hours, err := h.service.SetWorkingHours(r.Context(), tenantID, therapistID, inputs)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "ERROR", err.Error())
		return
	}

	resp := make([]map[string]any, 0, len(hours))
	for _, wh := range hours {
		sh, sm, ss := parseMicrosecondsToTime(wh.StartTime.Microseconds)
		eh, em, es := parseMicrosecondsToTime(wh.EndTime.Microseconds)
		resp = append(resp, map[string]any{
			"id":           fromPgUUID(wh.ID).String(),
			"therapist_id": fromPgUUID(wh.TherapistID).String(),
			"day_of_week":  wh.DayOfWeek,
			"start_time":   formatTimeHMS(sh, sm, ss),
			"end_time":     formatTimeHMS(eh, em, es),
			"is_active":    wh.IsActive,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleGetAdminScheduleExceptions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	exs, err := h.service.ListScheduleExceptions(r.Context(), tenantID, therapistID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت استثناها")
		return
	}

	resp := make([]map[string]any, 0, len(exs))
	for _, ex := range exs {
		item := map[string]any{
			"id":             fromPgUUID(ex.ID).String(),
			"therapist_id":   fromPgUUID(ex.TherapistID).String(),
			"date":           ex.Date.Time.Format("2006-01-02"),
			"is_unavailable": ex.IsUnavailable,
			"reason":         ex.Reason,
		}
		if ex.StartTime.Valid {
			sh, sm, ss := parseMicrosecondsToTime(ex.StartTime.Microseconds)
			item["start_time"] = formatTimeHMS(sh, sm, ss)
		}
		if ex.EndTime.Valid {
			eh, em, es := parseMicrosecondsToTime(ex.EndTime.Microseconds)
			item["end_time"] = formatTimeHMS(eh, em, es)
		}
		resp = append(resp, item)
	}

	writeJSON(w, http.StatusOK, resp)
}

type CreateScheduleExceptionRequest struct {
	Date          string  `json:"date"`
	IsUnavailable bool    `json:"is_unavailable"`
	StartTime     *string `json:"start_time"`
	EndTime       *string `json:"end_time"`
	Reason        string  `json:"reason"`
}

func (h *Handler) handleCreateAdminScheduleException(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	var req CreateScheduleExceptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_DATE", "فرمت تاریخ باید YYYY-MM-DD باشد.")
		return
	}

	var sTime, eTime string
	if req.StartTime != nil {
		sTime = *req.StartTime
	}
	if req.EndTime != nil {
		eTime = *req.EndTime
	}

	ex, err := h.service.CreateScheduleException(r.Context(), tenantID, therapistID, ScheduleExceptionInput{
		Date:          parsedDate,
		IsUnavailable: req.IsUnavailable,
		StartTime:     sTime,
		EndTime:       eTime,
		Reason:        req.Reason,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "ERROR", err.Error())
		return
	}

	item := map[string]any{
		"id":             fromPgUUID(ex.ID).String(),
		"therapist_id":   fromPgUUID(ex.TherapistID).String(),
		"date":           ex.Date.Time.Format("2006-01-02"),
		"is_unavailable": ex.IsUnavailable,
		"reason":         ex.Reason,
	}
	if ex.StartTime.Valid {
		sh, sm, ss := parseMicrosecondsToTime(ex.StartTime.Microseconds)
		item["start_time"] = formatTimeHMS(sh, sm, ss)
	}
	if ex.EndTime.Valid {
		eh, em, es := parseMicrosecondsToTime(ex.EndTime.Microseconds)
		item["end_time"] = formatTimeHMS(eh, em, es)
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) handleDeleteAdminScheduleException(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه درمانگر نامعتبر است.")
		return
	}

	exIDStr := chi.URLParam(r, "exceptionId")
	exID, err := uuid.Parse(exIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه استثنا نامعتبر است.")
		return
	}

	if err := h.service.DeleteScheduleException(r.Context(), tenantID, therapistID, exID); err != nil {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "استثنا یافت نشد.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "استثنا با موفقیت حذف شد."})
}

// ----------------------------------------------------
// Admin Slots
// ----------------------------------------------------

func (h *Handler) handleListAdminAppointmentSlots(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	from := time.Now().UTC()
	to := from.AddDate(0, 0, 30)

	if f := r.URL.Query().Get("from"); f != "" {
		if parsed, err := time.Parse(time.RFC3339, f); err == nil {
			from = parsed
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = parsed
		}
	}

	slots, err := h.service.ListSlots(r.Context(), tenantID, therapistID, from, to)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت سانس‌ها")
		return
	}

	resp := make([]map[string]any, 0, len(slots))
	for _, sl := range slots {
		resp = append(resp, map[string]any{
			"id":           fromPgUUID(sl.ID).String(),
			"therapist_id": fromPgUUID(sl.TherapistID).String(),
			"starts_at":    sl.StartsAt.Time.Format(time.RFC3339),
			"ends_at":      sl.EndsAt.Time.Format(time.RFC3339),
			"status":       sl.Status,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

type GenerateSlotsRequest struct {
	FromDate string `json:"from_date"`
	ToDate   string `json:"to_date"`
}

func (h *Handler) handleGenerateAdminAppointmentSlots(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	idStr := chi.URLParam(r, "id")
	therapistID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نامعتبر است.")
		return
	}

	var req GenerateSlotsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	fromDate, err := time.Parse("2006-01-02", req.FromDate)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_FROM_DATE", "فرمت from_date باید YYYY-MM-DD باشد.")
		return
	}
	toDate, err := time.Parse("2006-01-02", req.ToDate)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TO_DATE", "فرمت to_date باید YYYY-MM-DD باشد.")
		return
	}

	res, err := h.service.GenerateSlots(r.Context(), tenantID, therapistID, fromDate, toDate)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// ----------------------------------------------------
// Admin Appointments Handlers (PSY-2.4)
// ----------------------------------------------------

func (h *Handler) handleListAdminAppointments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده")
		return
	}

	role, _ := auth.RoleFromContext(r.Context())

	var filterTherapistID *uuid.UUID
	if thStr := r.URL.Query().Get("therapist_id"); thStr != "" {
		u, err := uuid.Parse(thStr)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_THERAPIST_ID", "شناسه درمانگر نامعتبر است.")
			return
		}
		filterTherapistID = &u
	}

	filterStatus := r.URL.Query().Get("status")

	items, err := h.service.ListAdminAppointments(r.Context(), tenantID, role, user.ID, filterTherapistID, filterStatus)
	if err != nil {
		if errors.Is(err, ErrUnauthorizedTherapist) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "شما دسترسی به تقویم یا نوبت‌های این درمانگر را ندارید.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت لیست نوبت‌ها")
		return
	}

	resp := make([]map[string]any, 0, len(items))
	for _, it := range items {
		resp = append(resp, map[string]any{
			"id":             it.ID.String(),
			"therapist_id":   it.TherapistID.String(),
			"therapist_name": it.TherapistName,
			"slot_id":        it.SlotID.String(),
			"starts_at":      it.StartsAt.Format(time.RFC3339),
			"ends_at":        it.EndsAt.Format(time.RFC3339),
			"guest_name":     it.GuestName,
			"guest_phone":    it.GuestPhone,
			"status":         it.Status,
			"created_at":     it.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleGetAdminAppointment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده")
		return
	}

	role, _ := auth.RoleFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نوبت نامعتبر است.")
		return
	}

	detail, err := h.service.GetAdminAppointment(r.Context(), tenantID, appID, role, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAppointmentNotFound):
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یافت نشد.")
		case errors.Is(err, ErrUnauthorizedTherapist):
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "شما دسترسی به این نوبت را ندارید.")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت جزئیات نوبت")
		}
		return
	}

	writeJSON(w, http.StatusOK, mapAdminDetail(detail))
}

func (h *Handler) handleConfirmAdminAppointment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده")
		return
	}

	role, _ := auth.RoleFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نوبت نامعتبر است.")
		return
	}

	detail, err := h.service.ConfirmAdminAppointment(r.Context(), tenantID, appID, role, user.ID, time.Now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, ErrAppointmentNotFound):
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یافت نشد.")
		case errors.Is(err, ErrUnauthorizedTherapist):
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "شما اجازه تأیید این نوبت را ندارید.")
		default:
			writeJSONError(w, http.StatusBadRequest, "ERROR", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, mapAdminDetail(detail))
}

func (h *Handler) handleCancelAdminAppointment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده")
		return
	}

	role, _ := auth.RoleFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نوبت نامعتبر است.")
		return
	}

	var req CancelAppointmentRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	detail, err := h.service.CancelAdminAppointment(r.Context(), tenantID, appID, role, user.ID, req.Reason, time.Now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, ErrAppointmentNotFound):
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یافت نشد.")
		case errors.Is(err, ErrUnauthorizedTherapist):
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "شما اجازه لغو این نوبت را ندارید.")
		case errors.Is(err, ErrCannotCancelPastSlot):
			writeJSONError(w, http.StatusBadRequest, "PAST_SLOT", "امکان لغو نوبت پس از آغاز زمان جلسه وجود ندارد.")
		default:
			writeJSONError(w, http.StatusBadRequest, "ERROR", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, mapAdminDetail(detail))
}

type RescheduleAppointmentRequest struct {
	NewSlotID string `json:"new_slot_id"`
	Reason    string `json:"reason"`
}

func (h *Handler) handleRescheduleAdminAppointment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده")
		return
	}

	role, _ := auth.RoleFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نوبت نامعتبر است.")
		return
	}

	var req RescheduleAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "فرمت بدنه درخواست نامعتبر است.")
		return
	}

	newSlotID, err := uuid.Parse(req.NewSlotID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_NEW_SLOT_ID", "شناسه سانس جدید نامعتبر است.")
		return
	}

	detail, err := h.service.RescheduleAdminAppointment(r.Context(), tenantID, appID, newSlotID, role, user.ID, req.Reason, time.Now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, ErrAppointmentNotFound), errors.Is(err, ErrSlotNotFound):
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یا سانس نوبت‌دهی یافت نشد.")
		case errors.Is(err, ErrUnauthorizedTherapist):
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "شما دسترسی به جابه‌جایی این نوبت را ندارید.")
		case errors.Is(err, ErrSlotNotAvailable):
			writeJSONError(w, http.StatusConflict, "SLOT_UNAVAILABLE", "سانس جدید انتخاب‌شده در دسترس نیست و قبلاً پر شده است.")
		case errors.Is(err, ErrSlotInThePast):
			writeJSONError(w, http.StatusBadRequest, "SLOT_IN_PAST", "امکان جابه‌جایی به سانس گذشته وجود ندارد.")
		default:
			writeJSONError(w, http.StatusBadRequest, "ERROR", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, mapAdminDetail(detail))
}

func (h *Handler) handleGetAdminAppointmentHistory(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "مستاجر نامشخص")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده")
		return
	}

	role, _ := auth.RoleFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه نوبت نامعتبر است.")
		return
	}

	logs, err := h.service.GetAdminAppointmentHistory(r.Context(), tenantID, appID, role, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAppointmentNotFound):
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "نوبت یافت نشد.")
		case errors.Is(err, ErrUnauthorizedTherapist):
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "شما دسترسی به تاریخچه این نوبت را ندارید.")
		default:
			writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطای دریافت تاریخچه نوبت")
		}
		return
	}

	resp := make([]map[string]any, 0, len(logs))
	for _, l := range logs {
		item := map[string]any{
			"id":             fromPgUUID(l.ID).String(),
			"appointment_id": fromPgUUID(l.AppointmentID).String(),
			"to_status":      l.ToStatus,
			"action":         l.Action,
			"actor_type":     l.ActorType,
			"reason":         l.Reason,
			"created_at":     l.CreatedAt.Time.Format(time.RFC3339),
		}
		if l.FromStatus.Valid {
			item["from_status"] = l.FromStatus.String
		}
		if l.ActorID.Valid {
			item["actor_id"] = fromPgUUID(l.ActorID).String()
		}
		if len(l.Metadata) > 0 {
			var meta any
			if err := json.Unmarshal(l.Metadata, &meta); err == nil {
				item["metadata"] = meta
			}
		}
		resp = append(resp, item)
	}

	writeJSON(w, http.StatusOK, resp)
}

func mapAdminDetail(d *AdminAppointmentDetail) map[string]any {
	res := map[string]any{
		"id":                  d.ID.String(),
		"therapist_id":        d.TherapistID.String(),
		"therapist_name":      d.TherapistName,
		"slot_id":             d.SlotID.String(),
		"starts_at":           d.StartsAt.Format(time.RFC3339),
		"ends_at":             d.EndsAt.Format(time.RFC3339),
		"guest_name":          d.GuestName,
		"guest_phone":         d.GuestPhone,
		"notes":               d.Notes,
		"status":              d.Status,
		"expires_at":          d.ExpiresAt.Format(time.RFC3339),
		"cancellation_reason": d.CancellationReason,
		"created_at":          d.CreatedAt.Format(time.RFC3339),
		"updated_at":          d.UpdatedAt.Format(time.RFC3339),
	}
	if d.ConfirmedAt != nil {
		res["confirmed_at"] = d.ConfirmedAt.Format(time.RFC3339)
	}
	if d.CancelledAt != nil {
		res["cancelled_at"] = d.CancelledAt.Format(time.RFC3339)
	}
	return res
}
