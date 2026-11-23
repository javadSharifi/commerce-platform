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
	"psychology/api/internal/booking"
	"psychology/api/internal/booking/bookingdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type pilotPhase2Fixture struct {
	tenantA uuid.UUID // کلینیک امید (omid-psych.ir)
	tenantB uuid.UUID // مرکز مشاوره ماهان (mahan-clinic.ir)

	hostA string
	hostB string

	tokenOwnerA        string
	tokenReceptionistA string
	tokenTherapist1A   string
	tokenTherapist2A   string

	tokenOwnerB        string
	tokenReceptionistB string
	tokenTherapistB    string

	userTherapist1A uuid.UUID
	userTherapist2A uuid.UUID
	userTherapistB  uuid.UUID

	therapist1AID uuid.UUID
	therapist2AID uuid.UUID
	therapistBID  uuid.UUID

	router http.Handler

	bookingStore *pilotBookingStore
	bookingSvc   *booking.Service
}

type pilotBookingStore struct {
	mu            sync.Mutex
	therapists    map[string]bookingdb.Therapist
	workingHours  map[string]bookingdb.TherapistWorkingHour
	exceptions    map[string]bookingdb.TherapistScheduleException
	slots         map[string]bookingdb.AppointmentSlot
	appointments  map[string]bookingdb.Appointment
	statusHistory map[string][]bookingdb.AppointmentStatusHistory
}

func toPgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func newPilotBookingStore() *pilotBookingStore {
	return &pilotBookingStore{
		therapists:    make(map[string]bookingdb.Therapist),
		workingHours:  make(map[string]bookingdb.TherapistWorkingHour),
		exceptions:    make(map[string]bookingdb.TherapistScheduleException),
		slots:         make(map[string]bookingdb.AppointmentSlot),
		appointments:  make(map[string]bookingdb.Appointment),
		statusHistory: make(map[string][]bookingdb.AppointmentStatusHistory),
	}
}

func (m *pilotBookingStore) CreateTherapist(ctx context.Context, arg bookingdb.CreateTherapistParams) (bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	now := time.Now().UTC()
	th := bookingdb.Therapist{
		ID:                     toPgUUID(id),
		TenantID:               arg.TenantID,
		UserID:                 arg.UserID,
		Name:                   arg.Name,
		Slug:                   arg.Slug,
		Title:                  arg.Title,
		Bio:                    arg.Bio,
		AvatarAssetID:          arg.AvatarAssetID,
		SessionDurationMinutes: arg.SessionDurationMinutes,
		IsActive:               arg.IsActive,
		CreatedAt:              toPgTimestamptz(now),
		UpdatedAt:              toPgTimestamptz(now),
	}
	m.therapists[id.String()] = th
	return th, nil
}

func (m *pilotBookingStore) GetTherapistByID(ctx context.Context, arg bookingdb.GetTherapistByIDParams) (bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tenantID := fromPgUUID(arg.TenantID)
	th, ok := m.therapists[id]
	if !ok || fromPgUUID(th.TenantID) != tenantID || th.DeletedAt.Valid {
		return bookingdb.Therapist{}, fmt.Errorf("therapist not found")
	}
	return th, nil
}

func (m *pilotBookingStore) GetTherapistByUserID(ctx context.Context, arg bookingdb.GetTherapistByUserIDParams) (bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantID := fromPgUUID(arg.TenantID)
	userID := fromPgUUID(arg.UserID)
	for _, th := range m.therapists {
		if fromPgUUID(th.TenantID) == tenantID && th.UserID.Valid && fromPgUUID(th.UserID) == userID && !th.DeletedAt.Valid {
			return th, nil
		}
	}
	return bookingdb.Therapist{}, fmt.Errorf("therapist not found for user")
}

func (m *pilotBookingStore) GetTherapistBySlug(ctx context.Context, arg bookingdb.GetTherapistBySlugParams) (bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantID := fromPgUUID(arg.TenantID)
	for _, th := range m.therapists {
		if fromPgUUID(th.TenantID) == tenantID && th.Slug == arg.Slug && !th.DeletedAt.Valid {
			return th, nil
		}
	}
	return bookingdb.Therapist{}, fmt.Errorf("therapist not found by slug")
}

func (m *pilotBookingStore) ListTherapists(ctx context.Context, tenantID pgtype.UUID) ([]bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(tenantID)
	var list []bookingdb.Therapist
	for _, th := range m.therapists {
		if fromPgUUID(th.TenantID) == tID && !th.DeletedAt.Valid {
			list = append(list, th)
		}
	}
	return list, nil
}

func (m *pilotBookingStore) ListActiveTherapists(ctx context.Context, tenantID pgtype.UUID) ([]bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(tenantID)
	var list []bookingdb.Therapist
	for _, th := range m.therapists {
		if fromPgUUID(th.TenantID) == tID && th.IsActive && !th.DeletedAt.Valid {
			list = append(list, th)
		}
	}
	return list, nil
}

func (m *pilotBookingStore) UpdateTherapist(ctx context.Context, arg bookingdb.UpdateTherapistParams) (bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	th, ok := m.therapists[id]
	if !ok || fromPgUUID(th.TenantID) != tID || th.DeletedAt.Valid {
		return bookingdb.Therapist{}, fmt.Errorf("not found")
	}
	th.UserID = arg.UserID
	th.Name = arg.Name
	th.Slug = arg.Slug
	th.Title = arg.Title
	th.Bio = arg.Bio
	th.AvatarAssetID = arg.AvatarAssetID
	th.SessionDurationMinutes = arg.SessionDurationMinutes
	th.IsActive = arg.IsActive
	th.UpdatedAt = toPgTimestamptz(time.Now())
	m.therapists[id] = th
	return th, nil
}

func (m *pilotBookingStore) SoftDeleteTherapist(ctx context.Context, arg bookingdb.SoftDeleteTherapistParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	th, ok := m.therapists[id]
	if !ok || fromPgUUID(th.TenantID) != tID || th.DeletedAt.Valid {
		return fmt.Errorf("not found")
	}
	th.DeletedAt = toPgTimestamptz(time.Now())
	m.therapists[id] = th
	return nil
}

