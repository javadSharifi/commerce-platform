package payments

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

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
	// Public payment callback (resolved by Host domain)
	r.Get("/callbacks/payments/zarinpal", h.handleZarinpalCallback)

	// Admin payment routes (strictly owner only)
	if h.authService != nil {
		r.Group(func(admin chi.Router) {
			admin.Use(auth.Authenticate(h.authService))
			admin.Use(auth.RequireTenantMembership(h.authService))
			admin.Use(auth.RequireRole("owner"))

			admin.Get("/admin/payments", h.handleListAdminPayments)

			admin.Group(func(mutating chi.Router) {
				mutating.Use(auth.CSRFProtection())
				mutating.Post("/admin/payments/{purchase_id}/refund", h.handleRecordAdminRefund)
			})
		})
	}
}

func (h *Handler) handleZarinpalCallback(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.domainResolver.Resolve(r.Context(), r.Host)
	if err != nil {
		http.Error(w, "Clinic domain not found", http.StatusNotFound)
		return
	}

	authority := r.URL.Query().Get("Authority")
	if authority == "" {
		http.Error(w, "Missing Authority parameter", http.StatusBadRequest)
		return
	}

	status := r.URL.Query().Get("Status")

	res, err := h.service.HandleCallbackAndVerify(r.Context(), tenantID, authority, status, time.Now())
	if err != nil {
		if errors.Is(err, ErrPaymentAttemptNotFound) {
			http.Error(w, "Payment attempt not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrAmountMismatch) {
			http.Error(w, "Payment amount mismatch", http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal payment processing error", http.StatusInternalServerError)
		return
	}

	redirectURL := "/workshops/registrations/" + res.RegistrationID.String() + "?payment_status=" + res.Status
	if res.CourseID != uuid.Nil {
		redirectURL = "/account/courses?payment_status=" + res.Status + "&purchase_id=" + res.PurchaseID.String()
	}
	if res.RefID != "" {
		redirectURL += "&ref_id=" + res.RefID
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *Handler) handleListAdminPayments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	var statusPtr *string
	if statusQ := r.URL.Query().Get("status"); statusQ != "" {
		statusPtr = &statusQ
	}

	items, err := h.service.ListAdminPurchases(r.Context(), tenantID, statusPtr)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در دریافت لیست پرداخت‌ها.")
		return
	}

	writeJSON(w, http.StatusOK, items)
}

type recordRefundReq struct {
	ReferenceNumber string `json:"reference_number"`
	Reason          string `json:"reason"`
	AmountRials     int64  `json:"amount_rials"`
}

func (h *Handler) handleRecordAdminRefund(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantctx.ID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "NO_TENANT", "شناسه کلینیک یافت نشد.")
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok || user == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "کاربر احراز هویت نشده است.")
		return
	}

	purchaseIDStr := chi.URLParam(r, "purchase_id")
	purchaseID, err := uuid.Parse(purchaseIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "شناسه خرید نامعتبر است.")
		return
	}

	var req recordRefundReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "فرمت داده‌های ارسالی نامعتبر است.")
		return
	}

	if req.ReferenceNumber == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "شماره مرجع بازپرداخت الزامی است.")
		return
	}
	if req.AmountRials <= 0 {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "مبلغ بازپرداخت باید بزرگتر از صفر باشد.")
		return
	}

	resp, err := h.service.RecordRefund(r.Context(), RecordRefundParams{
		TenantID:        tenantID,
		PurchaseID:      purchaseID,
		ReferenceNumber: req.ReferenceNumber,
		Reason:          req.Reason,
		AmountRials:     req.AmountRials,
		RegisteredBy:    user.ID,
		Now:             time.Now(),
	})
	if err != nil {
		if errors.Is(err, ErrPurchaseNotFound) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "خرید مورد نظر یافت نشد.")
			return
		}
		if errors.Is(err, ErrInvalidPurchaseStatusForRefund) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_STATUS", "تنها خریدهای با وضعیت refund_required قابل بازپرداخت هستند.")
			return
		}
		if errors.Is(err, ErrAmountMismatch) {
			writeJSONError(w, http.StatusBadRequest, "AMOUNT_MISMATCH", "مبلغ بازپرداخت با مبلغ خرید همخوانی ندارد.")
			return
		}
		if errors.Is(err, ErrDuplicateRefundReference) {
			writeJSONError(w, http.StatusConflict, "DUPLICATE_REFERENCE", "این شماره مرجع بازپرداخت قبلاً برای پرداخت دیگری ثبت شده است.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "خطا در ثبت بازپرداخت.")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
