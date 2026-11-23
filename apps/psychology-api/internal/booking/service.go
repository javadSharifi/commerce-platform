package booking

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"psychology/api/internal/booking/bookingdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrTherapistNotFound     = errors.New("booking: therapist not found")
	ErrSlotNotFound          = errors.New("booking: appointment slot not found")
	ErrSlotNotAvailable      = errors.New("booking: appointment slot is not available")
	ErrSlotInThePast         = errors.New("booking: cannot book or generate slot in the past")
	ErrDateRangeTooLarge     = errors.New("booking: date range exceeds maximum allowed 30 days")
	ErrInvalidWorkingHours   = errors.New("booking: start_time must be strictly before end_time")
	ErrExceptionNotFound     = errors.New("booking: schedule exception not found")
	ErrTherapistInactive     = errors.New("booking: therapist is inactive")
	ErrUnauthorizedTherapist = errors.New("booking: unauthorized to manage this therapist calendar")
	ErrAppointmentNotFound   = errors.New("booking: appointment not found")
	ErrInvalidGuestName      = errors.New("booking: guest name cannot be empty")
	ErrInvalidPhone          = errors.New("booking: invalid iranian mobile phone number")
	ErrCannotCancel          = errors.New("booking: appointment cannot be cancelled in its current state")
	ErrCannotCancelPastSlot  = errors.New("booking: cannot cancel appointment after session start time")
	ErrRateLimitExceeded     = errors.New("booking: rate limit exceeded, please try again later")
)

var phoneRegex = regexp.MustCompile(`^09[0-9]{9}$`)

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

func GenerateLookupHash() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type Querier interface {
	CreateTherapist(ctx context.Context, arg bookingdb.CreateTherapistParams) (bookingdb.Therapist, error)
	GetTherapistByID(ctx context.Context, arg bookingdb.GetTherapistByIDParams) (bookingdb.Therapist, error)
	GetTherapistBySlug(ctx context.Context, arg bookingdb.GetTherapistBySlugParams) (bookingdb.Therapist, error)
	GetTherapistByUserID(ctx context.Context, arg bookingdb.GetTherapistByUserIDParams) (bookingdb.Therapist, error)
	ListTherapists(ctx context.Context, tenantID pgtype.UUID) ([]bookingdb.Therapist, error)
	ListActiveTherapists(ctx context.Context, tenantID pgtype.UUID) ([]bookingdb.Therapist, error)
	UpdateTherapist(ctx context.Context, arg bookingdb.UpdateTherapistParams) (bookingdb.Therapist, error)
	SoftDeleteTherapist(ctx context.Context, arg bookingdb.SoftDeleteTherapistParams) error

	ListWorkingHoursByTherapist(ctx context.Context, arg bookingdb.ListWorkingHoursByTherapistParams) ([]bookingdb.TherapistWorkingHour, error)
	DeleteWorkingHoursByTherapist(ctx context.Context, arg bookingdb.DeleteWorkingHoursByTherapistParams) error
	CreateWorkingHour(ctx context.Context, arg bookingdb.CreateWorkingHourParams) (bookingdb.TherapistWorkingHour, error)

	ListScheduleExceptionsByTherapist(ctx context.Context, arg bookingdb.ListScheduleExceptionsByTherapistParams) ([]bookingdb.TherapistScheduleException, error)
	GetScheduleExceptionByID(ctx context.Context, arg bookingdb.GetScheduleExceptionByIDParams) (bookingdb.TherapistScheduleException, error)
	CreateScheduleException(ctx context.Context, arg bookingdb.CreateScheduleExceptionParams) (bookingdb.TherapistScheduleException, error)
	DeleteScheduleException(ctx context.Context, arg bookingdb.DeleteScheduleExceptionParams) error

	CreateAppointmentSlot(ctx context.Context, arg bookingdb.CreateAppointmentSlotParams) (bookingdb.AppointmentSlot, error)
	GetAppointmentSlotByID(ctx context.Context, arg bookingdb.GetAppointmentSlotByIDParams) (bookingdb.AppointmentSlot, error)
	GetAppointmentSlotByIDForUpdate(ctx context.Context, arg bookingdb.GetAppointmentSlotByIDForUpdateParams) (bookingdb.AppointmentSlot, error)
	ListSlotsByTherapist(ctx context.Context, arg bookingdb.ListSlotsByTherapistParams) ([]bookingdb.AppointmentSlot, error)
	ListAvailableSlots(ctx context.Context, arg bookingdb.ListAvailableSlotsParams) ([]bookingdb.AppointmentSlot, error)
	DeleteFutureAvailableSlots(ctx context.Context, arg bookingdb.DeleteFutureAvailableSlotsParams) error
	UpdateSlotStatus(ctx context.Context, arg bookingdb.UpdateSlotStatusParams) (bookingdb.AppointmentSlot, error)

	CreateAppointment(ctx context.Context, arg bookingdb.CreateAppointmentParams) (bookingdb.Appointment, error)
	GetAppointmentByID(ctx context.Context, arg bookingdb.GetAppointmentByIDParams) (bookingdb.Appointment, error)
	GetAppointmentByIDForUpdate(ctx context.Context, arg bookingdb.GetAppointmentByIDForUpdateParams) (bookingdb.Appointment, error)
	GetAppointmentBySlotID(ctx context.Context, arg bookingdb.GetAppointmentBySlotIDParams) (bookingdb.Appointment, error)
	GetAppointmentDetail(ctx context.Context, arg bookingdb.GetAppointmentDetailParams) (bookingdb.GetAppointmentDetailRow, error)
	ListAdminAppointments(ctx context.Context, arg bookingdb.ListAdminAppointmentsParams) ([]bookingdb.ListAdminAppointmentsRow, error)
	UpdateAppointmentStatus(ctx context.Context, arg bookingdb.UpdateAppointmentStatusParams) (bookingdb.Appointment, error)
	UpdateAppointmentSlot(ctx context.Context, arg bookingdb.UpdateAppointmentSlotParams) (bookingdb.Appointment, error)
	ListAppointmentsByTherapist(ctx context.Context, arg bookingdb.ListAppointmentsByTherapistParams) ([]bookingdb.Appointment, error)
	ListExpiredPendingAppointments(ctx context.Context, arg bookingdb.ListExpiredPendingAppointmentsParams) ([]bookingdb.Appointment, error)

	CreateAppointmentStatusHistory(ctx context.Context, arg bookingdb.CreateAppointmentStatusHistoryParams) (bookingdb.AppointmentStatusHistory, error)
	ListAppointmentHistory(ctx context.Context, arg bookingdb.ListAppointmentHistoryParams) ([]bookingdb.AppointmentStatusHistory, error)
}

type Service struct {
	scoped      bool
	queries     Querier
	tehranLoc   *time.Location
	rateLimiter *RateLimiter
	slotLocks   sync.Map
}