func (m *pilotBookingStore) ListWorkingHoursByTherapist(ctx context.Context, arg bookingdb.ListWorkingHoursByTherapistParams) ([]bookingdb.TherapistWorkingHour, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	thID := fromPgUUID(arg.TherapistID)
	var list []bookingdb.TherapistWorkingHour
	for _, wh := range m.workingHours {
		if fromPgUUID(wh.TenantID) == tID && fromPgUUID(wh.TherapistID) == thID {
			list = append(list, wh)
		}
	}
	return list, nil
}

func (m *pilotBookingStore) DeleteWorkingHoursByTherapist(ctx context.Context, arg bookingdb.DeleteWorkingHoursByTherapistParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	thID := fromPgUUID(arg.TherapistID)
	for k, wh := range m.workingHours {
		if fromPgUUID(wh.TenantID) == tID && fromPgUUID(wh.TherapistID) == thID {
			delete(m.workingHours, k)
		}
	}
	return nil
}

func (m *pilotBookingStore) CreateWorkingHour(ctx context.Context, arg bookingdb.CreateWorkingHourParams) (bookingdb.TherapistWorkingHour, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	wh := bookingdb.TherapistWorkingHour{
		ID:          toPgUUID(id),
		TenantID:    arg.TenantID,
		TherapistID: arg.TherapistID,
		DayOfWeek:   arg.DayOfWeek,
		StartTime:   arg.StartTime,
		EndTime:     arg.EndTime,
		IsActive:    arg.IsActive,
		CreatedAt:   toPgTimestamptz(time.Now()),
		UpdatedAt:   toPgTimestamptz(time.Now()),
	}
	m.workingHours[id.String()] = wh
	return wh, nil
}

func (m *pilotBookingStore) ListScheduleExceptionsByTherapist(ctx context.Context, arg bookingdb.ListScheduleExceptionsByTherapistParams) ([]bookingdb.TherapistScheduleException, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	thID := fromPgUUID(arg.TherapistID)
	var list []bookingdb.TherapistScheduleException
	for _, ex := range m.exceptions {
		if fromPgUUID(ex.TenantID) == tID && fromPgUUID(ex.TherapistID) == thID {
			list = append(list, ex)
		}
	}
	return list, nil
}

func (m *pilotBookingStore) GetScheduleExceptionByID(ctx context.Context, arg bookingdb.GetScheduleExceptionByIDParams) (bookingdb.TherapistScheduleException, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	ex, ok := m.exceptions[id]
	if !ok || fromPgUUID(ex.TenantID) != tID {
		return bookingdb.TherapistScheduleException{}, fmt.Errorf("not found")
	}
	return ex, nil
}

func (m *pilotBookingStore) CreateScheduleException(ctx context.Context, arg bookingdb.CreateScheduleExceptionParams) (bookingdb.TherapistScheduleException, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	ex := bookingdb.TherapistScheduleException{
		ID:            toPgUUID(id),
		TenantID:      arg.TenantID,
		TherapistID:   arg.TherapistID,
		Date:          arg.Date,
		IsUnavailable: arg.IsUnavailable,
		StartTime:     arg.StartTime,
		EndTime:       arg.EndTime,
		Reason:        arg.Reason,
		CreatedAt:     toPgTimestamptz(time.Now()),
	}
	m.exceptions[id.String()] = ex
	return ex, nil
}

func (m *pilotBookingStore) DeleteScheduleException(ctx context.Context, arg bookingdb.DeleteScheduleExceptionParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	ex, ok := m.exceptions[id]
	if !ok || fromPgUUID(ex.TenantID) != tID {
		return fmt.Errorf("not found")
	}
	delete(m.exceptions, id)
	return nil
}

func (m *pilotBookingStore) CreateAppointmentSlot(ctx context.Context, arg bookingdb.CreateAppointmentSlotParams) (bookingdb.AppointmentSlot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	sl := bookingdb.AppointmentSlot{
		ID:          toPgUUID(id),
		TenantID:    arg.TenantID,
		TherapistID: arg.TherapistID,
		StartsAt:    arg.StartsAt,
		EndsAt:      arg.EndsAt,
		Status:      arg.Status,
		CreatedAt:   toPgTimestamptz(time.Now()),
		UpdatedAt:   toPgTimestamptz(time.Now()),
	}
	m.slots[id.String()] = sl
	return sl, nil
}

func (m *pilotBookingStore) GetAppointmentSlotByID(ctx context.Context, arg bookingdb.GetAppointmentSlotByIDParams) (bookingdb.AppointmentSlot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	sl, ok := m.slots[id]
	if !ok || fromPgUUID(sl.TenantID) != tID {
		return bookingdb.AppointmentSlot{}, fmt.Errorf("not found")
	}
	return sl, nil
}

func (m *pilotBookingStore) GetAppointmentSlotByIDForUpdate(ctx context.Context, arg bookingdb.GetAppointmentSlotByIDForUpdateParams) (bookingdb.AppointmentSlot, error) {
	return m.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams(arg))
}

func (m *pilotBookingStore) ListSlotsByTherapist(ctx context.Context, arg bookingdb.ListSlotsByTherapistParams) ([]bookingdb.AppointmentSlot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	thID := fromPgUUID(arg.TherapistID)
	var list []bookingdb.AppointmentSlot
	for _, sl := range m.slots {
		if fromPgUUID(sl.TenantID) == tID && fromPgUUID(sl.TherapistID) == thID {
			if !sl.StartsAt.Time.Before(arg.StartsAt.Time) && !sl.EndsAt.Time.After(arg.EndsAt.Time) {
				list = append(list, sl)
			}
		}
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].StartsAt.Time.Before(list[i].StartsAt.Time) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list, nil
}

func (m *pilotBookingStore) ListAvailableSlots(ctx context.Context, arg bookingdb.ListAvailableSlotsParams) ([]bookingdb.AppointmentSlot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	thID := fromPgUUID(arg.TherapistID)
	var list []bookingdb.AppointmentSlot
	for _, sl := range m.slots {
		if fromPgUUID(sl.TenantID) == tID && fromPgUUID(sl.TherapistID) == thID {
			if sl.Status == "available" && !sl.StartsAt.Time.Before(arg.StartsAt.Time) && !sl.StartsAt.Time.After(arg.StartsAt_2.Time) {
				list = append(list, sl)
			}
		}
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].StartsAt.Time.Before(list[i].StartsAt.Time) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list, nil
}

