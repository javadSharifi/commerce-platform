package testsupport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/blog"
	"psychology/api/internal/blog/blogdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/media"
	"psychology/api/internal/media/mediadb"
	"psychology/api/internal/payments"
	"psychology/api/internal/payments/paymentsdb"
	"psychology/api/internal/workshops"
	"psychology/api/internal/workshops/workshopsdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// pilotPhase1Fixture sets up two independent tenants with complete mock stores
type pilotPhase1Fixture struct {
	tenantA uuid.UUID // امید (Free workshop, blog, media)
	tenantB uuid.UUID // ماهان (Paid workshop, payment gateway, concurrency)

	hostA string
	hostB string

	tokenOwnerA string
	tokenOwnerB string

	router http.Handler

	// Mock Queriers
	workshopsStore *pilotWorkshopsStore
	paymentsStore  *pilotPaymentsStore
	blogStore      *pilotBlogStore
	mediaStore     *pilotMediaStore
	storage        *media.MockStorageProvider
	gateway        *payments.MockPaymentProvider
}

// In-memory mock for workshops
type pilotWorkshopsStore struct {
	mu            sync.Mutex
	currentTime   time.Time
	workshops     map[string]workshopsdb.Workshop
	registrations map[string]workshopsdb.WorkshopRegistration
}

func newPilotWorkshopsStore() *pilotWorkshopsStore {
	return &pilotWorkshopsStore{
		workshops:     make(map[string]workshopsdb.Workshop),
		registrations: make(map[string]workshopsdb.WorkshopRegistration),
	}
}

func (m *pilotWorkshopsStore) getNow() time.Time {
	if !m.currentTime.IsZero() {
		return m.currentTime
	}
	return time.Now()
}

func (m *pilotWorkshopsStore) CreateWorkshop(ctx context.Context, arg workshopsdb.CreateWorkshopParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	now := m.getNow()
	w := workshopsdb.Workshop{
		ID:                   toPgUUID(id),
		TenantID:             arg.TenantID,
		Title:                arg.Title,
		Slug:                 arg.Slug,
		Description:          arg.Description,
		InstructorName:       arg.InstructorName,
		StartsAt:             arg.StartsAt,
		EndsAt:               arg.EndsAt,
		Location:             arg.Location,
		RegistrationDeadline: arg.RegistrationDeadline,
		Capacity:             arg.Capacity,
		PriceRials:           arg.PriceRials,
		Status:               arg.Status,
		CreatedAt:            pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:            pgtype.Timestamptz{Time: now, Valid: true},
	}
	m.workshops[id.String()] = w
	return w, nil
}

func (m *pilotWorkshopsStore) GetWorkshopByID(ctx context.Context, arg workshopsdb.GetWorkshopByIDParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.workshops[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(w.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.Workshop{}, pgx.ErrNoRows
	}
	return w, nil
}

func (m *pilotWorkshopsStore) GetWorkshopByIDForUpdate(ctx context.Context, arg workshopsdb.GetWorkshopByIDForUpdateParams) (workshopsdb.Workshop, error) {
	return m.GetWorkshopByID(ctx, workshopsdb.GetWorkshopByIDParams{TenantID: arg.TenantID, ID: arg.ID})
}

func (m *pilotWorkshopsStore) GetWorkshopBySlug(ctx context.Context, arg workshopsdb.GetWorkshopBySlugParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, w := range m.workshops {
		if fromPgUUID(w.TenantID) == fromPgUUID(arg.TenantID) && w.Slug == arg.Slug {
			return w, nil
		}
	}
	return workshopsdb.Workshop{}, pgx.ErrNoRows
}

func (m *pilotWorkshopsStore) ListWorkshops(ctx context.Context, arg workshopsdb.ListWorkshopsParams) ([]workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []workshopsdb.Workshop
	for _, w := range m.workshops {
		if fromPgUUID(w.TenantID) == fromPgUUID(arg.TenantID) {
			if !arg.Status.Valid || w.Status == arg.Status.String {
				res = append(res, w)
			}
		}
	}
	return res, nil
}

func (m *pilotWorkshopsStore) ListPublishedWorkshops(ctx context.Context, tenantID pgtype.UUID) ([]workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []workshopsdb.Workshop
	for _, w := range m.workshops {
		if fromPgUUID(w.TenantID) == fromPgUUID(tenantID) && w.Status == "published" {
			res = append(res, w)
		}
	}
	return res, nil
}

func (m *pilotWorkshopsStore) UpdateWorkshop(ctx context.Context, arg workshopsdb.UpdateWorkshopParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.workshops[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(w.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.Workshop{}, pgx.ErrNoRows
	}
	w.Title = arg.Title
	w.Slug = arg.Slug
	w.Description = arg.Description
	w.InstructorName = arg.InstructorName
	w.StartsAt = arg.StartsAt
	w.EndsAt = arg.EndsAt
	w.Location = arg.Location
	w.RegistrationDeadline = arg.RegistrationDeadline
	w.Capacity = arg.Capacity
	w.PriceRials = arg.PriceRials
	w.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.workshops[fromPgUUID(arg.ID).String()] = w
	return w, nil
}

func (m *pilotWorkshopsStore) UpdateWorkshopStatus(ctx context.Context, arg workshopsdb.UpdateWorkshopStatusParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.workshops[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(w.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.Workshop{}, pgx.ErrNoRows
	}
	w.Status = arg.Status
	w.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.workshops[fromPgUUID(arg.ID).String()] = w
	return w, nil
}

func (m *pilotWorkshopsStore) UpdateWorkshopCapacity(ctx context.Context, arg workshopsdb.UpdateWorkshopCapacityParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.workshops[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(w.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.Workshop{}, pgx.ErrNoRows
	}
	w.Capacity = arg.Capacity
	w.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.workshops[fromPgUUID(arg.ID).String()] = w
	return w, nil
}

func (m *pilotWorkshopsStore) CountActiveRegistrations(ctx context.Context, arg workshopsdb.CountActiveRegistrationsParams) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	now := m.getNow()
	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && fromPgUUID(r.WorkshopID) == fromPgUUID(arg.WorkshopID) {
			if r.Status == "confirmed" {
				count++
			} else if r.Status == "pending_payment" && r.ExpiresAt.Valid && r.ExpiresAt.Time.After(now) {
				count++
			}
		}
	}
	return count, nil
}

func (m *pilotWorkshopsStore) CountConfirmedRegistrations(ctx context.Context, arg workshopsdb.CountConfirmedRegistrationsParams) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && fromPgUUID(r.WorkshopID) == fromPgUUID(arg.WorkshopID) {
			if r.Status == "confirmed" {
				count++
			}
		}
	}
	return count, nil
}

func (m *pilotWorkshopsStore) CreateRegistration(ctx context.Context, arg workshopsdb.CreateRegistrationParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	now := m.getNow()
	r := workshopsdb.WorkshopRegistration{
		ID:              toPgUUID(id),
		TenantID:        arg.TenantID,
		WorkshopID:      arg.WorkshopID,
		GuestName:       arg.GuestName,
		GuestPhone:      arg.GuestPhone,
		Status:          arg.Status,
		ExpiresAt:       arg.ExpiresAt,
		PurchaseID:      arg.PurchaseID,
		GuestLookupHash: arg.GuestLookupHash,
		CreatedAt:       pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: now, Valid: true},
	}
	m.registrations[id.String()] = r
	return r, nil
}