func NewService(q Querier) *Service {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		loc = time.FixedZone("Asia/Tehran", 3*3600+30*60)
	}
	return &Service{
		queries:     q,
		tehranLoc:   loc,
		rateLimiter: NewRateLimiter(10, 1*time.Minute),
	}
}

func (s *Service) SetRateLimiter(rl *RateLimiter) {
	s.rateLimiter = rl
}

func (s *Service) getSlotLock(slotID uuid.UUID) *sync.Mutex {
	v, _ := s.slotLocks.LoadOrStore(slotID, &sync.Mutex{})
	return v.(*sync.Mutex)
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

func parseMicrosecondsToTime(micros int64) (hour, min, sec int) {
	totalSeconds := micros / 1000000
	hour = int(totalSeconds / 3600)
	min = int((totalSeconds % 3600) / 60)
	sec = int(totalSeconds % 60)
	return
}

func formatTimeHMS(h, m, sec int) string {
	return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
}

func timeStringToMicroseconds(s string) (int64, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid time format: %s", s)
	}
	var h, m, sec int
	fmt.Sscanf(parts[0], "%d", &h)
	fmt.Sscanf(parts[1], "%d", &m)
	if len(parts) >= 3 {
		fmt.Sscanf(parts[2], "%d", &sec)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 || sec < 0 || sec > 59 {
		return 0, fmt.Errorf("time out of range: %s", s)
	}
	return int64(h*3600+m*60+sec) * 1000000, nil
}

// GoWeekdayToDayOfWeek maps standard Go weekday (Sunday=0..Saturday=6) to Iran week order (Saturday=0..Friday=6)
func GoWeekdayToDayOfWeek(w time.Weekday) int16 {
	switch w {
	case time.Saturday:
		return 0
	case time.Sunday:
		return 1
	case time.Monday:
		return 2
	case time.Tuesday:
		return 3
	case time.Wednesday:
		return 4
	case time.Thursday:
		return 5
	case time.Friday:
		return 6
	}
	return 0
}

// ----------------------------------------------------
// Therapist Profile Management
// ----------------------------------------------------

type CreateTherapistInput struct {
	UserID                 *uuid.UUID
	Name                   string
	Slug                   string
	Title                  string
	Bio                    string
	AvatarAssetID          *uuid.UUID
	SessionDurationMinutes int32
	IsActive               bool
}

func (s *Service) CreateTherapist(ctx context.Context, tenantID uuid.UUID, in CreateTherapistInput) (*bookingdb.Therapist, error) {
	if in.SessionDurationMinutes <= 0 {
		in.SessionDurationMinutes = 45
	}
	var uid pgtype.UUID
	if in.UserID != nil {
		uid = toPgUUID(*in.UserID)
	}
	var avatarID pgtype.UUID
	if in.AvatarAssetID != nil {
		avatarID = toPgUUID(*in.AvatarAssetID)
	}

	th, err := s.queries.CreateTherapist(ctx, bookingdb.CreateTherapistParams{
		TenantID:               toPgUUID(tenantID),
		UserID:                 uid,
		Name:                   in.Name,
		Slug:                   in.Slug,
		Title:                  in.Title,
		Bio:                    in.Bio,
		AvatarAssetID:          avatarID,
		SessionDurationMinutes: in.SessionDurationMinutes,
		IsActive:               in.IsActive,
	})
	if err != nil {
		return nil, fmt.Errorf("create therapist: %w", err)
	}
	return &th, nil
}

func (s *Service) GetTherapistByID(ctx context.Context, tenantID, therapistID uuid.UUID) (*bookingdb.Therapist, error) {
	th, err := s.queries.GetTherapistByID(ctx, bookingdb.GetTherapistByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(therapistID),
	})
	if err != nil {
		return nil, ErrTherapistNotFound
	}
	return &th, nil
}

func (s *Service) GetTherapistBySlug(ctx context.Context, tenantID uuid.UUID, slug string) (*bookingdb.Therapist, error) {
	th, err := s.queries.GetTherapistBySlug(ctx, bookingdb.GetTherapistBySlugParams{
		TenantID: toPgUUID(tenantID),
		Slug:     slug,
	})
	if err != nil {
		return nil, ErrTherapistNotFound
	}
	return &th, nil
}

func (s *Service) ListTherapists(ctx context.Context, tenantID uuid.UUID) ([]bookingdb.Therapist, error) {
	return s.queries.ListTherapists(ctx, toPgUUID(tenantID))
}

func (s *Service) ListActiveTherapists(ctx context.Context, tenantID uuid.UUID) ([]bookingdb.Therapist, error) {
	return s.queries.ListActiveTherapists(ctx, toPgUUID(tenantID))
}

type UpdateTherapistInput struct {
	UserID                 *uuid.UUID
	Name                   string
	Slug                   string
	Title                  string
	Bio                    string
	AvatarAssetID          *uuid.UUID
	SessionDurationMinutes int32
	IsActive               bool
}

func (s *Service) UpdateTherapist(ctx context.Context, tenantID, therapistID uuid.UUID, in UpdateTherapistInput) (*bookingdb.Therapist, error) {
	if in.SessionDurationMinutes <= 0 {
		in.SessionDurationMinutes = 45
	}
	var uid pgtype.UUID
	if in.UserID != nil {
		uid = toPgUUID(*in.UserID)
	}
	var avatarID pgtype.UUID
	if in.AvatarAssetID != nil {
		avatarID = toPgUUID(*in.AvatarAssetID)
	}

	th, err := s.queries.UpdateTherapist(ctx, bookingdb.UpdateTherapistParams{
		TenantID:               toPgUUID(tenantID),
		ID:                     toPgUUID(therapistID),
		UserID:                 uid,
		Name:                   in.Name,
		Slug:                   in.Slug,
		Title:                  in.Title,
		Bio:                    in.Bio,
		AvatarAssetID:          avatarID,
		SessionDurationMinutes: in.SessionDurationMinutes,
		IsActive:               in.IsActive,
	})
	if err != nil {
		return nil, ErrTherapistNotFound
	}
	return &th, nil
}

func (s *Service) DeleteTherapist(ctx context.Context, tenantID, therapistID uuid.UUID) error {
	return s.queries.SoftDeleteTherapist(ctx, bookingdb.SoftDeleteTherapistParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(therapistID),
	})
}

// ----------------------------------------------------
// Working Hours & Schedule Exceptions
// ----------------------------------------------------

type WorkingHourInput struct {
	DayOfWeek int16
	StartTime string // "09:00:00"
	EndTime   string // "17:00:00"
	IsActive  bool
}

func (s *Service) ListWorkingHours(ctx context.Context, tenantID, therapistID uuid.UUID) ([]bookingdb.TherapistWorkingHour, error) {
	return s.queries.ListWorkingHoursByTherapist(ctx, bookingdb.ListWorkingHoursByTherapistParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
	})
}

