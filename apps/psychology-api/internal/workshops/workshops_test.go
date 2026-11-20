package workshops_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/workshops"
	"psychology/api/internal/workshops/workshopsdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockWorkshopsQuerier struct {
	mu            sync.Mutex
	currentTime   time.Time
	workshops     map[string]workshopsdb.Workshop
	registrations map[string]workshopsdb.WorkshopRegistration
}

func (m *mockWorkshopsQuerier) getNow() time.Time {
	if !m.currentTime.IsZero() {
		return m.currentTime
	}
	return time.Now()
}

func newMockQuerier() *mockWorkshopsQuerier {
	return &mockWorkshopsQuerier{
		workshops:     make(map[string]workshopsdb.Workshop),
		registrations: make(map[string]workshopsdb.WorkshopRegistration),
	}
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPgUUID(p pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], p.Bytes[:])
	return u
}

func (m *mockWorkshopsQuerier) CreateWorkshop(ctx context.Context, arg workshopsdb.CreateWorkshopParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
	now := time.Now()
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

func (m *mockWorkshopsQuerier) GetWorkshopByID(ctx context.Context, arg workshopsdb.GetWorkshopByIDParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workshops[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(w.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.Workshop{}, pgx.ErrNoRows
	}
	return w, nil
}

func (m *mockWorkshopsQuerier) GetWorkshopByIDForUpdate(ctx context.Context, arg workshopsdb.GetWorkshopByIDForUpdateParams) (workshopsdb.Workshop, error) {
	return m.GetWorkshopByID(ctx, workshopsdb.GetWorkshopByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *mockWorkshopsQuerier) GetWorkshopBySlug(ctx context.Context, arg workshopsdb.GetWorkshopBySlugParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, w := range m.workshops {
		if fromPgUUID(w.TenantID) == fromPgUUID(arg.TenantID) && w.Slug == arg.Slug {
			return w, nil
		}
	}
	return workshopsdb.Workshop{}, pgx.ErrNoRows
}

func (m *mockWorkshopsQuerier) ListWorkshops(ctx context.Context, arg workshopsdb.ListWorkshopsParams) ([]workshopsdb.Workshop, error) {
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

func (m *mockWorkshopsQuerier) ListPublishedWorkshops(ctx context.Context, tenantID pgtype.UUID) ([]workshopsdb.Workshop, error) {
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

func (m *mockWorkshopsQuerier) UpdateWorkshop(ctx context.Context, arg workshopsdb.UpdateWorkshopParams) (workshopsdb.Workshop, error) {
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

func (m *mockWorkshopsQuerier) UpdateWorkshopStatus(ctx context.Context, arg workshopsdb.UpdateWorkshopStatusParams) (workshopsdb.Workshop, error) {
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

func (m *mockWorkshopsQuerier) UpdateWorkshopCapacity(ctx context.Context, arg workshopsdb.UpdateWorkshopCapacityParams) (workshopsdb.Workshop, error) {
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

func (m *mockWorkshopsQuerier) CountActiveRegistrations(ctx context.Context, arg workshopsdb.CountActiveRegistrationsParams) (int64, error) {
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

func (m *mockWorkshopsQuerier) CountConfirmedRegistrations(ctx context.Context, arg workshopsdb.CountConfirmedRegistrationsParams) (int64, error) {
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

func (m *mockWorkshopsQuerier) CreateRegistration(ctx context.Context, arg workshopsdb.CreateRegistrationParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
	now := time.Now()
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

func (m *mockWorkshopsQuerier) GetRegistrationByID(ctx context.Context, arg workshopsdb.GetRegistrationByIDParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.registrations[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(r.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
	}
	return r, nil
}

func (m *mockWorkshopsQuerier) GetRegistrationByIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByIDForUpdateParams) (workshopsdb.WorkshopRegistration, error) {
	return m.GetRegistrationByID(ctx, workshopsdb.GetRegistrationByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *mockWorkshopsQuerier) GetRegistrationByPurchaseID(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && r.PurchaseID.Valid && fromPgUUID(r.PurchaseID) == fromPgUUID(arg.PurchaseID) {
			return r, nil
		}
	}
	return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
}

func (m *mockWorkshopsQuerier) GetRegistrationByPurchaseIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDForUpdateParams) (workshopsdb.WorkshopRegistration, error) {
	return m.GetRegistrationByPurchaseID(ctx, workshopsdb.GetRegistrationByPurchaseIDParams{
		TenantID:   arg.TenantID,
		PurchaseID: arg.PurchaseID,
	})
}

func (m *mockWorkshopsQuerier) GetRegistrationByLookupHash(ctx context.Context, arg workshopsdb.GetRegistrationByLookupHashParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && r.GuestLookupHash == arg.GuestLookupHash {
			return r, nil
		}
	}
	return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
}

func (m *mockWorkshopsQuerier) GetActiveRegistrationByPhone(ctx context.Context, arg workshopsdb.GetActiveRegistrationByPhoneParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.getNow()
	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(r.WorkshopID) == fromPgUUID(arg.WorkshopID) &&
			r.GuestPhone == arg.GuestPhone {
			if r.Status == "confirmed" {
				return r, nil
			}
			if r.Status == "pending_payment" && r.ExpiresAt.Valid && r.ExpiresAt.Time.After(now) {
				return r, nil
			}
		}
	}
	return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
}

func (m *mockWorkshopsQuerier) ListRegistrationsByWorkshop(ctx context.Context, arg workshopsdb.ListRegistrationsByWorkshopParams) ([]workshopsdb.WorkshopRegistration, error) {
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

func (m *mockWorkshopsQuerier) UpdateRegistrationStatus(ctx context.Context, arg workshopsdb.UpdateRegistrationStatusParams) (workshopsdb.WorkshopRegistration, error) {
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

func (m *mockWorkshopsQuerier) ExpirePendingRegistrations(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var count int64
	now := m.getNow()
	for id, r := range m.registrations {
		if r.Status == "pending_payment" && r.ExpiresAt.Valid && !r.ExpiresAt.Time.After(now) {
			r.Status = "expired"
			r.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
			m.registrations[id] = r
			count++
		}
	}
	return count, nil
}

// ----------------------------------------------------
// TESTS
// ----------------------------------------------------

func TestZeroSeats_CapacityFullRejection(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	svc := workshops.NewService(mock)

	tenantID := uuid.New()
	now := time.Now()

	// 1. Create a workshop with capacity = 2
	w, err := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه مدیریت اضطراب",
		Slug:                 "anxiety-management",
		InstructorName:       "دکتر علوی",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران، ونک",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             2,
		PriceRials:           5000000,
	})
	if err != nil {
		t.Fatalf("failed to create workshop: %v", err)
	}

	// Publish it
	_, err = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})
	if err != nil {
		t.Fatalf("failed to publish workshop: %v", err)
	}

	// 2. Register first attendee -> success
	res1, err := svc.RegisterGuest(ctx, tenantID, w.ID, "کاربر ۱", "09121111111", "", now)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if res1.Status != "pending_payment" {
		t.Errorf("expected pending_payment, got %s", res1.Status)
	}

	// 3. Register second attendee -> success
	res2, err := svc.RegisterGuest(ctx, tenantID, w.ID, "کاربر ۲", "09122222222", "", now)
	if err != nil {
		t.Fatalf("second registration failed: %v", err)
	}
	if res2.Status != "pending_payment" {
		t.Errorf("expected pending_payment, got %s", res2.Status)
	}

	// 4. Attempt to register third attendee -> must be REJECTED with ErrCapacityFull
	_, err = svc.RegisterGuest(ctx, tenantID, w.ID, "کاربر ۳", "09123333333", "", now)
	if err != workshops.ErrCapacityFull {
		t.Fatalf("expected ErrCapacityFull, got: %v", err)
	}
}

func TestExpiredReservation_DoesNotOccupySeat(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	svc := workshops.NewService(mock)

	tenantID := uuid.New()
	now := time.Now()

	// 1. Create a workshop with capacity = 1
	w, err := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه زوج‌درمانی",
		Slug:                 "couples-therapy",
		InstructorName:       "دکتر رضایی",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران، مطهری",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             1,
		PriceRials:           3000000,
	})
	if err != nil {
		t.Fatalf("failed to create workshop: %v", err)
	}

	_, err = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})
	if err != nil {
		t.Fatalf("failed to publish workshop: %v", err)
	}

	// 2. Guest 1 books the only seat at T0
	mock.currentTime = now
	res1, err := svc.RegisterGuest(ctx, tenantID, w.ID, "مهمان اول", "09121111111", "", now)
	if err != nil {
		t.Fatalf("first guest registration failed: %v", err)
	}

	// 3. At T0 + 5 minutes: Seat is still held; Guest 2 cannot register
	mock.currentTime = now.Add(5 * time.Minute)
	_, err = svc.RegisterGuest(ctx, tenantID, w.ID, "مهمان دوم", "09122222222", "", mock.currentTime)
	if err != workshops.ErrCapacityFull {
		t.Fatalf("expected ErrCapacityFull at T+5min, got: %v", err)
	}

	// 4. At T0 + 16 minutes: Hold has EXPIRED (TTL is 15 minutes)
	// Even without explicit background cleanup, the seat MUST be available!
	timeAfterExpiry := now.Add(16 * time.Minute)
	mock.currentTime = timeAfterExpiry
	res2, err := svc.RegisterGuest(ctx, tenantID, w.ID, "مهمان دوم", "09122222222", "", timeAfterExpiry)
	if err != nil {
		t.Fatalf("guest 2 should be able to claim expired seat, but got error: %v", err)
	}
	if res2.RegistrationID == res1.RegistrationID {
		t.Fatal("expected new registration ID for guest 2")
	}
}

func TestCapacityUpdate_ConstraintsAndValidation(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	svc := workshops.NewService(mock)

	tenantID := uuid.New()
	now := time.Now()

	// Create workshop with capacity = 5
	w, err := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "فرزندپروری مثبت",
		Slug:                 "positive-parenting",
		InstructorName:       "دکتر مهرابی",
		StartsAt:             now.Add(72 * time.Hour),
		EndsAt:               now.Add(76 * time.Hour),
		Location:             "تهران، سعادت‌آباد",
		RegistrationDeadline: now.Add(48 * time.Hour),
		Capacity:             5,
		PriceRials:           0, // Free workshop
	})
	if err != nil {
		t.Fatalf("failed to create workshop: %v", err)
	}

	_, _ = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})

	// 2 attendees register and are confirmed
	_, err = svc.RegisterGuest(ctx, tenantID, w.ID, "عضو اول", "09120000001", "", now)
	if err != nil {
		t.Fatalf("registration 1 failed: %v", err)
	}
	_, err = svc.RegisterGuest(ctx, tenantID, w.ID, "عضو دوم", "09120000002", "", now)
	if err != nil {
		t.Fatalf("registration 2 failed: %v", err)
	}

	// Reducing capacity to 3 (which is >= 2 active attendees) must SUCCEED
	updated, err := svc.UpdateWorkshopCapacity(ctx, tenantID, w.ID, 3)
	if err != nil {
		t.Fatalf("reducing capacity to 3 failed: %v", err)
	}
	if updated.Capacity != 3 {
		t.Errorf("expected capacity 3, got %d", updated.Capacity)
	}

	// Attempting to reduce capacity to 1 (which is < 2 active attendees) must FAIL
	_, err = svc.UpdateWorkshopCapacity(ctx, tenantID, w.ID, 1)
	if err != workshops.ErrCannotReduceCapacityBelowActive {
		t.Fatalf("expected ErrCannotReduceCapacityBelowActive, got: %v", err)
	}
}

