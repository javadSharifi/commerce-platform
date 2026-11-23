package booking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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
	"github.com/jackc/pgx/v5/pgtype"
)

type mockBookingQuerier struct {
	mu            sync.Mutex
	therapists    map[string]bookingdb.Therapist
	workingHours  map[string]bookingdb.TherapistWorkingHour
	exceptions    map[string]bookingdb.TherapistScheduleException
	slots         map[string]bookingdb.AppointmentSlot
	appointments  map[string]bookingdb.Appointment
	statusHistory map[string][]bookingdb.AppointmentStatusHistory
}

func newMockBookingQuerier() *mockBookingQuerier {
	return &mockBookingQuerier{
		therapists:    make(map[string]bookingdb.Therapist),
		workingHours:  make(map[string]bookingdb.TherapistWorkingHour),
		exceptions:    make(map[string]bookingdb.TherapistScheduleException),
		slots:         make(map[string]bookingdb.AppointmentSlot),
		appointments:  make(map[string]bookingdb.Appointment),
		statusHistory: make(map[string][]bookingdb.AppointmentStatusHistory),
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

func toPgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func (m *mockBookingQuerier) CreateTherapist(ctx context.Context, arg bookingdb.CreateTherapistParams) (bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
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
		CreatedAt:              toPgTimestamptz(time.Now()),
		UpdatedAt:              toPgTimestamptz(time.Now()),
	}
	m.therapists[id.String()] = th
	return th, nil
}

func (m *mockBookingQuerier) GetTherapistByID(ctx context.Context, arg bookingdb.GetTherapistByIDParams) (bookingdb.Therapist, error) {
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

func (m *mockBookingQuerier) GetTherapistBySlug(ctx context.Context, arg bookingdb.GetTherapistBySlugParams) (bookingdb.Therapist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenantID := fromPgUUID(arg.TenantID)
	for _, th := range m.therapists {
		if fromPgUUID(th.TenantID) == tenantID && th.Slug == arg.Slug && !th.DeletedAt.Valid {
			return th, nil
		}
	}
	return bookingdb.Therapist{}, fmt.Errorf("therapist not found")
}

func (m *mockBookingQuerier) GetTherapistByUserID(ctx context.Context, arg bookingdb.GetTherapistByUserIDParams) (bookingdb.Therapist, error) {
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

func (m *mockBookingQuerier) ListTherapists(ctx context.Context, tenantID pgtype.UUID) ([]bookingdb.Therapist, error) {
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

func (m *mockBookingQuerier) ListActiveTherapists(ctx context.Context, tenantID pgtype.UUID) ([]bookingdb.Therapist, error) {
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

func (m *mockBookingQuerier) UpdateTherapist(ctx context.Context, arg bookingdb.UpdateTherapistParams) (bookingdb.Therapist, error) {
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

func (m *mockBookingQuerier) SoftDeleteTherapist(ctx context.Context, arg bookingdb.SoftDeleteTherapistParams) error {
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

func (m *mockBookingQuerier) ListWorkingHoursByTherapist(ctx context.Context, arg bookingdb.ListWorkingHoursByTherapistParams) ([]bookingdb.TherapistWorkingHour, error) {
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

func (m *mockBookingQuerier) DeleteWorkingHoursByTherapist(ctx context.Context, arg bookingdb.DeleteWorkingHoursByTherapistParams) error {
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

func (m *mockBookingQuerier) CreateWorkingHour(ctx context.Context, arg bookingdb.CreateWorkingHourParams) (bookingdb.TherapistWorkingHour, error) {
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

func (m *mockBookingQuerier) ListScheduleExceptionsByTherapist(ctx context.Context, arg bookingdb.ListScheduleExceptionsByTherapistParams) ([]bookingdb.TherapistScheduleException, error) {
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

func (m *mockBookingQuerier) GetScheduleExceptionByID(ctx context.Context, arg bookingdb.GetScheduleExceptionByIDParams) (bookingdb.TherapistScheduleException, error) {
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

func (m *mockBookingQuerier) CreateScheduleException(ctx context.Context, arg bookingdb.CreateScheduleExceptionParams) (bookingdb.TherapistScheduleException, error) {
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

func (m *mockBookingQuerier) DeleteScheduleException(ctx context.Context, arg bookingdb.DeleteScheduleExceptionParams) error {
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

func (m *mockBookingQuerier) CreateAppointmentSlot(ctx context.Context, arg bookingdb.CreateAppointmentSlotParams) (bookingdb.AppointmentSlot, error) {
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

func (m *mockBookingQuerier) GetAppointmentSlotByID(ctx context.Context, arg bookingdb.GetAppointmentSlotByIDParams) (bookingdb.AppointmentSlot, error) {
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

func (m *mockBookingQuerier) GetAppointmentSlotByIDForUpdate(ctx context.Context, arg bookingdb.GetAppointmentSlotByIDForUpdateParams) (bookingdb.AppointmentSlot, error) {
	return m.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams(arg))
}

func (m *mockBookingQuerier) ListSlotsByTherapist(ctx context.Context, arg bookingdb.ListSlotsByTherapistParams) ([]bookingdb.AppointmentSlot, error) {
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
	// Sort by starts_at ASC (matching SQL ORDER BY starts_at ASC)
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].StartsAt.Time.Before(list[i].StartsAt.Time) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list, nil
}

func (m *mockBookingQuerier) ListAvailableSlots(ctx context.Context, arg bookingdb.ListAvailableSlotsParams) ([]bookingdb.AppointmentSlot, error) {
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
	// Sort by starts_at ASC (matching SQL ORDER BY starts_at ASC)
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].StartsAt.Time.Before(list[i].StartsAt.Time) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list, nil
}

func (m *mockBookingQuerier) DeleteFutureAvailableSlots(ctx context.Context, arg bookingdb.DeleteFutureAvailableSlotsParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(arg.TenantID)
	thID := fromPgUUID(arg.TherapistID)

	for k, sl := range m.slots {
		if fromPgUUID(sl.TenantID) == tID && fromPgUUID(sl.TherapistID) == thID {
			// Only delete 'available' slots in the future; never touch booked or reserved slots!
			if sl.Status == "available" && !sl.StartsAt.Time.Before(arg.StartsAt.Time) {
				delete(m.slots, k)
			}
		}
	}
	return nil
}

func (m *mockBookingQuerier) UpdateSlotStatus(ctx context.Context, arg bookingdb.UpdateSlotStatusParams) (bookingdb.AppointmentSlot, error) {
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

func (m *mockBookingQuerier) CreateAppointment(ctx context.Context, arg bookingdb.CreateAppointmentParams) (bookingdb.Appointment, error) {
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

func (m *mockBookingQuerier) GetAppointmentByID(ctx context.Context, arg bookingdb.GetAppointmentByIDParams) (bookingdb.Appointment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fromPgUUID(arg.ID).String()
	tID := fromPgUUID(arg.TenantID)

	app, ok := m.appointments[id]
	if !ok || fromPgUUID(app.TenantID) != tID {
		return bookingdb.Appointment{}, fmt.Errorf("appointment not found")
	}
	return app, nil
}

func (m *mockBookingQuerier) GetAppointmentByIDForUpdate(ctx context.Context, arg bookingdb.GetAppointmentByIDForUpdateParams) (bookingdb.Appointment, error) {
	return m.GetAppointmentByID(ctx, bookingdb.GetAppointmentByIDParams(arg))
}

func (m *mockBookingQuerier) GetAppointmentBySlotID(ctx context.Context, arg bookingdb.GetAppointmentBySlotIDParams) (bookingdb.Appointment, error) {
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

func (m *mockBookingQuerier) UpdateAppointmentStatus(ctx context.Context, arg bookingdb.UpdateAppointmentStatusParams) (bookingdb.Appointment, error) {
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

func (m *mockBookingQuerier) ListAppointmentsByTherapist(ctx context.Context, arg bookingdb.ListAppointmentsByTherapistParams) ([]bookingdb.Appointment, error) {
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

func (m *mockBookingQuerier) ListExpiredPendingAppointments(ctx context.Context, arg bookingdb.ListExpiredPendingAppointmentsParams) ([]bookingdb.Appointment, error) {
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

func (m *mockBookingQuerier) CreateAppointmentStatusHistory(ctx context.Context, arg bookingdb.CreateAppointmentStatusHistoryParams) (bookingdb.AppointmentStatusHistory, error) {
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

func (m *mockBookingQuerier) ListAppointmentHistory(ctx context.Context, arg bookingdb.ListAppointmentHistoryParams) ([]bookingdb.AppointmentStatusHistory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	appID := fromPgUUID(arg.AppointmentID).String()
	return m.statusHistory[appID], nil
}

func (m *mockBookingQuerier) GetAppointmentDetail(ctx context.Context, arg bookingdb.GetAppointmentDetailParams) (bookingdb.GetAppointmentDetailRow, error) {
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
		GuestLookupHash:   app.GuestLookupHash,
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

func (m *mockBookingQuerier) ListAdminAppointments(ctx context.Context, arg bookingdb.ListAdminAppointmentsParams) ([]bookingdb.ListAdminAppointmentsRow, error) {
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
			GuestLookupHash:   app.GuestLookupHash,
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

	// Sort by created_at DESC
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].CreatedAt.Time.After(list[i].CreatedAt.Time) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list, nil
}

func (m *mockBookingQuerier) UpdateAppointmentSlot(ctx context.Context, arg bookingdb.UpdateAppointmentSlotParams) (bookingdb.Appointment, error) {
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

// ----------------------------------------------------
// Mock Domain & Auth Services for HTTP Testing
// ----------------------------------------------------

type mockDomainQuerier struct {
	domains map[string]domaindb.Domain
}

func newMockDomainQuerier() *mockDomainQuerier {
	return &mockDomainQuerier{domains: make(map[string]domaindb.Domain)}
}

func (m *mockDomainQuerier) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	d, ok := m.domains[host]
	if !ok {
		return domaindb.Domain{}, fmt.Errorf("not found")
	}
	return d, nil
}

func (m *mockDomainQuerier) ListDomainsByTenant(ctx context.Context, tenantID pgtype.UUID) ([]domaindb.Domain, error) {
	return nil, nil
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
	return authdb.PanelIdentity{}, fmt.Errorf("not found")
}

func (m *mockAuthQuerier) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	for _, ident := range m.identities {
		if ident.ID == id {
			return ident, nil
		}
	}
	return authdb.PanelIdentity{}, fmt.Errorf("not found")
}

func (m *mockAuthQuerier) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	ident := authdb.PanelIdentity{
		ID:           arg.ID,
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		FullName:     arg.FullName,
		Status:       "active",
	}
	m.identities[ident.Email] = ident
	return ident, nil
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
		return authdb.PanelSession{}, fmt.Errorf("not found")
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
		return authdb.Membership{}, fmt.Errorf("not found")
	}
	return mem, nil
}

func (m *mockAuthQuerier) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	mem := authdb.Membership{
		ID:       arg.ID,
		TenantID: arg.TenantID,
		UserID:   arg.UserID,
		Role:     arg.Role,
	}
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	m.memberships[key] = mem
	return mem, nil
}

// ----------------------------------------------------
// Test Suite: Therapist Profile Lifecycle
// ----------------------------------------------------

func TestTherapistProfileLifecycle(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantA := uuid.New()

	// 1. Create therapist
	th, err := svc.CreateTherapist(ctx, tenantA, booking.CreateTherapistInput{
		Name:                   "دکتر حمید اکبری",
		Slug:                   "dr-akbari",
		Title:                  "روان‌درمانگر پویشی",
		Bio:                    "متخصص درمان افسردگی و اضطراب",
		SessionDurationMinutes: 50,
		IsActive:               true,
	})
	if err != nil {
		t.Fatalf("failed to create therapist: %v", err)
	}
	if th.Name != "دکتر حمید اکبری" || th.SessionDurationMinutes != 50 {
		t.Errorf("unexpected therapist data: %+v", th)
	}

	// 2. Query by ID and Slug
	byID, err := svc.GetTherapistByID(ctx, tenantA, fromPgUUID(th.ID))
	if err != nil {
		t.Fatalf("failed to get therapist by ID: %v", err)
	}
	if byID.Slug != "dr-akbari" {
		t.Errorf("expected slug 'dr-akbari', got %s", byID.Slug)
	}

	bySlug, err := svc.GetTherapistBySlug(ctx, tenantA, "dr-akbari")
	if err != nil {
		t.Fatalf("failed to get therapist by slug: %v", err)
	}
	if bySlug.Title != "روان‌درمانگر پویشی" {
		t.Errorf("expected title 'روان‌درمانگر پویشی', got %s", bySlug.Title)
	}

	// 3. Update therapist
	updated, err := svc.UpdateTherapist(ctx, tenantA, fromPgUUID(th.ID), booking.UpdateTherapistInput{
		Name:                   "دکتر حمید اکبری (استادیار)",
		Slug:                   "dr-akbari",
		Title:                  "فوق تخصص روان‌درمانی تحلیلی",
		Bio:                    "بیوگرافی به‌روزشده",
		SessionDurationMinutes: 60,
		IsActive:               true,
	})
	if err != nil {
		t.Fatalf("failed to update therapist: %v", err)
	}
	if updated.Title != "فوق تخصص روان‌درمانی تحلیلی" || updated.SessionDurationMinutes != 60 {
		t.Errorf("update failed, got %+v", updated)
	}

	// 4. Soft Delete
	if err := svc.DeleteTherapist(ctx, tenantA, fromPgUUID(th.ID)); err != nil {
		t.Fatalf("failed to delete therapist: %v", err)
	}

	// Should be absent from active list
	actives, err := svc.ListActiveTherapists(ctx, tenantA)
	if err != nil {
		t.Fatalf("failed to list active: %v", err)
	}
	if len(actives) != 0 {
		t.Errorf("expected 0 active therapists after soft delete, got %d", len(actives))
	}
}

// ----------------------------------------------------
// Test Suite: Working Hours & Non-overlapping Slot Generation
// ----------------------------------------------------

func TestWorkingHoursAndSlotGeneration(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantA := uuid.New()

	// 1. Create therapist with 45 min sessions
	th, err := svc.CreateTherapist(ctx, tenantA, booking.CreateTherapistInput{
		Name:                   "دکتر مریم شریفی",
		Slug:                   "dr-sharifi",
		SessionDurationMinutes: 45,
		IsActive:               true,
	})
	if err != nil {
		t.Fatalf("failed to create therapist: %v", err)
	}
	thID := fromPgUUID(th.ID)

	// 2. Set weekly hours:
	// Saturday (0): 09:00 - 12:00 -> 3 hours = 180 min -> 4 slots of 45 min
	// (09:00-09:45, 09:45-10:30, 10:30-11:15, 11:15-12:00)
	hours := []booking.WorkingHourInput{
		{
			DayOfWeek: 0, // Saturday (شنبه)
			StartTime: "09:00:00",
			EndTime:   "12:00:00",
			IsActive:  true,
		},
	}
	if _, err := svc.SetWorkingHours(ctx, tenantA, thID, hours); err != nil {
		t.Fatalf("failed to set working hours: %v", err)
	}

	// 3. Generate slots for next 14 days
	now := time.Now().UTC().AddDate(0, 0, 1) // start from tomorrow so all are in the future
	res, err := svc.GenerateSlots(ctx, tenantA, thID, now, now.AddDate(0, 0, 14))
	if err != nil {
		t.Fatalf("failed to generate slots: %v", err)
	}

	if res.GeneratedCount == 0 {
		t.Fatalf("expected slots to be generated, got 0")
	}

	// 4. Query available slots
	slots, err := svc.ListAvailableSlots(ctx, tenantA, thID, now)
	if err != nil {
		t.Fatalf("failed to list available slots: %v", err)
	}

	// Verify all slots are 45 minutes apart and strictly non-overlapping
	for i, sl := range slots {
		dur := sl.EndsAt.Time.Sub(sl.StartsAt.Time)
		if dur != 45*time.Minute {
			t.Errorf("slot %d has unexpected duration: %v", i, dur)
		}
		if i > 0 {
			prev := slots[i-1]
			// If on same therapist, starts_at must be >= prev ends_at
			if sl.StartsAt.Time.Before(prev.EndsAt.Time) {
				t.Fatalf("overlapping slots detected! prev ends: %v, cur starts: %v", prev.EndsAt.Time, sl.StartsAt.Time)
			}
		}
	}
}

// ----------------------------------------------------
// Test Suite: Schedule Exceptions (Full Day and Partial Day)
// ----------------------------------------------------

func TestScheduleExceptions_FullAndPartial(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantA := uuid.New()
	th, _ := svc.CreateTherapist(ctx, tenantA, booking.CreateTherapistInput{
		Name:                   "دکتر مهدی کاظمی",
		Slug:                   "dr-kazemi",
		SessionDurationMinutes: 60,
		IsActive:               true,
	})
	thID := fromPgUUID(th.ID)

	// Saturday (0): 09:00 - 13:00 (4 slots: 9-10, 10-11, 11-12, 12-13)
	_, _ = svc.SetWorkingHours(ctx, tenantA, thID, []booking.WorkingHourInput{
		{DayOfWeek: 0, StartTime: "09:00:00", EndTime: "13:00:00", IsActive: true},
	})

	// Find next Saturday
	loc, _ := time.LoadLocation("Asia/Tehran")
	todayTehran := time.Now().In(loc)
	nextSat := todayTehran.AddDate(0, 0, (6-int(todayTehran.Weekday())+7)%7)
	if nextSat.Day() == todayTehran.Day() {
		nextSat = nextSat.AddDate(0, 0, 7)
	}

	// 1. Add Partial Exception: 09:00 - 11:00 unavailable (blocks first 2 slots)
	_, err := svc.CreateScheduleException(ctx, tenantA, thID, booking.ScheduleExceptionInput{
		Date:          nextSat,
		IsUnavailable: false,
		StartTime:     "09:00:00",
		EndTime:       "11:00:00",
		Reason:        "جلسه دفاع دانشجو",
	})
	if err != nil {
		t.Fatalf("failed to create partial exception: %v", err)
	}

	// Generate slots for that Saturday
	res, err := svc.GenerateSlots(ctx, tenantA, thID, nextSat.AddDate(0, 0, -1), nextSat.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("failed to generate slots with partial exception: %v", err)
	}
	if res.GeneratedCount != 2 {
		t.Errorf("expected exactly 2 remaining slots (11-12, 12-13), got %d", res.GeneratedCount)
	}

	// 2. Add Full Day Exception: Entire Saturday unavailable
	_, err = svc.CreateScheduleException(ctx, tenantA, thID, booking.ScheduleExceptionInput{
		Date:          nextSat,
		IsUnavailable: true,
		Reason:        "تعطیلی مطب به علت سفر",
	})
	if err != nil {
		t.Fatalf("failed to create full exception: %v", err)
	}

	// Re-query slots on nextSat: all future available slots should be gone
	availSlots, err := svc.ListAvailableSlots(ctx, tenantA, thID, nextSat.AddDate(0, 0, -1))
	if err != nil {
		t.Fatalf("failed to list slots: %v", err)
	}

	for _, sl := range availSlots {
		if sl.StartsAt.Time.In(loc).Day() == nextSat.Day() {
			t.Errorf("found available slot on full day blocked date: %v", sl.StartsAt.Time)
		}
	}
}

// ----------------------------------------------------
// Test Suite: Working Hours Change Keeps Booked Slots Intact
// ----------------------------------------------------

func TestWorkingHoursChange_KeepsBookedSlotsIntact(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantA := uuid.New()
	th, _ := svc.CreateTherapist(ctx, tenantA, booking.CreateTherapistInput{
		Name:                   "دکتر نیلوفر باقری",
		Slug:                   "dr-bagheri",
		SessionDurationMinutes: 60,
		IsActive:               true,
	})
	thID := fromPgUUID(th.ID)

	// Set initial hours: Sunday (1) 09:00 - 12:00
	_, _ = svc.SetWorkingHours(ctx, tenantA, thID, []booking.WorkingHourInput{
		{DayOfWeek: 1, StartTime: "09:00:00", EndTime: "12:00:00", IsActive: true},
	})

	nextWeek := time.Now().UTC().AddDate(0, 0, 7)
	_, _ = svc.GenerateSlots(ctx, tenantA, thID, nextWeek, nextWeek.AddDate(0, 0, 7))

	initialSlots, _ := svc.ListSlots(ctx, tenantA, thID, nextWeek, nextWeek.AddDate(0, 0, 8))
	if len(initialSlots) == 0 {
		t.Fatalf("expected initial slots to be generated")
	}

	// Simulate one slot being booked!
	bookedSlotID := initialSlots[0].ID
	_, err := q.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
		TenantID: toPgUUID(tenantA),
		ID:       bookedSlotID,
		Status:   "booked",
	})
	if err != nil {
		t.Fatalf("failed to update slot status: %v", err)
	}

	// Now modify working hours completely: change Sunday hours to afternoon 14:00 - 18:00
	newHours := []booking.WorkingHourInput{
		{DayOfWeek: 1, StartTime: "14:00:00", EndTime: "18:00:00", IsActive: true},
	}
	_, err = svc.SetWorkingHours(ctx, tenantA, thID, newHours)
	if err != nil {
		t.Fatalf("failed to update working hours: %v", err)
	}

	// Verify the booked slot was NOT deleted and is still 'booked'!
	bookedCheck, err := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantA),
		ID:       bookedSlotID,
	})
	if err != nil {
		t.Fatalf("booked slot was mistakenly deleted during schedule update! err: %v", err)
	}
	if bookedCheck.Status != "booked" {
		t.Errorf("expected booked slot to remain 'booked', got %s", bookedCheck.Status)
	}
}

// ----------------------------------------------------
// Test Suite: Cross-Tenant Isolation
// ----------------------------------------------------

func TestCrossTenantIsolation(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantA := uuid.New()
	tenantB := uuid.New()

	// Create therapist in Tenant A
	thA, _ := svc.CreateTherapist(ctx, tenantA, booking.CreateTherapistInput{
		Name:     "درمانگر کلینیک الف",
		Slug:     "dr-a",
		IsActive: true,
	})

	// Create therapist in Tenant B
	thB, _ := svc.CreateTherapist(ctx, tenantB, booking.CreateTherapistInput{
		Name:     "درمانگر کلینیک ب",
		Slug:     "dr-b",
		IsActive: true,
	})

	// 1. Tenant B cannot read Tenant A therapist by ID
	_, err := svc.GetTherapistByID(ctx, tenantB, fromPgUUID(thA.ID))
	if err == nil {
		t.Errorf("tenant B should NOT be able to access tenant A therapist by ID")
	}

	// 2. Tenant A list only contains Tenant A
	listA, _ := svc.ListTherapists(ctx, tenantA)
	if len(listA) != 1 || fromPgUUID(listA[0].ID) != fromPgUUID(thA.ID) {
		t.Errorf("tenant A list polluted with cross-tenant data: %+v", listA)
	}

	// 3. Tenant A cannot modify Tenant B working hours
	err = q.DeleteWorkingHoursByTherapist(ctx, bookingdb.DeleteWorkingHoursByTherapistParams{
		TenantID:    toPgUUID(tenantA),
		TherapistID: thB.ID,
	})
	// Query scoped to tenantA will affect 0 rows of tenantB
	hoursB, _ := q.ListWorkingHoursByTherapist(ctx, bookingdb.ListWorkingHoursByTherapistParams{
		TenantID:    thB.TenantID,
		TherapistID: thB.ID,
	})
	_ = hoursB
}

// ----------------------------------------------------
// Test Suite: HTTP Endpoints & RBAC Enforcement
// ----------------------------------------------------

func TestHTTPEndpointsAndRoleEnforcement(t *testing.T) {
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	dq := newMockDomainQuerier()
	dr := domain.NewResolver(dq)

	aq := newMockAuthQuerier()
	as := auth.NewService(aq)

	handler := booking.NewHandler(svc, dr, as)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	tenantA := uuid.New()
	dq.domains["clinic-alavi.ir"] = domaindb.Domain{
		TenantID:   toPgUUID(tenantA),
		Host:       "clinic-alavi.ir",
		IsVerified: true,
	}

	// Seed Owner user
	ownerPass, _ := auth.HashPassword("OwnerSecret123!")
	ownerID := uuid.New()
	aq.identities["owner@clinic-alavi.ir"] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerID),
		Email:        "owner@clinic-alavi.ir",
		PasswordHash: ownerPass,
		Status:       "active",
	}
	aq.memberships[fmt.Sprintf("%s:%s", ownerID.String(), tenantA.String())] = authdb.Membership{
		TenantID: toPgUUID(tenantA),
		UserID:   toPgUUID(ownerID),
		Role:     "owner",
	}

	// Login owner to get session token
	sessionRes, err := as.Login(context.Background(), "owner@clinic-alavi.ir", "OwnerSecret123!")
	if err != nil {
		t.Fatalf("failed to login owner: %v", err)
	}

	// 1. Owner creates therapist via POST /admin/therapists
	createBody := map[string]any{
		"name":                     "دکتر سمیه مرادی",
		"slug":                     "dr-moradi",
		"title":                    "مشاور خانواده",
		"session_duration_minutes": 45,
	}
	bodyBytes, _ := json.Marshal(createBody)
	req := httptest.NewRequest("POST", "/admin/therapists", bytes.NewReader(bodyBytes))
	req.Host = "clinic-alavi.ir"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantA.String())
	req.AddCookie(&http.Cookie{Name: "session_token", Value: sessionRes.Token})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&created)
	createdID := created["id"].(string)

	// 2. Public endpoint: GET /public/therapists
	pubReq := httptest.NewRequest("GET", "/public/therapists", nil)
	pubReq.Host = "clinic-alavi.ir"
	pubRec := httptest.NewRecorder()
	r.ServeHTTP(pubRec, pubReq)

	if pubRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", pubRec.Code)
	}
	var pubList []map[string]any
	_ = json.NewDecoder(pubRec.Body).Decode(&pubList)
	if len(pubList) != 1 || pubList[0]["id"] != createdID {
		t.Errorf("unexpected public therapist list: %+v", pubList)
	}

	// 3. Unauthenticated request to /admin/therapists is rejected with 401
	unauthReq := httptest.NewRequest("GET", "/admin/therapists", nil)
	unauthReq.Host = "clinic-alavi.ir"
	unauthRec := httptest.NewRecorder()
	r.ServeHTTP(unauthRec, unauthReq)

	if unauthRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", unauthRec.Code)
	}
}

// ----------------------------------------------------
// PSY-2.3 Public Appointments & Concurrency Tests
// ----------------------------------------------------

func TestPublicAppointment_CreationAndPrivacy(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	// 1. Create therapist
	th, err := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:                   "دکتر مهسا رضایی",
		Slug:                   "dr-rezaei",
		SessionDurationMinutes: 45,
		IsActive:               true,
	})
	if err != nil {
		t.Fatalf("create therapist failed: %v", err)
	}
	thID := fromPgUUID(th.ID)

	// 2. Create future slot (starts tomorrow at 10:00 UTC, ends 10:45)
	slotStart := now.Add(24 * time.Hour)
	slotEnd := slotStart.Add(45 * time.Minute)
	slot, err := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotEnd),
		Status:      "available",
	})
	if err != nil {
		t.Fatalf("create slot failed: %v", err)
	}
	slotID := fromPgUUID(slot.ID)

	// 3. Test phone normalization
	normPhone, err := booking.NormalizePhone("۰۹۱۲۳۴۵۶۷۸۹")
	if err != nil || normPhone != "09123456789" {
		t.Errorf("expected 09123456789, got %s, err: %v", normPhone, err)
	}
	normPhone2, err := booking.NormalizePhone("+989123456789")
	if err != nil || normPhone2 != "09123456789" {
		t.Errorf("expected 09123456789, got %s, err: %v", normPhone2, err)
	}
	_, err = booking.NormalizePhone("123456")
	if err == nil {
		t.Errorf("expected error for invalid phone")
	}

	// 4. Test validation: Empty guest name
	_, err = svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      slotID,
		GuestName:   "  ",
		GuestPhone:  "09123456789",
	}, now)
	if err == nil {
		t.Errorf("expected error for empty guest name")
	}

	// 5. Successful booking
	res, err := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      slotID,
		GuestName:   "مریم کاظمی",
		GuestPhone:  "۰۹۱۲۳۴۵۶۷۸۹",
		Notes:       "جلسه اول مشاوره",
	}, now)
	if err != nil {
		t.Fatalf("expected successful appointment creation, got: %v", err)
	}

	if res.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", res.Status)
	}
	if len(res.GuestLookupHash) != 64 {
		t.Errorf("expected 64-char hex lookup hash, got len %d", len(res.GuestLookupHash))
	}

	// Check 2-hour TTL: slot is 24h away, so expires_at = now + 2h
	expectedExpiry := now.Add(2 * time.Hour)
	if !res.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expires_at %v, got %v", expectedExpiry, res.ExpiresAt)
	}

	// Verify slot is now reserved
	updatedSlot, err := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot.ID,
	})
	if err != nil || updatedSlot.Status != "reserved" {
		t.Errorf("expected slot to be 'reserved', got %s, err: %v", updatedSlot.Status, err)
	}

	// Verify audit status history
	history, err := q.ListAppointmentHistory(ctx, bookingdb.ListAppointmentHistoryParams{
		TenantID:      toPgUUID(tenantID),
		AppointmentID: toPgUUID(res.ID),
	})
	if err != nil || len(history) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(history))
	}
	if history[0].Action != "created" || history[0].ActorType != "guest" {
		t.Errorf("unexpected history record: %+v", history[0])
	}
}