func (m *pilotWorkshopsStore) GetRegistrationByID(ctx context.Context, arg workshopsdb.GetRegistrationByIDParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.registrations[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(r.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
	}
	return r, nil
}

func (m *pilotWorkshopsStore) GetRegistrationByIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByIDForUpdateParams) (workshopsdb.WorkshopRegistration, error) {
	return m.GetRegistrationByID(ctx, workshopsdb.GetRegistrationByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *pilotWorkshopsStore) GetRegistrationByPurchaseID(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && r.PurchaseID.Valid && fromPgUUID(r.PurchaseID) == fromPgUUID(arg.PurchaseID) {
			return r, nil
		}
	}
	return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
}

func (m *pilotWorkshopsStore) GetRegistrationByPurchaseIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDForUpdateParams) (workshopsdb.WorkshopRegistration, error) {
	return m.GetRegistrationByPurchaseID(ctx, workshopsdb.GetRegistrationByPurchaseIDParams{
		TenantID:   arg.TenantID,
		PurchaseID: arg.PurchaseID,
	})
}

func (m *pilotWorkshopsStore) GetRegistrationByLookupHash(ctx context.Context, arg workshopsdb.GetRegistrationByLookupHashParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && r.GuestLookupHash == arg.GuestLookupHash {
			return r, nil
		}
	}
	return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
}

func (m *pilotWorkshopsStore) GetActiveRegistrationByPhone(ctx context.Context, arg workshopsdb.GetActiveRegistrationByPhoneParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.getNow()
	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(r.WorkshopID) == fromPgUUID(arg.WorkshopID) &&
			r.GuestPhone == arg.GuestPhone {
			if r.Status == "confirmed" || (r.Status == "pending_payment" && r.ExpiresAt.Valid && r.ExpiresAt.Time.After(now)) {
				return r, nil
			}
		}
	}
	return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
}

func (m *pilotWorkshopsStore) ListRegistrationsByWorkshop(ctx context.Context, arg workshopsdb.ListRegistrationsByWorkshopParams) ([]workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []workshopsdb.WorkshopRegistration
	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && fromPgUUID(r.WorkshopID) == fromPgUUID(arg.WorkshopID) {
			res = append(res, r)
		}
	}
	return res, nil
}

func (m *pilotWorkshopsStore) UpdateRegistrationStatus(ctx context.Context, arg workshopsdb.UpdateRegistrationStatusParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.registrations[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(r.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
	}
	r.Status = arg.Status
	if arg.PurchaseID.Valid {
		r.PurchaseID = arg.PurchaseID
	}
	r.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.registrations[fromPgUUID(arg.ID).String()] = r
	return r, nil
}

func (m *pilotWorkshopsStore) ExpirePendingRegistrations(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.getNow()
	var count int64
	for id, r := range m.registrations {
		if r.Status == "pending_payment" && r.ExpiresAt.Valid && r.ExpiresAt.Time.Before(now) {
			r.Status = "expired"
			r.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
			m.registrations[id] = r
			count++
		}
	}
	return count, nil
}

// In-memory mock for payments
type pilotPaymentsStore struct {
	mu              sync.Mutex
	purchases       map[string]paymentsdb.Purchase
	paymentAttempts map[string]paymentsdb.PaymentAttempt
	refunds         map[string]paymentsdb.Refund
}

func newPilotPaymentsStore() *pilotPaymentsStore {
	return &pilotPaymentsStore{
		purchases:       make(map[string]paymentsdb.Purchase),
		paymentAttempts: make(map[string]paymentsdb.PaymentAttempt),
		refunds:         make(map[string]paymentsdb.Refund),
	}
}

func (p *pilotPaymentsStore) CreatePurchase(ctx context.Context, arg paymentsdb.CreatePurchaseParams) (paymentsdb.Purchase, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := uuid.New()
	now := time.Now()
	pur := paymentsdb.Purchase{
		ID:            toPgUUID(id),
		TenantID:      arg.TenantID,
		ItemType:      arg.ItemType,
		ItemID:        arg.ItemID,
		AmountRials:   arg.AmountRials,
		TitleSnapshot: arg.TitleSnapshot,
		Status:        arg.Status,
		CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
	}
	p.purchases[id.String()] = pur
	return pur, nil
}

func (p *pilotPaymentsStore) GetPurchaseByID(ctx context.Context, arg paymentsdb.GetPurchaseByIDParams) (paymentsdb.Purchase, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pur, ok := p.purchases[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(pur.TenantID) != fromPgUUID(arg.TenantID) {
		return paymentsdb.Purchase{}, pgx.ErrNoRows
	}
	return pur, nil
}

func (p *pilotPaymentsStore) GetPurchaseByIDForUpdate(ctx context.Context, arg paymentsdb.GetPurchaseByIDForUpdateParams) (paymentsdb.Purchase, error) {
	return p.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (p *pilotPaymentsStore) UpdatePurchaseStatus(ctx context.Context, arg paymentsdb.UpdatePurchaseStatusParams) (paymentsdb.Purchase, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pur, ok := p.purchases[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(pur.TenantID) != fromPgUUID(arg.TenantID) {
		return paymentsdb.Purchase{}, pgx.ErrNoRows
	}
	pur.Status = arg.Status
	pur.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	p.purchases[fromPgUUID(arg.ID).String()] = pur
	return pur, nil
}

func (p *pilotPaymentsStore) ListPurchases(ctx context.Context, arg paymentsdb.ListPurchasesParams) ([]paymentsdb.Purchase, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var res []paymentsdb.Purchase
	for _, pur := range p.purchases {
		if fromPgUUID(pur.TenantID) == fromPgUUID(arg.TenantID) {
			if !arg.Status.Valid || pur.Status == arg.Status.String {
				res = append(res, pur)
			}
		}
	}
	return res, nil
}

func (p *pilotPaymentsStore) CreatePaymentAttempt(ctx context.Context, arg paymentsdb.CreatePaymentAttemptParams) (paymentsdb.PaymentAttempt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := uuid.New()
	now := time.Now()
	pa := paymentsdb.PaymentAttempt{
		ID:             toPgUUID(id),
		TenantID:       arg.TenantID,
		PurchaseID:     arg.PurchaseID,
		Provider:       arg.Provider,
		Authority:      arg.Authority,
		AmountRials:    arg.AmountRials,
		Status:         arg.Status,
		IdempotencyKey: arg.IdempotencyKey,
		CreatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	}
	p.paymentAttempts[id.String()] = pa
	return pa, nil
}

func (p *pilotPaymentsStore) GetPaymentAttemptByAuthority(ctx context.Context, arg paymentsdb.GetPaymentAttemptByAuthorityParams) (paymentsdb.PaymentAttempt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pa := range p.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) && pa.Authority == arg.Authority {
			return pa, nil
		}
	}
	return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
}

func (p *pilotPaymentsStore) GetPaymentAttemptByIdempotencyKey(ctx context.Context, arg paymentsdb.GetPaymentAttemptByIdempotencyKeyParams) (paymentsdb.PaymentAttempt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pa := range p.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) && pa.IdempotencyKey == arg.IdempotencyKey {
			return pa, nil
		}
	}
	return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
}

