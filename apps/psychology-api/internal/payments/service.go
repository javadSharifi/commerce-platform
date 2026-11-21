package payments

import (
	"context"
	"errors"
	"strings"
	"time"

	"psychology/api/internal/courses/coursesdb"
	"psychology/api/internal/payments/paymentsdb"
	"psychology/api/internal/workshops/workshopsdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Querier interface {
	CreatePurchase(ctx context.Context, arg paymentsdb.CreatePurchaseParams) (paymentsdb.Purchase, error)
	GetPurchaseByID(ctx context.Context, arg paymentsdb.GetPurchaseByIDParams) (paymentsdb.Purchase, error)
	GetPurchaseByIDForUpdate(ctx context.Context, arg paymentsdb.GetPurchaseByIDForUpdateParams) (paymentsdb.Purchase, error)
	UpdatePurchaseStatus(ctx context.Context, arg paymentsdb.UpdatePurchaseStatusParams) (paymentsdb.Purchase, error)
	ListPurchases(ctx context.Context, arg paymentsdb.ListPurchasesParams) ([]paymentsdb.Purchase, error)
	CreatePaymentAttempt(ctx context.Context, arg paymentsdb.CreatePaymentAttemptParams) (paymentsdb.PaymentAttempt, error)
	GetPaymentAttemptByAuthority(ctx context.Context, arg paymentsdb.GetPaymentAttemptByAuthorityParams) (paymentsdb.PaymentAttempt, error)
	GetPaymentAttemptByIdempotencyKey(ctx context.Context, arg paymentsdb.GetPaymentAttemptByIdempotencyKeyParams) (paymentsdb.PaymentAttempt, error)
	UpdatePaymentAttemptStatus(ctx context.Context, arg paymentsdb.UpdatePaymentAttemptStatusParams) (paymentsdb.PaymentAttempt, error)
	GetActivePaymentAttemptByPurchase(ctx context.Context, arg paymentsdb.GetActivePaymentAttemptByPurchaseParams) (paymentsdb.PaymentAttempt, error)
	GetVerifiedPaymentAttemptByPurchase(ctx context.Context, arg paymentsdb.GetVerifiedPaymentAttemptByPurchaseParams) (paymentsdb.PaymentAttempt, error)
	ListStalePaymentAttempts(ctx context.Context, arg paymentsdb.ListStalePaymentAttemptsParams) ([]paymentsdb.PaymentAttempt, error)
	ListPaymentAttemptsByPurchase(ctx context.Context, arg paymentsdb.ListPaymentAttemptsByPurchaseParams) ([]paymentsdb.PaymentAttempt, error)
	CreateRefund(ctx context.Context, arg paymentsdb.CreateRefundParams) (paymentsdb.Refund, error)
	GetRefundByReference(ctx context.Context, arg paymentsdb.GetRefundByReferenceParams) (paymentsdb.Refund, error)
	ListRefundsByPurchase(ctx context.Context, arg paymentsdb.ListRefundsByPurchaseParams) ([]paymentsdb.Refund, error)
}

type WorkshopQuerier interface {
	GetRegistrationByID(ctx context.Context, arg workshopsdb.GetRegistrationByIDParams) (workshopsdb.WorkshopRegistration, error)
	GetRegistrationByIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByIDForUpdateParams) (workshopsdb.WorkshopRegistration, error)
	GetRegistrationByPurchaseID(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDParams) (workshopsdb.WorkshopRegistration, error)
	GetRegistrationByPurchaseIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDForUpdateParams) (workshopsdb.WorkshopRegistration, error)
	GetWorkshopByID(ctx context.Context, arg workshopsdb.GetWorkshopByIDParams) (workshopsdb.Workshop, error)
	GetWorkshopByIDForUpdate(ctx context.Context, arg workshopsdb.GetWorkshopByIDForUpdateParams) (workshopsdb.Workshop, error)
	UpdateRegistrationStatus(ctx context.Context, arg workshopsdb.UpdateRegistrationStatusParams) (workshopsdb.WorkshopRegistration, error)
	ExpirePendingRegistrations(ctx context.Context) (int64, error)
}

type CoursePaymentQuerier interface {
	GetCourseByID(ctx context.Context, arg coursesdb.GetCourseByIDParams) (coursesdb.Course, error)
	GetCourseByIDForUpdate(ctx context.Context, arg coursesdb.GetCourseByIDForUpdateParams) (coursesdb.Course, error)
	GetCourseBySlug(ctx context.Context, arg coursesdb.GetCourseBySlugParams) (coursesdb.Course, error)
	GetActiveCourseEntitlement(ctx context.Context, arg coursesdb.GetActiveCourseEntitlementParams) (coursesdb.CourseEntitlement, error)
	GetCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.GetCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error)
	CreateCourseEntitlement(ctx context.Context, arg coursesdb.CreateCourseEntitlementParams) (coursesdb.CourseEntitlement, error)
	ActivateCourseEntitlement(ctx context.Context, arg coursesdb.ActivateCourseEntitlementParams) (coursesdb.CourseEntitlement, error)
	RevokeCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.RevokeCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error)
	GetBuyerAccountByID(ctx context.Context, arg coursesdb.GetBuyerAccountByIDParams) (coursesdb.BuyerAccount, error)
}

type Service struct {
	queries         Querier
	provider        PaymentProvider
	workshopQueries WorkshopQuerier
	courseQueries   CoursePaymentQuerier
}

func NewService(q Querier, provider PaymentProvider, wq WorkshopQuerier, cq ...CoursePaymentQuerier) *Service {
	s := &Service{
		queries:         q,
		provider:        provider,
		workshopQueries: wq,
	}
	if len(cq) > 0 {
		s.courseQueries = cq[0]
	}
	return s
}

func (s *Service) SetCourseQueries(cq CoursePaymentQuerier) {
	s.courseQueries = cq
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPgUUID(p pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], p.Bytes[:])
	return u
}

func toPgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

type InitiateWorkshopPaymentParams struct {
	TenantID       uuid.UUID
	RegistrationID uuid.UUID
	IdempotencyKey string
	CallbackURL    string
	Now            time.Time
}

type InitiatePaymentResult struct {
	PaymentAttemptID uuid.UUID  `json:"payment_attempt_id"`
	PurchaseID       uuid.UUID  `json:"purchase_id"`
	Authority        string     `json:"authority"`
	PaymentURL       string     `json:"payment_url"`
	AmountRials      int64      `json:"amount_rials"`
	Status           string     `json:"status"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// InitiateWorkshopPayment initiates a payment for a workshop registration hold.
// Network calls to the payment gateway are strictly kept outside any database transactions.
func (s *Service) InitiateWorkshopPayment(ctx context.Context, p InitiateWorkshopPaymentParams) (*InitiatePaymentResult, error) {
	if p.TenantID == uuid.Nil {
		return nil, errors.New("payments: tenant_id cannot be nil")
	}
	if p.RegistrationID == uuid.Nil {
		return nil, errors.New("payments: registration_id cannot be nil")
	}
	idempotencyKey := strings.TrimSpace(p.IdempotencyKey)
	if idempotencyKey == "" {
		return nil, ErrInvalidIdempotencyKey
	}
	callbackURL := strings.TrimSpace(p.CallbackURL)
	if callbackURL == "" {
		return nil, errors.New("payments: callback_url cannot be empty")
	}

	now := p.Now
	if now.IsZero() {
		now = time.Now()
	}

	// 1. Idempotency Check: if this exact idempotency key was already processed, return clean idempotent response
	existingAttempt, err := s.queries.GetPaymentAttemptByIdempotencyKey(ctx, paymentsdb.GetPaymentAttemptByIdempotencyKeyParams{
		TenantID:       toPgUUID(p.TenantID),
		IdempotencyKey: idempotencyKey,
	})
	if err == nil {
		if existingAttempt.Status == "initiated" {
			purchase, err := s.queries.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
				TenantID: toPgUUID(p.TenantID),
				ID:       existingAttempt.PurchaseID,
			})
			if err != nil {
				return nil, err
			}

			// Generate payment URL based on authority and provider
			paymentURL := "https://payment.zarinpal.com/pg/StartPay/" + existingAttempt.Authority
			if s.provider != nil && s.provider.Name() == "mock" {
				paymentURL = "https://test-pay.example.com/mock-gateway?authority=" + existingAttempt.Authority
			}

			return &InitiatePaymentResult{
				PaymentAttemptID: fromPgUUID(existingAttempt.ID),
				PurchaseID:       fromPgUUID(existingAttempt.PurchaseID),
				Authority:        existingAttempt.Authority,
				PaymentURL:       paymentURL,
				AmountRials:      purchase.AmountRials,
				Status:           existingAttempt.Status,
			}, nil
		}
		if existingAttempt.Status == "verified" {
			return nil, ErrPaymentAlreadyVerified
		}
		return nil, errors.New("payments: payment attempt previously failed")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 2. Fetch and Validate Workshop Registration
	reg, err := s.workshopQueries.GetRegistrationByID(ctx, workshopsdb.GetRegistrationByIDParams{
		TenantID: toPgUUID(p.TenantID),
		ID:       toPgUUID(p.RegistrationID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRegistrationNotFound
		}
		return nil, err
	}

	if reg.Status != "pending_payment" {
		return nil, ErrInvalidRegistrationStatus
	}

	if reg.ExpiresAt.Valid && !reg.ExpiresAt.Time.After(now) {
		return nil, ErrRegistrationExpired
	}

	// 3. Fetch Parent Workshop to take price snapshot
	ws, err := s.workshopQueries.GetWorkshopByID(ctx, workshopsdb.GetWorkshopByIDParams{
		TenantID: toPgUUID(p.TenantID),
		ID:       reg.WorkshopID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	if ws.Status != "published" {
		return nil, ErrWorkshopNotPublished
	}

	if ws.PriceRials <= 0 {
		return nil, ErrZeroOrNegativeAmount
	}

	// 4. Create or reuse Purchase record with accurate Rials snapshot
	var purchase paymentsdb.Purchase
	if reg.PurchaseID.Valid {
		existingPurchase, err := s.queries.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
			TenantID: toPgUUID(p.TenantID),
			ID:       reg.PurchaseID,
		})
		if err == nil {
			purchase = existingPurchase
		}
	}

	if !purchase.ID.Valid {
		newPurchase, err := s.queries.CreatePurchase(ctx, paymentsdb.CreatePurchaseParams{
			TenantID:      toPgUUID(p.TenantID),
			ItemType:      "workshop",
			ItemID:        ws.ID,
			AmountRials:   ws.PriceRials, // Snapshot from backend DB in Rials!
			TitleSnapshot: ws.Title,      // Snapshot from backend DB!
			Status:        "pending",
		})
		if err != nil {
			return nil, err
		}
		purchase = newPurchase

		// Associate purchase with registration
		_, _ = s.workshopQueries.UpdateRegistrationStatus(ctx, workshopsdb.UpdateRegistrationStatusParams{
			TenantID:   toPgUUID(p.TenantID),
			ID:         reg.ID,
			Status:     reg.Status,
			PurchaseID: purchase.ID,
		})
	}

	// 5. Prevent duplicate uncoordinated attempts on the same purchase:
	// "تا زمان تعیین تکلیف تلاش قبلی، همان خرید دوباره به درگاه فرستاده نمی‌شود."
	activeAttempt, err := s.queries.GetActivePaymentAttemptByPurchase(ctx, paymentsdb.GetActivePaymentAttemptByPurchaseParams{
		TenantID:   toPgUUID(p.TenantID),
		PurchaseID: purchase.ID,
	})
	if err == nil {
		if activeAttempt.IdempotencyKey != idempotencyKey {
			return nil, ErrActiveAttemptInProgress
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 6. EXTERNAL NETWORK CALL OUTSIDE DATABASE TRANSACTION:
	// If the network call times out or fails with unknown result, no bogus payment_attempt is created,
	// and the purchase remains pending for safe retries.
	paymentResp, err := s.provider.RequestPayment(ctx, PaymentRequest{
		AmountRials: purchase.AmountRials,
		Description: "ثبت‌نام در کارگاه: " + ws.Title,
		CallbackURL: callbackURL,
		Mobile:      reg.GuestPhone,
	})
	if err != nil {
		// Handled unknown/timeout result
		return nil, err
	}

	// 7. Record payment attempt with obtained Authority
	attempt, err := s.queries.CreatePaymentAttempt(ctx, paymentsdb.CreatePaymentAttemptParams{
		TenantID:       toPgUUID(p.TenantID),
		PurchaseID:     purchase.ID,
		Provider:       s.provider.Name(),
		Authority:      paymentResp.Authority,
		AmountRials:    purchase.AmountRials,
		Status:         "initiated",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return nil, err
	}

	var exp *time.Time
	if reg.ExpiresAt.Valid {
		t := reg.ExpiresAt.Time
		exp = &t
	}

	return &InitiatePaymentResult{
		PaymentAttemptID: fromPgUUID(attempt.ID),
		PurchaseID:       fromPgUUID(purchase.ID),
		Authority:        paymentResp.Authority,
		PaymentURL:       paymentResp.PaymentURL,
		AmountRials:      purchase.AmountRials,
		Status:           attempt.Status,
		ExpiresAt:        exp,
	}, nil
}

type InitiateCoursePaymentParams struct {
	TenantID       uuid.UUID
	BuyerID        uuid.UUID
	CourseID       uuid.UUID
	IdempotencyKey string
	CallbackURL    string
	Now            time.Time
}

// InitiateCoursePayment initiates a payment for purchasing a single course.
// Enforces:
// 1. Snapshotting current price and title in Backend.
// 2. Rejecting fresh payment initiation if the buyer already holds an active entitlement.
// 3. Network calls to the payment gateway are strictly kept outside any database transactions.
func (s *Service) InitiateCoursePayment(ctx context.Context, p InitiateCoursePaymentParams) (*InitiatePaymentResult, error) {
	if p.TenantID == uuid.Nil {
		return nil, errors.New("payments: tenant_id cannot be nil")
	}
	if p.BuyerID == uuid.Nil {
		return nil, errors.New("payments: buyer_id cannot be nil")
	}
	if p.CourseID == uuid.Nil {
		return nil, errors.New("payments: course_id cannot be nil")
	}
	if s.courseQueries == nil {
		return nil, errors.New("payments: course querier not initialized")
	}
	idempotencyKey := strings.TrimSpace(p.IdempotencyKey)
	if idempotencyKey == "" {
		return nil, ErrInvalidIdempotencyKey
	}
	callbackURL := strings.TrimSpace(p.CallbackURL)
	if callbackURL == "" {
		return nil, errors.New("payments: callback_url cannot be empty")
	}

	now := p.Now
	if now.IsZero() {
		now = time.Now()
	}

	// 1. Idempotency Check: if this exact idempotency key was already processed, return clean idempotent response
	existingAttempt, err := s.queries.GetPaymentAttemptByIdempotencyKey(ctx, paymentsdb.GetPaymentAttemptByIdempotencyKeyParams{
		TenantID:       toPgUUID(p.TenantID),
		IdempotencyKey: idempotencyKey,
	})
	if err == nil {
		if existingAttempt.Status == "initiated" {
			purchase, err := s.queries.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
				TenantID: toPgUUID(p.TenantID),
				ID:       existingAttempt.PurchaseID,
			})
			if err != nil {
				return nil, err
			}

			paymentURL := "https://payment.zarinpal.com/pg/StartPay/" + existingAttempt.Authority
			if s.provider != nil && s.provider.Name() == "mock" {
				paymentURL = "https://test-pay.example.com/mock-gateway?authority=" + existingAttempt.Authority
			}

			return &InitiatePaymentResult{
				PaymentAttemptID: fromPgUUID(existingAttempt.ID),
				PurchaseID:       fromPgUUID(existingAttempt.PurchaseID),
				Authority:        existingAttempt.Authority,
				PaymentURL:       paymentURL,
				AmountRials:      purchase.AmountRials,
				Status:           existingAttempt.Status,
			}, nil
		}
		if existingAttempt.Status == "verified" {
			return nil, ErrPaymentAlreadyVerified
		}
		return nil, errors.New("payments: payment attempt previously failed")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 2. Validate buyer account
	buyer, err := s.courseQueries.GetBuyerAccountByID(ctx, coursesdb.GetBuyerAccountByIDParams{
		TenantID: toPgUUID(p.TenantID),
		ID:       toPgUUID(p.BuyerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("payments: حساب کاربری خریدار یافت نشد")
		}
		return nil, err
	}
	if buyer.Status != "active" {
		return nil, errors.New("payments: حساب کاربری شما مسدود شده است")
	}

	// 3. Validate course
	course, err := s.courseQueries.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: toPgUUID(p.TenantID),
		ID:       toPgUUID(p.CourseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("payments: دوره آموزشی یافت نشد")
		}
		return nil, err
	}
	if course.Status != "published" {
		return nil, errors.New("payments: دوره مورد نظر در وضعیت انتشار قرار ندارد")
	}
	if course.PriceRials <= 0 {
		return nil, errors.New("payments: مبلغ دوره نامعتبر است")
	}

	// 4. Check if buyer already holds an active entitlement for this course
	_, err = s.courseQueries.GetActiveCourseEntitlement(ctx, coursesdb.GetActiveCourseEntitlementParams{
		TenantID: toPgUUID(p.TenantID),
		BuyerID:  toPgUUID(p.BuyerID),
		CourseID: toPgUUID(p.CourseID),
	})
	if err == nil {
		return nil, ErrCourseAlreadyEntitled
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 5. Create Purchase record (snapshots course title and price in Rials)
	purchase, err := s.queries.CreatePurchase(ctx, paymentsdb.CreatePurchaseParams{
		TenantID:      toPgUUID(p.TenantID),
		ItemType:      "course",
		ItemID:        course.ID,
		AmountRials:   course.PriceRials,
		TitleSnapshot: course.Title,
		Status:        "initiated",
	})
	if err != nil {
		return nil, err
	}

	// 6. Create pending Course Entitlement record linked to purchase
	_, err = s.courseQueries.CreateCourseEntitlement(ctx, coursesdb.CreateCourseEntitlementParams{
		TenantID:            toPgUUID(p.TenantID),
		BuyerID:             toPgUUID(p.BuyerID),
		CourseID:            course.ID,
		PurchaseID:          purchase.ID,
		Status:              "pending",
		CourseTitleSnapshot: course.Title,
		PriceRialsSnapshot:  course.PriceRials,
		GrantedAt:           pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		return nil, err
	}

	// 7. EXTERNAL NETWORK CALL OUTSIDE DATABASE TRANSACTION:
	paymentResp, err := s.provider.RequestPayment(ctx, PaymentRequest{
		AmountRials: course.PriceRials,
		Description: "خرید دوره: " + course.Title,
		CallbackURL: callbackURL,
	})
	if err != nil {
		_, _ = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
			TenantID: toPgUUID(p.TenantID),
			ID:       purchase.ID,
			Status:   "failed",
		})
		return nil, err
	}

	// 8. Record payment attempt
	expiresAt := now.Add(15 * time.Minute)
	attempt, err := s.queries.CreatePaymentAttempt(ctx, paymentsdb.CreatePaymentAttemptParams{
		TenantID:       toPgUUID(p.TenantID),
		PurchaseID:     purchase.ID,
		Provider:       s.provider.Name(),
		Authority:      paymentResp.Authority,
		AmountRials:    course.PriceRials,
		Status:         "initiated",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return nil, err
	}

	return &InitiatePaymentResult{
		PaymentAttemptID: fromPgUUID(attempt.ID),
		PurchaseID:       fromPgUUID(purchase.ID),
		Authority:        paymentResp.Authority,
		PaymentURL:       paymentResp.PaymentURL,
		AmountRials:      purchase.AmountRials,
		Status:           attempt.Status,
		ExpiresAt:        &expiresAt,
	}, nil
}

type CallbackResult struct {
	Status         string    `json:"status"` // "confirmed", "refund_required", "failed"
	RegistrationID uuid.UUID `json:"registration_id,omitempty"`
	WorkshopID     uuid.UUID `json:"workshop_id,omitempty"`
	CourseID       uuid.UUID `json:"course_id,omitempty"`
	PurchaseID     uuid.UUID `json:"purchase_id"`
	AmountRials    int64     `json:"amount_rials"`
	RefID          string    `json:"ref_id,omitempty"`
	TrackingCode   string    `json:"tracking_code,omitempty"`
}

// HandleCallbackAndVerify processes the payment gateway return callback.
// Network calls to verify the payment are executed outside database transactions.
// Result reconciliation follows strict lock hierarchy: Workshop -> Registration -> Purchase / PaymentAttempt.
func (s *Service) HandleCallbackAndVerify(ctx context.Context, tenantID uuid.UUID, authority, statusParam string, now time.Time) (*CallbackResult, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("payments: tenant_id cannot be nil")
	}
	authority = strings.TrimSpace(authority)
	if authority == "" {
		return nil, ErrInvalidAuthority
	}

	if now.IsZero() {
		now = time.Now()
	}

	// 1. Fetch payment attempt by Authority in the current Tenant
	attempt, err := s.queries.GetPaymentAttemptByAuthority(ctx, paymentsdb.GetPaymentAttemptByAuthorityParams{
		TenantID:  toPgUUID(tenantID),
		Authority: authority,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentAttemptNotFound
		}
		return nil, err
	}

	purchaseID := fromPgUUID(attempt.PurchaseID)
	purchase, err := s.queries.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       attempt.PurchaseID,
	})
	if err != nil {
		return nil, err
	}

	if purchase.ItemType == "course" {
		return s.handleCourseCallbackAndVerify(ctx, tenantID, authority, statusParam, attempt, purchase, now)
	}

	// Fetch registration linked to purchase
	reg, err := s.workshopQueries.GetRegistrationByPurchaseID(ctx, workshopsdb.GetRegistrationByPurchaseIDParams{
		TenantID:   toPgUUID(tenantID),
		PurchaseID: attempt.PurchaseID,
	})
	if err != nil {
		return nil, err
	}
	regID := fromPgUUID(reg.ID)
	wsID := fromPgUUID(reg.WorkshopID)

	// 2. Idempotency Check: if callback is retried on an already verified attempt
	// "Callback تکراری و اجرای مجدد Worker تنها یک پرداخت و اثر تجاری ثبت می‌کنند."
	if attempt.Status == "verified" {
		refID := ""
		if attempt.GatewayRefID.Valid {
			refID = attempt.GatewayRefID.String
		}
		status := "confirmed"
		if purchase.Status == "refund_required" {
			status = "refund_required"
		}
		return &CallbackResult{
			Status:         status,
			RegistrationID: regID,
			WorkshopID:     wsID,
			PurchaseID:     purchaseID,
			AmountRials:    attempt.AmountRials,
			RefID:          refID,
			TrackingCode:   refID,
		}, nil
	}

	if attempt.Status == "failed" {
		return &CallbackResult{
			Status:         "failed",
			RegistrationID: regID,
			WorkshopID:     wsID,
			PurchaseID:     purchaseID,
			AmountRials:    attempt.AmountRials,
		}, nil
	}

	// 3. User canceled on gateway (Status != "OK")
	if strings.ToUpper(strings.TrimSpace(statusParam)) != "OK" {
		_, _ = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
			TenantID:     toPgUUID(tenantID),
			ID:           attempt.ID,
			Status:       "failed",
			ErrorMessage: pgtype.Text{String: "canceled by user on payment gateway", Valid: true},
		})
		_, _ = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       attempt.PurchaseID,
			Status:   "failed",
		})
		return &CallbackResult{
			Status:         "failed",
			RegistrationID: regID,
			WorkshopID:     wsID,
			PurchaseID:     purchaseID,
			AmountRials:    attempt.AmountRials,
		}, nil
	}

	// 4. Mismatch check: verify amount in attempt matches purchase
	if attempt.AmountRials != purchase.AmountRials {
		_, _ = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
			TenantID:     toPgUUID(tenantID),
			ID:           attempt.ID,
			Status:       "failed",
			ErrorMessage: pgtype.Text{String: "amount mismatch between attempt and purchase", Valid: true},
		})
		return nil, ErrAmountMismatch
	}

	// 5. EXTERNAL NETWORK CALL OUTSIDE DATABASE TRANSACTION:
	// Verify with payment gateway
	verifyResp, err := s.provider.VerifyPayment(ctx, VerifyRequest{
		Authority:   authority,
		AmountRials: attempt.AmountRials,
	})
	if err != nil {
		if errors.Is(err, ErrGatewayTimeout) || errors.Is(err, ErrGatewayUnavailable) {
			// Transient network/gateway error: do NOT mark as failed, return error for retry
			return nil, err
		}
		// If verification failed or rejected by gateway:
		_, _ = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
			TenantID:     toPgUUID(tenantID),
			ID:           attempt.ID,
			Status:       "failed",
			ErrorMessage: pgtype.Text{String: err.Error(), Valid: true},
		})
		_, _ = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       attempt.PurchaseID,
			Status:   "failed",
		})
		return &CallbackResult{
			Status:         "failed",
			RegistrationID: regID,
			WorkshopID:     wsID,
			PurchaseID:     purchaseID,
			AmountRials:    attempt.AmountRials,
		}, nil
	}

	refID := verifyResp.RefID

	// 6. ATOMIC DATABASE RECONCILIATION & STRICT LOCK HIERARCHY
	// Strict Lock order: Workshop -> Registration -> Purchase / PaymentAttempt
	ws, err := s.workshopQueries.GetWorkshopByIDForUpdate(ctx, workshopsdb.GetWorkshopByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       reg.WorkshopID,
	})
	if err != nil {
		return nil, err
	}

	lockedReg, err := s.workshopQueries.GetRegistrationByPurchaseIDForUpdate(ctx, workshopsdb.GetRegistrationByPurchaseIDForUpdateParams{
		TenantID:   toPgUUID(tenantID),
		PurchaseID: attempt.PurchaseID,
	})
	if err != nil {
		return nil, err
	}

	// Check conditions for confirming registration:
	// a) Workshop is still published (not cancelled or closed)
	// b) Registration hold is still pending_payment
	// c) Hold has not expired (ExpiresAt > now)
	isWorkshopActive := ws.Status == "published"
	isHoldActive := lockedReg.Status == "pending_payment" && lockedReg.ExpiresAt.Valid && lockedReg.ExpiresAt.Time.After(now)

	if isWorkshopActive && isHoldActive {
		// SUCCESSFUL CONFIRMATION:
		// Transition registration to 'confirmed', clear hold expiration
		_, err = s.workshopQueries.UpdateRegistrationStatus(ctx, workshopsdb.UpdateRegistrationStatusParams{
			TenantID:   toPgUUID(tenantID),
			ID:         lockedReg.ID,
			Status:     "confirmed",
			PurchaseID: attempt.PurchaseID,
		})
		if err != nil {
			return nil, err
		}

		_, err = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       attempt.PurchaseID,
			Status:   "completed",
		})
		if err != nil {
			return nil, err
		}

		_, err = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
			TenantID:     toPgUUID(tenantID),
			ID:           attempt.ID,
			Status:       "verified",
			GatewayRefID: pgtype.Text{String: refID, Valid: true},
			TrackingCode: pgtype.Text{String: refID, Valid: true},
		})
		if err != nil {
			return nil, err
		}

		return &CallbackResult{
			Status:         "confirmed",
			RegistrationID: regID,
			WorkshopID:     wsID,
			PurchaseID:     purchaseID,
			AmountRials:    attempt.AmountRials,
			RefID:          refID,
			TrackingCode:   refID,
		}, nil
	}

	// PAYMENT AFTER EXPIRY OR WORKSHOP CANCELLATION:
	// Customer paid, but seat hold expired or workshop was cancelled.
	// Must transition to refund_required and MUST NOT occupy a seat!
	// "اگر نگه‌داشت منقضی یا کارگاه لغو شده باشد، پرداخت موفق به refund_required می‌رود... صندلی جدید نمی‌گیرد."
	_, err = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       attempt.PurchaseID,
		Status:   "refund_required",
	})
	if err != nil {
		return nil, err
	}

	_, err = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
		TenantID:     toPgUUID(tenantID),
		ID:           attempt.ID,
		Status:       "verified",
		GatewayRefID: pgtype.Text{String: refID, Valid: true},
		TrackingCode: pgtype.Text{String: refID, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	// If registration was pending, mark as expired so it occupies zero seats
	if lockedReg.Status == "pending_payment" {
		_, _ = s.workshopQueries.UpdateRegistrationStatus(ctx, workshopsdb.UpdateRegistrationStatusParams{
			TenantID:   toPgUUID(tenantID),
			ID:         lockedReg.ID,
			Status:     "expired",
			PurchaseID: attempt.PurchaseID,
		})
	}

	return &CallbackResult{
		Status:         "refund_required",
		RegistrationID: regID,
		WorkshopID:     wsID,
		PurchaseID:     purchaseID,
		AmountRials:    attempt.AmountRials,
		RefID:          refID,
		TrackingCode:   refID,
	}, nil
}

func (s *Service) handleCourseCallbackAndVerify(
	ctx context.Context,
	tenantID uuid.UUID,
	authority, statusParam string,
	attempt paymentsdb.PaymentAttempt,
	purchase paymentsdb.Purchase,
	now time.Time,
) (*CallbackResult, error) {
	purchaseID := fromPgUUID(purchase.ID)
	courseID := fromPgUUID(purchase.ItemID)

	if s.courseQueries == nil {
		return nil, errors.New("payments: course querier not initialized")
	}

	// 1. Idempotency Check: if callback is retried on an already verified attempt
	// "Callback تکراری و اجرای مجدد Worker تنها یک پرداخت و اثر تجاری ثبت می‌کنند."
	if attempt.Status == "verified" {
		refID := ""
		if attempt.GatewayRefID.Valid {
			refID = attempt.GatewayRefID.String
		}
		status := "confirmed"
		if purchase.Status == "refund_required" {
			status = "refund_required"
		}
		return &CallbackResult{
			Status:       status,
			CourseID:     courseID,
			PurchaseID:   purchaseID,
			AmountRials:  attempt.AmountRials,
			RefID:        refID,
			TrackingCode: refID,
		}, nil
	}

	if attempt.Status == "failed" {
		return &CallbackResult{
			Status:      "failed",
			CourseID:    courseID,
			PurchaseID:  purchaseID,
			AmountRials: attempt.AmountRials,
		}, nil
	}

	// 2. User canceled on gateway (Status != "OK")
	if strings.ToUpper(strings.TrimSpace(statusParam)) != "OK" {
		_, _ = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
			TenantID:     toPgUUID(tenantID),
			ID:           attempt.ID,
			Status:       "failed",
			ErrorMessage: pgtype.Text{String: "canceled by user on payment gateway", Valid: true},
		})
		_, _ = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       attempt.PurchaseID,
			Status:   "failed",
		})
		return &CallbackResult{
			Status:      "failed",
			CourseID:    courseID,
			PurchaseID:  purchaseID,
			AmountRials: attempt.AmountRials,
		}, nil
	}

	// 3. Mismatch check: verify amount in attempt matches purchase
	if attempt.AmountRials != purchase.AmountRials {
		_, _ = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
			TenantID:     toPgUUID(tenantID),
			ID:           attempt.ID,
			Status:       "failed",
			ErrorMessage: pgtype.Text{String: "amount mismatch between attempt and purchase", Valid: true},
		})
		return nil, ErrAmountMismatch
	}

	// 4. EXTERNAL NETWORK CALL OUTSIDE DATABASE TRANSACTION: Verify with payment gateway
	verifyResp, err := s.provider.VerifyPayment(ctx, VerifyRequest{
		Authority:   authority,
		AmountRials: attempt.AmountRials,
	})
	if err != nil {
		if errors.Is(err, ErrGatewayTimeout) || errors.Is(err, ErrGatewayUnavailable) {
			return nil, err
		}
		_, _ = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
			TenantID:     toPgUUID(tenantID),
			ID:           attempt.ID,
			Status:       "failed",
			ErrorMessage: pgtype.Text{String: err.Error(), Valid: true},
		})
		_, _ = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       attempt.PurchaseID,
			Status:   "failed",
		})
		return &CallbackResult{
			Status:      "failed",
			CourseID:    courseID,
			PurchaseID:  purchaseID,
			AmountRials: attempt.AmountRials,
		}, nil
	}

	refID := verifyResp.RefID

	// 5. ATOMIC DATABASE RECONCILIATION & STRICT HIERARCHY:
	// Course -> Entitlement -> Purchase / PaymentAttempt
	lockedCourse, err := s.courseQueries.GetCourseByIDForUpdate(ctx, coursesdb.GetCourseByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       purchase.ItemID,
	})
	if err != nil {
		return nil, err
	}

	ent, err := s.courseQueries.GetCourseEntitlementByPurchaseID(ctx, coursesdb.GetCourseEntitlementByPurchaseIDParams{
		TenantID:   toPgUUID(tenantID),
		PurchaseID: attempt.PurchaseID,
	})
	if err != nil {
		return nil, err
	}

	// Check if buyer ALREADY holds another active entitlement for this course (concurrent duplicate payment)
	existingActive, err := s.courseQueries.GetActiveCourseEntitlement(ctx, coursesdb.GetActiveCourseEntitlementParams{
		TenantID: toPgUUID(tenantID),
		BuyerID:  ent.BuyerID,
		CourseID: ent.CourseID,
	})
	hasDuplicateActive := (err == nil && fromPgUUID(existingActive.ID) != fromPgUUID(ent.ID))

	isCourseActive := (lockedCourse.Status == "published")

	if isCourseActive && !hasDuplicateActive {
		// SUCCESSFUL CONFIRMATION:
		// Transition entitlement to active in the SAME atomic transaction
		_, err = s.courseQueries.ActivateCourseEntitlement(ctx, coursesdb.ActivateCourseEntitlementParams{
			TenantID:  toPgUUID(tenantID),
			ID:        ent.ID,
			GrantedAt: toPgTimestamptz(now),
		})
		if err != nil {
			return nil, err
		}

		_, err = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       attempt.PurchaseID,
			Status:   "completed",
		})
		if err != nil {
			return nil, err
		}

		_, err = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
			TenantID:     toPgUUID(tenantID),
			ID:           attempt.ID,
			Status:       "verified",
			GatewayRefID: pgtype.Text{String: refID, Valid: true},
			TrackingCode: pgtype.Text{String: refID, Valid: true},
		})
		if err != nil {
			return nil, err
		}

		return &CallbackResult{
			Status:       "confirmed",
			CourseID:     courseID,
			PurchaseID:   purchaseID,
			AmountRials:  attempt.AmountRials,
			RefID:        refID,
			TrackingCode: refID,
		}, nil
	}

	// DUPLICATE PAYMENT OR UNPUBLISHED COURSE:
	// Customer paid on gateway, but already has active entitlement or course is no longer published.
	// Transition to refund_required and DO NOT activate entitlement!
	// "تست پرداخت تکراری (پرداخت دوم هم‌زمان به refund_required می‌رود و entitlement تکراری ساخته نمی‌شود)"
	_, err = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       attempt.PurchaseID,
		Status:   "refund_required",
	})
	if err != nil {
		return nil, err
	}

	_, err = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
		TenantID:     toPgUUID(tenantID),
		ID:           attempt.ID,
		Status:       "verified",
		GatewayRefID: pgtype.Text{String: refID, Valid: true},
		TrackingCode: pgtype.Text{String: refID, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return &CallbackResult{
		Status:       "refund_required",
		CourseID:     courseID,
		PurchaseID:   purchaseID,
		AmountRials:  attempt.AmountRials,
		RefID:        refID,
		TrackingCode: refID,
	}, nil
}

type ReconcileStalePaymentsParams struct {
	TenantID  uuid.UUID
	OlderThan time.Duration
	Limit     int32
	Now       time.Time
}

type ReconcileResult struct {
	ProcessedCount int `json:"processed_count"`
	ConfirmedCount int `json:"confirmed_count"`
	RefundRequired int `json:"refund_required_count"`
	FailedCount    int `json:"failed_count"`
	TransientError int `json:"transient_error_count"`
}

// ReconcileStalePayments processes un-reconciled initiated payment attempts in bounded PostgreSQL batches.
// Crash recovery: Any remaining un-reconciled attempts are safely picked up on the next cycle or restart.
func (s *Service) ReconcileStalePayments(ctx context.Context, p ReconcileStalePaymentsParams) (*ReconcileResult, error) {
	if p.TenantID == uuid.Nil {
		return nil, errors.New("payments: tenant_id cannot be nil")
	}
	now := p.Now
	if now.IsZero() {
		now = time.Now()
	}
	olderThan := p.OlderThan
	if olderThan <= 0 {
		olderThan = 15 * time.Minute
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}

	// 1. Expire outdated seat holds in database first
	_, _ = s.workshopQueries.ExpirePendingRegistrations(ctx)

	// 2. Fetch stale initiated attempts
	cutoff := now.Add(-olderThan)
	staleAttempts, err := s.queries.ListStalePaymentAttempts(ctx, paymentsdb.ListStalePaymentAttemptsParams{
		TenantID:  toPgUUID(p.TenantID),
		CreatedAt: toPgTimestamptz(cutoff),
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}

	res := &ReconcileResult{
		ProcessedCount: len(staleAttempts),
	}

	for _, att := range staleAttempts {
		// Reconcile via HandleCallbackAndVerify
		cbRes, err := s.HandleCallbackAndVerify(ctx, p.TenantID, att.Authority, "OK", now)
		if err != nil {
			if errors.Is(err, ErrGatewayTimeout) || errors.Is(err, ErrGatewayUnavailable) {
				// Transient error; leave for next retry cycle
				res.TransientError++
				continue
			}
			// Mismatch or non-retryable error: fail the attempt
			_, _ = s.queries.UpdatePaymentAttemptStatus(ctx, paymentsdb.UpdatePaymentAttemptStatusParams{
				TenantID:     att.TenantID,
				ID:           att.ID,
				Status:       "failed",
				ErrorMessage: pgtype.Text{String: err.Error(), Valid: true},
			})
			res.FailedCount++
			continue
		}

		switch cbRes.Status {
		case "confirmed":
			res.ConfirmedCount++
		case "refund_required":
			res.RefundRequired++
		default:
			res.FailedCount++
		}
	}

	return res, nil
}

type AdminPurchaseItem struct {
	ID            uuid.UUID `json:"id"`
	ItemType      string    `json:"item_type"`
	ItemID        uuid.UUID `json:"item_id"`
	TitleSnapshot string    `json:"title_snapshot"`
	AmountRials   int64     `json:"amount_rials"`
	Status        string    `json:"status"`
	TrackingCode  *string   `json:"tracking_code,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Service) ListAdminPurchases(ctx context.Context, tenantID uuid.UUID, statusFilter *string) ([]AdminPurchaseItem, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("payments: tenant_id cannot be nil")
	}

	var statusParam pgtype.Text
	if statusFilter != nil && *statusFilter != "" {
		statusParam = pgtype.Text{String: *statusFilter, Valid: true}
	}

	rows, err := s.queries.ListPurchases(ctx, paymentsdb.ListPurchasesParams{
		TenantID: toPgUUID(tenantID),
		Status:   statusParam,
	})
	if err != nil {
		return nil, err
	}

	res := make([]AdminPurchaseItem, 0, len(rows))
	for _, r := range rows {
		var tracking *string
		attempts, err := s.queries.ListPaymentAttemptsByPurchase(ctx, paymentsdb.ListPaymentAttemptsByPurchaseParams{
			TenantID:   r.TenantID,
			PurchaseID: r.ID,
		})
		if err == nil && len(attempts) > 0 {
			for _, att := range attempts {
				if att.TrackingCode.Valid && att.TrackingCode.String != "" {
					code := att.TrackingCode.String
					tracking = &code
					break
				}
			}
		}

		res = append(res, AdminPurchaseItem{
			ID:            fromPgUUID(r.ID),
			ItemType:      r.ItemType,
			ItemID:        fromPgUUID(r.ItemID),
			TitleSnapshot: r.TitleSnapshot,
			AmountRials:   r.AmountRials,
			Status:        r.Status,
			TrackingCode:  tracking,
			CreatedAt:     r.CreatedAt.Time,
		})
	}
	return res, nil
}