func (s *Service) SetWorkingHours(ctx context.Context, tenantID, therapistID uuid.UUID, items []WorkingHourInput) ([]bookingdb.TherapistWorkingHour, error) {
	// 1. Verify therapist exists
	_, err := s.GetTherapistByID(ctx, tenantID, therapistID)
	if err != nil {
		return nil, err
	}

	// 2. Validate hours
	for _, it := range items {
		startMicros, err := timeStringToMicroseconds(it.StartTime)
		if err != nil {
			return nil, err
		}
		endMicros, err := timeStringToMicroseconds(it.EndTime)
		if err != nil {
			return nil, err
		}
		if startMicros >= endMicros {
			return nil, ErrInvalidWorkingHours
		}
	}

	// 3. Clear existing and rewrite
	if err := s.queries.DeleteWorkingHoursByTherapist(ctx, bookingdb.DeleteWorkingHoursByTherapistParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
	}); err != nil {
		return nil, fmt.Errorf("delete working hours: %w", err)
	}

	results := make([]bookingdb.TherapistWorkingHour, 0, len(items))
	for _, it := range items {
		startMicros, _ := timeStringToMicroseconds(it.StartTime)
		endMicros, _ := timeStringToMicroseconds(it.EndTime)

		wh, err := s.queries.CreateWorkingHour(ctx, bookingdb.CreateWorkingHourParams{
			TenantID:    toPgUUID(tenantID),
			TherapistID: toPgUUID(therapistID),
			DayOfWeek:   it.DayOfWeek,
			StartTime:   pgtype.Time{Microseconds: startMicros, Valid: true},
			EndTime:     pgtype.Time{Microseconds: endMicros, Valid: true},
			IsActive:    it.IsActive,
		})
		if err != nil {
			return nil, fmt.Errorf("create working hour: %w", err)
		}
		results = append(results, wh)
	}

	// 4. Invalidate future available slots
	// Deletes only available future slots; reserved/booked slots are left intact
	nowUTC := time.Now().UTC()
	_ = s.queries.DeleteFutureAvailableSlots(ctx, bookingdb.DeleteFutureAvailableSlotsParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
		StartsAt:    toPgTimestamptz(nowUTC),
	})

	return results, nil
}

type ScheduleExceptionInput struct {
	Date          time.Time
	IsUnavailable bool
	StartTime     string // optional
	EndTime       string // optional
	Reason        string
}

func (s *Service) ListScheduleExceptions(ctx context.Context, tenantID, therapistID uuid.UUID) ([]bookingdb.TherapistScheduleException, error) {
	return s.queries.ListScheduleExceptionsByTherapist(ctx, bookingdb.ListScheduleExceptionsByTherapistParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
	})
}

func (s *Service) CreateScheduleException(ctx context.Context, tenantID, therapistID uuid.UUID, in ScheduleExceptionInput) (*bookingdb.TherapistScheduleException, error) {
	_, err := s.GetTherapistByID(ctx, tenantID, therapistID)
	if err != nil {
		return nil, err
	}

	var startTimePg, endTimePg pgtype.Time
	if !in.IsUnavailable && in.StartTime != "" && in.EndTime != "" {
		sMicros, err := timeStringToMicroseconds(in.StartTime)
		if err != nil {
			return nil, err
		}
		eMicros, err := timeStringToMicroseconds(in.EndTime)
		if err != nil {
			return nil, err
		}
		if sMicros >= eMicros {
			return nil, ErrInvalidWorkingHours
		}
		startTimePg = pgtype.Time{Microseconds: sMicros, Valid: true}
		endTimePg = pgtype.Time{Microseconds: eMicros, Valid: true}
	}

	ex, err := s.queries.CreateScheduleException(ctx, bookingdb.CreateScheduleExceptionParams{
		TenantID:      toPgUUID(tenantID),
		TherapistID:   toPgUUID(therapistID),
		Date:          pgtype.Date{Time: in.Date, Valid: true},
		IsUnavailable: in.IsUnavailable,
		StartTime:     startTimePg,
		EndTime:       endTimePg,
		Reason:        in.Reason,
	})
	if err != nil {
		return nil, fmt.Errorf("create schedule exception: %w", err)
	}

	// Clean up future available slots
	nowUTC := time.Now().UTC()
	_ = s.queries.DeleteFutureAvailableSlots(ctx, bookingdb.DeleteFutureAvailableSlotsParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
		StartsAt:    toPgTimestamptz(nowUTC),
	})

	return &ex, nil
}

func (s *Service) DeleteScheduleException(ctx context.Context, tenantID, therapistID, exceptionID uuid.UUID) error {
	ex, err := s.queries.GetScheduleExceptionByID(ctx, bookingdb.GetScheduleExceptionByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(exceptionID),
	})
	if err != nil {
		return ErrExceptionNotFound
	}
	if fromPgUUID(ex.TherapistID) != therapistID {
		return ErrExceptionNotFound
	}

	if err := s.queries.DeleteScheduleException(ctx, bookingdb.DeleteScheduleExceptionParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(exceptionID),
	}); err != nil {
		return err
	}

	nowUTC := time.Now().UTC()
	_ = s.queries.DeleteFutureAvailableSlots(ctx, bookingdb.DeleteFutureAvailableSlotsParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
		StartsAt:    toPgTimestamptz(nowUTC),
	})

	return nil
}

// ----------------------------------------------------
// Slot Generation & Calendar Query
// ----------------------------------------------------

type GenerateSlotsResult struct {
	GeneratedCount int `json:"generated_count"`
	SkippedCount   int `json:"skipped_count"`
}