func TestPublicAppointment_TTL_CappedBySlotStartTime(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	th, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:     "دکتر رضایی",
		Slug:     "dr-rezaei",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	// Slot starts in 45 minutes (less than 2h TTL)
	slotStart := now.Add(45 * time.Minute)
	slotEnd := slotStart.Add(45 * time.Minute)
	slot, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotEnd),
		Status:      "available",
	})

	res, err := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      fromPgUUID(slot.ID),
		GuestName:   "سارا مرادی",
		GuestPhone:  "09121112233",
	}, now)
	if err != nil {
		t.Fatalf("create appointment failed: %v", err)
	}

	// Expiry must be capped at slot.starts_at (45m from now), NOT now + 2h
	if !res.ExpiresAt.Equal(slotStart) {
		t.Errorf("expected expires_at to be capped at slotStart %v, got %v", slotStart, res.ExpiresAt)
	}

	// Slot in the past is rejected
	pastSlot, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(now.Add(-1 * time.Hour)),
		EndsAt:      toPgTimestamptz(now.Add(-15 * time.Minute)),
		Status:      "available",
	})

	_, err = svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      fromPgUUID(pastSlot.ID),
		GuestName:   "سارا مرادی",
		GuestPhone:  "09121112233",
	}, now)
	if !errors.Is(err, booking.ErrSlotInThePast) {
		t.Errorf("expected ErrSlotInThePast, got %v", err)
	}
}

