package retention

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type mockRegistration struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	WorkshopID      uuid.UUID
	GuestName       string
	GuestPhone      string
	GuestLookupHash string
	Status          string
	PurchaseID      *uuid.UUID
	UpdatedAt       time.Time
}

type mockWorkshop struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	EndsAt     time.Time
	PriceRials int64
}

type mockPurchase struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	AmountRials   int64
	TitleSnapshot string
	Status        string
}

type mockAppointment struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	SlotID          uuid.UUID
	GuestName       string
	GuestPhone      string
	GuestLookupHash string
	Notes           string
	Status          string
	CancelledAt     *time.Time
	UpdatedAt       time.Time
}

type mockSlot struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	StartsAt time.Time
}

type mockOTP struct {
	ID         uuid.UUID
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

type mockSession struct {
	ID        uuid.UUID
	ExpiresAt time.Time
}

type inMemoryRetentionDB struct {
	registrations map[uuid.UUID]mockRegistration
	workshops     map[uuid.UUID]mockWorkshop
	purchases     map[uuid.UUID]mockPurchase
	appointments  map[uuid.UUID]mockAppointment
	slots         map[uuid.UUID]mockSlot
	otps          map[uuid.UUID]mockOTP
	sessions      map[uuid.UUID]mockSession
}

func newInMemoryRetentionDB() *inMemoryRetentionDB {
	return &inMemoryRetentionDB{
		registrations: make(map[uuid.UUID]mockRegistration),
		workshops:     make(map[uuid.UUID]mockWorkshop),
		purchases:     make(map[uuid.UUID]mockPurchase),
		appointments:  make(map[uuid.UUID]mockAppointment),
		slots:         make(map[uuid.UUID]mockSlot),
		otps:          make(map[uuid.UUID]mockOTP),
		sessions:      make(map[uuid.UUID]mockSession),
	}
}

func (m *inMemoryRetentionDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	cutoff := arguments[0].(time.Time)
	limit := arguments[1].(int)

	var affected int64

	switch {
	case strings.Contains(sql, "UPDATE workshop_registrations"):
		for id, r := range m.registrations {
			if int(affected) >= limit {
				break
			}

			w, ok := m.workshops[r.WorkshopID]
			if !ok || w.TenantID != r.TenantID {
				continue
			}

			eligibleDate := (w.EndsAt.Before(cutoff)) || (r.Status == "cancelled" && r.UpdatedAt.Before(cutoff))
			if !eligibleDate {
				continue
			}

			hasPII := r.GuestPhone != "" || r.GuestName != "مهمان" || r.GuestLookupHash != ""
			if !hasPII {
				continue
			}

			if r.PurchaseID != nil {
				p, pok := m.purchases[*r.PurchaseID]
				if !pok || (p.Status != "completed" && p.Status != "refunded" && p.Status != "failed") {
					continue
				}
			}

			r.GuestName = "مهمان"
			r.GuestPhone = ""
			r.GuestLookupHash = ""
			r.UpdatedAt = time.Now().UTC()
			m.registrations[id] = r
			affected++
		}
		return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", affected)), nil

	case strings.Contains(sql, "UPDATE appointments"):
		for id, a := range m.appointments {
			if int(affected) >= limit {
				break
			}

			s, ok := m.slots[a.SlotID]
			if !ok || s.TenantID != a.TenantID {
				continue
			}

			eligibleDate := (s.StartsAt.Before(cutoff)) || (a.Status == "cancelled" && a.CancelledAt != nil && a.CancelledAt.Before(cutoff))
			if !eligibleDate {
				continue
			}

			hasPII := a.GuestPhone != "" || a.GuestName != "مراجع" || a.GuestLookupHash != "" || a.Notes != ""
			if !hasPII {
				continue
			}

			a.GuestName = "مراجع"
			a.GuestPhone = ""
			a.GuestLookupHash = ""
			a.Notes = ""
			a.UpdatedAt = time.Now().UTC()
			m.appointments[id] = a
			affected++
		}
		return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", affected)), nil

	case strings.Contains(sql, "DELETE FROM buyer_otp_codes"):
		for id, otp := range m.otps {
			if int(affected) >= limit {
				break
			}
			if otp.ExpiresAt.Before(cutoff) || (otp.ConsumedAt != nil && otp.ConsumedAt.Before(cutoff)) {
				delete(m.otps, id)
				affected++
			}
		}
		return pgconn.NewCommandTag(fmt.Sprintf("DELETE %d", affected)), nil

	case strings.Contains(sql, "DELETE FROM buyer_sessions"):
		for id, sess := range m.sessions {
			if int(affected) >= limit {
				break
			}
			if sess.ExpiresAt.Before(cutoff) {
				delete(m.sessions, id)
				affected++
			}
		}
		return pgconn.NewCommandTag(fmt.Sprintf("DELETE %d", affected)), nil

	default:
		return pgconn.NewCommandTag("0"), nil
	}
}

func (m *inMemoryRetentionDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	return nil, nil
}

func TestRetentionCleaner_PersonalDataScrubbing(t *testing.T) {
	db := newInMemoryRetentionDB()
	cleaner := NewCleaner(db)

	tenantID := uuid.New()
	now := time.Now().UTC()
	cutoff := now.Add(-90 * 24 * time.Hour) // 90 days ago

	// 1. Old free workshop (ended 100 days ago) -> MUST be scrubbed
	wsOldFree := mockWorkshop{
		ID:         uuid.New(),
		TenantID:   tenantID,
		EndsAt:     now.Add(-100 * 24 * time.Hour),
		PriceRials: 0,
	}
	db.workshops[wsOldFree.ID] = wsOldFree

	regOldFree := mockRegistration{
		ID:              uuid.New(),
		TenantID:        tenantID,
		WorkshopID:      wsOldFree.ID,
		GuestName:       "علی رضایی",
		GuestPhone:      "09121111111",
		GuestLookupHash: "hash123",
		Status:          "confirmed",
		UpdatedAt:       now.Add(-100 * 24 * time.Hour),
	}
	db.registrations[regOldFree.ID] = regOldFree

	// 2. Recent free workshop (ended 30 days ago) -> MUST NOT be scrubbed
	wsRecentFree := mockWorkshop{
		ID:         uuid.New(),
		TenantID:   tenantID,
		EndsAt:     now.Add(-30 * 24 * time.Hour),
		PriceRials: 0,
	}
	db.workshops[wsRecentFree.ID] = wsRecentFree

	regRecentFree := mockRegistration{
		ID:              uuid.New(),
		TenantID:        tenantID,
		WorkshopID:      wsRecentFree.ID,
		GuestName:       "سارا احمدی",
		GuestPhone:      "09122222222",
		GuestLookupHash: "hash456",
		Status:          "confirmed",
		UpdatedAt:       now.Add(-30 * 24 * time.Hour),
	}
	db.registrations[regRecentFree.ID] = regRecentFree

	// 3. Old paid workshop with COMPLETED purchase -> MUST be scrubbed
	wsOldPaid := mockWorkshop{
		ID:         uuid.New(),
		TenantID:   tenantID,
		EndsAt:     now.Add(-120 * 24 * time.Hour),
		PriceRials: 5000000,
	}
	db.workshops[wsOldPaid.ID] = wsOldPaid

	pCompleted := mockPurchase{
		ID:            uuid.New(),
		TenantID:      tenantID,
		AmountRials:   5000000,
		TitleSnapshot: "کارگاه روان‌درمانی تحلیلی",
		Status:        "completed",
	}
	db.purchases[pCompleted.ID] = pCompleted

	regOldPaidCompleted := mockRegistration{
		ID:              uuid.New(),
		TenantID:        tenantID,
		WorkshopID:      wsOldPaid.ID,
		GuestName:       "مهدی کریمی",
		GuestPhone:      "09123333333",
		GuestLookupHash: "hash789",
		Status:          "confirmed",
		PurchaseID:      &pCompleted.ID,
		UpdatedAt:       now.Add(-120 * 24 * time.Hour),
	}
	db.registrations[regOldPaidCompleted.ID] = regOldPaidCompleted

	// 4. Old paid workshop with REFUND_REQUIRED (open dispute) -> MUST NOT be scrubbed yet
	pDisputed := mockPurchase{
		ID:            uuid.New(),
		TenantID:      tenantID,
		AmountRials:   5000000,
		TitleSnapshot: "کارگاه روان‌درمانی تحلیلی",
		Status:        "refund_required",
	}
	db.purchases[pDisputed.ID] = pDisputed

	regOldPaidDisputed := mockRegistration{
		ID:              uuid.New(),
		TenantID:        tenantID,
		WorkshopID:      wsOldPaid.ID,
		GuestName:       "نرگس حسینی",
		GuestPhone:      "09124444444",
		GuestLookupHash: "hash999",
		Status:          "confirmed",
		PurchaseID:      &pDisputed.ID,
		UpdatedAt:       now.Add(-120 * 24 * time.Hour),
	}
	db.registrations[regOldPaidDisputed.ID] = regOldPaidDisputed

	// 5. Cancelled registration (cancelled 95 days ago, workshop in future) -> MUST be scrubbed
	wsFuture := mockWorkshop{
		ID:         uuid.New(),
		TenantID:   tenantID,
		EndsAt:     now.Add(30 * 24 * time.Hour),
		PriceRials: 0,
	}
	db.workshops[wsFuture.ID] = wsFuture

	regCancelledOld := mockRegistration{
		ID:              uuid.New(),
		TenantID:        tenantID,
		WorkshopID:      wsFuture.ID,
		GuestName:       "محمد نوری",
		GuestPhone:      "09125555555",
		GuestLookupHash: "hash000",
		Status:          "cancelled",
		UpdatedAt:       now.Add(-95 * 24 * time.Hour),
	}
	db.registrations[regCancelledOld.ID] = regCancelledOld

	// Run retention cleaner
	affected, err := cleaner.AnonymizeExpiredGuestData(context.Background(), cutoff, 100)
	if err != nil {
		t.Fatalf("cleaner failed: %v", err)
	}

	// Exactly 3 items should be anonymized: 1 (old free), 3 (old paid completed), 5 (old cancelled)
	if affected != 3 {
		t.Errorf("expected 3 registrations anonymized, got %d", affected)
	}

	// Verify Case 1 (old free) is scrubbed
	r1 := db.registrations[regOldFree.ID]
	if r1.GuestName != "مهمان" || r1.GuestPhone != "" || r1.GuestLookupHash != "" {
		t.Errorf("case 1 failed to scrub PII: %+v", r1)
	}

	// Verify Case 2 (recent free) is NOT scrubbed
	r2 := db.registrations[regRecentFree.ID]
	if r2.GuestName != "سارا احمدی" || r2.GuestPhone != "09122222222" {
		t.Errorf("case 2 should not be scrubbed: %+v", r2)
	}

	// Verify Case 3 (old paid completed) is scrubbed
	r3 := db.registrations[regOldPaidCompleted.ID]
	if r3.GuestName != "مهمان" || r3.GuestPhone != "" || r3.GuestLookupHash != "" {
		t.Errorf("case 3 failed to scrub PII: %+v", r3)
	}
	// Verify financial record of Case 3 is 100% INTACT
	p3 := db.purchases[pCompleted.ID]
	if p3.AmountRials != 5000000 || p3.TitleSnapshot != "کارگاه روان‌درمانی تحلیلی" || p3.Status != "completed" {
		t.Errorf("case 3 financial audit data corrupted: %+v", p3)
	}

	// Verify Case 4 (old paid with dispute) is NOT scrubbed
	r4 := db.registrations[regOldPaidDisputed.ID]
	if r4.GuestName != "نرگس حسینی" || r4.GuestPhone != "09124444444" {
		t.Errorf("case 4 should not be scrubbed due to open dispute: %+v", r4)
	}

	// Verify Case 5 (old cancelled) is scrubbed
	r5 := db.registrations[regCancelledOld.ID]
	if r5.GuestName != "مهمان" || r5.GuestPhone != "" {
		t.Errorf("case 5 failed to scrub: %+v", r5)
	}
}

func TestRetentionCleaner_AppointmentDataScrubbing(t *testing.T) {
	db := newInMemoryRetentionDB()
	cleaner := NewCleaner(db)

	tenantID := uuid.New()
	now := time.Now().UTC()
	cutoff := now.Add(-90 * 24 * time.Hour) // 90 days ago

	// 1. Old appointment (slot started 100 days ago) -> MUST be scrubbed
	slotOld := mockSlot{
		ID:       uuid.New(),
		TenantID: tenantID,
		StartsAt: now.Add(-100 * 24 * time.Hour),
	}
	db.slots[slotOld.ID] = slotOld

	appOld := mockAppointment{
		ID:              uuid.New(),
		TenantID:        tenantID,
		SlotID:          slotOld.ID,
		GuestName:       "بیمار یک",
		GuestPhone:      "09121112233",
		GuestLookupHash: "hash-app-1",
		Notes:           "اضطراب حاد و سابقه پانیک",
		Status:          "completed",
		UpdatedAt:       now.Add(-100 * 24 * time.Hour),
	}
	db.appointments[appOld.ID] = appOld

	// 2. Recent appointment (slot started 10 days ago) -> MUST NOT be scrubbed
	slotRecent := mockSlot{
		ID:       uuid.New(),
		TenantID: tenantID,
		StartsAt: now.Add(-10 * 24 * time.Hour),
	}
	db.slots[slotRecent.ID] = slotRecent

	appRecent := mockAppointment{
		ID:              uuid.New(),
		TenantID:        tenantID,
		SlotID:          slotRecent.ID,
		GuestName:       "بیمار دو",
		GuestPhone:      "09124445566",
		GuestLookupHash: "hash-app-2",
		Notes:           "جلسه ارزیابی اولیه",
		Status:          "confirmed",
		UpdatedAt:       now.Add(-10 * 24 * time.Hour),
	}
	db.appointments[appRecent.ID] = appRecent

	// 3. Cancelled appointment (cancelled 120 days ago, slot was in future) -> MUST be scrubbed
	slotFuture := mockSlot{
		ID:       uuid.New(),
		TenantID: tenantID,
		StartsAt: now.Add(20 * 24 * time.Hour),
	}
	db.slots[slotFuture.ID] = slotFuture

	cancelledTime := now.Add(-120 * 24 * time.Hour)
	appCancelled := mockAppointment{
		ID:              uuid.New(),
		TenantID:        tenantID,
		SlotID:          slotFuture.ID,
		GuestName:       "بیمار سه",
		GuestPhone:      "09127778899",
		GuestLookupHash: "hash-app-3",
		Notes:           "لغو شده توسط مراجع",
		Status:          "cancelled",
		CancelledAt:     &cancelledTime,
		UpdatedAt:       cancelledTime,
	}
	db.appointments[appCancelled.ID] = appCancelled

	affected, err := cleaner.AnonymizeExpiredAppointmentData(context.Background(), cutoff, 100)
	if err != nil {
		t.Fatalf("AnonymizeExpiredAppointmentData failed: %v", err)
	}

	if affected != 2 {
		t.Fatalf("expected 2 appointments anonymized, got %d", affected)
	}

	// Verify old appointment is scrubbed
	resOld := db.appointments[appOld.ID]
	if resOld.GuestName != "مراجع" || resOld.GuestPhone != "" || resOld.GuestLookupHash != "" || resOld.Notes != "" {
		t.Errorf("appointment 1 was not properly scrubbed: %+v", resOld)
	}

	// Verify recent appointment is intact
	resRecent := db.appointments[appRecent.ID]
	if resRecent.GuestName != "بیمار دو" || resRecent.GuestPhone != "09124445566" || resRecent.Notes != "جلسه ارزیابی اولیه" {
		t.Errorf("appointment 2 should remain intact: %+v", resRecent)
	}

	// Verify cancelled old appointment is scrubbed
	resCancelled := db.appointments[appCancelled.ID]
	if resCancelled.GuestName != "مراجع" || resCancelled.GuestPhone != "" || resCancelled.Notes != "" {
		t.Errorf("appointment 3 was not properly scrubbed: %+v", resCancelled)
	}
}

func TestRetentionCleaner_ExpiredOTPsAndSessionsPurge(t *testing.T) {
	db := newInMemoryRetentionDB()
	cleaner := NewCleaner(db)

	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour) // 1 day ago

	// 1. Expired OTP (> 24 hours ago) -> MUST be purged
	otpExpired := mockOTP{
		ID:        uuid.New(),
		ExpiresAt: now.Add(-48 * time.Hour),
	}
	db.otps[otpExpired.ID] = otpExpired

	// 2. Active OTP (expires in 10 minutes) -> MUST NOT be purged
	otpActive := mockOTP{
		ID:        uuid.New(),
		ExpiresAt: now.Add(10 * time.Minute),
	}
	db.otps[otpActive.ID] = otpActive

	// 3. Consumed OTP (> 24 hours ago) -> MUST be purged
	consumedAt := now.Add(-36 * time.Hour)
	otpConsumed := mockOTP{
		ID:         uuid.New(),
		ExpiresAt:  now.Add(1 * time.Hour),
		ConsumedAt: &consumedAt,
	}
	db.otps[otpConsumed.ID] = otpConsumed

	// 4. Expired buyer session (> 24 hours ago) -> MUST be purged
	sessExpired := mockSession{
		ID:        uuid.New(),
		ExpiresAt: now.Add(-72 * time.Hour),
	}
	db.sessions[sessExpired.ID] = sessExpired

	// 5. Active buyer session (expires in 7 days) -> MUST NOT be purged
	sessActive := mockSession{
		ID:        uuid.New(),
		ExpiresAt: now.Add(7 * 24 * time.Hour),
	}
	db.sessions[sessActive.ID] = sessActive

	// Run OTP purge
	purgedOTPs, err := cleaner.PurgeExpiredOTPs(context.Background(), cutoff, 100)
	if err != nil {
		t.Fatalf("PurgeExpiredOTPs failed: %v", err)
	}
	if purgedOTPs != 2 {
		t.Errorf("expected 2 OTPs purged, got %d", purgedOTPs)
	}
	if _, exists := db.otps[otpExpired.ID]; exists {
		t.Errorf("otpExpired was not deleted")
	}
	if _, exists := db.otps[otpConsumed.ID]; exists {
		t.Errorf("otpConsumed was not deleted")
	}
	if _, exists := db.otps[otpActive.ID]; !exists {
		t.Errorf("otpActive was accidentally deleted")
	}

	// Run Session purge
	purgedSessions, err := cleaner.PurgeExpiredBuyerSessions(context.Background(), cutoff, 100)
	if err != nil {
		t.Fatalf("PurgeExpiredBuyerSessions failed: %v", err)
	}
	if purgedSessions != 1 {
		t.Errorf("expected 1 session purged, got %d", purgedSessions)
	}
	if _, exists := db.sessions[sessExpired.ID]; exists {
		t.Errorf("sessExpired was not deleted")
	}
	if _, exists := db.sessions[sessActive.ID]; !exists {
		t.Errorf("sessActive was accidentally deleted")
	}
}