func (p *pilotPaymentsStore) UpdatePaymentAttemptStatus(ctx context.Context, arg paymentsdb.UpdatePaymentAttemptStatusParams) (paymentsdb.PaymentAttempt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pa, ok := p.paymentAttempts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(pa.TenantID) != fromPgUUID(arg.TenantID) {
		return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
	}
	pa.Status = arg.Status
	if arg.TrackingCode.Valid {
		pa.TrackingCode = arg.TrackingCode
	}
	if arg.GatewayRefID.Valid {
		pa.GatewayRefID = arg.GatewayRefID
	}
	if arg.ErrorMessage.Valid {
		pa.ErrorMessage = arg.ErrorMessage
	}
	pa.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	p.paymentAttempts[fromPgUUID(arg.ID).String()] = pa
	return pa, nil
}

func (p *pilotPaymentsStore) GetActivePaymentAttemptByPurchase(ctx context.Context, arg paymentsdb.GetActivePaymentAttemptByPurchaseParams) (paymentsdb.PaymentAttempt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pa := range p.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(pa.PurchaseID) == fromPgUUID(arg.PurchaseID) &&
			pa.Status == "initiated" {
			return pa, nil
		}
	}
	return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
}

func (p *pilotPaymentsStore) GetVerifiedPaymentAttemptByPurchase(ctx context.Context, arg paymentsdb.GetVerifiedPaymentAttemptByPurchaseParams) (paymentsdb.PaymentAttempt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pa := range p.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(pa.PurchaseID) == fromPgUUID(arg.PurchaseID) &&
			pa.Status == "verified" {
			return pa, nil
		}
	}
	return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
}

func (p *pilotPaymentsStore) ListStalePaymentAttempts(ctx context.Context, arg paymentsdb.ListStalePaymentAttemptsParams) ([]paymentsdb.PaymentAttempt, error) {
	return nil, nil
}

func (p *pilotPaymentsStore) ListPaymentAttemptsByPurchase(ctx context.Context, arg paymentsdb.ListPaymentAttemptsByPurchaseParams) ([]paymentsdb.PaymentAttempt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var res []paymentsdb.PaymentAttempt
	for _, pa := range p.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) && fromPgUUID(pa.PurchaseID) == fromPgUUID(arg.PurchaseID) {
			res = append(res, pa)
		}
	}
	return res, nil
}

func (p *pilotPaymentsStore) CreateRefund(ctx context.Context, arg paymentsdb.CreateRefundParams) (paymentsdb.Refund, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := uuid.New()
	ref := paymentsdb.Refund{
		ID:               toPgUUID(id),
		TenantID:         arg.TenantID,
		PurchaseID:       arg.PurchaseID,
		PaymentAttemptID: arg.PaymentAttemptID,
		AmountRials:      arg.AmountRials,
		Reason:           arg.Reason,
		ReferenceNumber:  arg.ReferenceNumber,
		RegisteredBy:     arg.RegisteredBy,
		RegisteredAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CreatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	p.refunds[id.String()] = ref
	return ref, nil
}

func (p *pilotPaymentsStore) GetRefundByReference(ctx context.Context, arg paymentsdb.GetRefundByReferenceParams) (paymentsdb.Refund, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, r := range p.refunds {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && r.ReferenceNumber == arg.ReferenceNumber {
			return r, nil
		}
	}
	return paymentsdb.Refund{}, pgx.ErrNoRows
}

func (p *pilotPaymentsStore) ListRefundsByPurchase(ctx context.Context, arg paymentsdb.ListRefundsByPurchaseParams) ([]paymentsdb.Refund, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var res []paymentsdb.Refund
	for _, r := range p.refunds {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && fromPgUUID(r.PurchaseID) == fromPgUUID(arg.PurchaseID) {
			res = append(res, r)
		}
	}
	return res, nil
}

// In-memory mock for blog
type pilotBlogStore struct {
	mu    sync.Mutex
	posts map[string]blogdb.BlogPost
}

func newPilotBlogStore() *pilotBlogStore {
	return &pilotBlogStore{posts: make(map[string]blogdb.BlogPost)}
}

func (b *pilotBlogStore) CreateBlogPost(ctx context.Context, arg blogdb.CreateBlogPostParams) (blogdb.BlogPost, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, p := range b.posts {
		if fromPgUUID(p.TenantID) == fromPgUUID(arg.TenantID) && p.Slug == arg.Slug {
			return blogdb.BlogPost{}, &pgconn.PgError{Code: "23505", Message: "duplicate slug"}
		}
	}

	id := uuid.New()
	now := time.Now()
	bp := blogdb.BlogPost{
		ID:             toPgUUID(id),
		TenantID:       arg.TenantID,
		Title:          arg.Title,
		Slug:           arg.Slug,
		SeoDescription: arg.SeoDescription,
		ContentBlocks:  arg.ContentBlocks,
		Status:         arg.Status,
		PublishedAt:    arg.PublishedAt,
		CreatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	}
	b.posts[id.String()] = bp
	return bp, nil
}

func (b *pilotBlogStore) GetBlogPostByID(ctx context.Context, arg blogdb.GetBlogPostByIDParams) (blogdb.BlogPost, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	bp, ok := b.posts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(bp.TenantID) != fromPgUUID(arg.TenantID) {
		return blogdb.BlogPost{}, pgx.ErrNoRows
	}
	return bp, nil
}

func (b *pilotBlogStore) GetPublishedBlogPostBySlug(ctx context.Context, arg blogdb.GetPublishedBlogPostBySlugParams) (blogdb.BlogPost, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, bp := range b.posts {
		if fromPgUUID(bp.TenantID) == fromPgUUID(arg.TenantID) && bp.Slug == arg.Slug && bp.Status == "published" {
			return bp, nil
		}
	}
	return blogdb.BlogPost{}, pgx.ErrNoRows
}

func (b *pilotBlogStore) ListPublishedBlogPosts(ctx context.Context, tenantID pgtype.UUID) ([]blogdb.ListPublishedBlogPostsRow, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var res []blogdb.ListPublishedBlogPostsRow
	for _, bp := range b.posts {
		if fromPgUUID(bp.TenantID) == fromPgUUID(tenantID) && bp.Status == "published" {
			res = append(res, blogdb.ListPublishedBlogPostsRow{
				ID:             bp.ID,
				TenantID:       bp.TenantID,
				Title:          bp.Title,
				Slug:           bp.Slug,
				SeoDescription: bp.SeoDescription,
				PublishedAt:    bp.PublishedAt,
			})
		}
	}
	return res, nil
}

func (b *pilotBlogStore) ListAdminBlogPosts(ctx context.Context, arg blogdb.ListAdminBlogPostsParams) ([]blogdb.BlogPost, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var res []blogdb.BlogPost
	for _, bp := range b.posts {
		if fromPgUUID(bp.TenantID) == fromPgUUID(arg.TenantID) {
			if !arg.Status.Valid || bp.Status == arg.Status.String {
				res = append(res, bp)
			}
		}
	}
	return res, nil
}

func (b *pilotBlogStore) UpdateBlogPost(ctx context.Context, arg blogdb.UpdateBlogPostParams) (blogdb.BlogPost, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	bp, ok := b.posts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(bp.TenantID) != fromPgUUID(arg.TenantID) {
		return blogdb.BlogPost{}, pgx.ErrNoRows
	}
	for id, other := range b.posts {
		if id != fromPgUUID(arg.ID).String() && fromPgUUID(other.TenantID) == fromPgUUID(arg.TenantID) && other.Slug == arg.Slug {
			return blogdb.BlogPost{}, &pgconn.PgError{Code: "23505", Message: "duplicate slug"}
		}
	}
	bp.Title = arg.Title
	bp.Slug = arg.Slug
	bp.SeoDescription = arg.SeoDescription
	bp.ContentBlocks = arg.ContentBlocks
	bp.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	b.posts[fromPgUUID(arg.ID).String()] = bp
	return bp, nil
}

func (b *pilotBlogStore) PublishBlogPost(ctx context.Context, arg blogdb.PublishBlogPostParams) (blogdb.BlogPost, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	bp, ok := b.posts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(bp.TenantID) != fromPgUUID(arg.TenantID) {
		return blogdb.BlogPost{}, pgx.ErrNoRows
	}
	now := time.Now().UTC()
	bp.Status = "published"
	bp.PublishedAt = pgtype.Timestamptz{Time: now, Valid: true}
	bp.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
	b.posts[fromPgUUID(arg.ID).String()] = bp
	return bp, nil
}

func (b *pilotBlogStore) UnpublishBlogPost(ctx context.Context, arg blogdb.UnpublishBlogPostParams) (blogdb.BlogPost, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	bp, ok := b.posts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(bp.TenantID) != fromPgUUID(arg.TenantID) {
		return blogdb.BlogPost{}, pgx.ErrNoRows
	}
	now := time.Now().UTC()
	bp.Status = "draft"
	bp.PublishedAt = pgtype.Timestamptz{Valid: false}
	bp.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
	b.posts[fromPgUUID(arg.ID).String()] = bp
	return bp, nil
}

func (b *pilotBlogStore) DeleteBlogPost(ctx context.Context, arg blogdb.DeleteBlogPostParams) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.posts, fromPgUUID(arg.ID).String())
	return nil
}