func (s *Service) GenerateSlots(ctx context.Context, tenantID, therapistID uuid.UUID, fromDate, toDate time.Time) (*GenerateSlotsResult, error) {
	// 1. Validate date range (max 30 days)
	if toDate.Sub(fromDate) > 31*24*time.Hour {
		return nil, ErrDateRangeTooLarge
	}

	th, err := s.GetTherapistByID(ctx, tenantID, therapistID)
	if err != nil {
		return nil, err
	}
	if !th.IsActive {
		return nil, ErrTherapistInactive
	}

	duration := time.Duration(th.SessionDurationMinutes) * time.Minute
	if duration <= 0 {
		duration = 45 * time.Minute
	}

	// 2. Load working hours
	workingHours, err := s.queries.ListWorkingHoursByTherapist(ctx, bookingdb.ListWorkingHoursByTherapistParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
	})
	if err != nil {
		return nil, fmt.Errorf("list working hours: %w", err)
	}

	// Map working hours by DayOfWeek
	hoursByDay := make(map[int16][]bookingdb.TherapistWorkingHour)
	for _, wh := range workingHours {
		if wh.IsActive {
			hoursByDay[wh.DayOfWeek] = append(hoursByDay[wh.DayOfWeek], wh)
		}
	}

	// 3. Load schedule exceptions
	exceptions, err := s.queries.ListScheduleExceptionsByTherapist(ctx, bookingdb.ListScheduleExceptionsByTherapistParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
	})
	if err != nil {
		return nil, fmt.Errorf("list exceptions: %w", err)
	}

	exceptionsByDate := make(map[string][]bookingdb.TherapistScheduleException)
	for _, ex := range exceptions {
		dateStr := ex.Date.Time.Format("2006-01-02")
		exceptionsByDate[dateStr] = append(exceptionsByDate[dateStr], ex)
	}

	// 4. Load existing slots in the range to avoid duplicate entries
	existingSlots, err := s.queries.ListSlotsByTherapist(ctx, bookingdb.ListSlotsByTherapistParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
		StartsAt:    toPgTimestamptz(fromDate),
		EndsAt:      toPgTimestamptz(toDate.AddDate(0, 0, 1)),
	})
	if err != nil {
		return nil, fmt.Errorf("list existing slots: %w", err)
	}

	existingSlotMap := make(map[int64]bool)
	for _, slot := range existingSlots {
		existingSlotMap[slot.StartsAt.Time.Unix()] = true
	}

	nowUTC := time.Now().UTC()
	result := &GenerateSlotsResult{}

	// Convert fromDate to Tehran day start
	curTehran := time.Date(fromDate.In(s.tehranLoc).Year(), fromDate.In(s.tehranLoc).Month(), fromDate.In(s.tehranLoc).Day(), 0, 0, 0, 0, s.tehranLoc)
	endTehran := time.Date(toDate.In(s.tehranLoc).Year(), toDate.In(s.tehranLoc).Month(), toDate.In(s.tehranLoc).Day(), 23, 59, 59, 0, s.tehranLoc)

	for !curTehran.After(endTehran) {
		dateStr := curTehran.Format("2006-01-02")
		dayExceptions := exceptionsByDate[dateStr]

		// Check full-day unavailable exception
		isFullDayBlocked := false
		for _, ex := range dayExceptions {
			if ex.IsUnavailable && !ex.StartTime.Valid {
				isFullDayBlocked = true
				break
			}
		}

		if !isFullDayBlocked {
			dayOfWeek := GoWeekdayToDayOfWeek(curTehran.Weekday())
			whs := hoursByDay[dayOfWeek]

			for _, wh := range whs {
				sh, sm, ss := parseMicrosecondsToTime(wh.StartTime.Microseconds)
				eh, em, es := parseMicrosecondsToTime(wh.EndTime.Microseconds)

				windowStart := time.Date(curTehran.Year(), curTehran.Month(), curTehran.Day(), sh, sm, ss, 0, s.tehranLoc)
				windowEnd := time.Date(curTehran.Year(), curTehran.Month(), curTehran.Day(), eh, em, es, 0, s.tehranLoc)

				for slotStart := windowStart; !slotStart.Add(duration).After(windowEnd); slotStart = slotStart.Add(duration) {
					slotEnd := slotStart.Add(duration)
					slotStartUTC := slotStart.UTC()
					slotEndUTC := slotEnd.UTC()

					// Skip slots in the past
					if slotStartUTC.Before(nowUTC) {
						result.SkippedCount++
						continue
					}

					// Check partial exceptions
					isBlockedByException := false
					for _, ex := range dayExceptions {
						if ex.StartTime.Valid && ex.EndTime.Valid {
							exSh, exSm, exSs := parseMicrosecondsToTime(ex.StartTime.Microseconds)
							exEh, exEm, exEs := parseMicrosecondsToTime(ex.EndTime.Microseconds)
							exStart := time.Date(curTehran.Year(), curTehran.Month(), curTehran.Day(), exSh, exSm, exSs, 0, s.tehranLoc)
							exEnd := time.Date(curTehran.Year(), curTehran.Month(), curTehran.Day(), exEh, exEm, exEs, 0, s.tehranLoc)

							// Overlap check
							if slotStart.Before(exEnd) && slotEnd.After(exStart) {
								isBlockedByException = true
								break
							}
						}
					}

					if isBlockedByException {
						result.SkippedCount++
						continue
					}

					// Check existing slot collision
					if existingSlotMap[slotStartUTC.Unix()] {
						result.SkippedCount++
						continue
					}

					// Create slot
					_, err := s.queries.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{
						TenantID:    toPgUUID(tenantID),
						TherapistID: toPgUUID(therapistID),
						StartsAt:    toPgTimestamptz(slotStartUTC),
						EndsAt:      toPgTimestamptz(slotEndUTC),
						Status:      "available",
					})
					if err != nil {
						// Likely unique conflict, skip
						result.SkippedCount++
						continue
					}

					existingSlotMap[slotStartUTC.Unix()] = true
					result.GeneratedCount++
				}
			}
		}

		curTehran = curTehran.AddDate(0, 0, 1)
	}

	return result, nil
}

func (s *Service) ListAvailableSlots(ctx context.Context, tenantID, therapistID uuid.UUID, fromDate time.Time) ([]bookingdb.AppointmentSlot, error) {
	nowUTC := time.Now().UTC()
	start := fromDate.UTC()
	if start.Before(nowUTC) {
		start = nowUTC
	}
	end := start.AddDate(0, 0, 30)

	return s.queries.ListAvailableSlots(ctx, bookingdb.ListAvailableSlotsParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
		StartsAt:    toPgTimestamptz(start),
		StartsAt_2:  toPgTimestamptz(end),
	})
}

func (s *Service) ListSlots(ctx context.Context, tenantID, therapistID uuid.UUID, from, to time.Time) ([]bookingdb.AppointmentSlot, error) {
	return s.queries.ListSlotsByTherapist(ctx, bookingdb.ListSlotsByTherapistParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: toPgUUID(therapistID),
		StartsAt:    toPgTimestamptz(from),
		EndsAt:      toPgTimestamptz(to),
	})
}

// ----------------------------------------------------
// Public Appointments Booking (PSY-2.3)
// ----------------------------------------------------

type CreatePublicAppointmentInput struct {
	TherapistID uuid.UUID
	SlotID      uuid.UUID
	GuestName   string
	GuestPhone  string
	Notes       string
	ClientIP    string
}

type AppointmentResult struct {
	ID              uuid.UUID `json:"id"`
	TherapistID     uuid.UUID `json:"therapist_id"`
	SlotID          uuid.UUID `json:"slot_id"`
	GuestName       string    `json:"guest_name"`
	GuestLookupHash string    `json:"guest_lookup_hash"`
	Status          string    `json:"status"`
	ExpiresAt       time.Time `json:"expires_at"`
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
}