func (m *pilotBookingStore) DeleteFutureAvailableSlots(ctx context.Context, arg bookingdb.DeleteFutureAvailableSlotsParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	thID := fromPgUUID(arg.TherapistID)
	for k, sl := range m.slots {
		if fromPgUUID(sl.TenantID) == tID && fromPgUUID(sl.TherapistID) == thID {
			if sl.Status == "available" && !sl.StartsAt.Time.Before(arg.StartsAt.Time) {
				delete(m.slots, k)
			}
		}
	}
	return nil
}

func (m *pilotBookingStore) UpdateSlotStatus(ctx context.Context, arg bookingdb.UpdateSlotStatusParams) (bookingdb.AppointmentSlot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	sl, ok := m.slots[id]
	if !ok || fromPgUUID(sl.TenantID) != tID {
		return bookingdb.AppointmentSlot{}, fmt.Errorf("not found")
	}
	sl.Status = arg.Status
	sl.UpdatedAt = toPgTimestamptz(time.Now())
	m.slots[id] = sl
	return sl, nil
}

func (m *pilotBookingStore) CreateAppointment(ctx context.Context, arg bookingdb.CreateAppointmentParams) (bookingdb.Appointment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	app := bookingdb.Appointment{
		ID:              toPgUUID(id),
		TenantID:        arg.TenantID,
		TherapistID:     arg.TherapistID,
		SlotID:          arg.SlotID,
		Status:          arg.Status,
		GuestName:       arg.GuestName,
		GuestPhone:      arg.GuestPhone,
		GuestLookupHash: arg.GuestLookupHash,
		Notes:           arg.Notes,
		ExpiresAt:       arg.ExpiresAt,
		CreatedAt:       toPgTimestamptz(time.Now()),
		UpdatedAt:       toPgTimestamptz(time.Now()),
	}
	m.appointments[id.String()] = app
	return app, nil
}

func (m *pilotBookingStore) GetAppointmentByID(ctx context.Context, arg bookingdb.GetAppointmentByIDParams) (bookingdb.Appointment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	app, ok := m.appointments[id]
	if !ok || fromPgUUID(app.TenantID) != tID {
		return bookingdb.Appointment{}, pgx.ErrNoRows
	}
	return app, nil
}

func (m *pilotBookingStore) GetAppointmentByIDForUpdate(ctx context.Context, arg bookingdb.GetAppointmentByIDForUpdateParams) (bookingdb.Appointment, error) {
	return m.GetAppointmentByID(ctx, bookingdb.GetAppointmentByIDParams(arg))
}

func (m *pilotBookingStore) GetAppointmentBySlotID(ctx context.Context, arg bookingdb.GetAppointmentBySlotIDParams) (bookingdb.Appointment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	slotID := fromPgUUID(arg.SlotID)
	for _, app := range m.appointments {
		if fromPgUUID(app.TenantID) == tID && fromPgUUID(app.SlotID) == slotID {
			if app.Status == "pending" || app.Status == "confirmed" || app.Status == "completed" {
				return app, nil
			}
		}
	}
	return bookingdb.Appointment{}, fmt.Errorf("active appointment not found")
}

func (m *pilotBookingStore) UpdateAppointmentStatus(ctx context.Context, arg bookingdb.UpdateAppointmentStatusParams) (bookingdb.Appointment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	app, ok := m.appointments[id]
	if !ok || fromPgUUID(app.TenantID) != tID {
		return bookingdb.Appointment{}, fmt.Errorf("appointment not found")
	}
	app.Status = arg.Status
	app.UpdatedAt = toPgTimestamptz(time.Now())
	if arg.Status == "confirmed" {
		app.ConfirmedAt = toPgTimestamptz(time.Now())
	} else if arg.Status == "cancelled" {
		app.CancelledAt = toPgTimestamptz(time.Now())
		app.CancellationReason = arg.CancellationReason
	}
	m.appointments[id] = app
	return app, nil
}

func (m *pilotBookingStore) ListAppointmentsByTherapist(ctx context.Context, arg bookingdb.ListAppointmentsByTherapistParams) ([]bookingdb.Appointment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	thID := fromPgUUID(arg.TherapistID)
	var list []bookingdb.Appointment
	for _, app := range m.appointments {
		if fromPgUUID(app.TenantID) == tID && fromPgUUID(app.TherapistID) == thID {
			list = append(list, app)
		}
	}
	return list, nil
}

func (m *pilotBookingStore) ListExpiredPendingAppointments(ctx context.Context, arg bookingdb.ListExpiredPendingAppointmentsParams) ([]bookingdb.Appointment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	now := arg.ExpiresAt.Time
	var list []bookingdb.Appointment
	for _, app := range m.appointments {
		if fromPgUUID(app.TenantID) == tID && app.Status == "pending" {
			if !app.ExpiresAt.Time.After(now) {
				list = append(list, app)
			}
		}
	}
	return list, nil
}

func (m *pilotBookingStore) CreateAppointmentStatusHistory(ctx context.Context, arg bookingdb.CreateAppointmentStatusHistoryParams) (bookingdb.AppointmentStatusHistory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	h := bookingdb.AppointmentStatusHistory{
		ID:            toPgUUID(id),
		TenantID:      arg.TenantID,
		AppointmentID: arg.AppointmentID,
		FromStatus:    arg.FromStatus,
		ToStatus:      arg.ToStatus,
		Action:        arg.Action,
		ActorType:     arg.ActorType,
		ActorID:       arg.ActorID,
		Reason:        arg.Reason,
		Metadata:      arg.Metadata,
		CreatedAt:     toPgTimestamptz(time.Now()),
	}
	appID := fromPgUUID(arg.AppointmentID).String()
	m.statusHistory[appID] = append(m.statusHistory[appID], h)
	return h, nil
}

func (m *pilotBookingStore) ListAppointmentHistory(ctx context.Context, arg bookingdb.ListAppointmentHistoryParams) ([]bookingdb.AppointmentStatusHistory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	appID := fromPgUUID(arg.AppointmentID).String()
	return m.statusHistory[appID], nil
}