func TestDuplicateActivePhone_Blocked(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	svc := workshops.NewService(mock)

	tenantID := uuid.New()
	now := time.Now()

	w, _ := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "مدیریت استرس",
		Slug:                 "stress-management",
		InstructorName:       "دکتر کاظمی",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             10,
		PriceRials:           2000000,
	})
	_, _ = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})

	// First registration with phone 09121234567
	_, err := svc.RegisterGuest(ctx, tenantID, w.ID, "کاربر تکراری", "09121234567", "", now)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration with SAME phone number while hold is active must FAIL
	_, err = svc.RegisterGuest(ctx, tenantID, w.ID, "کاربر تکراری دوباره", "09121234567", "", now)
	if err != workshops.ErrDuplicateActiveRegistration {
		t.Fatalf("expected ErrDuplicateActiveRegistration, got: %v", err)
	}
}

func TestCrossTenantIsolation_WorkshopAccess(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	svc := workshops.NewService(mock)

	tenantA := uuid.New()
	tenantB := uuid.New()
	now := time.Now()

	// Create workshop under Tenant A
	wA, err := svc.CreateWorkshop(ctx, tenantA, workshops.CreateWorkshopParams{
		Title:                "کارگاه کلینیک الف",
		Slug:                 "workshop-a",
		InstructorName:       "مدرس الف",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "مطب الف",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             5,
		PriceRials:           1000000,
	})
	if err != nil {
		t.Fatalf("failed to create workshop in tenant A: %v", err)
	}

	// Tenant B tries to get workshop A -> must return ErrWorkshopNotFound (RLS / isolation)
	_, err = svc.GetWorkshop(ctx, tenantB, wA.ID)
	if err != workshops.ErrWorkshopNotFound {
		t.Fatalf("expected ErrWorkshopNotFound for cross-tenant access, got: %v", err)
	}

	// Tenant B tries to register into Tenant A's workshop -> must be rejected
	_, err = svc.RegisterGuest(ctx, tenantB, wA.ID, "نفوذی", "09129999999", "", now)
	if err != workshops.ErrWorkshopNotFound {
		t.Fatalf("expected ErrWorkshopNotFound for cross-tenant registration, got: %v", err)
	}
}