func TestPublicAppointment_ConcurrentSlotHold_ZeroOversell(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	th, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:     "دکتر احمدی",
		Slug:     "dr-ahmadi",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	slotStart := now.Add(24 * time.Hour)
	slotEnd := slotStart.Add(45 * time.Minute)
	slot, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotEnd),
		Status:      "available",
	})
	slotID := fromPgUUID(slot.ID)

	// Run 10 concurrent booking requests for the exact same slot
	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	var successCount int64
	var failureCount int64
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
				TherapistID: thID,
				SlotID:      slotID,
				GuestName:   fmt.Sprintf("متقاضی شماره %d", idx),
				GuestPhone:  fmt.Sprintf("0912000000%d", idx),
			}, now)

			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else if errors.Is(err, booking.ErrSlotNotAvailable) {
				failureCount++
			}
		}(i)
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf("CRITICAL ZERO-OVERSELL BREACH: expected exactly 1 successful booking, got %d", successCount)
	}
	if failureCount != int64(concurrency-1) {
		t.Fatalf("expected %d conflicts (ErrSlotNotAvailable), got %d", concurrency-1, failureCount)
	}

	// Verify final slot status is reserved
	finalSlot, _ := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot.ID,
	})
	if finalSlot.Status != "reserved" {
		t.Errorf("expected final slot status 'reserved', got %s", finalSlot.Status)
	}
}