func (s *Service) CreatePublicAppointment(ctx context.Context, tenantID uuid.UUID, in CreatePublicAppointmentInput, now time.Time) (*AppointmentResult, error) {
	// Rate limiting check
	if in.ClientIP != "" && s.rateLimiter != nil {
		rateKey := tenantID.String() + ":" + in.ClientIP
		if !s.rateLimiter.Allow(rateKey, now) {
			return nil, ErrRateLimitExceeded
		}
	}

	name := strings.TrimSpace(in.GuestName)
	if name == "" {
		return nil, ErrInvalidGuestName
	}

	phone, err := NormalizePhone(in.GuestPhone)
	if err != nil {
		return nil, err
	}

	// Verify therapist exists and is active
	th, err := s.GetTherapistByID(ctx, tenantID, in.TherapistID)
	if err != nil {
		return nil, ErrTherapistNotFound
	}
	if !th.IsActive {
		return nil, ErrTherapistInactive
	}

	// Concurrency lock per slot
	slotMu := s.getSlotLock(in.SlotID)
	slotMu.Lock()
	defer slotMu.Unlock()

	// 1. Lock slot row in database
	slot, err := s.queries.GetAppointmentSlotByIDForUpdate(ctx, bookingdb.GetAppointmentSlotByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(in.SlotID),
	})
	if err != nil {
		return nil, ErrSlotNotFound
	}
	if fromPgUUID(slot.TherapistID) != in.TherapistID {
		return nil, ErrSlotNotFound
	}

	// 2. Validate slot start time is in the future
	if !slot.StartsAt.Time.After(now) {
		return nil, ErrSlotInThePast
	}

	// 3. Ensure slot is currently available (zero oversell guarantee)
	if slot.Status != "available" {
		return nil, ErrSlotNotAvailable
	}

	// 4. Compute 2-hour pending TTL: min(now + 2h, slot.starts_at)
	expiresAt := now.Add(2 * time.Hour)
	if expiresAt.After(slot.StartsAt.Time) {
		expiresAt = slot.StartsAt.Time
	}

	// 5. Generate secure guest lookup token
	lookupHash, err := GenerateLookupHash()
	if err != nil {
		return nil, fmt.Errorf("generate lookup hash: %w", err)
	}

	// 6. Transition slot to reserved
	_, err = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(in.SlotID),
		Status:   "reserved",
	})
	if err != nil {
		return nil, fmt.Errorf("reserve slot: %w", err)
	}

	// 7. Insert appointment record
	app, err := s.queries.CreateAppointment(ctx, bookingdb.CreateAppointmentParams{
		TenantID:        toPgUUID(tenantID),
		TherapistID:     toPgUUID(in.TherapistID),
		SlotID:          toPgUUID(in.SlotID),
		Status:          "pending",
		GuestName:       name,
		GuestPhone:      phone,
		GuestLookupHash: lookupHash,
		Notes:           "",
		ExpiresAt:       toPgTimestamptz(expiresAt),
	})
	if err != nil {
		if s.scoped {
			return nil, fmt.Errorf("create appointment: %w", err)
		}
		_, releaseErr := s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       toPgUUID(in.SlotID),
			Status:   "available",
		})
		return nil, fmt.Errorf("create appointment: %w", errors.Join(err, releaseErr))
	}

	// 8. Record audit status history
	_, err = s.queries.CreateAppointmentStatusHistory(ctx, bookingdb.CreateAppointmentStatusHistoryParams{
		TenantID:      toPgUUID(tenantID),
		AppointmentID: app.ID,
		FromStatus:    pgtype.Text{Valid: false},
		ToStatus:      "pending",
		Action:        "created",
		ActorType:     "guest",
		ActorID:       pgtype.UUID{Valid: false},
		Reason:        "ثبت درخواست نوبت توسط مراجع",
		Metadata:      []byte("{}"),
	})

	if err != nil {
		return nil, fmt.Errorf("record appointment creation: %w", err)
	}
	return &AppointmentResult{
		ID:              fromPgUUID(app.ID),
		TherapistID:     in.TherapistID,
		SlotID:          in.SlotID,
		GuestName:       app.GuestName,
		GuestLookupHash: app.GuestLookupHash,
		Status:          app.Status,
		ExpiresAt:       app.ExpiresAt.Time,
		StartsAt:        slot.StartsAt.Time,
		EndsAt:          slot.EndsAt.Time,
	}, nil
}

func (s *Service) publicAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID) (bookingdb.Appointment, error) {
	params := bookingdb.GetAppointmentByIDParams{TenantID: toPgUUID(tenantID), ID: toPgUUID(appointmentID)}
	var app bookingdb.Appointment
	var err error
	if s.scoped {
		app, err = s.queries.GetAppointmentByIDForUpdate(ctx, bookingdb.GetAppointmentByIDForUpdateParams(params))
	} else {
		app, err = s.queries.GetAppointmentByID(ctx, params)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return app, ErrAppointmentNotFound
	}
	return app, err
}

func (s *Service) GetPublicAppointmentStatus(ctx context.Context, tenantID, appointmentID uuid.UUID, lookupHash string, now time.Time) (*AppointmentResult, error) {
	app, err := s.publicAppointment(ctx, tenantID, appointmentID)
	if err != nil {
		return nil, err
	}

	if app.GuestLookupHash != lookupHash {
		return nil, ErrAppointmentNotFound
	}

	slotID := fromPgUUID(app.SlotID)

	// If pending and expired, transition status and release slot
	if app.Status == "pending" && (now.After(app.ExpiresAt.Time) || now.Equal(app.ExpiresAt.Time)) {
		slotMu := s.getSlotLock(slotID)
		slotMu.Lock()
		defer slotMu.Unlock()

		updatedApp, err := s.queries.UpdateAppointmentStatus(ctx, bookingdb.UpdateAppointmentStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       app.ID,
			Status:   "expired",
		})
		if err != nil {
			return nil, fmt.Errorf("expire appointment: %w", err)
		}
		app = updatedApp

		_, err = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       app.SlotID,
			Status:   "available",
		})

		if err != nil {
			return nil, fmt.Errorf("release expired slot: %w", err)
		}
		_, err = s.queries.CreateAppointmentStatusHistory(ctx, bookingdb.CreateAppointmentStatusHistoryParams{
			TenantID:      toPgUUID(tenantID),
			AppointmentID: app.ID,
			FromStatus:    pgtype.Text{String: "pending", Valid: true},
			ToStatus:      "expired",
			Action:        "expired",
			ActorType:     "system",
			ActorID:       pgtype.UUID{Valid: false},
			Reason:        "انقضای مهلت ۲ ساعته تأیید نوبت",
			Metadata:      []byte("{}"),
		})

		if err != nil {
			return nil, fmt.Errorf("record appointment expiration: %w", err)
		}
	}

	slot, err := s.queries.GetAppointmentSlotByID(ctx, bookingdb.GetAppointmentSlotByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       app.SlotID,
	})
	if err != nil {
		return nil, fmt.Errorf("get slot: %w", err)
	}

	return &AppointmentResult{
		ID:              fromPgUUID(app.ID),
		TherapistID:     fromPgUUID(app.TherapistID),
		SlotID:          slotID,
		GuestName:       app.GuestName,
		GuestLookupHash: app.GuestLookupHash,
		Status:          app.Status,
		ExpiresAt:       app.ExpiresAt.Time,
		StartsAt:        slot.StartsAt.Time,
		EndsAt:          slot.EndsAt.Time,
	}, nil
}