// ----------------------------------------------------
// Mock Domain and Auth Queriers for HTTP Integration
// ----------------------------------------------------

type mockDomainQuerier struct {
	domains map[string]domaindb.Domain
}

func (m *mockDomainQuerier) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	d, ok := m.domains[host]
	if !ok {
		return domaindb.Domain{}, pgx.ErrNoRows
	}
	return d, nil
}

type mockAuthQuerier struct {
	identities  map[string]authdb.PanelIdentity
	sessions    map[string]authdb.PanelSession
	memberships map[string]authdb.Membership
}

func newMockAuthQuerier() *mockAuthQuerier {
	return &mockAuthQuerier{
		identities:  make(map[string]authdb.PanelIdentity),
		sessions:    make(map[string]authdb.PanelSession),
		memberships: make(map[string]authdb.Membership),
	}
}

func (m *mockAuthQuerier) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	for _, id := range m.identities {
		if id.Email == email {
			return id, nil
		}
	}
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}

func (m *mockAuthQuerier) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	for _, ident := range m.identities {
		if ident.ID == id {
			return ident, nil
		}
	}
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}

func (m *mockAuthQuerier) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}

func (m *mockAuthQuerier) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	sess := authdb.PanelSession{
		ID:        arg.ID,
		UserID:    arg.UserID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}
	m.sessions[arg.TokenHash] = sess
	return sess, nil
}

func (m *mockAuthQuerier) GetPanelSessionByTokenHash(ctx context.Context, tokenHash string) (authdb.PanelSession, error) {
	sess, ok := m.sessions[tokenHash]
	if !ok || sess.ExpiresAt.Time.Before(time.Now()) {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return sess, nil
}

func (m *mockAuthQuerier) DeletePanelSession(ctx context.Context, tokenHash string) error {
	delete(m.sessions, tokenHash)
	return nil
}

func (m *mockAuthQuerier) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	var res []authdb.ListMembershipsByUserIDRow
	for _, mem := range m.memberships {
		if mem.UserID == userID {
			res = append(res, authdb.ListMembershipsByUserIDRow{
				ID:         mem.ID,
				TenantID:   mem.TenantID,
				UserID:     mem.UserID,
				Role:       mem.Role,
				TenantName: "Clinic",
				TenantSlug: "clinic",
			})
		}
	}
	return res, nil
}

func (m *mockAuthQuerier) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	mem, ok := m.memberships[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return mem, nil
}

func (m *mockAuthQuerier) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	mem := authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: arg.TenantID,
		UserID:   arg.UserID,
		Role:     arg.Role,
	}
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	m.memberships[key] = mem
	return mem, nil
}