// In-memory mock for media
type pilotMediaStore struct {
	mu      sync.Mutex
	uploads map[string]mediadb.MediaUpload
	assets  map[string]mediadb.MediaAsset
}

func newPilotMediaStore() *pilotMediaStore {
	return &pilotMediaStore{
		uploads: make(map[string]mediadb.MediaUpload),
		assets:  make(map[string]mediadb.MediaAsset),
	}
}

func (m *pilotMediaStore) CreateMediaUpload(ctx context.Context, arg mediadb.CreateMediaUploadParams) (mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID)
	if id == uuid.Nil {
		id = uuid.New()
	}
	now := time.Now().UTC()
	u := mediadb.MediaUpload{
		ID:                toPgUUID(id),
		TenantID:          arg.TenantID,
		StagingKey:        arg.StagingKey,
		OriginalName:      arg.OriginalName,
		AccessClass:       arg.AccessClass,
		ExpectedMimeType:  arg.ExpectedMimeType,
		ExpectedSizeBytes: arg.ExpectedSizeBytes,
		Status:            "reserved",
		ExpiresAt:         arg.ExpiresAt,
		InitiatedBy:       arg.InitiatedBy,
		CreatedAt:         pgtype.Timestamptz{Time: now, Valid: true},
	}
	m.uploads[id.String()] = u
	return u, nil
}