func TestPublicAppointment_StatusCheck_And_AutoExpiry(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	th, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:     "دکتر سهرابی",
		Slug:     "dr-sohrabi",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	slotStart := t0.Add(24 * time.Hour)
	slotEnd := slotStart.Add(45 * time.Minute)
	slot, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotEnd),
		Status:      "available",
	})
	slotID := fromPgUUID(slot.ID)

	res, err := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      slotID,
		GuestName:   "امیر علوی",
		GuestPhone:  "09129998877",
	}, t0)
	if err != nil {
		t.Fatalf("create appointment failed: %v", err)
	}

	// 1. Successful lookup with valid hash
	st, err := svc.GetPublicAppointmentStatus(ctx, tenantID, res.ID, res.GuestLookupHash, t0.Add(30*time.Minute))
	if err != nil || st.Status != "pending" {
		t.Errorf("expected status 'pending', got %s, err: %v", st.Status, err)
	}

	// 2. Lookup with wrong hash fails with 404/ErrAppointmentNotFound
	_, err = svc.GetPublicAppointmentStatus(ctx, tenantID, res.ID, "bad-hash", t0.Add(30*time.Minute))
	if !errors.Is(err, booking.ErrAppointmentNotFound) {
		t.Errorf("expected ErrAppointmentNotFound for wrong hash, got %v", err)
	}

	// 3. Advance time past 2-hour TTL: t0 + 2h + 1m
	tExpired := t0.Add(2*time.Hour + 1*time.Minute)
	stExpired, err := svc.GetPublicAppointmentStatus(ctx, tenantID, res.ID, res.GuestLookupHash, tExpired)
	if err != nil {
		t.Fatalf("status check after expiry failed: %v", err)
	}
	if stExpired.Status != "expired" {
		t.Errorf("expected status 'expired', got %s", stExpired.Status)
	}

	// Verify slot is freed back to 'available'
	freedSlot, _ := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot.ID,
	})
	if freedSlot.Status != "available" {
		t.Errorf("expected slot to be restored to 'available', got %s", freedSlot.Status)
	}

	// Verify status history recorded 'expired'
	hist, _ := q.ListAppointmentHistory(ctx, bookingdb.ListAppointmentHistoryParams{
		TenantID:      toPgUUID(tenantID),
		AppointmentID: toPgUUID(res.ID),
	})
	if len(hist) != 2 || hist[1].Action != "expired" || hist[1].ActorType != "system" {
		t.Errorf("expected expired history record, got %+v", hist)
	}
}