// ----------------------------------------------------
// HTTP Handler Tests
// ----------------------------------------------------

func TestPublicWorkshops_OnlyTenantData(t *testing.T) {
	ctx := context.Background()
	wsMock := newMockQuerier()
	wsSvc := workshops.NewService(wsMock)

	tenantA := uuid.New()
	tenantB := uuid.New()
	now := time.Now()

	domainMock := &mockDomainQuerier{
		domains: map[string]domaindb.Domain{
			"clinic-a.ir": {
				ID:         toPgUUID(uuid.New()),
				TenantID:   toPgUUID(tenantA),
				Host:       "clinic-a.ir",
				IsVerified: true,
			},
			"clinic-b.ir": {
				ID:         toPgUUID(uuid.New()),
				TenantID:   toPgUUID(tenantB),
				Host:       "clinic-b.ir",
				IsVerified: true,
			},
		},
	}
	resolver := domain.NewResolver(domainMock)
	authMock := newMockAuthQuerier()
	authSvc := auth.NewService(authMock)
	handler := workshops.NewHandler(wsSvc, resolver, authSvc)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Create Published Workshop for Clinic A
	wA, _ := wsSvc.CreateWorkshop(ctx, tenantA, workshops.CreateWorkshopParams{
		Title:                "کارگاه کلینیک الف",
		Slug:                 "workshop-a",
		InstructorName:       "دکتر الف",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             10,
		PriceRials:           5000000,
	})
	_, _ = wsMock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantA),
		ID:       toPgUUID(wA.ID),
		Status:   "published",
	})

	// Create Draft Workshop for Clinic A (must NOT be visible publicly)
	_, _ = wsSvc.CreateWorkshop(ctx, tenantA, workshops.CreateWorkshopParams{
		Title:                "پیش‌نویس الف",
		Slug:                 "draft-a",
		InstructorName:       "دکتر الف",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             5,
		PriceRials:           0,
	})

	// Create Published Workshop for Clinic B
	wB, _ := wsSvc.CreateWorkshop(ctx, tenantB, workshops.CreateWorkshopParams{
		Title:                "کارگاه کلینیک ب",
		Slug:                 "workshop-b",
		InstructorName:       "دکتر ب",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "شیراز",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             20,
		PriceRials:           3000000,
	})
	_, _ = wsMock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantB),
		ID:       toPgUUID(wB.ID),
		Status:   "published",
	})

	// Request GET /public/workshops with Host: clinic-a.ir
	reqA := httptest.NewRequest("GET", "/public/workshops", nil)
	reqA.Host = "clinic-a.ir"
	recA := httptest.NewRecorder()
	r.ServeHTTP(recA, reqA)

	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d (%s)", recA.Code, recA.Body.String())
	}

	var itemsA []workshops.PublicWorkshopItem
	if err := json.Unmarshal(recA.Body.Bytes(), &itemsA); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Must contain ONLY published workshop of Clinic A
	if len(itemsA) != 1 {
		t.Fatalf("expected exactly 1 item for clinic A, got %d", len(itemsA))
	}
	if itemsA[0].Title != "کارگاه کلینیک الف" {
		t.Errorf("expected title 'کارگاه کلینیک الف', got: %s", itemsA[0].Title)
	}

	// Request GET /public/workshops/{id} for Workshop A with Host: clinic-b.ir -> must be 404
	reqCross := httptest.NewRequest("GET", "/public/workshops/"+wA.ID.String(), nil)
	reqCross.Host = "clinic-b.ir"
	recCross := httptest.NewRecorder()
	r.ServeHTTP(recCross, reqCross)

	if recCross.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for cross-tenant workshop access, got: %d", recCross.Code)
	}
}