func (m *pilotMediaStore) GetMediaUploadByID(ctx context.Context, arg mediadb.GetMediaUploadByIDParams) (mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.uploads[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(u.TenantID) != fromPgUUID(arg.TenantID) {
		return mediadb.MediaUpload{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *pilotMediaStore) GetMediaUploadByStagingKey(ctx context.Context, arg mediadb.GetMediaUploadByStagingKeyParams) (mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.uploads {
		if fromPgUUID(u.TenantID) == fromPgUUID(arg.TenantID) && u.StagingKey == arg.StagingKey {
			return u, nil
		}
	}
	return mediadb.MediaUpload{}, pgx.ErrNoRows
}

func (m *pilotMediaStore) MarkMediaUploadCompleted(ctx context.Context, arg mediadb.MarkMediaUploadCompletedParams) (mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.uploads[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(u.TenantID) != fromPgUUID(arg.TenantID) {
		return mediadb.MediaUpload{}, pgx.ErrNoRows
	}
	u.Status = "completed"
	m.uploads[fromPgUUID(arg.ID).String()] = u
	return u, nil
}

func (m *pilotMediaStore) MarkMediaUploadExpired(ctx context.Context, arg mediadb.MarkMediaUploadExpiredParams) error {
	return nil
}

func (m *pilotMediaStore) ListExpiredMediaUploads(ctx context.Context, arg mediadb.ListExpiredMediaUploadsParams) ([]mediadb.MediaUpload, error) {
	return nil, nil
}

func (m *pilotMediaStore) GetTotalMediaUsageBytes(ctx context.Context, arg mediadb.GetTotalMediaUsageBytesParams) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, a := range m.assets {
		if fromPgUUID(a.TenantID) == fromPgUUID(arg.TenantID) && a.ValidationStatus == "verified" {
			total += a.SizeBytes
		}
	}
	for _, u := range m.uploads {
		if fromPgUUID(u.TenantID) == fromPgUUID(arg.TenantID) && u.Status == "reserved" && u.ExpiresAt.Time.After(arg.ExpiresAt.Time) {
			total += u.ExpectedSizeBytes
		}
	}
	return total, nil
}

func (m *pilotMediaStore) CreateMediaAsset(ctx context.Context, arg mediadb.CreateMediaAssetParams) (mediadb.MediaAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	now := time.Now().UTC()
	a := mediadb.MediaAsset{
		ID:               toPgUUID(id),
		TenantID:         arg.TenantID,
		StorageKey:       arg.StorageKey,
		OriginalName:     arg.OriginalName,
		MimeType:         arg.MimeType,
		SizeBytes:        arg.SizeBytes,
		AccessClass:      arg.AccessClass,
		ValidationStatus: arg.ValidationStatus,
		UploadedBy:       arg.UploadedBy,
		VerifiedAt:       arg.VerifiedAt,
		CreatedAt:        pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:        pgtype.Timestamptz{Time: now, Valid: true},
	}
	m.assets[id.String()] = a
	return a, nil
}

func (m *pilotMediaStore) GetMediaAssetByID(ctx context.Context, arg mediadb.GetMediaAssetByIDParams) (mediadb.MediaAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.assets[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(a.TenantID) != fromPgUUID(arg.TenantID) {
		return mediadb.MediaAsset{}, pgx.ErrNoRows
	}
	return a, nil
}

func (m *pilotMediaStore) GetMediaAssetByStorageKey(ctx context.Context, arg mediadb.GetMediaAssetByStorageKeyParams) (mediadb.MediaAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.assets {
		if fromPgUUID(a.TenantID) == fromPgUUID(arg.TenantID) && a.StorageKey == arg.StorageKey {
			return a, nil
		}
	}
	return mediadb.MediaAsset{}, pgx.ErrNoRows
}

func (m *pilotMediaStore) ListMediaAssetsByTenant(ctx context.Context, tenantID pgtype.UUID) ([]mediadb.MediaAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []mediadb.MediaAsset
	for _, a := range m.assets {
		if fromPgUUID(a.TenantID) == fromPgUUID(tenantID) {
			res = append(res, a)
		}
	}
	return res, nil
}

// setupPilotFixture creates full wiring of two clinics
func setupPilotFixture(t *testing.T) *pilotPhase1Fixture {
	tenantA := uuid.New()
	tenantB := uuid.New()
	hostA := "omid-psych.ir"
	hostB := "mahan-clinic.ir"

	// Mock stores
	domainStore := &pilotDomainStore{
		domains: map[string]domaindb.Domain{
			hostA: {TenantID: toPgUUID(tenantA), Host: hostA, IsVerified: true},
			hostB: {TenantID: toPgUUID(tenantB), Host: hostB, IsVerified: true},
		},
	}
	authStore := &pilotAuthStore{
		identities: make(map[string]authdb.PanelIdentity),
		sessions:   make(map[string]authdb.PanelSession),
		members:    make(map[string]authdb.Membership),
	}
	workshopsStore := newPilotWorkshopsStore()
	paymentsStore := newPilotPaymentsStore()
	blogStore := newPilotBlogStore()
	mediaStore := newPilotMediaStore()
	storage := media.NewMockStorageProvider()
	gateway := payments.NewMockPaymentProvider()

	// Create Owners
	ownerAUser, _ := authStore.CreateUser("owner@omid.ir", "omidPass123", "دکتر امیدی")
	authStore.AddMembership(ownerAUser.ID, tenantA, "owner")
	rawTokenA, _ := authStore.CreateSession(ownerAUser.ID)

	ownerBUser, _ := authStore.CreateUser("owner@mahan.ir", "mahanPass123", "دکتر ماهانی")
	authStore.AddMembership(ownerBUser.ID, tenantB, "owner")
	rawTokenB, _ := authStore.CreateSession(ownerBUser.ID)

	// Services & Resolvers
	domainResolver := domain.NewResolver(domainStore)
	authService := auth.NewService(authStore)
	workshopsService := workshops.NewService(workshopsStore)
	paymentsService := payments.NewService(paymentsStore, gateway, workshopsStore)
	blogService := blog.NewService(blogStore)
	mediaService := media.NewService(mediaStore, storage)

	// Handlers
	authHandler := auth.NewHandler(authService)
	workshopsHandler := workshops.NewHandler(workshopsService, domainResolver, authService)
	paymentsHandler := payments.NewHandler(paymentsService, domainResolver, authService)
	blogHandler := blog.NewHandler(blogService, domainResolver, authService)
	mediaHandler := media.NewHandler(mediaService, domainResolver, authService)

	workshopsHandler.SetPaymentService(paymentsService)

	r := chi.NewRouter()
	authHandler.MountRoutes(r)
	workshopsHandler.MountRoutes(r)
	paymentsHandler.MountRoutes(r)
	blogHandler.MountRoutes(r)
	mediaHandler.MountRoutes(r)

	return &pilotPhase1Fixture{
		tenantA:        tenantA,
		tenantB:        tenantB,
		hostA:          hostA,
		hostB:          hostB,
		tokenOwnerA:    rawTokenA,
		tokenOwnerB:    rawTokenB,
		router:         r,
		workshopsStore: workshopsStore,
		paymentsStore:  paymentsStore,
		blogStore:      blogStore,
		mediaStore:     mediaStore,
		storage:        storage,
		gateway:        gateway,
	}
}

// Helpers for auth/domain in pilot mock
type pilotDomainStore struct {
	domains map[string]domaindb.Domain
}

func (d *pilotDomainStore) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	dm, ok := d.domains[host]
	if !ok {
		return domaindb.Domain{}, pgx.ErrNoRows
	}
	return dm, nil
}

type pilotAuthStore struct {
	mu         sync.Mutex
	identities map[string]authdb.PanelIdentity
	sessions   map[string]authdb.PanelSession
	members    map[string]authdb.Membership
}

func (a *pilotAuthStore) CreateUser(email, pass, name string) (authdb.PanelIdentity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := uuid.New()
	hash, _ := auth.HashPassword(pass)
	pi := authdb.PanelIdentity{
		ID:           toPgUUID(id),
		Email:        email,
		PasswordHash: hash,
		FullName:     name,
		Status:       "active",
	}
	a.identities[email] = pi
	return pi, nil
}

func (a *pilotAuthStore) AddMembership(userID pgtype.UUID, tenantID uuid.UUID, role string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	mID := uuid.New()
	m := authdb.Membership{
		ID:        toPgUUID(mID),
		UserID:    userID,
		TenantID:  toPgUUID(tenantID),
		Role:      role,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	a.members[fromPgUUID(userID).String()+":"+tenantID.String()] = m
}

func (a *pilotAuthStore) CreateSession(userID pgtype.UUID) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	sID := uuid.New()
	rawToken := "token_" + uuid.NewString()
	tokenHash := auth.HashToken(rawToken)
	sess := authdb.PanelSession{
		ID:        toPgUUID(sID),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}
	a.sessions[tokenHash] = sess
	return rawToken, nil
}

func (a *pilotAuthStore) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	pi, ok := a.identities[email]
	if !ok {
		return authdb.PanelIdentity{}, pgx.ErrNoRows
	}
	return pi, nil
}

func (a *pilotAuthStore) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, pi := range a.identities {
		if fromPgUUID(pi.ID) == fromPgUUID(id) {
			return pi, nil
		}
	}
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}

func (a *pilotAuthStore) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}

func (a *pilotAuthStore) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	return authdb.PanelSession{}, nil
}

func (a *pilotAuthStore) GetPanelSessionByTokenHash(ctx context.Context, hash string) (authdb.PanelSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[hash]
	if !ok {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return s, nil
}

func (a *pilotAuthStore) DeletePanelSession(ctx context.Context, hash string) error {
	return nil
}

func (a *pilotAuthStore) DeleteExpiredPanelSessions(ctx context.Context) error {
	return nil
}

func (a *pilotAuthStore) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	m, ok := a.members[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return m, nil
}

func (a *pilotAuthStore) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var res []authdb.ListMembershipsByUserIDRow
	for _, m := range a.members {
		if fromPgUUID(m.UserID) == fromPgUUID(userID) {
			res = append(res, authdb.ListMembershipsByUserIDRow{
				ID:         m.ID,
				TenantID:   m.TenantID,
				UserID:     m.UserID,
				Role:       m.Role,
				TenantName: "کلینیک",
				TenantSlug: "clinic",
			})
		}
	}
	return res, nil
}

func (a *pilotAuthStore) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := uuid.New()
	m := authdb.Membership{
		ID:        toPgUUID(id),
		TenantID:  arg.TenantID,
		UserID:    arg.UserID,
		Role:      arg.Role,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	a.members[key] = m
	return m, nil
}