func (m *pilotBookingStore) GetAppointmentDetail(ctx context.Context, arg bookingdb.GetAppointmentDetailParams) (bookingdb.GetAppointmentDetailRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	app, ok := m.appointments[id]
	if !ok || fromPgUUID(app.TenantID) != tID {
		return bookingdb.GetAppointmentDetailRow{}, fmt.Errorf("appointment not found")
	}
	th := m.therapists[fromPgUUID(app.TherapistID).String()]
	slot := m.slots[fromPgUUID(app.SlotID).String()]
	return bookingdb.GetAppointmentDetailRow{
		ID:                 app.ID,
		TenantID:           app.TenantID,
		TherapistID:        app.TherapistID,
		SlotID:             app.SlotID,
		Status:             app.Status,
		GuestName:          app.GuestName,
		GuestPhone:         app.GuestPhone,
		GuestLookupHash:    app.GuestLookupHash,
		Notes:              app.Notes,
		ExpiresAt:          app.ExpiresAt,
		ConfirmedAt:        app.ConfirmedAt,
		CancelledAt:        app.CancelledAt,
		CancellationReason: app.CancellationReason,
		CreatedAt:          app.CreatedAt,
		UpdatedAt:          app.UpdatedAt,
		TherapistName:      th.Name,
		StartsAt:           slot.StartsAt,
		EndsAt:             slot.EndsAt,
	}, nil
}

func (m *pilotBookingStore) ListAdminAppointments(ctx context.Context, arg bookingdb.ListAdminAppointmentsParams) ([]bookingdb.ListAdminAppointmentsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tID := fromPgUUID(arg.TenantID)
	var list []bookingdb.ListAdminAppointmentsRow
	for _, app := range m.appointments {
		if fromPgUUID(app.TenantID) != tID {
			continue
		}
		if arg.TherapistID.Valid && fromPgUUID(app.TherapistID) != fromPgUUID(arg.TherapistID) {
			continue
		}
		if arg.Status.Valid && app.Status != arg.Status.String {
			continue
		}
		th := m.therapists[fromPgUUID(app.TherapistID).String()]
		slot := m.slots[fromPgUUID(app.SlotID).String()]
		list = append(list, bookingdb.ListAdminAppointmentsRow{
			ID:                 app.ID,
			TenantID:           app.TenantID,
			TherapistID:        app.TherapistID,
			SlotID:             app.SlotID,
			Status:             app.Status,
			GuestName:          app.GuestName,
			GuestPhone:         app.GuestPhone,
			GuestLookupHash:    app.GuestLookupHash,
			Notes:              app.Notes,
			ExpiresAt:          app.ExpiresAt,
			ConfirmedAt:        app.ConfirmedAt,
			CancelledAt:        app.CancelledAt,
			CancellationReason: app.CancellationReason,
			CreatedAt:          app.CreatedAt,
			UpdatedAt:          app.UpdatedAt,
			TherapistName:      th.Name,
			StartsAt:           slot.StartsAt,
			EndsAt:             slot.EndsAt,
		})
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].CreatedAt.Time.After(list[i].CreatedAt.Time) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list, nil
}

func (m *pilotBookingStore) UpdateAppointmentSlot(ctx context.Context, arg bookingdb.UpdateAppointmentSlotParams) (bookingdb.Appointment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)
	app, ok := m.appointments[id]
	if !ok || fromPgUUID(app.TenantID) != tID {
		return bookingdb.Appointment{}, fmt.Errorf("appointment not found")
	}
	app.SlotID = arg.SlotID
	app.UpdatedAt = toPgTimestamptz(time.Now())
	m.appointments[id] = app
	return app, nil
}

// setupPilotPhase2Fixture wires up two independent clinics (امید & ماهان)
func setupPilotPhase2Fixture(t *testing.T) *pilotPhase2Fixture {
	tenantA := uuid.New()
	tenantB := uuid.New()
	hostA := "omid-psych.ir"
	hostB := "mahan-clinic.ir"

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

	// Tenant A Staff
	ownerAUser, _ := authStore.CreateUser("owner@omid.ir", "omidPass123", "دکتر امیدی (مالک)")
	authStore.AddMembership(ownerAUser.ID, tenantA, "owner")
	tokenOwnerA, _ := authStore.CreateSession(ownerAUser.ID)

	recAUser, _ := authStore.CreateUser("rec@omid.ir", "omidPass123", "منشی کلینیک امید")
	authStore.AddMembership(recAUser.ID, tenantA, "receptionist")
	tokenRecA, _ := authStore.CreateSession(recAUser.ID)

	th1AUser, _ := authStore.CreateUser("th1@omid.ir", "omidPass123", "دکتر مریم علوی")
	authStore.AddMembership(th1AUser.ID, tenantA, "therapist")
	tokenTh1A, _ := authStore.CreateSession(th1AUser.ID)

	th2AUser, _ := authStore.CreateUser("th2@omid.ir", "omidPass123", "دکتر کامران راد")
	authStore.AddMembership(th2AUser.ID, tenantA, "therapist")
	tokenTh2A, _ := authStore.CreateSession(th2AUser.ID)

	// Tenant B Staff
	ownerBUser, _ := authStore.CreateUser("owner@mahan.ir", "mahanPass123", "دکتر ماهانی (مالک)")
	authStore.AddMembership(ownerBUser.ID, tenantB, "owner")
	tokenOwnerB, _ := authStore.CreateSession(ownerBUser.ID)

	recBUser, _ := authStore.CreateUser("rec@mahan.ir", "mahanPass123", "منشی مرکز ماهان")
	authStore.AddMembership(recBUser.ID, tenantB, "receptionist")
	tokenRecB, _ := authStore.CreateSession(recBUser.ID)

	thBUser, _ := authStore.CreateUser("th@mahan.ir", "mahanPass123", "دکتر رضا مرادی")
	authStore.AddMembership(thBUser.ID, tenantB, "therapist")
	tokenThB, _ := authStore.CreateSession(thBUser.ID)

	bookingStore := newPilotBookingStore()
	domainResolver := domain.NewResolver(domainStore)
	authService := auth.NewService(authStore)
	bookingService := booking.NewService(bookingStore)

	// Create Therapists in Store
	u1 := fromPgUUID(th1AUser.ID)
	th1A, _ := bookingStore.CreateTherapist(context.Background(), bookingdb.CreateTherapistParams{
		TenantID:               toPgUUID(tenantA),
		UserID:                 toPgUUID(u1),
		Name:                   "دکتر مریم علوی",
		Slug:                   "dr-maryam-alavi",
		Title:                  "روان‌درمانگر تحلیلی",
		Bio:                    "متخصص بالینی",
		SessionDurationMinutes: 50,
		IsActive:               true,
	})

	u2 := fromPgUUID(th2AUser.ID)
	th2A, _ := bookingStore.CreateTherapist(context.Background(), bookingdb.CreateTherapistParams{
		TenantID:               toPgUUID(tenantA),
		UserID:                 toPgUUID(u2),
		Name:                   "دکتر کامران راد",
		Slug:                   "dr-kamran-rad",
		Title:                  "روان‌شناس شناختی",
		Bio:                    "متخصص CBT",
		SessionDurationMinutes: 45,
		IsActive:               true,
	})

	ub := fromPgUUID(thBUser.ID)
	thB, _ := bookingStore.CreateTherapist(context.Background(), bookingdb.CreateTherapistParams{
		TenantID:               toPgUUID(tenantB),
		UserID:                 toPgUUID(ub),
		Name:                   "دکتر رضا مرادی",
		Slug:                   "dr-reza-moradi",
		Title:                  "روان‌شناس سلامت",
		Bio:                    "مشاوره فردی",
		SessionDurationMinutes: 50,
		IsActive:               true,
	})

	bookingHandler := booking.NewHandler(bookingService, domainResolver, authService)
	authHandler := auth.NewHandler(authService)

	r := chi.NewRouter()
	authHandler.MountRoutes(r)
	bookingHandler.MountRoutes(r)

	return &pilotPhase2Fixture{
		tenantA:            tenantA,
		tenantB:            tenantB,
		hostA:              hostA,
		hostB:              hostB,
		tokenOwnerA:        tokenOwnerA,
		tokenReceptionistA: tokenRecA,
		tokenTherapist1A:   tokenTh1A,
		tokenTherapist2A:   tokenTh2A,
		tokenOwnerB:        tokenOwnerB,
		tokenReceptionistB: tokenRecB,
		tokenTherapistB:    tokenThB,
		userTherapist1A:    u1,
		userTherapist2A:    u2,
		userTherapistB:     ub,
		therapist1AID:      fromPgUUID(th1A.ID),
		therapist2AID:      fromPgUUID(th2A.ID),
		therapistBID:       fromPgUUID(thB.ID),
		router:             r,
		bookingStore:       bookingStore,
		bookingSvc:         bookingService,
	}
}