func TestAdminWorkshops_RoleAccessAndTransitions(t *testing.T) {
	wsMock := newMockQuerier()
	wsSvc := workshops.NewService(wsMock)

	tenantID := uuid.New()
	userID := uuid.New()

	domainMock := &mockDomainQuerier{domains: make(map[string]domaindb.Domain)}
	resolver := domain.NewResolver(domainMock)
	authMock := newMockAuthQuerier()

	// Register user with content_manager role
	hash, _ := auth.HashPassword("managerPass123")
	authMock.identities[userID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(userID),
		Email:        "manager@clinic.ir",
		PasswordHash: hash,
		FullName:     "مدیر محتوا",
		Status:       "active",
	}
	authMock.memberships[userID.String()+":"+tenantID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantID),
		UserID:   toPgUUID(userID),
		Role:     "content_manager",
	}

	authSvc := auth.NewService(authMock)
	sess, err := authSvc.Login(context.Background(), "manager@clinic.ir", "managerPass123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	token := sess.Token

	handler := workshops.NewHandler(wsSvc, resolver, authSvc)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	now := time.Now()
	// 1. Content Manager creates workshop draft
	createPayload := map[string]any{
		"title":                 "کارگاه مهارت‌های زندگی",
		"slug":                  "life-skills",
		"description":           "توضیحات کامل",
		"instructor_name":       "دکتر حسینی",
		"starts_at":             now.Add(48 * time.Hour).Format(time.RFC3339),
		"ends_at":               now.Add(52 * time.Hour).Format(time.RFC3339),
		"location":              "تهران",
		"registration_deadline": now.Add(24 * time.Hour).Format(time.RFC3339),
		"capacity":              15,
		"price_rials":           4000000,
	}
	body, _ := json.Marshal(createPayload)

	reqCreate := httptest.NewRequest("POST", "/admin/workshops", bytes.NewReader(body))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("X-Tenant-ID", tenantID.String())
	reqCreate.Header.Set("Cookie", "session_token="+token)
	reqCreate.Header.Set("X-Requested-With", "XMLHttpRequest") // Pass CSRF check
	recCreate := httptest.NewRecorder()
	r.ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got: %d (%s)", recCreate.Code, recCreate.Body.String())
	}

	var created workshops.AdminWorkshopDetail
	_ = json.Unmarshal(recCreate.Body.Bytes(), &created)
	if created.Status != "draft" {
		t.Errorf("expected status 'draft', got: %s", created.Status)
	}

	// 2. Publish workshop
	reqPub := httptest.NewRequest("POST", "/admin/workshops/"+created.ID.String()+"/publish", nil)
	reqPub.Header.Set("Content-Type", "application/json")
	reqPub.Header.Set("X-Tenant-ID", tenantID.String())
	reqPub.Header.Set("Cookie", "session_token="+token)
	reqPub.Header.Set("X-Requested-With", "XMLHttpRequest")
	recPub := httptest.NewRecorder()
	r.ServeHTTP(recPub, reqPub)

	if recPub.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for publish, got: %d", recPub.Code)
	}

	// 3. Content Manager attempts to Cancel workshop -> must be 403 (Owner only!)
	reqCancel := httptest.NewRequest("POST", "/admin/workshops/"+created.ID.String()+"/cancel", nil)
	reqCancel.Header.Set("Content-Type", "application/json")
	reqCancel.Header.Set("X-Tenant-ID", tenantID.String())
	reqCancel.Header.Set("Cookie", "session_token="+token)
	reqCancel.Header.Set("X-Requested-With", "XMLHttpRequest")
	recCancel := httptest.NewRecorder()
	r.ServeHTTP(recCancel, reqCancel)

	if recCancel.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-owner cancelling workshop, got: %d", recCancel.Code)
	}

	// 4. Upgrade role to owner -> Cancel workshop must SUCCEED
	authMock.memberships[userID.String()+":"+tenantID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantID),
		UserID:   toPgUUID(userID),
		Role:     "owner",
	}

	reqCancelOwner := httptest.NewRequest("POST", "/admin/workshops/"+created.ID.String()+"/cancel", nil)
	reqCancelOwner.Header.Set("Content-Type", "application/json")
	reqCancelOwner.Header.Set("X-Tenant-ID", tenantID.String())
	reqCancelOwner.Header.Set("Cookie", "session_token="+token)
	reqCancelOwner.Header.Set("X-Requested-With", "XMLHttpRequest")
	recCancelOwner := httptest.NewRecorder()
	r.ServeHTTP(recCancelOwner, reqCancelOwner)

	if recCancelOwner.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for owner cancelling workshop, got: %d", recCancelOwner.Code)
	}
}

func TestAdminWorkshops_ActiveWorkshopModificationBlocked(t *testing.T) {
	ctx := context.Background()
	wsMock := newMockQuerier()
	wsSvc := workshops.NewService(wsMock)

	tenantID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	domainMock := &mockDomainQuerier{domains: make(map[string]domaindb.Domain)}
	authMock := newMockAuthQuerier()
	ownerHash, _ := auth.HashPassword("ownerPass123")
	authMock.identities[userID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(userID),
		Email:        "owner@clinic.ir",
		PasswordHash: ownerHash,
		FullName:     "دکتر مالک",
		Status:       "active",
	}
	authMock.memberships[userID.String()+":"+tenantID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantID),
		UserID:   toPgUUID(userID),
		Role:     "owner",
	}

	authSvc := auth.NewService(authMock)
	ownerSess, err := authSvc.Login(ctx, "owner@clinic.ir", "ownerPass123")
	if err != nil {
		t.Fatalf("owner login failed: %v", err)
	}
	token := ownerSess.Token

	handler := workshops.NewHandler(wsSvc, domain.NewResolver(domainMock), authSvc)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Create and publish workshop
	w, _ := wsSvc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه با ثبت‌نام قطعی",
		Slug:                 "confirmed-workshop",
		InstructorName:       "دکتر الف",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             10,
		PriceRials:           2000000,
	})
	_, _ = wsMock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})

	// Add a confirmed registration
	_, err = wsMock.CreateRegistration(ctx, workshopsdb.CreateRegistrationParams{
		TenantID:        toPgUUID(tenantID),
		WorkshopID:      toPgUUID(w.ID),
		GuestName:       "مشتری قطعی",
		GuestPhone:      "09121111111",
		Status:          "confirmed",
		GuestLookupHash: "hash-123",
	})
	if err != nil {
		t.Fatalf("failed to add confirmed registration: %v", err)
	}

	// Attempt to change price via PUT /admin/workshops/{id} -> must be 409 Conflict
	newPrice := int64(3000000)
	updatePayload := map[string]any{
		"price_rials": newPrice,
	}
	body, _ := json.Marshal(updatePayload)

	req := httptest.NewRequest("PUT", "/admin/workshops/"+w.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("Cookie", "session_token="+token)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when modifying price of workshop with confirmed attendees, got: %d", rec.Code)
	}
}

// ----------------------------------------------------
// PSY-1.4: Guest Registration & Atomic Seat Reservation Tests
// ----------------------------------------------------