// -------------------------------------------------------------
// Test 1: PSY-A02 Workshop Publish and Registration
// -------------------------------------------------------------
func TestAcceptance_PSY_A02_WorkshopPublishAndRegistration(t *testing.T) {
	f := setupPilotFixture(t)

	// Owner A publishes free workshop
	freeBody, _ := json.Marshal(map[string]any{
		"title":                 "کارگاه رایگان مدیریت استرس",
		"slug":                  "stress-management-free",
		"description":           "کارگاه عمومی برای عموم مردم",
		"instructor_name":       "دکتر امیدی",
		"price_rials":           0,
		"capacity":              5,
		"starts_at":             time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"ends_at":               time.Now().Add(50 * time.Hour).Format(time.RFC3339),
		"registration_deadline": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"location":              "تهران، کلینیک امید",
	})
	reqCreate := httptest.NewRequest(http.MethodPost, "/admin/workshops", bytes.NewBuffer(freeBody))
	reqCreate.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	reqCreate.Header.Set("X-Tenant-ID", f.tenantA.String())
	reqCreate.Header.Set("Content-Type", "application/json")
	rrCreate := httptest.NewRecorder()
	f.router.ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for free workshop, got %d: %s", rrCreate.Code, rrCreate.Body.String())
	}
	var createdWorkshop map[string]any
	json.Unmarshal(rrCreate.Body.Bytes(), &createdWorkshop)
	wsID := createdWorkshop["id"].(string)

	// Publish workshop
	reqPub := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/workshops/%s/publish", wsID), nil)
	reqPub.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	reqPub.Header.Set("X-Tenant-ID", f.tenantA.String())
	reqPub.Header.Set("Content-Type", "application/json")
	rrPub := httptest.NewRecorder()
	f.router.ServeHTTP(rrPub, reqPub)
	if rrPub.Code != http.StatusOK {
		t.Fatalf("failed to publish workshop: %s", rrPub.Body.String())
	}

	// Guest registers on Tenant A (Free -> immediately confirmed)
	regBody, _ := json.Marshal(map[string]any{
		"guest_name":  "سارا محمدی",
		"guest_phone": "09121112233",
	})
	reqReg := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/public/workshops/%s/register", wsID), bytes.NewBuffer(regBody))
	reqReg.Host = f.hostA
	reqReg.Header.Set("Content-Type", "application/json")
	rrReg := httptest.NewRecorder()
	f.router.ServeHTTP(rrReg, reqReg)

	if rrReg.Code != http.StatusCreated {
		t.Fatalf("expected 201 for free registration, got %d: %s", rrReg.Code, rrReg.Body.String())
	}
	var regResp map[string]any
	json.Unmarshal(rrReg.Body.Bytes(), &regResp)
	if regResp["status"] != "confirmed" {
		t.Fatalf("expected confirmed status for free registration, got %v", regResp["status"])
	}
}

// -------------------------------------------------------------
// Test 2: PSY-A03 Concurrent Seat Contention & Zero Oversell
// -------------------------------------------------------------
func TestAcceptance_PSY_A03_ConcurrentSeatContentionAndZeroOversell(t *testing.T) {
	f := setupPilotFixture(t)

	// Owner B creates workshop with capacity 1
	singleBody, _ := json.Marshal(map[string]any{
		"title":                 "کارگاه فشرده درمان شناختی (ظرفیت ۱)",
		"slug":                  "cbt-single-seat",
		"description":           "تک صندلی",
		"instructor_name":       "دکتر ماهانی",
		"price_rials":           5000000, // 500,000 Tomans
		"capacity":              1,
		"starts_at":             time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"ends_at":               time.Now().Add(50 * time.Hour).Format(time.RFC3339),
		"registration_deadline": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"location":              "تهران، کلینیک ماهان",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/workshops", bytes.NewBuffer(singleBody))
	req.Header.Set("Authorization", "Bearer "+f.tokenOwnerB)
	req.Header.Set("X-Tenant-ID", f.tenantB.String())
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("failed to create single seat workshop: %s", rr.Body.String())
	}
	var createdWs map[string]any
	json.Unmarshal(rr.Body.Bytes(), &createdWs)
	wsID := createdWs["id"].(string)

	// Publish
	rPub := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/workshops/%s/publish", wsID), nil)
	rPub.Header.Set("Authorization", "Bearer "+f.tokenOwnerB)
	rPub.Header.Set("X-Tenant-ID", f.tenantB.String())
	rPub.Header.Set("Content-Type", "application/json")
	recPub := httptest.NewRecorder()
	f.router.ServeHTTP(recPub, rPub)
	if recPub.Code != http.StatusOK {
		t.Fatalf("failed to publish single seat workshop: %s", recPub.Body.String())
	}

	// 5 concurrent requests fighting for the single seat
	var wg sync.WaitGroup
	var successCount int32
	var soldOutCount int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b, _ := json.Marshal(map[string]any{
				"guest_name":  fmt.Sprintf("متقاضی شماره %d", idx),
				"guest_phone": fmt.Sprintf("0912000000%d", idx),
			})
			r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/public/workshops/%s/register", wsID), bytes.NewBuffer(b))
			r.Host = f.hostB
			r.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			f.router.ServeHTTP(rec, r)

			if rec.Code == http.StatusCreated {
				atomic.AddInt32(&successCount, 1)
			} else if rec.Code == http.StatusConflict || rec.Code == http.StatusBadRequest {
				atomic.AddInt32(&soldOutCount, 1)
			}
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("zero-oversell violated: expected exactly 1 successful registration, got %d", successCount)
	}
	if soldOutCount != 4 {
		t.Fatalf("expected 4 sold out responses, got %d", soldOutCount)
	}
}

