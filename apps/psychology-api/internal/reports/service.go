package reports

import (
	"context"
	"errors"
	"time"

	"psychology/api/internal/reports/reportsdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	// StorageQuotaBytes represents the 10 GiB limit for tenant media storage
	StorageQuotaBytes int64 = 10 * 1024 * 1024 * 1024
)

var (
	ErrNilTenantID = errors.New("reports: شناسه کلینیک نمی‌تواند خالی باشد")
)

type Querier interface {
	GetWorkshopMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetWorkshopMetricsRow, error)
	GetRegistrationMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetRegistrationMetricsRow, error)
	GetTherapistCount(ctx context.Context, tenantID pgtype.UUID) (int64, error)
	GetSlotMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetSlotMetricsRow, error)
	GetAppointmentMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetAppointmentMetricsRow, error)
	GetCourseMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetCourseMetricsRow, error)
	GetLessonCount(ctx context.Context, tenantID pgtype.UUID) (int64, error)
	GetEntitlementMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetEntitlementMetricsRow, error)
	GetFinancialMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetFinancialMetricsRow, error)
	GetRefundMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetRefundMetricsRow, error)
	GetPaymentAttemptMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetPaymentAttemptMetricsRow, error)
	GetMediaMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetMediaMetricsRow, error)
}

// Non-PII Metrics Data Structures

type WorkshopMetrics struct {
	TotalWorkshops         int64 `json:"total_workshops"`
	PublishedWorkshops     int64 `json:"published_workshops"`
	DraftWorkshops         int64 `json:"draft_workshops"`
	ClosedWorkshops        int64 `json:"closed_workshops"`
	CancelledWorkshops     int64 `json:"cancelled_workshops"`
	TotalRegistrations     int64 `json:"total_registrations"`
	ConfirmedRegistrations int64 `json:"confirmed_registrations"`
	PendingRegistrations   int64 `json:"pending_registrations"`
	CancelledRegistrations int64 `json:"cancelled_registrations"`
}

type BookingMetrics struct {
	ActiveTherapists      int64 `json:"active_therapists"`
	TotalSlots            int64 `json:"total_slots"`
	AvailableSlots        int64 `json:"available_slots"`
	BookedSlots           int64 `json:"booked_slots"`
	HeldSlots             int64 `json:"held_slots"`
	TotalAppointments     int64 `json:"total_appointments"`
	ConfirmedAppointments int64 `json:"confirmed_appointments"`
	PendingAppointments   int64 `json:"pending_appointments"`
	CompletedAppointments int64 `json:"completed_appointments"`
	CancelledAppointments int64 `json:"cancelled_appointments"`
}

type CourseMetrics struct {
	TotalCourses        int64 `json:"total_courses"`
	PublishedCourses    int64 `json:"published_courses"`
	DraftCourses        int64 `json:"draft_courses"`
	ArchivedCourses     int64 `json:"archived_courses"`
	TotalLessons        int64 `json:"total_lessons"`
	TotalEntitlements   int64 `json:"total_entitlements"`
	ActiveEntitlements  int64 `json:"active_entitlements"`
	RevokedEntitlements int64 `json:"revoked_entitlements"`
	PendingEntitlements int64 `json:"pending_entitlements"`
}

type FinancialMetrics struct {
	TotalPurchases          int64 `json:"total_purchases"`
	CompletedPurchases      int64 `json:"completed_purchases"`
	PendingPurchases        int64 `json:"pending_purchases"`
	FailedPurchases         int64 `json:"failed_purchases"`
	RefundRequiredPurchases int64 `json:"refund_required_purchases"`
	RefundedPurchases       int64 `json:"refunded_purchases"`
	GrossRevenueRials       int64 `json:"gross_revenue_rials"`
	TotalRefundedRials      int64 `json:"total_refunded_rials"`
	NetRevenueRials         int64 `json:"net_revenue_rials"`
	WorkshopRevenueRials    int64 `json:"workshop_revenue_rials"`
	CourseRevenueRials      int64 `json:"course_revenue_rials"`
}

type PaymentAttemptMetrics struct {
	TotalAttempts     int64 `json:"total_attempts"`
	InitiatedAttempts int64 `json:"initiated_attempts"`
	VerifiedAttempts  int64 `json:"verified_attempts"`
	FailedAttempts    int64 `json:"failed_attempts"`
}

type MediaMetrics struct {
	TotalAssets             int64 `json:"total_assets"`
	TotalBytes              int64 `json:"total_bytes"`
	StorageQuotaBytes       int64 `json:"storage_quota_bytes"`
	UsagePercentagePermille int64 `json:"usage_percentage_permille"`
	UsagePercentage         int64 `json:"usage_percentage"`
	PublicBytes             int64 `json:"public_bytes"`
	CoursePrivateBytes      int64 `json:"course_private_bytes"`
	VideoCount              int64 `json:"video_count"`
	AudioCount              int64 `json:"audio_count"`
	ImageCount              int64 `json:"image_count"`
}

type TenantMetricsReport struct {
	TenantID        uuid.UUID             `json:"tenant_id"`
	GeneratedAt     time.Time             `json:"generated_at"`
	Workshops       WorkshopMetrics       `json:"workshops"`
	Booking         BookingMetrics        `json:"booking"`
	Courses         CourseMetrics         `json:"courses"`
	Financial       FinancialMetrics      `json:"financial"`
	PaymentAttempts PaymentAttemptMetrics `json:"payment_attempts"`
	Media           MediaMetrics          `json:"media"`
}