func TestPhoneNormalization(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"standard", "09123456789", "09123456789", false},
		{"with plus 98", "+989123456789", "09123456789", false},
		{"with 0098", "00989123456789", "09123456789", false},
		{"with 98 prefix", "989123456789", "09123456789", false},
		{"without leading zero", "9123456789", "09123456789", false},
		{"persian numerals", "۰۹۱۲۳۴۵۶۷۸۹", "09123456789", false},
		{"arabic numerals", "٠٩١٢٣٤٥٦٧٨٩", "09123456789", false},
		{"with spaces and dashes", " 0912-345 6789 ", "09123456789", false},
		{"invalid prefix", "08123456789", "", true},
		{"too short", "091234567", "", true},
		{"too long", "091234567890", "", true},
		{"letters", "0912abcdefg", "", true},
		{"empty", "", "", true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := workshops.NormalizePhone(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizePhone(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRegisterGuest_FreeVsPaidWorkshops(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	svc := workshops.NewService(mock)
	tenantID := uuid.New()
	now := time.Now()

	// 1. Free workshop (PriceRials = 0) -> immediately confirmed, nil expires_at
	wFree, err := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه رایگان خودشناسی",
		Slug:                 "free-workshop",
		InstructorName:       "دکتر رضایی",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "آنلاین",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             10,
		PriceRials:           0,
	})
	if err != nil {
		t.Fatalf("failed to create free workshop: %v", err)
	}
	_, _ = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(wFree.ID),
		Status:   "published",
	})

	resFree, err := svc.RegisterGuest(ctx, tenantID, wFree.ID, "مهمان رایگان", "۰۹۱۲۳۳۳۴۴۵۵", "", now)
	if err != nil {
		t.Fatalf("free workshop registration failed: %v", err)
	}
	if resFree.Status != "confirmed" {
		t.Errorf("expected confirmed status for free workshop, got %s", resFree.Status)
	}
	if resFree.ExpiresAt != nil {
		t.Errorf("expected nil ExpiresAt for confirmed free registration, got %v", resFree.ExpiresAt)
	}
	if !resFree.IsFree {
		t.Errorf("expected IsFree=true for free workshop")
	}

	// 2. Paid workshop (PriceRials > 0) -> pending_payment with 15m TTL
	wPaid, err := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه پولی مهارت‌های بالینی",
		Slug:                 "paid-workshop",
		InstructorName:       "دکتر طاهری",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             10,
		PriceRials:           25000000,
	})
	if err != nil {
		t.Fatalf("failed to create paid workshop: %v", err)
	}
	_, _ = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(wPaid.ID),
		Status:   "published",
	})

	resPaid, err := svc.RegisterGuest(ctx, tenantID, wPaid.ID, "مهمان پولی", "09124445566", "", now)
	if err != nil {
		t.Fatalf("paid workshop registration failed: %v", err)
	}
	if resPaid.Status != "pending_payment" {
		t.Errorf("expected pending_payment status for paid workshop, got %s", resPaid.Status)
	}
	if resPaid.ExpiresAt == nil {
		t.Fatal("expected non-nil ExpiresAt for pending_payment registration")
	}
	expectedExp := now.Add(15 * time.Minute)
	if !resPaid.ExpiresAt.Equal(expectedExp) {
		t.Errorf("expected ExpiresAt=%v, got %v", expectedExp, resPaid.ExpiresAt)
	}
	if resPaid.IsFree {
		t.Errorf("expected IsFree=false for paid workshop")
	}

	// 3. Paid workshop near deadline (deadline < 15 minutes away)
	// Hold MUST be capped at registration deadline!
	nearDeadline := now.Add(8 * time.Minute)
	wNearDeadline, err := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه نزدیک به مهلت",
		Slug:                 "near-deadline-workshop",
		InstructorName:       "دکتر مرادی",
		StartsAt:             now.Add(10 * time.Minute),
		EndsAt:               now.Add(50 * time.Minute),
		Location:             "تهران",
		RegistrationDeadline: nearDeadline,
		Capacity:             5,
		PriceRials:           10000000,
	})
	if err != nil {
		t.Fatalf("failed to create near-deadline workshop: %v", err)
	}
	_, _ = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(wNearDeadline.ID),
		Status:   "published",
	})

	resCapped, err := svc.RegisterGuest(ctx, tenantID, wNearDeadline.ID, "مهمان لب مرز", "09125556677", "", now)
	if err != nil {
		t.Fatalf("near-deadline registration failed: %v", err)
	}
	if resCapped.ExpiresAt == nil || !resCapped.ExpiresAt.Equal(nearDeadline) {
		t.Fatalf("expected ExpiresAt capped at deadline %v, got %v", nearDeadline, resCapped.ExpiresAt)
	}
}

func TestDuplicateRegistration_IdempotencyAndNoDataLeak(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	svc := workshops.NewService(mock)
	tenantID := uuid.New()
	now := time.Now()

	w, _ := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه روان‌درمانی فردی",
		Slug:                 "individual-therapy",
		InstructorName:       "دکتر احمدی",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             5,
		PriceRials:           15000000,
	})
	_, _ = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})

	callerToken := "my-secret-idempotency-token"
	// First registration with callerToken
	res1, err := svc.RegisterGuest(ctx, tenantID, w.ID, "علی محمدی", "09127778899", callerToken, now)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Retry with identical callerToken -> Idempotent response with exact same registration ID
	resRetry, err := svc.RegisterGuest(ctx, tenantID, w.ID, "علی محمدی", "09127778899", callerToken, now)
	if err != nil {
		t.Fatalf("idempotent retry failed: %v", err)
	}
	if resRetry.RegistrationID != res1.RegistrationID {
		t.Fatalf("expected same registration ID %s, got %s", res1.RegistrationID, resRetry.RegistrationID)
	}

	// Another caller sends the same phone number (with different token or empty token)
	// Must return ErrDuplicateActiveRegistration and leak NO attendee data
	resOther, err := svc.RegisterGuest(ctx, tenantID, w.ID, "کاربر ناشناس", "09127778899", "other-token", now)
	if err != workshops.ErrDuplicateActiveRegistration {
		t.Fatalf("expected ErrDuplicateActiveRegistration, got: %v", err)
	}
	if resOther != nil {
		t.Fatalf("expected nil result to prevent leaking attendee data, got: %+v", resOther)
	}
}