// -------------------------------------------------------------
// Test 3: PSY-A04 Payment Idempotency, Mismatch & Refund Routing
// -------------------------------------------------------------
func TestAcceptance_PSY_A04_PaymentIdempotencyAndRefundRouting(t *testing.T) {
	f := setupPilotFixture(t)

	// Create paid workshop on Tenant B
	wBody, _ := json.Marshal(map[string]any{
		"title":                 "کارگاه فرزندپروری مثبت",
		"slug":                  "positive-parenting",
		"instructor_name":       "دکتر ماهانی",
		"price_rials":           10000000, // 1,000,000 Tomans
		"capacity":              2,
		"starts_at":             time.Now().Add(72 * time.Hour).Format(time.RFC3339),
		"ends_at":               time.Now().Add(74 * time.Hour).Format(time.RFC3339),
		"registration_deadline": time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"location":              "آنلاین",
	})
	rCreate := httptest.NewRequest(http.MethodPost, "/admin/workshops", bytes.NewBuffer(wBody))
	rCreate.Header.Set("Authorization", "Bearer "+f.tokenOwnerB)
	rCreate.Header.Set("X-Tenant-ID", f.tenantB.String())
	rCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	f.router.ServeHTTP(recCreate, rCreate)
	var wsData map[string]any
	json.Unmarshal(recCreate.Body.Bytes(), &wsData)
	wsID := wsData["id"].(string)

	// Publish
	rPub := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/workshops/%s/publish", wsID), nil)
	rPub.Header.Set("Authorization", "Bearer "+f.tokenOwnerB)
	rPub.Header.Set("X-Tenant-ID", f.tenantB.String())
	rPub.Header.Set("Content-Type", "application/json")
	recPub := httptest.NewRecorder()
	f.router.ServeHTTP(recPub, rPub)

	// 1. Register Guest (Hold created & payment initiated)
	rReg := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/public/workshops/%s/register", wsID), bytes.NewBufferString(`{"guest_name":"مهدی نظری","guest_phone":"09123334455"}`))
	rReg.Host = f.hostB
	rReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	f.router.ServeHTTP(recReg, rReg)
	if recReg.Code != http.StatusCreated {
		t.Fatalf("registration failed: %s", recReg.Body.String())
	}
	var regData map[string]any
	json.Unmarshal(recReg.Body.Bytes(), &regData)
	regID := regData["registration_id"].(string)

	// Authority from attempts map
	var authID string
	for _, pa := range f.paymentsStore.paymentAttempts {
		authID = pa.Authority
		break
	}

	// 2. Callback Verification (1st call: Success)
	rCB := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/callbacks/payments/zarinpal?Authority=%s&Status=OK", authID), nil)
	rCB.Host = f.hostB
	recCB := httptest.NewRecorder()
	f.router.ServeHTTP(recCB, rCB)
	if recCB.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on callback, got %d", recCB.Code)
	}

	// Verify Registration is Confirmed
	reg, _ := f.workshopsStore.GetRegistrationByID(context.Background(), workshopsdb.GetRegistrationByIDParams{
		TenantID: toPgUUID(f.tenantB),
		ID:       toPgUUID(uuid.MustParse(regID)),
	})
	if reg.Status != "confirmed" {
		t.Fatalf("expected registration status confirmed, got %s", reg.Status)
	}

	// 3. Duplicate Callback (Idempotency: Must redirect without error)
	rCBDup := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/callbacks/payments/zarinpal?Authority=%s&Status=OK", authID), nil)
	rCBDup.Host = f.hostB
	recCBDup := httptest.NewRecorder()
	f.router.ServeHTTP(recCBDup, rCBDup)
	if recCBDup.Code != http.StatusFound {
		t.Fatalf("expected 302 on duplicate callback, got %d", recCBDup.Code)
	}

	// 4. Test Refund Routing: Register another guest, expire hold, then pay
	rReg2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/public/workshops/%s/register", wsID), bytes.NewBufferString(`{"guest_name":"حمید کریمی","guest_phone":"09127778899"}`))
	rReg2.Host = f.hostB
	rReg2.Header.Set("Content-Type", "application/json")
	recReg2 := httptest.NewRecorder()
	f.router.ServeHTTP(recReg2, rReg2)
	var regData2 map[string]any
	json.Unmarshal(recReg2.Body.Bytes(), &regData2)
	regID2 := regData2["registration_id"].(string)

	// Find the second authority
	var authID2 string
	for _, pa := range f.paymentsStore.paymentAttempts {
		if pa.Authority != authID {
			authID2 = pa.Authority
			break
		}
	}

	// Simulate hold expiration by changing registration status to expired
	reg2Obj := f.workshopsStore.registrations[regID2]
	reg2Obj.Status = "expired"
	f.workshopsStore.registrations[regID2] = reg2Obj

	// Gateway payment succeeds after hold expired -> Routes to refund_required
	rCB2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/callbacks/payments/zarinpal?Authority=%s&Status=OK", authID2), nil)
	rCB2.Host = f.hostB
	recCB2 := httptest.NewRecorder()
	f.router.ServeHTTP(recCB2, rCB2)

	// Verify purchase status is refund_required
	pa2, _ := f.paymentsStore.GetPaymentAttemptByAuthority(context.Background(), paymentsdb.GetPaymentAttemptByAuthorityParams{
		TenantID:  toPgUUID(f.tenantB),
		Authority: authID2,
	})
	pur2, _ := f.paymentsStore.GetPurchaseByID(context.Background(), paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(f.tenantB),
		ID:       pa2.PurchaseID,
	})
	if pur2.Status != "refund_required" {
		t.Fatalf("expected purchase status refund_required, got %s", pur2.Status)
	}

	// 5. Owner B lists refund_required and processes manual refund
	rListRef := httptest.NewRequest(http.MethodGet, "/admin/payments?status=refund_required", nil)
	rListRef.Header.Set("Authorization", "Bearer "+f.tokenOwnerB)
	rListRef.Header.Set("X-Tenant-ID", f.tenantB.String())
	recListRef := httptest.NewRecorder()
	f.router.ServeHTTP(recListRef, rListRef)
	if recListRef.Code != http.StatusOK {
		t.Fatalf("failed to list refunds: %s", recListRef.Body.String())
	}

	// Process refund
	refBody := fmt.Sprintf(`{"amount_rials":10000000,"reference_number":"REF-%s","reason":"انقضای مهلت ثبت‌نام"}`, uuid.NewString()[:8])
	rRef := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/payments/%s/refund", fromPgUUID(pur2.ID).String()), bytes.NewBufferString(refBody))
	rRef.Header.Set("Authorization", "Bearer "+f.tokenOwnerB)
	rRef.Header.Set("X-Tenant-ID", f.tenantB.String())
	rRef.Header.Set("Content-Type", "application/json")
	recRef := httptest.NewRecorder()
	f.router.ServeHTTP(recRef, rRef)
	if recRef.Code != http.StatusOK {
		t.Fatalf("failed to record refund: %s", recRef.Body.String())
	}
}

// -------------------------------------------------------------
// Test 4: PSY-A05 Blog Draft Isolation & Per-Host Sitemap
// -------------------------------------------------------------
func TestAcceptance_PSY_A05_BlogDraftIsolationAndPerHostSitemap(t *testing.T) {
	f := setupPilotFixture(t)

	// Owner A creates draft blog post
	draftBody, _ := json.Marshal(map[string]any{
		"title":   "پیش‌نویس محرمانه درمان بالینی",
		"slug":    "clinical-confidential-draft",
		"content": []map[string]any{{"type": "paragraph", "data": map[string]any{"text": "محتوای پیش‌نویس"}}},
	})
	rDraft := httptest.NewRequest(http.MethodPost, "/admin/blog", bytes.NewBuffer(draftBody))
	rDraft.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	rDraft.Header.Set("X-Tenant-ID", f.tenantA.String())
	rDraft.Header.Set("Content-Type", "application/json")
	recDraft := httptest.NewRecorder()
	f.router.ServeHTTP(recDraft, rDraft)
	if recDraft.Code != http.StatusCreated {
		t.Fatalf("failed to create draft blog post: %s", recDraft.Body.String())
	}
	var draftData map[string]any
	json.Unmarshal(recDraft.Body.Bytes(), &draftData)
	postID := draftData["id"].(string)

	// Public request for draft -> 404 Not Found
	rPub := httptest.NewRequest(http.MethodGet, "/public/blog/clinical-confidential-draft", nil)
	rPub.Host = f.hostA
	recPub := httptest.NewRecorder()
	f.router.ServeHTTP(recPub, rPub)
	if recPub.Code != http.StatusNotFound {
		t.Fatalf("draft blog post leaked publicly! Expected 404, got %d", recPub.Code)
	}

	// Owner A publishes post
	rPubPost := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/blog/%s/publish", postID), nil)
	rPubPost.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	rPubPost.Header.Set("X-Tenant-ID", f.tenantA.String())
	rPubPost.Header.Set("Content-Type", "application/json")
	recPubPost := httptest.NewRecorder()
	f.router.ServeHTTP(recPubPost, rPubPost)
	if recPubPost.Code != http.StatusOK {
		t.Fatalf("failed to publish post: %s", recPubPost.Body.String())
	}

	// Now publicly accessible on Host A
	rPubOK := httptest.NewRequest(http.MethodGet, "/public/blog/clinical-confidential-draft", nil)
	rPubOK.Host = f.hostA
	recPubOK := httptest.NewRecorder()
	f.router.ServeHTTP(recPubOK, rPubOK)
	if recPubOK.Code != http.StatusOK {
		t.Fatalf("expected 200 on published post, got %d", recPubOK.Code)
	}

	// Cross-tenant check: Host B cannot access Tenant A's post -> 404 Not Found
	rCross := httptest.NewRequest(http.MethodGet, "/public/blog/clinical-confidential-draft", nil)
	rCross.Host = f.hostB
	recCross := httptest.NewRecorder()
	f.router.ServeHTTP(recCross, rCross)
	if recCross.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant leak! Tenant B accessed Tenant A's post: %d", recCross.Code)
	}

	// Unpublish post
	rUnpub := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/blog/%s/unpublish", postID), nil)
	rUnpub.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	rUnpub.Header.Set("X-Tenant-ID", f.tenantA.String())
	rUnpub.Header.Set("Content-Type", "application/json")
	recUnpub := httptest.NewRecorder()
	f.router.ServeHTTP(recUnpub, rUnpub)
	if recUnpub.Code != http.StatusOK {
		t.Fatalf("failed to unpublish post: %s", recUnpub.Body.String())
	}

	// Once unpublished, immediately 404 again on Host A
	rPubAfter := httptest.NewRequest(http.MethodGet, "/public/blog/clinical-confidential-draft", nil)
	rPubAfter.Host = f.hostA
	recPubAfter := httptest.NewRecorder()
	f.router.ServeHTTP(recPubAfter, rPubAfter)
	if recPubAfter.Code != http.StatusNotFound {
		t.Fatalf("unpublished post still accessible! Got %d", recPubAfter.Code)
	}
}