// ------------------------------------------------------------------------------------------------
// Acceptance Scenarios (PSY-A07)
// ------------------------------------------------------------------------------------------------

func TestAcceptance_PSY_A07_TwoIndependentCalendarsAndDomainIsolation(t *testing.T) {
	f := setupPilotPhase2Fixture(t)

	// 1. Query therapists on Tenant A (omid-psych.ir)
	reqA := httptest.NewRequest(http.MethodGet, "/public/therapists", nil)
	reqA.Host = f.hostA
	recA := httptest.NewRecorder()
	f.router.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200 on hostA therapists, got %d", recA.Code)
	}

	var thListA []map[string]any
	_ = json.Unmarshal(recA.Body.Bytes(), &thListA)
	if len(thListA) != 2 {
		t.Fatalf("expected 2 therapists in Tenant A, got %d", len(thListA))
	}
	foundMaryam := false
	foundKamran := false
	for _, th := range thListA {
		if th["name"] == "دکتر مریم علوی" {
			foundMaryam = true
		}
		if th["name"] == "دکتر کامران راد" {
			foundKamran = true
		}
		if th["name"] == "دکتر رضا مرادی" {
			t.Fatalf("Tenant B therapist leaked into Tenant A list")
		}
	}
	if !foundMaryam || !foundKamran {
		t.Fatalf("expected Dr. Maryam and Dr. Kamran in Tenant A")
	}

	// 2. Query therapists on Tenant B (mahan-clinic.ir)
	reqB := httptest.NewRequest(http.MethodGet, "/public/therapists", nil)
	reqB.Host = f.hostB
	recB := httptest.NewRecorder()
	f.router.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200 on hostB therapists, got %d", recB.Code)
	}

	var thListB []map[string]any
	_ = json.Unmarshal(recB.Body.Bytes(), &thListB)
	if len(thListB) != 1 {
		t.Fatalf("expected 1 therapist in Tenant B, got %d", len(thListB))
	}
	if thListB[0]["name"] != "دکتر رضا مرادی" {
		t.Fatalf("expected Dr. Reza in Tenant B, got %v", thListB[0]["name"])
	}

	// 3. Cross-Tenant slot query by ID: Tenant A therapist ID requested via Tenant B domain
	reqCross := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/therapists/%s/slots", f.therapist1AID), nil)
	reqCross.Host = f.hostB
	recCross := httptest.NewRecorder()
	f.router.ServeHTTP(recCross, reqCross)
	var crossSlots []map[string]any
	_ = json.Unmarshal(recCross.Body.Bytes(), &crossSlots)
	if len(crossSlots) != 0 {
		t.Fatalf("expected 0 slots when querying Tenant A therapist via Tenant B domain, got %d", len(crossSlots))
	}
}