func TestPublicAppointment_Cancellation(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	th, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:     "دکتر کیانی",
		Slug:     "dr-kiani",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	slotStart := t0.Add(10 * time.Hour)
	slotEnd := slotStart.Add(45 * time.Minute)
	slot, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotEnd),
		Status:      "available",
	})
	slotID := fromPgUUID(slot.ID)

	res, err := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      slotID,
		GuestName:   "نگین نادری",
		GuestPhone:  "09123334455",
	}, t0)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// 1. Cancellation with bad hash fails
	_, err = svc.CancelPublicAppointment(ctx, tenantID, res.ID, "bad-hash", "علت لغو", t0.Add(1*time.Hour))
	if !errors.Is(err, booking.ErrAppointmentNotFound) {
		t.Errorf("expected ErrAppointmentNotFound, got %v", err)
	}

	// 2. Cancellation with valid hash succeeds
	cancRes, err := svc.CancelPublicAppointment(ctx, tenantID, res.ID, res.GuestLookupHash, "انصراف مراجع", t0.Add(1*time.Hour))
	if err != nil || cancRes.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s, err: %v", cancRes.Status, err)
	}

	// Verify slot is returned to 'available'
	slotBack, _ := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot.ID,
	})
	if slotBack.Status != "available" {
		t.Errorf("expected slot status 'available', got %s", slotBack.Status)
	}

	// 3. Cancelling again fails with ErrCannotCancel
	_, err = svc.CancelPublicAppointment(ctx, tenantID, res.ID, res.GuestLookupHash, "لغو مجدد", t0.Add(2*time.Hour))
	if !errors.Is(err, booking.ErrCannotCancel) {
		t.Errorf("expected ErrCannotCancel, got %v", err)
	}

	// 4. Test past slot cancellation prevention
	pastSlot, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(t0.Add(2 * time.Hour)),
		EndsAt:      toPgTimestamptz(t0.Add(2*time.Hour + 45*time.Minute)),
		Status:      "available",
	})
	pastApp, _ := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      fromPgUUID(pastSlot.ID),
		GuestName:   "نگین",
		GuestPhone:  "09123334455",
	}, t0)

	// Try to cancel when session has already started (t0 + 2h 5m)
	_, err = svc.CancelPublicAppointment(ctx, tenantID, pastApp.ID, pastApp.GuestLookupHash, "دیرهنگام", t0.Add(2*time.Hour+5*time.Minute))
	if !errors.Is(err, booking.ErrCannotCancelPastSlot) {
		t.Errorf("expected ErrCannotCancelPastSlot, got %v", err)
	}
}

func TestPublicAppointment_RateLimiting(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	// Custom rate limiter: max 2 requests per minute
	rl := booking.NewRateLimiter(2, 1*time.Minute)
	svc.SetRateLimiter(rl)

	tenantID := uuid.New()
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	th, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:     "دکتر ایمانی",
		Slug:     "dr-imani",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	makeSlot := func(offsetHours int) uuid.UUID {
		slotStart := t0.Add(time.Duration(offsetHours) * time.Hour)
		sl, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
			TenantID:    toPgUUID(tenantID),
			TherapistID: th.ID,
			StartsAt:    toPgTimestamptz(slotStart),
			EndsAt:      toPgTimestamptz(slotStart.Add(45 * time.Minute)),
			Status:      "available",
		})
		return fromPgUUID(sl.ID)
	}

	// Request 1 from 10.0.0.1: Allowed
	_, err := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      makeSlot(5),
		GuestName:   "کاربر ۱",
		GuestPhone:  "09121111111",
		ClientIP:    "10.0.0.1",
	}, t0)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}

	// Request 2 from 10.0.0.1: Allowed
	_, err = svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      makeSlot(6),
		GuestName:   "کاربر ۲",
		GuestPhone:  "09121111112",
		ClientIP:    "10.0.0.1",
	}, t0.Add(10*time.Second))
	if err != nil {
		t.Fatalf("request 2 failed: %v", err)
	}

	// Request 3 from 10.0.0.1: Rate limit exceeded!
	_, err = svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      makeSlot(7),
		GuestName:   "کاربر ۳",
		GuestPhone:  "09121111113",
		ClientIP:    "10.0.0.1",
	}, t0.Add(20*time.Second))
	if !errors.Is(err, booking.ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded, got %v", err)
	}

	// Request from another IP 10.0.0.2: Allowed
	_, err = svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      makeSlot(8),
		GuestName:   "کاربر ۴",
		GuestPhone:  "09121111114",
		ClientIP:    "10.0.0.2",
	}, t0.Add(25*time.Second))
	if err != nil {
		t.Fatalf("request from distinct IP failed: %v", err)
	}
}