func TestAtomicSeatReservation_ConcurrentRaceCondition(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	svc := workshops.NewService(mock)
	tenantID := uuid.New()
	now := time.Now()

	// Workshop with capacity = 1
	w, err := svc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه با تنها یک صندلی خالی",
		Slug:                 "single-seat-workshop",
		InstructorName:       "دکتر یکتا",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             1,
		PriceRials:           5000000,
	})
	if err != nil {
		t.Fatalf("failed to create workshop: %v", err)
	}
	_, _ = mock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})

	const numGoroutines = 25
	var (
		wg                sync.WaitGroup
		successCount      int64
		capacityFullCount int64
		otherErrCount     int64
	)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		phone := fmt.Sprintf("0912000%04d", i)
		name := fmt.Sprintf("کاربر رقیب %d", i)
		go func(p, n string) {
			defer wg.Done()
			_, err := svc.RegisterGuest(ctx, tenantID, w.ID, n, p, "", now)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else if errors.Is(err, workshops.ErrCapacityFull) {
				atomic.AddInt64(&capacityFullCount, 1)
			} else {
				atomic.AddInt64(&otherErrCount, 1)
			}
		}(phone, name)
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful registration for 1 seat, got %d", successCount)
	}
	if capacityFullCount != numGoroutines-1 {
		t.Fatalf("expected %d capacity full rejections, got %d", numGoroutines-1, capacityFullCount)
	}
	if otherErrCount != 0 {
		t.Fatalf("expected 0 other errors, got %d", otherErrCount)
	}

	// Verify count in mock is strictly 1 (Zero Oversell!)
	activeCount, err := mock.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(w.ID),
	})
	if err != nil {
		t.Fatalf("failed to count registrations: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("zero oversell violated! active count is %d, expected 1", activeCount)
	}
}