func TestAcceptance_PSY_A07_ConcurrentSlotContention_ZeroOversell(t *testing.T) {
	f := setupPilotPhase2Fixture(t)

	// Create a single future slot for Dr. Maryam
	slotTime := time.Now().UTC().Add(48 * time.Hour)
	slot, err := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist1AID),
		StartsAt:    toPgTimestamptz(slotTime),
		EndsAt:      toPgTimestamptz(slotTime.Add(50 * time.Minute)),
		Status:      "available",
	})
	if err != nil {
		t.Fatalf("failed to insert slot: %v", err)
	}
	slotID := fromPgUUID(slot.ID).String()

	// 10 concurrent guest requests racing for this single slot
	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	var successCount int64
	var conflictCount int64

	for i := 0; i < concurrency; i++ {
		go func(guestIdx int) {
			defer wg.Done()
			payload := map[string]any{
				"therapist_id": f.therapist1AID.String(),
				"slot_id":      slotID,
				"guest_name":   fmt.Sprintf("متقاضی شماره %d", guestIdx),
				"guest_phone":  fmt.Sprintf("0912345%04d", guestIdx),
				"notes":        "رزرو تست هم‌زمانی",
			}
			bodyBytes, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/public/appointments", bytes.NewReader(bodyBytes))
			req.Host = f.hostA
			req.RemoteAddr = fmt.Sprintf("192.168.1.%d:1234", guestIdx+1) // Distinct IPs to avoid rate limiter

			rec := httptest.NewRecorder()
			f.router.ServeHTTP(rec, req)

			if rec.Code == http.StatusCreated {
				atomic.AddInt64(&successCount, 1)
			} else if rec.Code == http.StatusConflict {
				atomic.AddInt64(&conflictCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf("zero oversell violated: expected exactly 1 successful hold, got %d", successCount)
	}
	if conflictCount != int64(concurrency-1) {
		t.Fatalf("expected %d conflicts, got %d", concurrency-1, conflictCount)
	}

	// Verify slot status in store is 'reserved'
	sCheck, _ := f.bookingStore.GetAppointmentSlotByID(context.Background(), bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(f.tenantA),
		ID:       toPgUUID(fromPgUUID(slot.ID)),
	})
	if sCheck.Status != "reserved" {
		t.Fatalf("expected slot status to be 'reserved', got %s", sCheck.Status)
	}
}

func TestAcceptance_PSY_A07_LateConfirmationAndAutoExpiry(t *testing.T) {
	f := setupPilotPhase2Fixture(t)

	now := time.Now().UTC()
	slotTime := now.Add(3 * time.Hour)
	slot, _ := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist1AID),
		StartsAt:    toPgTimestamptz(slotTime),
		EndsAt:      toPgTimestamptz(slotTime.Add(50 * time.Minute)),
		Status:      "available",
	})
	slotID := fromPgUUID(slot.ID)

	// Guest books appointment
	appt, err := f.bookingSvc.CreatePublicAppointment(context.Background(), f.tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: f.therapist1AID,
		SlotID:      slotID,
		GuestName:   "سارا محمدی",
		GuestPhone:  "09121112233",
		ClientIP:    "10.0.0.1",
	}, now)
	if err != nil {
		t.Fatalf("failed to create appointment: %v", err)
	}

	// 1. Attempt confirmation after 2-hour TTL has expired (now + 2h1m)
	futureTime := now.Add(2*time.Hour + 1*time.Minute)
	_, err = f.bookingSvc.ConfirmAdminAppointment(context.Background(), f.tenantA, appt.ID, "owner", uuid.Nil, futureTime)
	if err == nil {
		t.Fatalf("expected error when confirming expired appointment, got nil")
	}

	// 2. Run background expiration worker
	expiredCount, err := f.bookingSvc.ExpirePendingAppointments(context.Background(), f.tenantA, futureTime)
	if err != nil {
		t.Fatalf("failed to expire appointments: %v", err)
	}
	if expiredCount != 1 {
		t.Fatalf("expected 1 expired appointment, got %d", expiredCount)
	}

	// 3. Slot should now be released back to 'available'
	sCheck, _ := f.bookingStore.GetAppointmentSlotByID(context.Background(), bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(f.tenantA),
		ID:       toPgUUID(slotID),
	})
	if sCheck.Status != "available" {
		t.Fatalf("expected slot to be released back to 'available', got %s", sCheck.Status)
	}

	// 4. A new guest can now successfully book this slot!
	newAppt, err := f.bookingSvc.CreatePublicAppointment(context.Background(), f.tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: f.therapist1AID,
		SlotID:      slotID,
		GuestName:   "متقاضی جدید",
		GuestPhone:  "09129998877",
		ClientIP:    "10.0.0.2",
	}, futureTime)
	if err != nil {
		t.Fatalf("expected new booking to succeed on released slot, got %v", err)
	}
	if newAppt.Status != "pending" {
		t.Fatalf("expected pending status, got %s", newAppt.Status)
	}
}

func TestAcceptance_PSY_A07_ReschedulingCollision_KeepsOriginalIntact(t *testing.T) {
	f := setupPilotPhase2Fixture(t)
	now := time.Now().UTC()

	// Slot 1 (for Appt 1)
	slot1, _ := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist1AID),
		StartsAt:    toPgTimestamptz(now.Add(24 * time.Hour)),
		EndsAt:      toPgTimestamptz(now.Add(24*time.Hour + 50*time.Minute)),
		Status:      "available",
	})
	slot1ID := fromPgUUID(slot1.ID)

	// Slot 2 (booked by Appt 2)
	slot2, _ := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist1AID),
		StartsAt:    toPgTimestamptz(now.Add(48 * time.Hour)),
		EndsAt:      toPgTimestamptz(now.Add(48*time.Hour + 50*time.Minute)),
		Status:      "available",
	})
	slot2ID := fromPgUUID(slot2.ID)

	// Create and confirm Appt 1 in Slot 1
	appt1, _ := f.bookingSvc.CreatePublicAppointment(context.Background(), f.tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: f.therapist1AID,
		SlotID:      slot1ID,
		GuestName:   "مراجع اول",
		GuestPhone:  "09121111111",
		ClientIP:    "10.1.1.1",
	}, now)
	_, _ = f.bookingSvc.ConfirmAdminAppointment(context.Background(), f.tenantA, appt1.ID, "owner", uuid.Nil, now)

	// Create and confirm Appt 2 in Slot 2
	appt2, _ := f.bookingSvc.CreatePublicAppointment(context.Background(), f.tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: f.therapist1AID,
		SlotID:      slot2ID,
		GuestName:   "مراجع دوم",
		GuestPhone:  "09122222222",
		ClientIP:    "10.1.1.2",
	}, now)
	_, _ = f.bookingSvc.ConfirmAdminAppointment(context.Background(), f.tenantA, appt2.ID, "owner", uuid.Nil, now)

	// Staff attempts to reschedule Appt 1 into Slot 2 (which is already booked!)
	_, err := f.bookingSvc.RescheduleAdminAppointment(context.Background(), f.tenantA, appt1.ID, slot2ID, "owner", uuid.Nil, "تلاش برای جابه‌جایی به سانس پر", now)
	if err == nil {
		t.Fatalf("expected error when rescheduling to an already booked slot, got nil")
	}

	// CRITICAL INVARIANT VERIFICATION:
	// Appt 1 must remain Confirmed in Slot 1
	appt1Check, _ := f.bookingStore.GetAppointmentByID(context.Background(), bookingdb.GetAppointmentByIDParams{
		TenantID: toPgUUID(f.tenantA),
		ID:       toPgUUID(appt1.ID),
	})
	if appt1Check.Status != "confirmed" || fromPgUUID(appt1Check.SlotID) != slot1ID {
		t.Fatalf("invariant broken: appt1 altered after failed reschedule: status=%s, slot=%s", appt1Check.Status, fromPgUUID(appt1Check.SlotID))
	}

	// Slot 1 must remain Booked
	s1Check, _ := f.bookingStore.GetAppointmentSlotByID(context.Background(), bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(f.tenantA),
		ID:       toPgUUID(slot1ID),
	})
	if s1Check.Status != "booked" {
		t.Fatalf("invariant broken: slot1 status became %s", s1Check.Status)
	}

	// Slot 2 must remain Booked by Appt 2
	s2Check, _ := f.bookingStore.GetAppointmentSlotByID(context.Background(), bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(f.tenantA),
		ID:       toPgUUID(slot2ID),
	})
	if s2Check.Status != "booked" {
		t.Fatalf("invariant broken: slot2 status became %s", s2Check.Status)
	}
}