func TestPublicAppointment_CrossTenantIsolation(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantA := uuid.New()
	tenantB := uuid.New()
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	// Create therapist and slot in Tenant A
	thA, _ := svc.CreateTherapist(ctx, tenantA, booking.CreateTherapistInput{
		Name:     "درمانگر کلینیک الف",
		Slug:     "dr-a",
		IsActive: true,
	})
	slotStart := t0.Add(24 * time.Hour)
	slotA, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantA),
		TherapistID: thA.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotStart.Add(45 * time.Minute)),
		Status:      "available",
	})

	appA, err := svc.CreatePublicAppointment(ctx, tenantA, booking.CreatePublicAppointmentInput{
		TherapistID: fromPgUUID(thA.ID),
		SlotID:      fromPgUUID(slotA.ID),
		GuestName:   "مراجعه‌کننده الف",
		GuestPhone:  "09125556677",
	}, t0)
	if err != nil {
		t.Fatalf("create in tenant A failed: %v", err)
	}

	// Tenant B tries to access Tenant A's appointment
	_, err = svc.GetPublicAppointmentStatus(ctx, tenantB, appA.ID, appA.GuestLookupHash, t0)
	if !errors.Is(err, booking.ErrAppointmentNotFound) {
		t.Errorf("CRITICAL SECURITY VIOLATION: Tenant B accessed Tenant A's appointment: %v", err)
	}

	// Tenant B tries to cancel Tenant A's appointment
	_, err = svc.CancelPublicAppointment(ctx, tenantB, appA.ID, appA.GuestLookupHash, "حمله", t0)
	if !errors.Is(err, booking.ErrAppointmentNotFound) {
		t.Errorf("CRITICAL SECURITY VIOLATION: Tenant B cancelled Tenant A's appointment: %v", err)
	}
}

func TestPublicAppointment_HTTPEndpoints(t *testing.T) {
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	t0 := time.Now().UTC()
	slotStart := t0.Add(24 * time.Hour)
	slotEnd := slotStart.Add(45 * time.Minute)

	th, err := svc.CreateTherapist(context.Background(), tenantID, booking.CreateTherapistInput{
		Name:                   "دکتر کیانی",
		Slug:                   "dr-kiani",
		SessionDurationMinutes: 45,
		IsActive:               true,
	})
	if err != nil {
		t.Fatalf("create therapist: %v", err)
	}
	thID := fromPgUUID(th.ID)

	slot, err := q.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotEnd),
		Status:      "available",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}
	slotID := fromPgUUID(slot.ID)

	domQ := newMockDomainQuerier()
	domQ.domains["clinic-test.ir"] = domaindb.Domain{
		TenantID:   toPgUUID(tenantID),
		Host:       "clinic-test.ir",
		IsVerified: true,
	}
	dr := domain.NewResolver(domQ)
	authSvc := auth.NewService(newMockAuthQuerier())

	h := booking.NewHandler(svc, dr, authSvc)
	r := chi.NewRouter()
	h.MountRoutes(r)

	// 1. POST /public/appointments
	bodyJSON, _ := json.Marshal(map[string]any{
		"therapist_id": thID.String(),
		"slot_id":      slotID.String(),
		"guest_name":   "رویا حسینی",
		"guest_phone":  "09121234567",
		"notes":        "جلسه مشاوره فردی",
	})
	req := httptest.NewRequest("POST", "/public/appointments", bytes.NewReader(bodyJSON))
	req.Host = "clinic-test.ir"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&res)

	appID := res["id"].(string)
	hash := res["guest_lookup_hash"].(string)

	if _, hasPhone := res["guest_phone"]; hasPhone {
		t.Errorf("CRITICAL PRIVACY BREACH: guest_phone returned in public appointment response!")
	}
	if res["status"] != "pending" {
		t.Errorf("expected pending status, got %s", res["status"])
	}

	// 2. Second POST for same slot returns 409 Conflict (zero oversell)
	req2 := httptest.NewRequest("POST", "/public/appointments", bytes.NewReader(bodyJSON))
	req2.Host = "clinic-test.ir"
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// 3. GET /public/appointments/{id} without hash -> 404
	req3 := httptest.NewRequest("GET", "/public/appointments/"+appID, nil)
	req3.Host = "clinic-test.ir"
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found without hash, got %d", rec3.Code)
	}

	// 4. GET /public/appointments/{id} with valid hash -> 200 OK
	req4 := httptest.NewRequest("GET", "/public/appointments/"+appID, nil)
	req4.Host = "clinic-test.ir"
	req4.Header.Set("X-Guest-Lookup-Hash", hash)
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req4)

	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec4.Code)
	}

	// 5. POST /public/appointments/{id}/cancel with valid hash -> 200 OK
	cancelJSON, _ := json.Marshal(map[string]any{"reason": "انصراف مراجع"})
	req5 := httptest.NewRequest("POST", "/public/appointments/"+appID+"/cancel", bytes.NewReader(cancelJSON))
	req5.Host = "clinic-test.ir"
	req5.Header.Set("X-Guest-Lookup-Hash", hash)
	rec5 := httptest.NewRecorder()
	r.ServeHTTP(rec5, req5)

	if rec5.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for cancel, got %d: %s", rec5.Code, rec5.Body.String())
	}

	// 6. Verify slot is now available again
	slBack, _ := q.GetAppointmentSlotByID(context.Background(), bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(slotID),
	})
	if slBack.Status != "available" {
		t.Errorf("expected slot back to available after cancel, got %s", slBack.Status)
	}
}

// ----------------------------------------------------
// PSY-2.4 Admin Appointment Management & RBAC Tests
// ----------------------------------------------------

func TestAdminAppointment_Confirmation(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	ownerUserID := uuid.New()
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	th, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:     "دکتر حسینی",
		Slug:     "dr-hosseini",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	slotStart := t0.Add(24 * time.Hour)
	slot, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotStart.Add(45 * time.Minute)),
		Status:      "available",
	})
	slotID := fromPgUUID(slot.ID)

	app, err := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      slotID,
		GuestName:   "مینا شمس",
		GuestPhone:  "09123332211",
	}, t0)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// 1. Confirm pending appointment
	confirmed, err := svc.ConfirmAdminAppointment(ctx, tenantID, app.ID, "receptionist", ownerUserID, t0.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if confirmed.Status != "confirmed" || confirmed.ConfirmedAt == nil {
		t.Errorf("expected confirmed status and non-nil confirmed_at")
	}

	// Slot status must now be 'booked'
	slotCheck, _ := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot.ID,
	})
	if slotCheck.Status != "booked" {
		t.Errorf("expected slot status 'booked', got %s", slotCheck.Status)
	}

	// History must record 'confirmed'
	hist, _ := svc.GetAdminAppointmentHistory(ctx, tenantID, app.ID, "owner", ownerUserID)
	if len(hist) != 2 || hist[1].Action != "confirmed" || hist[1].ActorType != "staff" {
		t.Errorf("expected confirmed history record, got %+v", hist)
	}

	// 2. Confirming an already confirmed appointment fails
	_, err = svc.ConfirmAdminAppointment(ctx, tenantID, app.ID, "receptionist", ownerUserID, t0.Add(35*time.Minute))
	if err == nil {
		t.Errorf("expected error when confirming already confirmed appointment")
	}

	// 3. Confirming after expiration fails
	slot2, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart.Add(2 * time.Hour)),
		EndsAt:      toPgTimestamptz(slotStart.Add(2*time.Hour + 45*time.Minute)),
		Status:      "available",
	})
	app2, _ := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      fromPgUUID(slot2.ID),
		GuestName:   "علی پناهی",
		GuestPhone:  "09121112233",
	}, t0)

	// Attempt confirm after 2 hours
	_, err = svc.ConfirmAdminAppointment(ctx, tenantID, app2.ID, "owner", ownerUserID, t0.Add(2*time.Hour+5*time.Minute))
	if err == nil {
		t.Errorf("expected error confirming expired appointment")
	}
}