func (s *Service) CancelPublicAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID, lookupHash, reason string, now time.Time) (*AppointmentResult, error) {
	app, err := s.publicAppointment(ctx, tenantID, appointmentID)
	if err != nil {
		return nil, err
	}

	if app.GuestLookupHash != lookupHash {
		return nil, ErrAppointmentNotFound
	}

	if app.Status != "pending" && app.Status != "confirmed" {
		return nil, ErrCannotCancel
	}

	slotID := fromPgUUID(app.SlotID)
	slotMu := s.getSlotLock(slotID)
	slotMu.Lock()
	defer slotMu.Unlock()

	slot, err := s.queries.GetAppointmentSlotByIDForUpdate(ctx, bookingdb.GetAppointmentSlotByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       app.SlotID,
	})
	if err != nil {
		return nil, ErrSlotNotFound
	}

	// Cancellation allowed only before session start time
	if !slot.StartsAt.Time.After(now) {
		return nil, ErrCannotCancelPastSlot
	}

	if reason == "" {
		reason = "لغو نوبت توسط مراجع"
	}

	updatedApp, err := s.queries.UpdateAppointmentStatus(ctx, bookingdb.UpdateAppointmentStatusParams{
		TenantID:           toPgUUID(tenantID),
		ID:                 app.ID,
		Status:             "cancelled",
		CancellationReason: reason,
	})
	if err != nil {
		return nil, fmt.Errorf("cancel appointment: %w", err)
	}

	// Release slot back to available
	_, err = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       app.SlotID,
		Status:   "available",
	})
	if err != nil {
		return nil, fmt.Errorf("release slot: %w", err)
	}

	// Record audit history
	_, err = s.queries.CreateAppointmentStatusHistory(ctx, bookingdb.CreateAppointmentStatusHistoryParams{
		TenantID:      toPgUUID(tenantID),
		AppointmentID: app.ID,
		FromStatus:    pgtype.Text{String: app.Status, Valid: true},
		ToStatus:      "cancelled",
		Action:        "cancelled",
		ActorType:     "guest",
		ActorID:       pgtype.UUID{Valid: false},
		Reason:        reason,
		Metadata:      []byte("{}"),
	})

	if err != nil {
		return nil, fmt.Errorf("record appointment cancellation: %w", err)
	}

	return &AppointmentResult{
		ID:              fromPgUUID(updatedApp.ID),
		TherapistID:     fromPgUUID(updatedApp.TherapistID),
		SlotID:          slotID,
		GuestName:       updatedApp.GuestName,
		GuestLookupHash: updatedApp.GuestLookupHash,
		Status:          updatedApp.Status,
		ExpiresAt:       updatedApp.ExpiresAt.Time,
		StartsAt:        slot.StartsAt.Time,
		EndsAt:          slot.EndsAt.Time,
	}, nil
}

func (s *Service) ExpirePendingAppointments(ctx context.Context, tenantID uuid.UUID, now time.Time) (int, error) {
	expiredList, err := s.queries.ListExpiredPendingAppointments(ctx, bookingdb.ListExpiredPendingAppointmentsParams{
		TenantID:  toPgUUID(tenantID),
		ExpiresAt: toPgTimestamptz(now),
	})
	if err != nil {
		return 0, err
	}

	count := 0
	for _, app := range expiredList {
		slotID := fromPgUUID(app.SlotID)
		slotMu := s.getSlotLock(slotID)
		slotMu.Lock()

		_, err := s.queries.UpdateAppointmentStatus(ctx, bookingdb.UpdateAppointmentStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       app.ID,
			Status:   "expired",
		})
		if err == nil {
			_, _ = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
				TenantID: toPgUUID(tenantID),
				ID:       app.SlotID,
				Status:   "available",
			})

			_, err = s.queries.CreateAppointmentStatusHistory(ctx, bookingdb.CreateAppointmentStatusHistoryParams{
				TenantID:      toPgUUID(tenantID),
				AppointmentID: app.ID,
				FromStatus:    pgtype.Text{String: "pending", Valid: true},
				ToStatus:      "expired",
				Action:        "expired",
				ActorType:     "system",
				ActorID:       pgtype.UUID{Valid: false},
				Reason:        "انقضای مهلت ۲ ساعته توسط Worker",
				Metadata:      []byte("{}"),
			})
			count++
		}

		slotMu.Unlock()
	}

	return count, nil
}

// ----------------------------------------------------
// Admin Appointments Management (PSY-2.4)
// ----------------------------------------------------

type AdminAppointmentDetail struct {
	ID                 uuid.UUID  `json:"id"`
	TherapistID        uuid.UUID  `json:"therapist_id"`
	TherapistName      string     `json:"therapist_name"`
	SlotID             uuid.UUID  `json:"slot_id"`
	StartsAt           time.Time  `json:"starts_at"`
	EndsAt             time.Time  `json:"ends_at"`
	GuestName          string     `json:"guest_name"`
	GuestPhone         string     `json:"guest_phone"`
	Notes              string     `json:"notes"`
	Status             string     `json:"status"`
	ExpiresAt          time.Time  `json:"expires_at"`
	ConfirmedAt        *time.Time `json:"confirmed_at,omitempty"`
	CancelledAt        *time.Time `json:"cancelled_at,omitempty"`
	CancellationReason string     `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type AdminAppointmentListItem struct {
	ID            uuid.UUID `json:"id"`
	TherapistID   uuid.UUID `json:"therapist_id"`
	TherapistName string    `json:"therapist_name"`
	SlotID        uuid.UUID `json:"slot_id"`
	StartsAt      time.Time `json:"starts_at"`
	EndsAt        time.Time `json:"ends_at"`
	GuestName     string    `json:"guest_name"`
	GuestPhone    string    `json:"guest_phone"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Service) GetTherapistByUserID(ctx context.Context, tenantID, userID uuid.UUID) (*bookingdb.Therapist, error) {
	th, err := s.queries.GetTherapistByUserID(ctx, bookingdb.GetTherapistByUserIDParams{
		TenantID: toPgUUID(tenantID),
		UserID:   toPgUUID(userID),
	})
	if err != nil {
		return nil, ErrTherapistNotFound
	}
	return &th, nil
}