type RecordRefundParams struct {
	TenantID        uuid.UUID
	PurchaseID      uuid.UUID
	ReferenceNumber string
	Reason          string
	AmountRials     int64
	RegisteredBy    uuid.UUID
	Now             time.Time
}

type RefundResponse struct {
	RefundID        uuid.UUID `json:"refund_id"`
	PurchaseID      uuid.UUID `json:"purchase_id"`
	ReferenceNumber string    `json:"reference_number"`
	AmountRials     int64     `json:"amount_rials"`
	RegisteredAt    time.Time `json:"registered_at"`
}

// RecordRefund records a manual refund performed by the clinic owner for a refund_required purchase.
// Idempotency: "ثبت دوبارهٔ همان مرجع اثر تازه ندارد."
func (s *Service) RecordRefund(ctx context.Context, p RecordRefundParams) (*RefundResponse, error) {
	if p.TenantID == uuid.Nil {
		return nil, errors.New("payments: tenant_id cannot be nil")
	}
	if p.PurchaseID == uuid.Nil {
		return nil, errors.New("payments: purchase_id cannot be nil")
	}
	if p.RegisteredBy == uuid.Nil {
		return nil, errors.New("payments: registered_by cannot be nil")
	}
	refNum := strings.TrimSpace(p.ReferenceNumber)
	if len(refNum) < 3 {
		return nil, errors.New("payments: reference_number must be at least 3 characters")
	}
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		return nil, errors.New("payments: reason cannot be empty")
	}
	if p.AmountRials <= 0 {
		return nil, ErrInvalidAmount
	}

	now := p.Now
	if now.IsZero() {
		now = time.Now()
	}

	// 1. Lock purchase
	purchase, err := s.queries.GetPurchaseByIDForUpdate(ctx, paymentsdb.GetPurchaseByIDForUpdateParams{
		TenantID: toPgUUID(p.TenantID),
		ID:       toPgUUID(p.PurchaseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPurchaseNotFound
		}
		return nil, err
	}

	// 2. Check if already refunded
	if purchase.Status == "refunded" {
		// Idempotency: "ثبت دوبارهٔ همان مرجع اثر تازه ندارد."
		existingRefund, err := s.queries.GetRefundByReference(ctx, paymentsdb.GetRefundByReferenceParams{
			TenantID:        toPgUUID(p.TenantID),
			ReferenceNumber: refNum,
		})
		if err == nil {
			if fromPgUUID(existingRefund.PurchaseID) == p.PurchaseID {
				// Same purchase, same reference number -> clean idempotent return
				return &RefundResponse{
					RefundID:        fromPgUUID(existingRefund.ID),
					PurchaseID:      fromPgUUID(existingRefund.PurchaseID),
					ReferenceNumber: existingRefund.ReferenceNumber,
					AmountRials:     existingRefund.AmountRials,
					RegisteredAt:    existingRefund.RegisteredAt.Time,
				}, nil
			}
			return nil, ErrDuplicateRefundReference
		}
		return nil, ErrPurchaseAlreadyRefunded
	}

	if purchase.ItemType == "course" {
		if purchase.Status != "refund_required" && purchase.Status != "completed" {
			return nil, ErrInvalidPurchaseStatusForRefund
		}
	} else {
		if purchase.Status != "refund_required" {
			return nil, ErrInvalidPurchaseStatusForRefund
		}
	}

	if p.AmountRials != purchase.AmountRials {
		return nil, ErrAmountMismatch
	}

	// 3. Check reference number uniqueness across tenant
	existingRef, err := s.queries.GetRefundByReference(ctx, paymentsdb.GetRefundByReferenceParams{
		TenantID:        toPgUUID(p.TenantID),
		ReferenceNumber: refNum,
	})
	if err == nil {
		_ = existingRef
		return nil, ErrDuplicateRefundReference
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 4. Find verified payment attempt for reference link
	var paymentAttemptID pgtype.UUID
	att, err := s.queries.GetVerifiedPaymentAttemptByPurchase(ctx, paymentsdb.GetVerifiedPaymentAttemptByPurchaseParams{
		TenantID:   toPgUUID(p.TenantID),
		PurchaseID: purchase.ID,
	})
	if err == nil {
		paymentAttemptID = att.ID
	}

	// 5. Create refund record
	refund, err := s.queries.CreateRefund(ctx, paymentsdb.CreateRefundParams{
		TenantID:         toPgUUID(p.TenantID),
		PurchaseID:       purchase.ID,
		PaymentAttemptID: paymentAttemptID,
		AmountRials:      p.AmountRials,
		Reason:           reason,
		ReferenceNumber:  refNum,
		RegisteredBy:     toPgUUID(p.RegisteredBy),
	})
	if err != nil {
		return nil, err
	}

	// 6. Update purchase status to 'refunded'
	_, err = s.queries.UpdatePurchaseStatus(ctx, paymentsdb.UpdatePurchaseStatusParams{
		TenantID: toPgUUID(p.TenantID),
		ID:       purchase.ID,
		Status:   "refunded",
	})
	if err != nil {
		return nil, err
	}

	// 7. If course purchase: revoke entitlement
	// "بازپرداخت ثبت‌شده entitlement همان خرید را لغو می‌کند."
	if purchase.ItemType == "course" && s.courseQueries != nil {
		_, _ = s.courseQueries.RevokeCourseEntitlementByPurchaseID(ctx, coursesdb.RevokeCourseEntitlementByPurchaseIDParams{
			TenantID:         toPgUUID(p.TenantID),
			PurchaseID:       purchase.ID,
			RevokedAt:        toPgTimestamptz(now),
			RevocationReason: pgtype.Text{String: reason, Valid: true},
		})
	}

	return &RefundResponse{
		RefundID:        fromPgUUID(refund.ID),
		PurchaseID:      fromPgUUID(refund.PurchaseID),
		ReferenceNumber: refund.ReferenceNumber,
		AmountRials:     refund.AmountRials,
		RegisteredAt:    refund.RegisteredAt.Time,
	}, nil
}