func TestAdminAppointment_Cancellation(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	staffUserID := uuid.New()
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	th, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:     "دکتر کاویانی",
		Slug:     "dr-kavyani",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	slotStart := t0.Add(24 * time.Hour)
	slot, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(slotStart),
		EndsAt:      toPgTimestamptz(slotStart.Add(45 * time.Minute)),
		Status:      "available",
	})
	slotID := fromPgUUID(slot.ID)

	app, _ := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      slotID,
		GuestName:   "مریم قنبری",
		GuestPhone:  "09124445566",
	}, t0)

	// Confirm first
	_, _ = svc.ConfirmAdminAppointment(ctx, tenantID, app.ID, "owner", staffUserID, t0.Add(10*time.Minute))

	// Staff cancels confirmed appointment
	cancelled, err := svc.CancelAdminAppointment(ctx, tenantID, app.ID, "receptionist", staffUserID, "هماهنگی لغو تلفنی", t0.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if cancelled.Status != "cancelled" || cancelled.CancelledAt == nil {
		t.Errorf("expected cancelled status")
	}

	// Slot must be returned to available
	slotCheck, _ := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot.ID,
	})
	if slotCheck.Status != "available" {
		t.Errorf("expected slot to return to available, got %s", slotCheck.Status)
	}

	// Cancelling again fails
	_, err = svc.CancelAdminAppointment(ctx, tenantID, app.ID, "receptionist", staffUserID, "لغو مجدد", t0.Add(2*time.Hour))
	if !errors.Is(err, booking.ErrCannotCancel) {
		t.Errorf("expected ErrCannotCancel, got %v", err)
	}
}

func TestAdminAppointment_Rescheduling_AtomicLockAndCollision(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	staffUserID := uuid.New()
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	th, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		Name:     "دکتر طاهری",
		Slug:     "dr-taheri",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	// Slot 1 (initial appointment)
	s1Start := t0.Add(24 * time.Hour)
	slot1, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(s1Start),
		EndsAt:      toPgTimestamptz(s1Start.Add(45 * time.Minute)),
		Status:      "available",
	})
	s1ID := fromPgUUID(slot1.ID)

	// Slot 2 (empty target slot)
	s2Start := t0.Add(26 * time.Hour)
	slot2, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(s2Start),
		EndsAt:      toPgTimestamptz(s2Start.Add(45 * time.Minute)),
		Status:      "available",
	})
	s2ID := fromPgUUID(slot2.ID)

	// Slot 3 (already booked slot)
	s3Start := t0.Add(28 * time.Hour)
	slot3, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(s3Start),
		EndsAt:      toPgTimestamptz(s3Start.Add(45 * time.Minute)),
		Status:      "booked",
	})
	s3ID := fromPgUUID(slot3.ID)

	// Create and confirm appointment on Slot 1
	app, _ := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      s1ID,
		GuestName:   "کیان آریا",
		GuestPhone:  "09127778899",
	}, t0)
	_, _ = svc.ConfirmAdminAppointment(ctx, tenantID, app.ID, "owner", staffUserID, t0.Add(10*time.Minute))

	// 1. Reschedule collision test: target slot 3 is full -> must fail, old appointment untouched!
	_, err := svc.RescheduleAdminAppointment(ctx, tenantID, app.ID, s3ID, "receptionist", staffUserID, "تغییر ساعت", t0.Add(1*time.Hour))
	if !errors.Is(err, booking.ErrSlotNotAvailable) {
		t.Fatalf("expected ErrSlotNotAvailable on full target slot, got %v", err)
	}

	// Verify old appointment and Slot 1 are intact
	appCheck, _ := svc.GetAdminAppointment(ctx, tenantID, app.ID, "owner", staffUserID)
	if appCheck.SlotID != s1ID || appCheck.Status != "confirmed" {
		t.Errorf("CRITICAL INVARIANT BREACH: appointment changed when target slot was full!")
	}
	s1Check, _ := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot1.ID,
	})
	if s1Check.Status != "booked" {
		t.Errorf("CRITICAL INVARIANT BREACH: previous slot was altered when target slot was full!")
	}

	// 2. Successful atomic reschedule to Slot 2
	rescheduled, err := svc.RescheduleAdminAppointment(ctx, tenantID, app.ID, s2ID, "receptionist", staffUserID, "هماهنگی تغییر ساعت مراجع", t0.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("reschedule to empty slot failed: %v", err)
	}
	if rescheduled.SlotID != s2ID || rescheduled.Status != "confirmed" {
		t.Errorf("expected appointment moved to slot 2, got slot: %s", rescheduled.SlotID)
	}

	// Slot 2 must now be 'booked'
	s2Check, _ := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot2.ID,
	})
	if s2Check.Status != "booked" {
		t.Errorf("expected slot 2 to be booked, got %s", s2Check.Status)
	}

	// Slot 1 must now be freed to 'available'
	s1Freed, _ := q.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot1.ID,
	})
	if s1Freed.Status != "available" {
		t.Errorf("expected slot 1 to be freed to available, got %s", s1Freed.Status)
	}

	// History must record 'rescheduled' with metadata
	hist, _ := svc.GetAdminAppointmentHistory(ctx, tenantID, app.ID, "owner", staffUserID)
	lastHist := hist[len(hist)-1]
	if lastHist.Action != "rescheduled" || lastHist.ActorType != "staff" {
		t.Errorf("expected rescheduled action, got %+v", lastHist)
	}
	if !bytes.Contains(lastHist.Metadata, []byte(s1ID.String())) || !bytes.Contains(lastHist.Metadata, []byte(s2ID.String())) {
		t.Errorf("expected history metadata to contain previous and new slot IDs, got: %s", string(lastHist.Metadata))
	}
}