// -------------------------------------------------------------
// Test 5: PSY-A06 Media Validation, Quota and Cross-Tenant Isolation
// -------------------------------------------------------------
func TestAcceptance_PSY_A06_MediaStorageValidationAndQuota(t *testing.T) {
	f := setupPilotFixture(t)

	// Owner A requests upload URL for valid image
	imgBody := `{"filename":"poster.jpg","mime_type":"image/jpeg","size_bytes":1048576,"access_class":"public"}`
	rUpload := httptest.NewRequest(http.MethodPost, "/admin/media/upload-url", bytes.NewBufferString(imgBody))
	rUpload.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	rUpload.Header.Set("X-Tenant-ID", f.tenantA.String())
	rUpload.Header.Set("Content-Type", "application/json")
	recUpload := httptest.NewRecorder()
	f.router.ServeHTTP(recUpload, rUpload)
	if recUpload.Code != http.StatusCreated {
		t.Fatalf("failed to request upload URL: %s", recUpload.Body.String())
	}
	var upData map[string]any
	json.Unmarshal(recUpload.Body.Bytes(), &upData)
	upID := upData["UploadID"].(string)
	stagingKey := upData["StagingKey"].(string)

	// Simulate S3 staging upload
	f.storage.PutMockObject(stagingKey, &media.ObjectMeta{
		Key:         stagingKey,
		Size:        1048576,
		ContentType: "image/jpeg",
	})

	// Finalize upload
	finBody := fmt.Sprintf(`{"upload_id":"%s"}`, upID)
	rFin := httptest.NewRequest(http.MethodPost, "/admin/media/finalize", bytes.NewBufferString(finBody))
	rFin.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	rFin.Header.Set("X-Tenant-ID", f.tenantA.String())
	rFin.Header.Set("Content-Type", "application/json")
	recFin := httptest.NewRecorder()
	f.router.ServeHTTP(recFin, rFin)
	if recFin.Code != http.StatusOK {
		t.Fatalf("failed to finalize media: %s", recFin.Body.String())
	}
	var finData map[string]any
	json.Unmarshal(recFin.Body.Bytes(), &finData)
	assetID := finData["id"].(string)

	// Test invalid MIME type rejection
	invBody := `{"filename":"malware.exe","mime_type":"application/x-msdownload","size_bytes":500,"access_class":"public"}`
	rInv := httptest.NewRequest(http.MethodPost, "/admin/media/upload-url", bytes.NewBufferString(invBody))
	rInv.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	rInv.Header.Set("X-Tenant-ID", f.tenantA.String())
	rInv.Header.Set("Content-Type", "application/json")
	recInv := httptest.NewRecorder()
	f.router.ServeHTTP(recInv, rInv)
	if recInv.Code != http.StatusBadRequest {
		t.Fatalf("invalid mime type was not rejected! Got %d", recInv.Code)
	}

	// Cross-tenant media protection: Owner B tries to access Tenant A's media asset
	rCrossMedia := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/media/%s/stream", assetID), nil)
	rCrossMedia.Host = f.hostB // Sent to Tenant B's domain
	recCrossMedia := httptest.NewRecorder()
	f.router.ServeHTTP(recCrossMedia, rCrossMedia)
	if recCrossMedia.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant media leak! Expected 404, got %d", recCrossMedia.Code)
	}
}

// -------------------------------------------------------------
// Test 6: PSY-A10 Pilot Full Scenarios & PII Sanitization
// -------------------------------------------------------------
func TestAcceptance_PSY_A10_PilotFullScenariosAndPIIProtection(t *testing.T) {
	f := setupPilotFixture(t)

	// 1. Create registration with personal info
	regBody, _ := json.Marshal(map[string]any{
		"title":                 "کارگاه عمومی سلامت روان",
		"slug":                  "public-mental-health",
		"instructor_name":       "دکتر امیدی",
		"price_rials":           0,
		"capacity":              10,
		"starts_at":             time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"ends_at":               time.Now().Add(50 * time.Hour).Format(time.RFC3339),
		"registration_deadline": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"location":              "آنلاین",
	})
	rW := httptest.NewRequest(http.MethodPost, "/admin/workshops", bytes.NewBuffer(regBody))
	rW.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	rW.Header.Set("X-Tenant-ID", f.tenantA.String())
	rW.Header.Set("Content-Type", "application/json")
	recW := httptest.NewRecorder()
	f.router.ServeHTTP(recW, rW)
	var wsData map[string]any
	json.Unmarshal(recW.Body.Bytes(), &wsData)
	wsID := wsData["id"].(string)

	// Publish
	rPub := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/workshops/%s/publish", wsID), nil)
	rPub.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	rPub.Header.Set("X-Tenant-ID", f.tenantA.String())
	rPub.Header.Set("Content-Type", "application/json")
	recPub := httptest.NewRecorder()
	f.router.ServeHTTP(recPub, rPub)

	rReg := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/public/workshops/%s/register", wsID), bytes.NewBufferString(`{"guest_name":"نگار رضایی","guest_phone":"09129998877"}`))
	rReg.Host = f.hostA
	rReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	f.router.ServeHTTP(recReg, rReg)

	var regRes map[string]any
	json.Unmarshal(recReg.Body.Bytes(), &regRes)
	regID := regRes["registration_id"].(string)
	lookupToken := regRes["lookup_token"].(string)

	// 2. Public lookup using private token
	rLook := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/workshops/registrations/%s?lookup_token=%s", regID, lookupToken), nil)
	rLook.Host = f.hostA
	recLook := httptest.NewRecorder()
	f.router.ServeHTTP(recLook, rLook)

	if recLook.Code != http.StatusOK {
		t.Fatalf("lookup failed: %d (%s)", recLook.Code, recLook.Body.String())
	}

	// 3. Verify public listing never includes participant names or phones
	rListPub := httptest.NewRequest(http.MethodGet, "/public/workshops", nil)
	rListPub.Host = f.hostA
	recListPub := httptest.NewRecorder()
	f.router.ServeHTTP(recListPub, rListPub)

	pubListStr := recListPub.Body.String()
	if bytes.Contains([]byte(pubListStr), []byte("09129998877")) || bytes.Contains([]byte(pubListStr), []byte("نگار رضایی")) {
		t.Fatalf("PII leaked in public workshops list: %s", pubListStr)
	}
}
