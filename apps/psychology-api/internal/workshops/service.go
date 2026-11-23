package workshops

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"psychology/api/internal/education"
	"psychology/api/internal/platform/db"
	"regexp"
	"strings"
	"sync"
	"time"

	"psychology/api/internal/workshops/workshopsdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrWorkshopNotFound                = errors.New("workshops: workshop not found")
	ErrWorkshopNotPublished            = errors.New("workshops: workshop is not open for registration")
	ErrRegistrationDeadlinePassed      = errors.New("workshops: registration deadline has passed")
	ErrCapacityFull                    = errors.New("workshops: workshop capacity is full (0 seats available)")
	ErrDuplicateActiveRegistration     = errors.New("workshops: active registration already exists for this phone number")
	ErrCannotReduceCapacityBelowActive = errors.New("workshops: cannot reduce capacity below currently booked seats")
	ErrInvalidDates                    = errors.New("workshops: ends_at must be after starts_at and registration_deadline <= starts_at")
	ErrRegistrationNotFound            = errors.New("workshops: registration not found")
	ErrCannotModifyActiveWorkshop      = errors.New("workshops: cannot change price or schedule after registrations exist")
	ErrInvalidPhone                    = errors.New("workshops: invalid Iranian phone format (must be 09XXXXXXXXX)")
)

var phoneRegex = regexp.MustCompile(`^09[0-9]{9}$`)

// NormalizePhone normalizes Iranian phone numbers to standard 11-digit format: 09XXXXXXXXX
func NormalizePhone(phone string) (string, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r >= '۰' && r <= '۹' {
			return r - '۰' + '0'
		}
		if r >= '٠' && r <= '٩' {
			return r - '٠' + '0'
		}
		if (r >= '0' && r <= '9') || r == '+' {
			return r
		}
		return -1
	}, phone)

	if strings.HasPrefix(cleaned, "+98") {
		cleaned = "0" + cleaned[3:]
	} else if strings.HasPrefix(cleaned, "0098") {
		cleaned = "0" + cleaned[4:]
	} else if strings.HasPrefix(cleaned, "98") && len(cleaned) == 12 {
		cleaned = "0" + cleaned[2:]
	} else if strings.HasPrefix(cleaned, "9") && len(cleaned) == 10 {
		cleaned = "0" + cleaned
	}

	if !phoneRegex.MatchString(cleaned) {
		return "", ErrInvalidPhone
	}
	return cleaned, nil
}

func HashLookupToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type Querier interface {
	CreateWorkshop(ctx context.Context, arg workshopsdb.CreateWorkshopParams) (workshopsdb.Workshop, error)
	GetWorkshopByID(ctx context.Context, arg workshopsdb.GetWorkshopByIDParams) (workshopsdb.Workshop, error)
	GetWorkshopByIDForUpdate(ctx context.Context, arg workshopsdb.GetWorkshopByIDForUpdateParams) (workshopsdb.Workshop, error)
	GetWorkshopBySlug(ctx context.Context, arg workshopsdb.GetWorkshopBySlugParams) (workshopsdb.Workshop, error)
	ListWorkshops(ctx context.Context, arg workshopsdb.ListWorkshopsParams) ([]workshopsdb.Workshop, error)
	ListPublishedWorkshops(ctx context.Context, tenantID pgtype.UUID) ([]workshopsdb.Workshop, error)
	UpdateWorkshop(ctx context.Context, arg workshopsdb.UpdateWorkshopParams) (workshopsdb.Workshop, error)
	UpdateWorkshopStatus(ctx context.Context, arg workshopsdb.UpdateWorkshopStatusParams) (workshopsdb.Workshop, error)
	UpdateWorkshopCapacity(ctx context.Context, arg workshopsdb.UpdateWorkshopCapacityParams) (workshopsdb.Workshop, error)
	CountActiveRegistrations(ctx context.Context, arg workshopsdb.CountActiveRegistrationsParams) (int64, error)
	CountConfirmedRegistrations(ctx context.Context, arg workshopsdb.CountConfirmedRegistrationsParams) (int64, error)
	CreateRegistration(ctx context.Context, arg workshopsdb.CreateRegistrationParams) (workshopsdb.WorkshopRegistration, error)
	GetRegistrationByID(ctx context.Context, arg workshopsdb.GetRegistrationByIDParams) (workshopsdb.WorkshopRegistration, error)
	GetRegistrationByIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByIDForUpdateParams) (workshopsdb.WorkshopRegistration, error)
	GetRegistrationByPurchaseID(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDParams) (workshopsdb.WorkshopRegistration, error)
	GetRegistrationByPurchaseIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDForUpdateParams) (workshopsdb.WorkshopRegistration, error)
	GetRegistrationByLookupHash(ctx context.Context, arg workshopsdb.GetRegistrationByLookupHashParams) (workshopsdb.WorkshopRegistration, error)
	GetActiveRegistrationByPhone(ctx context.Context, arg workshopsdb.GetActiveRegistrationByPhoneParams) (workshopsdb.WorkshopRegistration, error)
	ListRegistrationsByWorkshop(ctx context.Context, arg workshopsdb.ListRegistrationsByWorkshopParams) ([]workshopsdb.WorkshopRegistration, error)
	UpdateRegistrationStatus(ctx context.Context, arg workshopsdb.UpdateRegistrationStatusParams) (workshopsdb.WorkshopRegistration, error)
	ExpirePendingRegistrations(ctx context.Context) (int64, error)
}