func TestPublicRegistrationStatusQuery(t *testing.T) {
	ctx := context.Background()
	wsMock := newMockQuerier()
	wsSvc := workshops.NewService(wsMock)
	tenantA := uuid.New()
	tenantB := uuid.New()
	now := time.Now()

	domainMock := &mockDomainQuerier{
		domains: map[string]domaindb.Domain{
			"clinic-a.ir": {
				ID:         toPgUUID(uuid.New()),
				TenantID:   toPgUUID(tenantA),
				Host:       "clinic-a.ir",
				IsVerified: true,
			},
			"clinic-b.ir": {
				ID:         toPgUUID(uuid.New()),
				TenantID:   toPgUUID(tenantB),
				Host:       "clinic-b.ir",
				IsVerified: true,
			},
		},
	}
	authMock := newMockAuthQuerier()
	handler := workshops.NewHandler(wsSvc, domain.NewResolver(domainMock), auth.NewService(authMock))
	r := chi.NewRouter()
	handler.MountRoutes(r)

	w, _ := wsSvc.CreateWorkshop(ctx, tenantA, workshops.CreateWorkshopParams{
		Title:                "کارگاه عمومی الف",
		Slug:                 "public-workshop-a",
		InstructorName:       "دکتر الف",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             10,
		PriceRials:           2000000,
	})
	_, _ = wsMock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantA),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})

	// Register a guest via POST /public/workshops/{id}/register
	regPayload := map[string]string{
		"guest_name":  "سارا عباسی",
		"guest_phone": "09121112233",
	}
	body, _ := json.Marshal(regPayload)
	reqReg := httptest.NewRequest("POST", "/public/workshops/"+w.ID.String()+"/register", bytes.NewReader(body))
	reqReg.Host = "clinic-a.ir"
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	r.ServeHTTP(recReg, reqReg)

	if recReg.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got: %d (%s)", recReg.Code, recReg.Body.String())
	}

	var regResp struct {
		RegistrationID string `json:"registration_id"`
		LookupToken    string `json:"lookup_token"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal(recReg.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("failed to decode register response: %v", err)
	}

	// 1. Query status with VALID token on Clinic A domain -> 200 OK
	reqStatus := httptest.NewRequest("GET", "/public/workshops/registrations/"+regResp.RegistrationID+"?lookup_token="+regResp.LookupToken, nil)
	reqStatus.Host = "clinic-a.ir"
	recStatus := httptest.NewRecorder()
	r.ServeHTTP(recStatus, reqStatus)

	if recStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for status query, got: %d (%s)", recStatus.Code, recStatus.Body.String())
	}

	var statusResp workshops.RegistrationStatusResponse
	_ = json.Unmarshal(recStatus.Body.Bytes(), &statusResp)
	if statusResp.GuestName != "سارا عباسی" {
		t.Errorf("expected guest name 'سارا عباسی', got %s", statusResp.GuestName)
	}
	if statusResp.Status != "pending_payment" {
		t.Errorf("expected status 'pending_payment', got %s", statusResp.Status)
	}

	// 2. Query status with INVALID token -> 404 Not Found (no leakage)
	reqBadToken := httptest.NewRequest("GET", "/public/workshops/registrations/"+regResp.RegistrationID+"?lookup_token=wrong-token", nil)
	reqBadToken.Host = "clinic-a.ir"
	recBadToken := httptest.NewRecorder()
	r.ServeHTTP(recBadToken, reqBadToken)

	if recBadToken.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for bad token, got: %d", recBadToken.Code)
	}

	// 3. Query status WITHOUT token -> 404 Not Found
	reqNoToken := httptest.NewRequest("GET", "/public/workshops/registrations/"+regResp.RegistrationID, nil)
	reqNoToken.Host = "clinic-a.ir"
	recNoToken := httptest.NewRecorder()
	r.ServeHTTP(recNoToken, reqNoToken)

	if recNoToken.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for missing token, got: %d", recNoToken.Code)
	}

	// 4. Query status from Tenant B's domain -> 404 Not Found (Cross-tenant isolation)
	reqCross := httptest.NewRequest("GET", "/public/workshops/registrations/"+regResp.RegistrationID+"?lookup_token="+regResp.LookupToken, nil)
	reqCross.Host = "clinic-b.ir"
	recCross := httptest.NewRecorder()
	r.ServeHTTP(recCross, reqCross)

	if recCross.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for cross-tenant registration query, got: %d", recCross.Code)
	}
}

func TestAdminAttendeeManagement(t *testing.T) {
	ctx := context.Background()
	wsMock := newMockQuerier()
	wsSvc := workshops.NewService(wsMock)
	tenantID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	domainMock := &mockDomainQuerier{domains: make(map[string]domaindb.Domain)}
	authMock := newMockAuthQuerier()

	// Register user with receptionist role
	hash, _ := auth.HashPassword("receptionistPass123")
	authMock.identities[userID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(userID),
		Email:        "reception@clinic.ir",
		PasswordHash: hash,
		FullName:     "مسئول پذیرش",
		Status:       "active",
	}
	authMock.memberships[userID.String()+":"+tenantID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantID),
		UserID:   toPgUUID(userID),
		Role:     "receptionist",
	}

	authSvc := auth.NewService(authMock)
	sess, err := authSvc.Login(ctx, "reception@clinic.ir", "receptionistPass123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	token := sess.Token

	handler := workshops.NewHandler(wsSvc, domain.NewResolver(domainMock), authSvc)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Create workshop with capacity = 1
	w, _ := wsSvc.CreateWorkshop(ctx, tenantID, workshops.CreateWorkshopParams{
		Title:                "کارگاه مدیریت هیجان",
		Slug:                 "emotion-management",
		InstructorName:       "دکتر امینی",
		StartsAt:             now.Add(48 * time.Hour),
		EndsAt:               now.Add(52 * time.Hour),
		Location:             "تهران",
		RegistrationDeadline: now.Add(24 * time.Hour),
		Capacity:             1,
		PriceRials:           3000000,
	})
	_, _ = wsMock.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(w.ID),
		Status:   "published",
	})

	// Attendee 1 registers
	reg1, err := wsSvc.RegisterGuest(ctx, tenantID, w.ID, "مشتری اول", "09121112233", "", now)
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// 1. Receptionist lists attendees: GET /admin/workshops/{id}/registrations
	reqList := httptest.NewRequest("GET", "/admin/workshops/"+w.ID.String()+"/registrations", nil)
	reqList.Header.Set("X-Tenant-ID", tenantID.String())
	reqList.Header.Set("Cookie", "session_token="+token)
	recList := httptest.NewRecorder()
	r.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for receptionist listing registrations, got: %d (%s)", recList.Code, recList.Body.String())
	}

	var list []workshops.AdminRegistrationItem
	if err := json.Unmarshal(recList.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode registrations list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(list))
	}
	if list[0].GuestName != "مشتری اول" {
		t.Errorf("expected guest name 'مشتری اول', got %s", list[0].GuestName)
	}

	// 2. Since capacity=1 is full, second attendee cannot register
	_, err = wsSvc.RegisterGuest(ctx, tenantID, w.ID, "مشتری دوم", "09129998877", "", now)
	if err != workshops.ErrCapacityFull {
		t.Fatalf("expected ErrCapacityFull, got: %v", err)
	}

	// 3. Receptionist cancels attendee 1: POST /admin/workshops/{id}/registrations/{reg_id}/cancel
	reqCancel := httptest.NewRequest("POST", "/admin/workshops/"+w.ID.String()+"/registrations/"+reg1.RegistrationID.String()+"/cancel", nil)
	reqCancel.Header.Set("Content-Type", "application/json")
	reqCancel.Header.Set("X-Tenant-ID", tenantID.String())
	reqCancel.Header.Set("Cookie", "session_token="+token)
	reqCancel.Header.Set("X-Requested-With", "XMLHttpRequest")
	recCancel := httptest.NewRecorder()
	r.ServeHTTP(recCancel, reqCancel)

	if recCancel.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for cancellation, got: %d (%s)", recCancel.Code, recCancel.Body.String())
	}

	// 4. Now the seat is freed! Attendee 2 can successfully register
	reg2, err := wsSvc.RegisterGuest(ctx, tenantID, w.ID, "مشتری دوم", "09129998877", "", now)
	if err != nil {
		t.Fatalf("expected successful registration after cancellation, got error: %v", err)
	}
	if reg2.Status != "pending_payment" {
		t.Errorf("expected pending_payment status, got %s", reg2.Status)
	}
}