type Service struct {
	queries Querier
	nowFunc func() time.Time
}

func NewService(q Querier) *Service {
	return &Service{
		queries: q,
		nowFunc: time.Now,
	}
}

func (s *Service) GetTenantMetrics(ctx context.Context, tenantID uuid.UUID) (*TenantMetricsReport, error) {
	if tenantID == uuid.Nil {
		return nil, ErrNilTenantID
	}

	var pgTenantID pgtype.UUID
	_ = pgTenantID.Scan(tenantID.String())

	// 1. Workshops & Registrations
	wsRow, err := s.queries.GetWorkshopMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}
	regRow, err := s.queries.GetRegistrationMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}

	// 2. Booking (Therapists, Slots, Appointments)
	thCount, err := s.queries.GetTherapistCount(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}
	slotRow, err := s.queries.GetSlotMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}
	apptRow, err := s.queries.GetAppointmentMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}

	// 3. Courses & Lessons & Entitlements
	cRow, err := s.queries.GetCourseMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}
	lCount, err := s.queries.GetLessonCount(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}
	entRow, err := s.queries.GetEntitlementMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}

	// 4. Financial & Refunds & Payment Attempts
	finRow, err := s.queries.GetFinancialMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}
	refRow, err := s.queries.GetRefundMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}
	attRow, err := s.queries.GetPaymentAttemptMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}

	// 5. Media & Storage
	mediaRow, err := s.queries.GetMediaMetrics(ctx, pgTenantID)
	if err != nil {
		return nil, err
	}

	// Calculations
	netRev := finRow.GrossRevenueRials - refRow.TotalRefundedRials
	if netRev < 0 {
		netRev = 0
	}

	var permille int64
	var percent int64
	if StorageQuotaBytes > 0 {
		permille = (mediaRow.TotalBytes * 1000) / StorageQuotaBytes
		percent = (mediaRow.TotalBytes * 100) / StorageQuotaBytes
	}

	now := s.nowFunc().UTC()

	return &TenantMetricsReport{
		TenantID:    tenantID,
		GeneratedAt: now,
		Workshops: WorkshopMetrics{
			TotalWorkshops:         wsRow.TotalWorkshops,
			PublishedWorkshops:     wsRow.PublishedWorkshops,
			DraftWorkshops:         wsRow.DraftWorkshops,
			ClosedWorkshops:        wsRow.ClosedWorkshops,
			CancelledWorkshops:     wsRow.CancelledWorkshops,
			TotalRegistrations:     regRow.TotalRegistrations,
			ConfirmedRegistrations: regRow.ConfirmedRegistrations,
			PendingRegistrations:   regRow.PendingRegistrations,
			CancelledRegistrations: regRow.CancelledRegistrations,
		},
		Booking: BookingMetrics{
			ActiveTherapists:      thCount,
			TotalSlots:            slotRow.TotalSlots,
			AvailableSlots:        slotRow.AvailableSlots,
			BookedSlots:           slotRow.BookedSlots,
			HeldSlots:             slotRow.HeldSlots,
			TotalAppointments:     apptRow.TotalAppointments,
			ConfirmedAppointments: apptRow.ConfirmedAppointments,
			PendingAppointments:   apptRow.PendingAppointments,
			CompletedAppointments: apptRow.CompletedAppointments,
			CancelledAppointments: apptRow.CancelledAppointments,
		},
		Courses: CourseMetrics{
			TotalCourses:        cRow.TotalCourses,
			PublishedCourses:    cRow.PublishedCourses,
			DraftCourses:        cRow.DraftCourses,
			ArchivedCourses:     cRow.ArchivedCourses,
			TotalLessons:        lCount,
			TotalEntitlements:   entRow.TotalEntitlements,
			ActiveEntitlements:  entRow.ActiveEntitlements,
			RevokedEntitlements: entRow.RevokedEntitlements,
			PendingEntitlements: entRow.PendingEntitlements,
		},
		Financial: FinancialMetrics{
			TotalPurchases:          finRow.TotalPurchases,
			CompletedPurchases:      finRow.CompletedPurchases,
			PendingPurchases:        finRow.PendingPurchases,
			FailedPurchases:         finRow.FailedPurchases,
			RefundRequiredPurchases: finRow.RefundRequiredPurchases,
			RefundedPurchases:       finRow.RefundedPurchases,
			GrossRevenueRials:       finRow.GrossRevenueRials,
			TotalRefundedRials:      refRow.TotalRefundedRials,
			NetRevenueRials:         netRev,
			WorkshopRevenueRials:    finRow.WorkshopRevenueRials,
			CourseRevenueRials:      finRow.CourseRevenueRials,
		},
		PaymentAttempts: PaymentAttemptMetrics{
			TotalAttempts:     attRow.TotalAttempts,
			InitiatedAttempts: attRow.InitiatedAttempts,
			VerifiedAttempts:  attRow.VerifiedAttempts,
			FailedAttempts:    attRow.FailedAttempts,
		},
		Media: MediaMetrics{
			TotalAssets:             mediaRow.TotalAssets,
			TotalBytes:              mediaRow.TotalBytes,
			StorageQuotaBytes:       StorageQuotaBytes,
			UsagePercentagePermille: permille,
			UsagePercentage:         percent,
			PublicBytes:             mediaRow.PublicBytes,
			CoursePrivateBytes:      mediaRow.CoursePrivateBytes,
			VideoCount:              mediaRow.VideoCount,
			AudioCount:              mediaRow.AudioCount,
			ImageCount:              mediaRow.ImageCount,
		},
	}, nil
}