func TestAcceptance_PSY_A07_SuccessfulAtomicRescheduling(t *testing.T) {
	f := setupPilotPhase2Fixture(t)
	now := time.Now().UTC()

	// Slot 1 (currently booked)
	slot1, _ := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist1AID),
		StartsAt:    toPgTimestamptz(now.Add(24 * time.Hour)),
		EndsAt:      toPgTimestamptz(now.Add(24*time.Hour + 50*time.Minute)),
		Status:      "available",
	})
	slot1ID := fromPgUUID(slot1.ID)

	// Slot 3 (empty available future slot)
	slot3, _ := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist1AID),
		StartsAt:    toPgTimestamptz(now.Add(72 * time.Hour)),
		EndsAt:      toPgTimestamptz(now.Add(72*time.Hour + 50*time.Minute)),
		Status:      "available",
	})
	slot3ID := fromPgUUID(slot3.ID)

	// Create and confirm Appt 1
	appt1, _ := f.bookingSvc.CreatePublicAppointment(context.Background(), f.tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: f.therapist1AID,
		SlotID:      slot1ID,
		GuestName:   "سارا محمدی",
		GuestPhone:  "09121111111",
		ClientIP:    "10.1.1.1",
	}, now)
	_, _ = f.bookingSvc.ConfirmAdminAppointment(context.Background(), f.tenantA, appt1.ID, "owner", uuid.Nil, now)

	// Staff performs atomic reschedule via HTTP endpoint
	payload := map[string]any{
		"new_slot_id": slot3ID.String(),
		"reason":      "هماهنگی تلفنی با مراجع جهت تغییر ساعت",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/appointments/%s/reschedule", appt1.ID), bytes.NewReader(bodyBytes))
	req.Host = f.hostA
	req.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	req.Header.Set("X-Tenant-ID", f.tenantA.String())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on reschedule, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify Slot 1 released back to 'available'
	s1Check, _ := f.bookingStore.GetAppointmentSlotByID(context.Background(), bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(f.tenantA),
		ID:       toPgUUID(slot1ID),
	})
	if s1Check.Status != "available" {
		t.Fatalf("expected old slot1 to become available, got %s", s1Check.Status)
	}

	// Verify Slot 3 is now 'booked'
	s3Check, _ := f.bookingStore.GetAppointmentSlotByID(context.Background(), bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(f.tenantA),
		ID:       toPgUUID(slot3ID),
	})
	if s3Check.Status != "booked" {
		t.Fatalf("expected new slot3 to become booked, got %s", s3Check.Status)
	}

	// Verify Audit History
	history, err := f.bookingStore.ListAppointmentHistory(context.Background(), bookingdb.ListAppointmentHistoryParams{
		TenantID:      toPgUUID(f.tenantA),
		AppointmentID: toPgUUID(appt1.ID),
	})
	if err != nil || len(history) == 0 {
		t.Fatalf("expected history audit records, got %v", history)
	}
	foundReschedule := false
	for _, h := range history {
		if h.ToStatus == "confirmed" && h.FromStatus.Valid && h.FromStatus.String == "confirmed" {
			foundReschedule = true
		}
	}
	if !foundReschedule {
		t.Fatalf("expected reschedule history record not found")
	}
}

func TestAcceptance_PSY_A07_TherapistRBACIsolation(t *testing.T) {
	f := setupPilotPhase2Fixture(t)
	now := time.Now().UTC()

	// Create slot for Therapist 2 (Dr. Kamran)
	slotKamran, _ := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist2AID),
		StartsAt:    toPgTimestamptz(now.Add(24 * time.Hour)),
		EndsAt:      toPgTimestamptz(now.Add(24*time.Hour + 45*time.Minute)),
		Status:      "available",
	})
	slotKamranID := fromPgUUID(slotKamran.ID)

	// Create appointment for Therapist 2
	apptKamran, _ := f.bookingSvc.CreatePublicAppointment(context.Background(), f.tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: f.therapist2AID,
		SlotID:      slotKamranID,
		GuestName:   "مراجع دکتر کامران",
		GuestPhone:  "09123334455",
		ClientIP:    "10.2.2.1",
	}, now)

	// 1. Therapist 1 (Dr. Maryam) tries to GET details of Kamran's appointment -> MUST RETURN 403
	reqTh1 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/appointments/%s", apptKamran.ID), nil)
	reqTh1.Host = f.hostA
	reqTh1.Header.Set("Authorization", "Bearer "+f.tokenTherapist1A)
	reqTh1.Header.Set("X-Tenant-ID", f.tenantA.String())
	recTh1 := httptest.NewRecorder()
	f.router.ServeHTTP(recTh1, reqTh1)
	if recTh1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when therapist accesses another therapist's appointment, got %d", recTh1.Code)
	}

	// 2. Therapist 1 tries to CONFIRM Kamran's appointment -> MUST RETURN 403
	reqConf := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/appointments/%s/confirm", apptKamran.ID), bytes.NewReader([]byte("{}")))
	reqConf.Host = f.hostA
	reqConf.Header.Set("Authorization", "Bearer "+f.tokenTherapist1A)
	reqConf.Header.Set("X-Tenant-ID", f.tenantA.String())
	reqConf.Header.Set("Content-Type", "application/json")
	reqConf.Header.Set("Sec-Fetch-Site", "same-origin")
	recConf := httptest.NewRecorder()
	f.router.ServeHTTP(recConf, reqConf)
	if recConf.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when therapist confirms another therapist's appointment, got %d", recConf.Code)
	}

	// 3. Receptionist (منشی) CAN view and confirm Kamran's appointment -> 200 OK
	reqRec := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/appointments/%s/confirm", apptKamran.ID), bytes.NewReader([]byte("{}")))
	reqRec.Host = f.hostA
	reqRec.Header.Set("Authorization", "Bearer "+f.tokenReceptionistA)
	reqRec.Header.Set("X-Tenant-ID", f.tenantA.String())
	reqRec.Header.Set("Content-Type", "application/json")
	reqRec.Header.Set("Sec-Fetch-Site", "same-origin")
	recRec := httptest.NewRecorder()
	f.router.ServeHTTP(recRec, reqRec)
	if recRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when receptionist confirms appointment, got %d: %s", recRec.Code, recRec.Body.String())
	}
}