func (s *Service) checkTherapistRoleAccess(ctx context.Context, tenantID uuid.UUID, callerRole string, callerUserID uuid.UUID, targetTherapistID uuid.UUID) error {
	if callerRole == "therapist" {
		ownTh, err := s.GetTherapistByUserID(ctx, tenantID, callerUserID)
		if err != nil || fromPgUUID(ownTh.ID) != targetTherapistID {
			return ErrUnauthorizedTherapist
		}
	}
	return nil
}

func (s *Service) ListAdminAppointments(ctx context.Context, tenantID uuid.UUID, callerRole string, callerUserID uuid.UUID, filterTherapistID *uuid.UUID, filterStatus string) ([]AdminAppointmentListItem, error) {
	var targetTherapistID pgtype.UUID

	if callerRole == "therapist" {
		ownTh, err := s.GetTherapistByUserID(ctx, tenantID, callerUserID)
		if err != nil {
			return nil, ErrUnauthorizedTherapist
		}
		ownID := fromPgUUID(ownTh.ID)
		if filterTherapistID != nil && *filterTherapistID != ownID {
			return nil, ErrUnauthorizedTherapist
		}
		targetTherapistID = toPgUUID(ownID)
	} else if filterTherapistID != nil {
		targetTherapistID = toPgUUID(*filterTherapistID)
	}

	var statusParam pgtype.Text
	if filterStatus != "" {
		statusParam = pgtype.Text{String: filterStatus, Valid: true}
	}

	rows, err := s.queries.ListAdminAppointments(ctx, bookingdb.ListAdminAppointmentsParams{
		TenantID:    toPgUUID(tenantID),
		TherapistID: targetTherapistID,
		Status:      statusParam,
	})
	if err != nil {
		return nil, err
	}

	res := make([]AdminAppointmentListItem, 0, len(rows))
	for _, r := range rows {
		res = append(res, AdminAppointmentListItem{
			ID:            fromPgUUID(r.ID),
			TherapistID:   fromPgUUID(r.TherapistID),
			TherapistName: r.TherapistName,
			SlotID:        fromPgUUID(r.SlotID),
			StartsAt:      r.StartsAt.Time,
			EndsAt:        r.EndsAt.Time,
			GuestName:     r.GuestName,
			GuestPhone:    r.GuestPhone,
			Status:        r.Status,
			CreatedAt:     r.CreatedAt.Time,
		})
	}
	return res, nil
}

func (s *Service) GetAdminAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID, callerRole string, callerUserID uuid.UUID) (*AdminAppointmentDetail, error) {
	row, err := s.queries.GetAppointmentDetail(ctx, bookingdb.GetAppointmentDetailParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(appointmentID),
	})
	if err != nil {
		return nil, ErrAppointmentNotFound
	}

	therapistID := fromPgUUID(row.TherapistID)
	if err := s.checkTherapistRoleAccess(ctx, tenantID, callerRole, callerUserID, therapistID); err != nil {
		return nil, err
	}

	var confirmedAt, cancelledAt *time.Time
	if row.ConfirmedAt.Valid {
		t := row.ConfirmedAt.Time
		confirmedAt = &t
	}
	if row.CancelledAt.Valid {
		t := row.CancelledAt.Time
		cancelledAt = &t
	}

	return &AdminAppointmentDetail{
		ID:                 fromPgUUID(row.ID),
		TherapistID:        therapistID,
		TherapistName:      row.TherapistName,
		SlotID:             fromPgUUID(row.SlotID),
		StartsAt:           row.StartsAt.Time,
		EndsAt:             row.EndsAt.Time,
		GuestName:          row.GuestName,
		GuestPhone:         row.GuestPhone,
		Notes:              row.Notes,
		Status:             row.Status,
		ExpiresAt:          row.ExpiresAt.Time,
		ConfirmedAt:        confirmedAt,
		CancelledAt:        cancelledAt,
		CancellationReason: row.CancellationReason,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func (s *Service) ConfirmAdminAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID, callerRole string, callerUserID uuid.UUID, now time.Time) (*AdminAppointmentDetail, error) {
	detail, err := s.GetAdminAppointment(ctx, tenantID, appointmentID, callerRole, callerUserID)
	if err != nil {
		return nil, err
	}

	if detail.Status != "pending" {
		return nil, errors.New("booking: only pending appointments can be confirmed")
	}

	if now.After(detail.ExpiresAt) || now.Equal(detail.ExpiresAt) {
		return nil, errors.New("booking: appointment has expired and cannot be confirmed")
	}

	if !detail.StartsAt.After(now) {
		return nil, errors.New("booking: cannot confirm appointment for a session that has already started")
	}

	slotMu := s.getSlotLock(detail.SlotID)
	slotMu.Lock()
	defer slotMu.Unlock()

	slot, err := s.queries.GetAppointmentSlotByIDForUpdate(ctx, bookingdb.GetAppointmentSlotByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(detail.SlotID),
	})
	if err != nil {
		return nil, ErrSlotNotFound
	}
	if slot.Status != "reserved" {
		return nil, errors.New("booking: slot is not in reserved status")
	}

	// Update slot to booked
	_, err = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot.ID,
		Status:   "booked",
	})
	if err != nil {
		return nil, fmt.Errorf("update slot to booked: %w", err)
	}

	// Update appointment to confirmed
	_, err = s.queries.UpdateAppointmentStatus(ctx, bookingdb.UpdateAppointmentStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(appointmentID),
		Status:   "confirmed",
	})
	if err != nil {
		return nil, fmt.Errorf("confirm appointment: %w", err)
	}

	// Record audit history
	_, _ = s.queries.CreateAppointmentStatusHistory(ctx, bookingdb.CreateAppointmentStatusHistoryParams{
		TenantID:      toPgUUID(tenantID),
		AppointmentID: toPgUUID(appointmentID),
		FromStatus:    pgtype.Text{String: "pending", Valid: true},
		ToStatus:      "confirmed",
		Action:        "confirmed",
		ActorType:     "staff",
		ActorID:       toPgUUID(callerUserID),
		Reason:        "تأیید نوبت پس از هماهنگی تلفنی توسط پرسنل کلینیک",
		Metadata:      []byte("{}"),
	})

	return s.GetAdminAppointment(ctx, tenantID, appointmentID, callerRole, callerUserID)
}

