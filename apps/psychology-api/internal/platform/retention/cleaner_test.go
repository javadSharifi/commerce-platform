package retention

import (
	"context"
	"fmt"
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

type inMemoryRetentionDB struct {
	registrations map[uuid.UUID]mockRegistration
	workshops     map[uuid.UUID]mockWorkshop
	purchases     map[uuid.UUID]mockPurchase
}

func newInMemoryRetentionDB() *inMemoryRetentionDB {
	return &inMemoryRetentionDB{
		registrations: make(map[uuid.UUID]mockRegistration),
		workshops:     make(map[uuid.UUID]mockWorkshop),
		purchases:     make(map[uuid.UUID]mockPurchase),
	}
}

func (m *inMemoryRetentionDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	cutoff := arguments[0].(time.Time)
	limit := arguments[1].(int)

	var affected int64
	for id, r := range m.registrations {
		if int(affected) >= limit {
			break
		}

		w, ok := m.workshops[r.WorkshopID]
		if !ok || w.TenantID != r.TenantID {
			continue
		}

		// Condition 1: Ended or cancelled before cutoff
		eligibleDate := (w.EndsAt.Before(cutoff)) || (r.Status == "cancelled" && r.UpdatedAt.Before(cutoff))
		if !eligibleDate {
			continue
		}

		// Condition 2: Has PII to scrub
		hasPII := r.GuestPhone != "" || r.GuestName != "مهمان" || r.GuestLookupHash != ""
		if !hasPII {
			continue
		}

		// Condition 3: Financial disputes resolved
		if r.PurchaseID != nil {
			p, pok := m.purchases[*r.PurchaseID]
			if !pok || (p.Status != "completed" && p.Status != "refunded" && p.Status != "failed") {
				continue
			}
		}

		// Scrub PII
		r.GuestName = "مهمان"
		r.GuestPhone = ""
		r.GuestLookupHash = ""
		r.UpdatedAt = time.Now().UTC()
		m.registrations[id] = r
		affected++
	}

	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", affected)), nil
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