func TestAcceptance_PSY_A07_PIIProtectionAndCacheHeaders(t *testing.T) {
	f := setupPilotPhase2Fixture(t)
	now := time.Now().UTC()

	slot, _ := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist1AID),
		StartsAt:    toPgTimestamptz(now.Add(24 * time.Hour)),
		EndsAt:      toPgTimestamptz(now.Add(24*time.Hour + 50*time.Minute)),
		Status:      "available",
	})
	slotID := fromPgUUID(slot.ID)

	appt, _ := f.bookingSvc.CreatePublicAppointment(context.Background(), f.tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: f.therapist1AID,
		SlotID:      slotID,
		GuestName:   "سارا محمدی",
		GuestPhone:  "09129876543",
		ClientIP:    "10.3.3.1",
	}, now)

	// 1. Verify Public Slots list contains NO PII
	reqSlots := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/therapists/%s/slots", f.therapist1AID), nil)
	reqSlots.Host = f.hostA
	recSlots := httptest.NewRecorder()
	f.router.ServeHTTP(recSlots, reqSlots)
	slotsBody := recSlots.Body.String()
	if bytes.Contains([]byte(slotsBody), []byte("09129876543")) || bytes.Contains([]byte(slotsBody), []byte("سارا محمدی")) {
		t.Fatalf("PII leaked in public slots response")
	}

	// 2. Public appointment status check WITHOUT lookup hash -> MUST RETURN 404
	reqNoHash := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/appointments/%s", appt.ID), nil)
	reqNoHash.Host = f.hostA
	recNoHash := httptest.NewRecorder()
	f.router.ServeHTTP(recNoHash, reqNoHash)
	if recNoHash.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when querying appointment without lookup hash, got %d", recNoHash.Code)
	}

	// 3. Public appointment status check WITH valid lookup hash:
	reqWithHash := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/appointments/%s", appt.ID), nil)
	reqWithHash.Host = f.hostA
	reqWithHash.Header.Set("X-Guest-Lookup-Hash", appt.GuestLookupHash)
	recWithHash := httptest.NewRecorder()
	f.router.ServeHTTP(recWithHash, reqWithHash)
	if recWithHash.Code != http.StatusOK {
		t.Fatalf("expected 200 with lookup hash, got %d", recWithHash.Code)
	}

	// Verify Cache-Control header is no-store
	cacheHeader := recWithHash.Header().Get("Cache-Control")
	if cacheHeader != "no-store, no-cache, must-revalidate, private" {
		t.Fatalf("expected strict no-store Cache-Control header, got %s", cacheHeader)
	}

	// Verify GuestPhone is NOT in public response
	var apptResp map[string]any
	_ = json.Unmarshal(recWithHash.Body.Bytes(), &apptResp)
	if _, hasPhone := apptResp["guest_phone"]; hasPhone {
		t.Fatalf("PII leak: guest_phone present in public appointment response")
	}
	if apptResp["guest_name"] != "سارا محمدی" {
		t.Fatalf("expected guest_name to be preserved for caller holding token")
	}
}

func TestAcceptance_PSY_A07_GuestCancellationBeforeStartTime(t *testing.T) {
	f := setupPilotPhase2Fixture(t)
	now := time.Now().UTC()

	slot, _ := f.bookingStore.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(f.tenantA),
		TherapistID: toPgUUID(f.therapist1AID),
		StartsAt:    toPgTimestamptz(now.Add(24 * time.Hour)),
		EndsAt:      toPgTimestamptz(now.Add(24*time.Hour + 50*time.Minute)),
		Status:      "available",
	})
	slotID := fromPgUUID(slot.ID)

	appt, _ := f.bookingSvc.CreatePublicAppointment(context.Background(), f.tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: f.therapist1AID,
		SlotID:      slotID,
		GuestName:   "متقاضی انصراف",
		GuestPhone:  "09125556677",
		ClientIP:    "10.4.4.1",
	}, now)

	// Guest cancels appointment via public cancellation endpoint
	payload := map[string]any{"reason": "انصراف به دلیل تغییر برنامه سفر"}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/public/appointments/%s/cancel", appt.ID), bytes.NewReader(bodyBytes))
	req.Host = f.hostA
	req.Header.Set("X-Guest-Lookup-Hash", appt.GuestLookupHash)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on guest cancellation, got %d: %s", rec.Code, rec.Body.String())
	}

	// Slot should be returned to 'available'
	sCheck, _ := f.bookingStore.GetAppointmentSlotByID(context.Background(), bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(f.tenantA),
		ID:       toPgUUID(slotID),
	})
	if sCheck.Status != "available" {
		t.Fatalf("expected slot to return to available after cancellation, got %s", sCheck.Status)
	}

	// Subsequent cancellation attempt should fail (CANNOT_CANCEL)
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/public/appointments/%s/cancel", appt.ID), bytes.NewReader(bodyBytes))
	req2.Host = f.hostA
	req2.Header.Set("X-Guest-Lookup-Hash", appt.GuestLookupHash)
	rec2 := httptest.NewRecorder()
	f.router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when cancelling already cancelled appointment, got %d", rec2.Code)
	}
}