func TestAdminAppointment_TherapistAccessControl(t *testing.T) {
	ctx := context.Background()
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	userTh1 := uuid.New()
	userTh2 := uuid.New()
	t0 := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	// Create Therapist 1 (linked to userTh1)
	th1, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		UserID:   &userTh1,
		Name:     "دکتر ۱",
		Slug:     "dr-1",
		IsActive: true,
	})
	th1ID := fromPgUUID(th1.ID)

	// Create Therapist 2 (linked to userTh2)
	th2, _ := svc.CreateTherapist(ctx, tenantID, booking.CreateTherapistInput{
		UserID:   &userTh2,
		Name:     "دکتر ۲",
		Slug:     "dr-2",
		IsActive: true,
	})
	th2ID := fromPgUUID(th2.ID)

	slot1, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th1.ID,
		StartsAt:    toPgTimestamptz(t0.Add(24 * time.Hour)),
		EndsAt:      toPgTimestamptz(t0.Add(24*time.Hour + 45*time.Minute)),
		Status:      "available",
	})
	slot2, _ := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th2.ID,
		StartsAt:    toPgTimestamptz(t0.Add(24 * time.Hour)),
		EndsAt:      toPgTimestamptz(t0.Add(24*time.Hour + 45*time.Minute)),
		Status:      "available",
	})

	app1, _ := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: th1ID,
		SlotID:      fromPgUUID(slot1.ID),
		GuestName:   "بیمار دکتر ۱",
		GuestPhone:  "09121111111",
	}, t0)

	app2, _ := svc.CreatePublicAppointment(ctx, tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: th2ID,
		SlotID:      fromPgUUID(slot2.ID),
		GuestName:   "بیمار دکتر ۲",
		GuestPhone:  "09122222222",
	}, t0)

	// 1. Therapist 1 CAN read, confirm, cancel their own appointment app1
	detail1, err := svc.GetAdminAppointment(ctx, tenantID, app1.ID, "therapist", userTh1)
	if err != nil || detail1.GuestName != "بیمار دکتر ۱" {
		t.Fatalf("therapist 1 failed to view own appointment: %v", err)
	}

	_, err = svc.ConfirmAdminAppointment(ctx, tenantID, app1.ID, "therapist", userTh1, t0.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("therapist 1 failed to confirm own appointment: %v", err)
	}

	// 2. Therapist 1 CANNOT view, confirm, cancel, or view history of Therapist 2's app2
	_, err = svc.GetAdminAppointment(ctx, tenantID, app2.ID, "therapist", userTh1)
	if !errors.Is(err, booking.ErrUnauthorizedTherapist) {
		t.Errorf("CRITICAL RBAC VIOLATION: therapist 1 accessed therapist 2 appointment: %v", err)
	}

	_, err = svc.ConfirmAdminAppointment(ctx, tenantID, app2.ID, "therapist", userTh1, t0.Add(10*time.Minute))
	if !errors.Is(err, booking.ErrUnauthorizedTherapist) {
		t.Errorf("CRITICAL RBAC VIOLATION: therapist 1 confirmed therapist 2 appointment: %v", err)
	}

	_, err = svc.CancelAdminAppointment(ctx, tenantID, app2.ID, "therapist", userTh1, "لغو غیرمجاز", t0.Add(10*time.Minute))
	if !errors.Is(err, booking.ErrUnauthorizedTherapist) {
		t.Errorf("CRITICAL RBAC VIOLATION: therapist 1 cancelled therapist 2 appointment: %v", err)
	}

	_, err = svc.GetAdminAppointmentHistory(ctx, tenantID, app2.ID, "therapist", userTh1)
	if !errors.Is(err, booking.ErrUnauthorizedTherapist) {
		t.Errorf("CRITICAL RBAC VIOLATION: therapist 1 accessed therapist 2 history: %v", err)
	}

	// 3. Therapist 1 listing appointments: automatically restricted to therapist 1
	list1, err := svc.ListAdminAppointments(ctx, tenantID, "therapist", userTh1, nil, "")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list1) != 1 || list1[0].TherapistID != th1ID {
		t.Errorf("expected only therapist 1 appointments, got: %+v", list1)
	}

	// 4. Receptionist CAN view all appointments in clinic
	listAll, err := svc.ListAdminAppointments(ctx, tenantID, "receptionist", uuid.New(), nil, "")
	if err != nil || len(listAll) != 2 {
		t.Errorf("receptionist should see all clinic appointments, got: %+v", listAll)
	}
}

func TestAdminAppointment_HTTPEndpoints(t *testing.T) {
	q := newMockBookingQuerier()
	svc := booking.NewService(q)

	tenantID := uuid.New()
	t0 := time.Now().UTC()
	s1Start := t0.Add(24 * time.Hour)
	s2Start := t0.Add(26 * time.Hour)

	th, _ := svc.CreateTherapist(context.Background(), tenantID, booking.CreateTherapistInput{
		Name:     "دکتر شجاعی",
		Slug:     "dr-shojaei",
		IsActive: true,
	})
	thID := fromPgUUID(th.ID)

	slot1, _ := q.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(s1Start),
		EndsAt:      toPgTimestamptz(s1Start.Add(45 * time.Minute)),
		Status:      "available",
	})
	s1ID := fromPgUUID(slot1.ID)

	slot2, _ := q.CreateAppointmentSlot(context.Background(), bookingdb.CreateAppointmentSlotParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: th.ID,
		StartsAt:    toPgTimestamptz(s2Start),
		EndsAt:      toPgTimestamptz(s2Start.Add(45 * time.Minute)),
		Status:      "available",
	})
	s2ID := fromPgUUID(slot2.ID)

	app, _ := svc.CreatePublicAppointment(context.Background(), tenantID, booking.CreatePublicAppointmentInput{
		TherapistID: thID,
		SlotID:      s1ID,
		GuestName:   "بهار صادقی",
		GuestPhone:  "09121113355",
	}, t0)

	domQ := newMockDomainQuerier()
	domQ.domains["clinic-test.ir"] = domaindb.Domain{
		TenantID:   toPgUUID(tenantID),
		Host:       "clinic-test.ir",
		IsVerified: true,
	}
	dr := domain.NewResolver(domQ)

	aq := newMockAuthQuerier()
	ownerPass, _ := auth.HashPassword("SecretPassword123!")
	ownerID := uuid.New()
	aq.identities["owner@test.ir"] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerID),
		Email:        "owner@test.ir",
		PasswordHash: ownerPass,
		Status:       "active",
	}
	aq.memberships[fmt.Sprintf("%s:%s", ownerID.String(), tenantID.String())] = authdb.Membership{
		TenantID: toPgUUID(tenantID),
		UserID:   toPgUUID(ownerID),
		Role:     "owner",
	}

	authSvc := auth.NewService(aq)
	sess, _ := authSvc.Login(context.Background(), "owner@test.ir", "SecretPassword123!")

	h := booking.NewHandler(svc, dr, authSvc)
	r := chi.NewRouter()
	h.MountRoutes(r)

	// Helper for admin request
	makeAdminReq := func(method, path string, body any) *httptest.ResponseRecorder {
		var bodyReader *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(b)
		} else {
			bodyReader = bytes.NewReader([]byte{})
		}
		req := httptest.NewRequest(method, path, bodyReader)
		req.Host = "clinic-test.ir"
		req.Header.Set("Authorization", "Bearer "+sess.Token)
		req.Header.Set("X-Tenant-ID", tenantID.String())
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 1. GET /admin/appointments
	rec := makeAdminReq("GET", "/admin/appointments", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list admin appointments failed: %d: %s", rec.Code, rec.Body.String())
	}
	var listResp []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&listResp)
	if len(listResp) != 1 || listResp[0]["id"] != app.ID.String() {
		t.Errorf("unexpected list response: %+v", listResp)
	}

	// 2. GET /admin/appointments/{id}
	rec = makeAdminReq("GET", "/admin/appointments/"+app.ID.String(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get admin appointment failed: %d", rec.Code)
	}
	var detailResp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&detailResp)
	if detailResp["guest_name"] != "بهار صادقی" || detailResp["status"] != "pending" {
		t.Errorf("unexpected detail response: %+v", detailResp)
	}

	// 3. POST /admin/appointments/{id}/confirm
	rec = makeAdminReq("POST", "/admin/appointments/"+app.ID.String()+"/confirm", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm admin appointment failed: %d: %s", rec.Code, rec.Body.String())
	}
	_ = json.NewDecoder(rec.Body).Decode(&detailResp)
	if detailResp["status"] != "confirmed" {
		t.Errorf("expected confirmed status, got %s", detailResp["status"])
	}

	// 4. POST /admin/appointments/{id}/reschedule
	rec = makeAdminReq("POST", "/admin/appointments/"+app.ID.String()+"/reschedule", map[string]any{
		"new_slot_id": s2ID.String(),
		"reason":      "جابه‌جایی نوبت با تماس تلفنی",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reschedule admin appointment failed: %d: %s", rec.Code, rec.Body.String())
	}
	_ = json.NewDecoder(rec.Body).Decode(&detailResp)
	if detailResp["slot_id"] != s2ID.String() {
		t.Errorf("expected slot_id to be %s, got %s", s2ID.String(), detailResp["slot_id"])
	}

	// 5. GET /admin/appointments/{id}/history
	rec = makeAdminReq("GET", "/admin/appointments/"+app.ID.String()+"/history", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get history failed: %d", rec.Code)
	}
	var histResp []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&histResp)
	if len(histResp) != 3 {
		t.Errorf("expected 3 history records (created, confirmed, rescheduled), got %d", len(histResp))
	}

	// 6. POST /admin/appointments/{id}/cancel
	rec = makeAdminReq("POST", "/admin/appointments/"+app.ID.String()+"/cancel", map[string]any{
		"reason": "لغو به درخواست مراجع",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel admin appointment failed: %d: %s", rec.Code, rec.Body.String())
	}
	_ = json.NewDecoder(rec.Body).Decode(&detailResp)
	if detailResp["status"] != "cancelled" {
		t.Errorf("expected cancelled status, got %s", detailResp["status"])
	}
}