type Service struct {
	catalogStore *db.Store
	queries      Querier
	wsLocks      sync.Map
}

func (s *Service) getWorkshopLock(id uuid.UUID) *sync.Mutex {
	val, _ := s.wsLocks.LoadOrStore(id, &sync.Mutex{})
	return val.(*sync.Mutex)
}

func NewService(q Querier) *Service {
	return &Service{queries: q}
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

type CreateWorkshopParams struct {
	Topic                string
	Title                string
	Slug                 string
	Description          string
	InstructorName       string
	StartsAt             time.Time
	EndsAt               time.Time
	Location             string
	RegistrationDeadline time.Time
	Capacity             int
	PriceRials           int64
}

type Workshop struct {
	Topic                string
	ID                   uuid.UUID
	TenantID             uuid.UUID
	Title                string
	Slug                 string
	Description          string
	InstructorName       string
	StartsAt             time.Time
	EndsAt               time.Time
	Location             string
	RegistrationDeadline time.Time
	Capacity             int
	PriceRials           int64
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type WorkshopDetail struct {
	Workshop
	ActiveRegistrations int64
	RemainingCapacity   int64
	IsRegistrationOpen  bool
}

type PublicWorkshopItem struct {
	Topic              string    `json:"topic,omitempty"`
	ID                 uuid.UUID `json:"id"`
	Title              string    `json:"title"`
	Slug               string    `json:"slug"`
	InstructorName     string    `json:"instructor_name"`
	StartsAt           time.Time `json:"starts_at"`
	EndsAt             time.Time `json:"ends_at"`
	Location           string    `json:"location"`
	PriceRials         int64     `json:"price_rials"`
	RemainingCapacity  int64     `json:"remaining_capacity"`
	IsRegistrationOpen bool      `json:"is_registration_open"`
}

type PublicWorkshopDetail struct {
	PublicWorkshopItem
	Description          string    `json:"description"`
	RegistrationDeadline time.Time `json:"registration_deadline"`
}

type AdminWorkshopItem struct {
	Topic          string    `json:"topic,omitempty"`
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	Slug           string    `json:"slug"`
	InstructorName string    `json:"instructor_name"`
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	Capacity       int       `json:"capacity"`
	ConfirmedCount int64     `json:"confirmed_count"`
	PendingCount   int64     `json:"pending_count"`
	PriceRials     int64     `json:"price_rials"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type AdminWorkshopDetail struct {
	AdminWorkshopItem
	Description          string    `json:"description"`
	Location             string    `json:"location"`
	RegistrationDeadline time.Time `json:"registration_deadline"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type UpdateWorkshopParams struct {
	Topic                *string
	Title                *string
	Slug                 *string
	Description          *string
	InstructorName       *string
	StartsAt             *time.Time
	EndsAt               *time.Time
	Location             *string
	RegistrationDeadline *time.Time
	Capacity             *int
	PriceRials           *int64
}

type RegistrationResult struct {
	RegistrationID uuid.UUID  `json:"registration_id"`
	WorkshopID     uuid.UUID  `json:"workshop_id"`
	Status         string     `json:"status"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LookupToken    string     `json:"lookup_token"`
	IsFree         bool       `json:"is_free"`
}

type RegistrationStatusResponse struct {
	RegistrationID uuid.UUID  `json:"registration_id"`
	WorkshopID     uuid.UUID  `json:"workshop_id"`
	WorkshopTitle  string     `json:"workshop_title"`
	GuestName      string     `json:"guest_name"`
	Status         string     `json:"status"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type AdminRegistrationItem struct {
	ID         uuid.UUID  `json:"id"`
	GuestName  string     `json:"guest_name"`
	GuestPhone string     `json:"guest_phone"`
	Status     string     `json:"status"`
	PurchaseID *uuid.UUID `json:"purchase_id,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (s *Service) CreateWorkshop(ctx context.Context, tenantID uuid.UUID, p CreateWorkshopParams) (*Workshop, error) {
	if s.catalogStore == nil {
		return s.createWorkshop(ctx, s.queries, tenantID, p)
	}
	return education.WriteCatalog(ctx, s.catalogStore, tenantID, func(tx pgx.Tx) (*Workshop, error) {
		return s.createWorkshop(ctx, workshopsdb.New(tx), tenantID, p)
	})
}

func (s *Service) createWorkshop(ctx context.Context, q Querier, tenantID uuid.UUID, p CreateWorkshopParams) (*Workshop, error) {
	topic, err := education.NormalizeTopic(p.Topic)
	if err != nil {
		return nil, err
	}

	if !p.EndsAt.After(p.StartsAt) || p.RegistrationDeadline.After(p.StartsAt) {
		return nil, ErrInvalidDates
	}
	if p.Capacity < 1 {
		p.Capacity = 1
	}

	w, err := q.CreateWorkshop(ctx, workshopsdb.CreateWorkshopParams{
		TenantID:             toPgUUID(tenantID),
		Title:                p.Title,
		Topic:                topic,
		Slug:                 p.Slug,
		Description:          p.Description,
		InstructorName:       p.InstructorName,
		StartsAt:             toPgTimestamptz(p.StartsAt),
		EndsAt:               toPgTimestamptz(p.EndsAt),
		Location:             p.Location,
		RegistrationDeadline: toPgTimestamptz(p.RegistrationDeadline),
		Capacity:             int32(p.Capacity),
		PriceRials:           p.PriceRials,
		Status:               "draft",
	})
	if err != nil {
		return nil, err
	}

	return mapWorkshop(w), nil
}

func (s *Service) GetWorkshop(ctx context.Context, tenantID, id uuid.UUID) (*WorkshopDetail, error) {
	w, err := s.queries.GetWorkshopByID(ctx, workshopsdb.GetWorkshopByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	activeCount, err := s.queries.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(id),
	})
	if err != nil {
		return nil, err
	}

	remaining := int64(w.Capacity) - activeCount
	if remaining < 0 {
		remaining = 0
	}

	isOpen := w.Status == "published" && time.Now().Before(w.RegistrationDeadline.Time) && remaining > 0

	return &WorkshopDetail{
		Workshop:            *mapWorkshop(w),
		ActiveRegistrations: activeCount,
		RemainingCapacity:   remaining,
		IsRegistrationOpen:  isOpen,
	}, nil
}

// UpdateWorkshopCapacity atomically updates capacity after verifying that currently booked seats do not exceed the new capacity.
func (s *Service) UpdateWorkshopCapacity(ctx context.Context, tenantID, id uuid.UUID, newCapacity int) (*Workshop, error) {
	if newCapacity < 1 {
		return nil, errors.New("capacity must be at least 1")
	}

	// 1. Lock workshop row for update
	w, err := s.queries.GetWorkshopByIDForUpdate(ctx, workshopsdb.GetWorkshopByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	// 2. Count active registrations (confirmed + valid pending_payment holds)
	activeCount, err := s.queries.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(id),
	})
	if err != nil {
		return nil, err
	}

	// 3. Ensure new capacity is not lower than active bookings
	if int64(newCapacity) < activeCount {
		return nil, ErrCannotReduceCapacityBelowActive
	}

	// 4. Update capacity
	updated, err := s.queries.UpdateWorkshopCapacity(ctx, workshopsdb.UpdateWorkshopCapacityParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
		Capacity: int32(newCapacity),
	})
	if err != nil {
		return nil, err
	}

	_ = w
	return mapWorkshop(updated), nil
}

// RegisterGuest executes seat reservation following strict lock order and zero-oversell guarantee.
func (s *Service) RegisterGuest(ctx context.Context, tenantID, workshopID uuid.UUID, guestName, rawPhone string, callerToken string, now time.Time) (*RegistrationResult, error) {
	// Normalize phone
	guestPhone, err := NormalizePhone(rawPhone)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(guestName) == "" {
		return nil, errors.New("workshops: guest name cannot be empty")
	}

	mu := s.getWorkshopLock(workshopID)
	mu.Lock()
	defer mu.Unlock()

	// 1. Lock parent workshop row first (Preempts race conditions and deadlocks)
	w, err := s.queries.GetWorkshopByIDForUpdate(ctx, workshopsdb.GetWorkshopByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(workshopID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	// 2. Validate workshop status and deadline
	if w.Status != "published" {
		return nil, ErrWorkshopNotPublished
	}
	if now.After(w.RegistrationDeadline.Time) {
		return nil, ErrRegistrationDeadlinePassed
	}

	// 3. Prevent duplicate active registration for the same phone number (or idempotent retry)
	activeReg, err := s.queries.GetActiveRegistrationByPhone(ctx, workshopsdb.GetActiveRegistrationByPhoneParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(workshopID),
		GuestPhone: guestPhone,
	})
	if err == nil {
		// Idempotent retry: if callerToken matches existing active registration hash, return it cleanly
		if callerToken != "" && HashLookupToken(callerToken) == activeReg.GuestLookupHash {
			var exp *time.Time
			if activeReg.ExpiresAt.Valid {
				t := activeReg.ExpiresAt.Time
				exp = &t
			}
			return &RegistrationResult{
				RegistrationID: fromPgUUID(activeReg.ID),
				WorkshopID:     workshopID,
				Status:         activeReg.Status,
				ExpiresAt:      exp,
				LookupToken:    callerToken,
				IsFree:         w.PriceRials == 0,
			}, nil
		}
		return nil, ErrDuplicateActiveRegistration
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 4. Atomically count active reservations (confirmed + active holds)
	// Expired holds (status='pending_payment' AND expires_at <= now) are NOT counted!
	activeCount, err := s.queries.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(workshopID),
	})
	if err != nil {
		return nil, err
	}

	// 5. Check zero seats / capacity full
	if activeCount >= int64(w.Capacity) {
		return nil, ErrCapacityFull
	}

	// 6. Generate secure lookup token and hash
	lookupToken := callerToken
	if lookupToken == "" {
		tokenBytes := make([]byte, 16)
		if _, err := rand.Read(tokenBytes); err != nil {
			return nil, err
		}
		lookupToken = "tok_" + hex.EncodeToString(tokenBytes)
	}
	lookupHash := HashLookupToken(lookupToken)

	isFree := w.PriceRials == 0
	var status string
	var expiresAt pgtype.Timestamptz

	if isFree {
		status = "confirmed"
		expiresAt = pgtype.Timestamptz{Valid: false}
	} else {
		status = "pending_payment"
		// 15-minute hold TTL, capped at workshop registration deadline
		holdEnd := now.Add(15 * time.Minute)
		if holdEnd.After(w.RegistrationDeadline.Time) {
			holdEnd = w.RegistrationDeadline.Time
		}
		expiresAt = toPgTimestamptz(holdEnd)
	}

	reg, err := s.queries.CreateRegistration(ctx, workshopsdb.CreateRegistrationParams{
		TenantID:        toPgUUID(tenantID),
		WorkshopID:      toPgUUID(workshopID),
		GuestName:       guestName,
		GuestPhone:      guestPhone,
		Status:          status,
		ExpiresAt:       expiresAt,
		PurchaseID:      pgtype.UUID{Valid: false},
		GuestLookupHash: lookupHash,
	})
	if err != nil {
		return nil, err
	}

	var exp *time.Time
	if expiresAt.Valid {
		t := expiresAt.Time
		exp = &t
	}

	return &RegistrationResult{
		RegistrationID: fromPgUUID(reg.ID),
		WorkshopID:     workshopID,
		Status:         status,
		ExpiresAt:      exp,
		LookupToken:    lookupToken,
		IsFree:         isFree,
	}, nil
}

func (s *Service) GetRegistrationStatus(ctx context.Context, tenantID, regID uuid.UUID, lookupToken string) (*RegistrationStatusResponse, error) {
	if lookupToken == "" {
		return nil, ErrRegistrationNotFound
	}

	reg, err := s.queries.GetRegistrationByID(ctx, workshopsdb.GetRegistrationByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(regID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRegistrationNotFound
		}
		return nil, err
	}

	if reg.GuestLookupHash != HashLookupToken(lookupToken) {
		return nil, ErrRegistrationNotFound
	}

	ws, err := s.queries.GetWorkshopByID(ctx, workshopsdb.GetWorkshopByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       reg.WorkshopID,
	})
	if err != nil {
		return nil, err
	}

	var exp *time.Time
	if reg.ExpiresAt.Valid {
		t := reg.ExpiresAt.Time
		exp = &t
	}

	return &RegistrationStatusResponse{
		RegistrationID: fromPgUUID(reg.ID),
		WorkshopID:     fromPgUUID(ws.ID),
		WorkshopTitle:  ws.Title,
		GuestName:      reg.GuestName,
		Status:         reg.Status,
		ExpiresAt:      exp,
	}, nil
}

func (s *Service) ListAdminRegistrations(ctx context.Context, tenantID, workshopID uuid.UUID) ([]AdminRegistrationItem, error) {
	rows, err := s.queries.ListRegistrationsByWorkshop(ctx, workshopsdb.ListRegistrationsByWorkshopParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(workshopID),
	})
	if err != nil {
		return nil, err
	}

	res := make([]AdminRegistrationItem, 0, len(rows))
	for _, r := range rows {
		var exp *time.Time
		if r.ExpiresAt.Valid {
			t := r.ExpiresAt.Time
			exp = &t
		}
		var pID *uuid.UUID
		if r.PurchaseID.Valid {
			u := fromPgUUID(r.PurchaseID)
			pID = &u
		}

		res = append(res, AdminRegistrationItem{
			ID:         fromPgUUID(r.ID),
			GuestName:  r.GuestName,
			GuestPhone: r.GuestPhone,
			Status:     r.Status,
			PurchaseID: pID,
			ExpiresAt:  exp,
			CreatedAt:  r.CreatedAt.Time,
		})
	}
	return res, nil
}

func (s *Service) CancelRegistrationByAdmin(ctx context.Context, tenantID, workshopID, registrationID uuid.UUID) error {
	mu := s.getWorkshopLock(workshopID)
	mu.Lock()
	defer mu.Unlock()

	// Lock workshop first
	_, err := s.queries.GetWorkshopByIDForUpdate(ctx, workshopsdb.GetWorkshopByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(workshopID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrWorkshopNotFound
		}
		return err
	}

	reg, err := s.queries.GetRegistrationByIDForUpdate(ctx, workshopsdb.GetRegistrationByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(registrationID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRegistrationNotFound
		}
		return err
	}

	if fromPgUUID(reg.WorkshopID) != workshopID {
		return ErrRegistrationNotFound
	}

	_, err = s.queries.UpdateRegistrationStatus(ctx, workshopsdb.UpdateRegistrationStatusParams{
		TenantID:   toPgUUID(tenantID),
		ID:         toPgUUID(registrationID),
		Status:     "cancelled",
		PurchaseID: reg.PurchaseID,
	})
	return err
}

func mapWorkshop(w workshopsdb.Workshop) *Workshop {
	return &Workshop{
		ID:                   fromPgUUID(w.ID),
		TenantID:             fromPgUUID(w.TenantID),
		Title:                w.Title,
		Topic:                w.Topic,
		Slug:                 w.Slug,
		Description:          w.Description,
		InstructorName:       w.InstructorName,
		StartsAt:             w.StartsAt.Time,
		EndsAt:               w.EndsAt.Time,
		Location:             w.Location,
		RegistrationDeadline: w.RegistrationDeadline.Time,
		Capacity:             int(w.Capacity),
		PriceRials:           w.PriceRials,
		Status:               w.Status,
		CreatedAt:            w.CreatedAt.Time,
		UpdatedAt:            w.UpdatedAt.Time,
	}
}

func (s *Service) WithCatalogStore(store *db.Store) *Service {
	s.catalogStore = store
	return s
}

func (s *Service) ListPublishedWorkshops(ctx context.Context, tenantID uuid.UUID) ([]PublicWorkshopItem, error) {
	return db.ReadTenant(ctx, s.catalogStore, tenantID, s.queries,
		func(tx pgx.Tx) Querier { return workshopsdb.New(tx) },
		func(q Querier) ([]PublicWorkshopItem, error) {
			rows, err := q.ListPublishedWorkshops(ctx, toPgUUID(tenantID))
			if err != nil {
				return nil, err
			}

			res := make([]PublicWorkshopItem, 0, len(rows))
			now := time.Now()
			for _, w := range rows {
				activeCount, err := q.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
					TenantID:   w.TenantID,
					WorkshopID: w.ID,
				})
				if err != nil {
					return nil, err
				}

				remaining := int64(w.Capacity) - activeCount
				if remaining < 0 {
					remaining = 0
				}

				isOpen := remaining > 0 && now.Before(w.RegistrationDeadline.Time)

				res = append(res, PublicWorkshopItem{
					ID:                 fromPgUUID(w.ID),
					Title:              w.Title,
					Topic:              w.Topic,
					Slug:               w.Slug,
					InstructorName:     w.InstructorName,
					StartsAt:           w.StartsAt.Time,
					EndsAt:             w.EndsAt.Time,
					Location:           w.Location,
					PriceRials:         w.PriceRials,
					RemainingCapacity:  remaining,
					IsRegistrationOpen: isOpen,
				})
			}

			return res, nil
		},
	)
}

func (s *Service) GetPublicWorkshopDetail(ctx context.Context, tenantID, id uuid.UUID) (*PublicWorkshopDetail, error) {
	return db.ReadTenant(ctx, s.catalogStore, tenantID, s.queries,
		func(tx pgx.Tx) Querier { return workshopsdb.New(tx) },
		func(q Querier) (*PublicWorkshopDetail, error) {
			w, err := q.GetWorkshopByID(ctx, workshopsdb.GetWorkshopByIDParams{
				TenantID: toPgUUID(tenantID),
				ID:       toPgUUID(id),
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, ErrWorkshopNotFound
				}
				return nil, err
			}

			if w.Status != "published" {
				return nil, ErrWorkshopNotFound
			}

			activeCount, err := q.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
				TenantID:   toPgUUID(tenantID),
				WorkshopID: toPgUUID(id),
			})
			if err != nil {
				return nil, err
			}

			remaining := int64(w.Capacity) - activeCount
			if remaining < 0 {
				remaining = 0
			}

			isOpen := remaining > 0 && time.Now().Before(w.RegistrationDeadline.Time)

			return &PublicWorkshopDetail{
				PublicWorkshopItem: PublicWorkshopItem{
					ID:                 fromPgUUID(w.ID),
					Title:              w.Title,
					Topic:              w.Topic,
					Slug:               w.Slug,
					InstructorName:     w.InstructorName,
					StartsAt:           w.StartsAt.Time,
					EndsAt:             w.EndsAt.Time,
					Location:           w.Location,
					PriceRials:         w.PriceRials,
					RemainingCapacity:  remaining,
					IsRegistrationOpen: isOpen,
				},
				Description:          w.Description,
				RegistrationDeadline: w.RegistrationDeadline.Time,
			}, nil
		},
	)
}

func (s *Service) ListAdminWorkshops(ctx context.Context, tenantID uuid.UUID, status *string) ([]AdminWorkshopItem, error) {
	return db.ReadTenant(ctx, s.catalogStore, tenantID, s.queries,
		func(tx pgx.Tx) Querier { return workshopsdb.New(tx) },
		func(q Querier) ([]AdminWorkshopItem, error) { return s.listAdminWorkshops(ctx, q, tenantID, status) },
	)
}

func (s *Service) listAdminWorkshops(ctx context.Context, q Querier, tenantID uuid.UUID, status *string) ([]AdminWorkshopItem, error) {
	var statusArg pgtype.Text
	if status != nil && *status != "" {
		statusArg = pgtype.Text{String: *status, Valid: true}
	}

	rows, err := q.ListWorkshops(ctx, workshopsdb.ListWorkshopsParams{
		TenantID: toPgUUID(tenantID),
		Status:   statusArg,
	})
	if err != nil {
		return nil, err
	}

	res := make([]AdminWorkshopItem, 0, len(rows))
	for _, w := range rows {
		confirmedCount, err := q.CountConfirmedRegistrations(ctx, workshopsdb.CountConfirmedRegistrationsParams{
			TenantID:   w.TenantID,
			WorkshopID: w.ID,
		})
		if err != nil {
			return nil, err
		}

		activeCount, err := q.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
			TenantID:   w.TenantID,
			WorkshopID: w.ID,
		})
		if err != nil {
			return nil, err
		}

		pendingCount := activeCount - confirmedCount
		if pendingCount < 0 {
			pendingCount = 0
		}

		res = append(res, AdminWorkshopItem{
			ID:             fromPgUUID(w.ID),
			Title:          w.Title,
			Topic:          w.Topic,
			Slug:           w.Slug,
			InstructorName: w.InstructorName,
			StartsAt:       w.StartsAt.Time,
			EndsAt:         w.EndsAt.Time,
			Capacity:       int(w.Capacity),
			ConfirmedCount: confirmedCount,
			PendingCount:   pendingCount,
			PriceRials:     w.PriceRials,
			Status:         w.Status,
			CreatedAt:      w.CreatedAt.Time,
		})
	}

	return res, nil
}

func (s *Service) GetAdminWorkshopDetail(ctx context.Context, tenantID, id uuid.UUID) (*AdminWorkshopDetail, error) {
	return db.ReadTenant(ctx, s.catalogStore, tenantID, s.queries,
		func(tx pgx.Tx) Querier { return workshopsdb.New(tx) },
		func(q Querier) (*AdminWorkshopDetail, error) { return s.getAdminWorkshopDetail(ctx, q, tenantID, id) },
	)
}

func (s *Service) getAdminWorkshopDetail(ctx context.Context, q Querier, tenantID, id uuid.UUID) (*AdminWorkshopDetail, error) {
	w, err := q.GetWorkshopByID(ctx, workshopsdb.GetWorkshopByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	confirmedCount, err := q.CountConfirmedRegistrations(ctx, workshopsdb.CountConfirmedRegistrationsParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(id),
	})
	if err != nil {
		return nil, err
	}

	activeCount, err := q.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(id),
	})
	if err != nil {
		return nil, err
	}

	pendingCount := activeCount - confirmedCount
	if pendingCount < 0 {
		pendingCount = 0
	}

	return &AdminWorkshopDetail{
		AdminWorkshopItem: AdminWorkshopItem{
			ID:             fromPgUUID(w.ID),
			Title:          w.Title,
			Topic:          w.Topic,
			Slug:           w.Slug,
			InstructorName: w.InstructorName,
			StartsAt:       w.StartsAt.Time,
			EndsAt:         w.EndsAt.Time,
			Capacity:       int(w.Capacity),
			ConfirmedCount: confirmedCount,
			PendingCount:   pendingCount,
			PriceRials:     w.PriceRials,
			Status:         w.Status,
			CreatedAt:      w.CreatedAt.Time,
		},
		Description:          w.Description,
		Location:             w.Location,
		RegistrationDeadline: w.RegistrationDeadline.Time,
		UpdatedAt:            w.UpdatedAt.Time,
	}, nil
}

func (s *Service) UpdateWorkshop(ctx context.Context, tenantID, id uuid.UUID, p UpdateWorkshopParams) (*AdminWorkshopDetail, error) {
	if s.catalogStore == nil {
		return s.updateWorkshop(ctx, s.queries, nil, tenantID, id, p)
	}
	return education.WriteCatalog(ctx, s.catalogStore, tenantID, func(tx pgx.Tx) (*AdminWorkshopDetail, error) {
		return s.updateWorkshop(ctx, workshopsdb.New(tx), tx, tenantID, id, p)
	})
}

func (s *Service) updateWorkshop(ctx context.Context, q Querier, tx pgx.Tx, tenantID, id uuid.UUID, p UpdateWorkshopParams) (*AdminWorkshopDetail, error) {
	var topic pgtype.Text
	if p.Topic != nil {
		value, err := education.NormalizeTopic(*p.Topic)
		if err != nil {
			return nil, err
		}
		topic = pgtype.Text{String: value, Valid: true}
	}

	// 1. Lock workshop row for update
	current, err := q.GetWorkshopByIDForUpdate(ctx, workshopsdb.GetWorkshopByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	// 2. Count confirmed registrations
	confirmedCount, err := q.CountConfirmedRegistrations(ctx, workshopsdb.CountConfirmedRegistrationsParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(id),
	})
	if err != nil {
		return nil, err
	}

	// If confirmed registrations exist: price, dates, or location CANNOT be changed
	if confirmedCount > 0 {
		if p.PriceRials != nil && *p.PriceRials != current.PriceRials {
			return nil, ErrCannotModifyActiveWorkshop
		}
		if p.StartsAt != nil && !p.StartsAt.Equal(current.StartsAt.Time) {
			return nil, ErrCannotModifyActiveWorkshop
		}
		if p.EndsAt != nil && !p.EndsAt.Equal(current.EndsAt.Time) {
			return nil, ErrCannotModifyActiveWorkshop
		}
		if p.Location != nil && *p.Location != current.Location {
			return nil, ErrCannotModifyActiveWorkshop
		}
	}

	// Calculate target values
	targetTitle := current.Title
	if p.Title != nil {
		targetTitle = *p.Title
	}
	targetSlug := current.Slug
	if p.Slug != nil {
		targetSlug = *p.Slug
	}
	targetDesc := current.Description
	if p.Description != nil {
		targetDesc = *p.Description
	}
	targetInstructor := current.InstructorName
	if p.InstructorName != nil {
		targetInstructor = *p.InstructorName
	}
	targetLocation := current.Location
	if p.Location != nil {
		targetLocation = *p.Location
	}
	targetStarts := current.StartsAt.Time
	if p.StartsAt != nil {
		targetStarts = *p.StartsAt
	}
	targetEnds := current.EndsAt.Time
	if p.EndsAt != nil {
		targetEnds = *p.EndsAt
	}
	targetDeadline := current.RegistrationDeadline.Time
	if p.RegistrationDeadline != nil {
		targetDeadline = *p.RegistrationDeadline
	}
	targetCapacity := current.Capacity
	if p.Capacity != nil {
		targetCapacity = int32(*p.Capacity)
	}
	targetPrice := current.PriceRials
	if p.PriceRials != nil {
		targetPrice = *p.PriceRials
	}

	// Validate date ordering
	if !targetEnds.After(targetStarts) || targetDeadline.After(targetStarts) {
		return nil, ErrInvalidDates
	}

	// Validate capacity against active registrations
	activeCount, err := q.CountActiveRegistrations(ctx, workshopsdb.CountActiveRegistrationsParams{
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(id),
	})
	if err != nil {
		return nil, err
	}
	if int64(targetCapacity) < activeCount {
		return nil, ErrCannotReduceCapacityBelowActive
	}

	_, err = q.UpdateWorkshop(ctx, workshopsdb.UpdateWorkshopParams{
		TenantID:             toPgUUID(tenantID),
		ID:                   toPgUUID(id),
		Title:                targetTitle,
		Slug:                 targetSlug,
		Description:          targetDesc,
		InstructorName:       targetInstructor,
		StartsAt:             toPgTimestamptz(targetStarts),
		EndsAt:               toPgTimestamptz(targetEnds),
		Location:             targetLocation,
		RegistrationDeadline: toPgTimestamptz(targetDeadline),
		Capacity:             targetCapacity,
		PriceRials:           targetPrice,
		Topic:                topic,
	})
	if err != nil {
		return nil, err
	}

	if tx != nil && current.Status == "published" {
		if err := education.InvalidateCatalog(ctx, tx, tenantID, "/", "/courses", "/workshops", "/workshops/"+id.String()); err != nil {
			return nil, err
		}
	}
	return s.getAdminWorkshopDetail(ctx, q, tenantID, id)
}

func (s *Service) PublishWorkshop(ctx context.Context, tenantID, id uuid.UUID) (*AdminWorkshopDetail, error) {
	_, err := s.queries.GetWorkshopByIDForUpdate(ctx, workshopsdb.GetWorkshopByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	_, err = s.queries.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
		Status:   "published",
	})
	if err != nil {
		return nil, err
	}

	return s.GetAdminWorkshopDetail(ctx, tenantID, id)
}

func (s *Service) CloseWorkshop(ctx context.Context, tenantID, id uuid.UUID) (*AdminWorkshopDetail, error) {
	_, err := s.queries.GetWorkshopByIDForUpdate(ctx, workshopsdb.GetWorkshopByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	_, err = s.queries.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
		Status:   "closed",
	})
	if err != nil {
		return nil, err
	}

	return s.GetAdminWorkshopDetail(ctx, tenantID, id)
}

func (s *Service) CancelWorkshop(ctx context.Context, tenantID, id uuid.UUID) (*AdminWorkshopDetail, error) {
	_, err := s.queries.GetWorkshopByIDForUpdate(ctx, workshopsdb.GetWorkshopByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	_, err = s.queries.UpdateWorkshopStatus(ctx, workshopsdb.UpdateWorkshopStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(id),
		Status:   "cancelled",
	})
	if err != nil {
		return nil, err
	}

	return s.GetAdminWorkshopDetail(ctx, tenantID, id)
}