func (s *Service) CancelAdminAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID, callerRole string, callerUserID uuid.UUID, reason string, now time.Time) (*AdminAppointmentDetail, error) {
	detail, err := s.GetAdminAppointment(ctx, tenantID, appointmentID, callerRole, callerUserID)
	if err != nil {
		return nil, err
	}

	if detail.Status != "pending" && detail.Status != "confirmed" {
		return nil, ErrCannotCancel
	}

	slotMu := s.getSlotLock(detail.SlotID)
	slotMu.Lock()
	defer slotMu.Unlock()

	slot, err := s.queries.GetAppointmentSlotByIDForUpdate(ctx, bookingdb.GetAppointmentSlotByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(detail.SlotID),
	})
	if err != nil {
		return nil, ErrSlotNotFound
	}

	if !slot.StartsAt.Time.After(now) {
		return nil, ErrCannotCancelPastSlot
	}

	if reason == "" {
		reason = "لغو نوبت توسط پرسنل کلینیک"
	}

	// Update appointment status to cancelled
	_, err = s.queries.UpdateAppointmentStatus(ctx, bookingdb.UpdateAppointmentStatusParams{
		TenantID:           toPgUUID(tenantID),
		ID:                 toPgUUID(appointmentID),
		Status:             "cancelled",
		CancellationReason: reason,
	})
	if err != nil {
		return nil, fmt.Errorf("cancel appointment: %w", err)
	}

	// Release slot back to available
	_, err = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       slot.ID,
		Status:   "available",
	})
	if err != nil {
		return nil, fmt.Errorf("release slot: %w", err)
	}

	// Record audit history
	_, _ = s.queries.CreateAppointmentStatusHistory(ctx, bookingdb.CreateAppointmentStatusHistoryParams{
		TenantID:      toPgUUID(tenantID),
		AppointmentID: toPgUUID(appointmentID),
		FromStatus:    pgtype.Text{String: detail.Status, Valid: true},
		ToStatus:      "cancelled",
		Action:        "cancelled",
		ActorType:     "staff",
		ActorID:       toPgUUID(callerUserID),
		Reason:        reason,
		Metadata:      []byte("{}"),
	})

	return s.GetAdminAppointment(ctx, tenantID, appointmentID, callerRole, callerUserID)
}

func (s *Service) RescheduleAdminAppointment(ctx context.Context, tenantID, appointmentID, newSlotID uuid.UUID, callerRole string, callerUserID uuid.UUID, reason string, now time.Time) (*AdminAppointmentDetail, error) {
	detail, err := s.GetAdminAppointment(ctx, tenantID, appointmentID, callerRole, callerUserID)
	if err != nil {
		return nil, err
	}

	if detail.Status != "confirmed" && detail.Status != "pending" {
		return nil, errors.New("booking: only confirmed or pending appointments can be rescheduled")
	}

	if detail.SlotID == newSlotID {
		return nil, errors.New("booking: new slot must be different from current slot")
	}

	// Strict Lock Ordering: lock slots in deterministic UUID order to prevent deadlocks
	firstID, secondID := detail.SlotID, newSlotID
	if bytes.Compare(firstID[:], secondID[:]) > 0 {
		firstID, secondID = secondID, firstID
	}
	mu1 := s.getSlotLock(firstID)
	mu1.Lock()
	defer mu1.Unlock()

	mu2 := s.getSlotLock(secondID)
	mu2.Lock()
	defer mu2.Unlock()

	// 1. Fetch old slot
	oldSlot, err := s.queries.GetAppointmentSlotByIDForUpdate(ctx, bookingdb.GetAppointmentSlotByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(detail.SlotID),
	})
	if err != nil {
		return nil, ErrSlotNotFound
	}

	// 2. Fetch new slot
	newSlot, err := s.queries.GetAppointmentSlotByIDForUpdate(ctx, bookingdb.GetAppointmentSlotByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(newSlotID),
	})
	if err != nil {
		return nil, ErrSlotNotFound
	}

	// Verify new slot belongs to the same therapist
	if fromPgUUID(newSlot.TherapistID) != detail.TherapistID {
		return nil, errors.New("booking: new slot must belong to the same therapist")
	}

	// Verify new slot is in the future
	if !newSlot.StartsAt.Time.After(now) {
		return nil, ErrSlotInThePast
	}

	// Verify new slot is available (Zero-Oversell / Preemption protection)
	// If new slot is full, the previous appointment remains completely intact!
	if newSlot.Status != "available" {
		return nil, ErrSlotNotAvailable
	}

	// 3. Atomically switch slots
	// Mark new slot as booked
	_, err = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(newSlotID),
		Status:   "booked",
	})
	if err != nil {
		return nil, fmt.Errorf("reserve new slot: %w", err)
	}

	// Release previous slot to available
	_, err = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
		TenantID: toPgUUID(tenantID),
		ID:       oldSlot.ID,
		Status:   "available",
	})
	if err != nil {
		// Rollback new slot
		_, _ = s.queries.UpdateSlotStatus(ctx, bookingdb.UpdateSlotStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       toPgUUID(newSlotID),
			Status:   "available",
		})
		return nil, fmt.Errorf("release old slot: %w", err)
	}

	// Update appointment slot_id
	_, err = s.queries.UpdateAppointmentSlot(ctx, bookingdb.UpdateAppointmentSlotParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(appointmentID),
		SlotID:   toPgUUID(newSlotID),
	})
	if err != nil {
		return nil, fmt.Errorf("update appointment slot: %w", err)
	}

	// Ensure appointment is marked confirmed
	if detail.Status == "pending" {
		_, _ = s.queries.UpdateAppointmentStatus(ctx, bookingdb.UpdateAppointmentStatusParams{
			TenantID: toPgUUID(tenantID),
			ID:       toPgUUID(appointmentID),
			Status:   "confirmed",
		})
	}

	if reason == "" {
		reason = "جابه‌جایی نوبت به سانس جدید توسط پرسنل کلینیک"
	}

	// Record audit history with metadata
	meta, _ := json.Marshal(map[string]any{
		"previous_slot_id": detail.SlotID.String(),
		"new_slot_id":      newSlotID.String(),
	})

	_, _ = s.queries.CreateAppointmentStatusHistory(ctx, bookingdb.CreateAppointmentStatusHistoryParams{
		TenantID:      toPgUUID(tenantID),
		AppointmentID: toPgUUID(appointmentID),
		FromStatus:    pgtype.Text{String: detail.Status, Valid: true},
		ToStatus:      "confirmed",
		Action:        "rescheduled",
		ActorType:     "staff",
		ActorID:       toPgUUID(callerUserID),
		Reason:        reason,
		Metadata:      meta,
	})

	return s.GetAdminAppointment(ctx, tenantID, appointmentID, callerRole, callerUserID)
}

func (s *Service) GetAdminAppointmentHistory(ctx context.Context, tenantID, appointmentID uuid.UUID, callerRole string, callerUserID uuid.UUID) ([]bookingdb.AppointmentStatusHistory, error) {
	_, err := s.GetAdminAppointment(ctx, tenantID, appointmentID, callerRole, callerUserID)
	if err != nil {
		return nil, err
	}

	return s.queries.ListAppointmentHistory(ctx, bookingdb.ListAppointmentHistoryParams{
		TenantID:      toPgUUID(tenantID),
		AppointmentID: toPgUUID(appointmentID),
	})
}
