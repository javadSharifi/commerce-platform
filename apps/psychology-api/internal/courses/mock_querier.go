package courses

import (
	"context"
	"sync"
	"time"

	"psychology/api/internal/courses/coursesdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type MockCoursesQuerier struct {
	mu           sync.RWMutex
	Accounts     map[string]coursesdb.BuyerAccount
	Sessions     map[string]coursesdb.BuyerSession
	Otps         map[string]coursesdb.BuyerOtpCode
	Courses      map[string]coursesdb.Course
	Lessons      map[string]coursesdb.CourseLesson
	Entitlements map[string]coursesdb.CourseEntitlement
}

func NewMockCoursesQuerier() *MockCoursesQuerier {
	return &MockCoursesQuerier{
		Accounts:     make(map[string]coursesdb.BuyerAccount),
		Sessions:     make(map[string]coursesdb.BuyerSession),
		Otps:         make(map[string]coursesdb.BuyerOtpCode),
		Courses:      make(map[string]coursesdb.Course),
		Lessons:      make(map[string]coursesdb.CourseLesson),
		Entitlements: make(map[string]coursesdb.CourseEntitlement),
	}
}

func (m *MockCoursesQuerier) CreateBuyerAccount(ctx context.Context, arg coursesdb.CreateBuyerAccountParams) (coursesdb.BuyerAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	acc := coursesdb.BuyerAccount{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		TenantID:  arg.TenantID,
		Phone:     arg.Phone,
		FullName:  arg.FullName,
		Status:    arg.Status,
		CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	m.Accounts[id.String()] = acc
	return acc, nil
}

func (m *MockCoursesQuerier) GetBuyerAccountByID(ctx context.Context, arg coursesdb.GetBuyerAccountByIDParams) (coursesdb.BuyerAccount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var targetID uuid.UUID
	copy(targetID[:], arg.ID.Bytes[:])
	acc, ok := m.Accounts[targetID.String()]
	if !ok || acc.TenantID != arg.TenantID {
		return coursesdb.BuyerAccount{}, pgx.ErrNoRows
	}
	return acc, nil
}

func (m *MockCoursesQuerier) GetBuyerAccountByPhone(ctx context.Context, arg coursesdb.GetBuyerAccountByPhoneParams) (coursesdb.BuyerAccount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, acc := range m.Accounts {
		if acc.TenantID == arg.TenantID && acc.Phone == arg.Phone {
			return acc, nil
		}
	}
	return coursesdb.BuyerAccount{}, pgx.ErrNoRows
}

func (m *MockCoursesQuerier) UpdateBuyerAccount(ctx context.Context, arg coursesdb.UpdateBuyerAccountParams) (coursesdb.BuyerAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var targetID uuid.UUID
	copy(targetID[:], arg.ID.Bytes[:])
	acc, ok := m.Accounts[targetID.String()]
	if !ok || acc.TenantID != arg.TenantID {
		return coursesdb.BuyerAccount{}, pgx.ErrNoRows
	}
	acc.FullName = arg.FullName
	acc.Status = arg.Status
	acc.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.Accounts[targetID.String()] = acc
	return acc, nil
}

func (m *MockCoursesQuerier) CreateBuyerSession(ctx context.Context, arg coursesdb.CreateBuyerSessionParams) (coursesdb.BuyerSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	sess := coursesdb.BuyerSession{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		TenantID:  arg.TenantID,
		BuyerID:   arg.BuyerID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	m.Sessions[arg.TokenHash] = sess
	return sess, nil
}

func (m *MockCoursesQuerier) GetBuyerSessionByTokenHash(ctx context.Context, arg coursesdb.GetBuyerSessionByTokenHashParams) (coursesdb.GetBuyerSessionByTokenHashRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.Sessions[arg.TokenHash]
	if !ok || sess.TenantID != arg.TenantID {
		return coursesdb.GetBuyerSessionByTokenHashRow{}, pgx.ErrNoRows
	}
	if sess.ExpiresAt.Time.Before(time.Now().UTC()) {
		return coursesdb.GetBuyerSessionByTokenHashRow{}, pgx.ErrNoRows
	}

	var buyerID uuid.UUID
	copy(buyerID[:], sess.BuyerID.Bytes[:])
	acc, ok := m.Accounts[buyerID.String()]
	if !ok {
		return coursesdb.GetBuyerSessionByTokenHashRow{}, pgx.ErrNoRows
	}

	return coursesdb.GetBuyerSessionByTokenHashRow{
		ID:          sess.ID,
		TenantID:    sess.TenantID,
		BuyerID:     sess.BuyerID,
		TokenHash:   sess.TokenHash,
		CreatedAt:   sess.CreatedAt,
		ExpiresAt:   sess.ExpiresAt,
		Phone:       acc.Phone,
		FullName:    acc.FullName,
		BuyerStatus: acc.Status,
	}, nil
}

func (m *MockCoursesQuerier) DeleteBuyerSession(ctx context.Context, arg coursesdb.DeleteBuyerSessionParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.Sessions {
		if k == arg.TokenHash && v.TenantID == arg.TenantID {
			delete(m.Sessions, k)
		}
	}
	return nil
}

func (m *MockCoursesQuerier) DeleteBuyerSessionsByBuyerID(ctx context.Context, arg coursesdb.DeleteBuyerSessionsByBuyerIDParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.Sessions {
		if v.TenantID == arg.TenantID && v.BuyerID == arg.BuyerID {
			delete(m.Sessions, k)
		}
	}
	return nil
}

func (m *MockCoursesQuerier) CreateBuyerOTPCode(ctx context.Context, arg coursesdb.CreateBuyerOTPCodeParams) (coursesdb.BuyerOtpCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	otp := coursesdb.BuyerOtpCode{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		TenantID:  arg.TenantID,
		Phone:     arg.Phone,
		CodeHash:  arg.CodeHash,
		ExpiresAt: arg.ExpiresAt,
		Attempts:  0,
		CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	m.Otps[id.String()] = otp
	return otp, nil
}

func (m *MockCoursesQuerier) GetLatestActiveBuyerOTP(ctx context.Context, arg coursesdb.GetLatestActiveBuyerOTPParams) (coursesdb.BuyerOtpCode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest coursesdb.BuyerOtpCode
	found := false
	for _, o := range m.Otps {
		if o.TenantID == arg.TenantID && o.Phone == arg.Phone && !o.ConsumedAt.Valid && o.ExpiresAt.Time.After(arg.ExpiresAt.Time) {
			if !found || o.CreatedAt.Time.After(latest.CreatedAt.Time) {
				latest = o
				found = true
			}
		}
	}
	if !found {
		return coursesdb.BuyerOtpCode{}, pgx.ErrNoRows
	}
	return latest, nil
}

func (m *MockCoursesQuerier) IncrementBuyerOTPAttempts(ctx context.Context, id pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var uid uuid.UUID
	copy(uid[:], id.Bytes[:])
	if otp, ok := m.Otps[uid.String()]; ok {
		otp.Attempts++
		m.Otps[uid.String()] = otp
	}
	return nil
}

func (m *MockCoursesQuerier) ConsumeBuyerOTP(ctx context.Context, arg coursesdb.ConsumeBuyerOTPParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var uid uuid.UUID
	copy(uid[:], arg.ID.Bytes[:])
	if otp, ok := m.Otps[uid.String()]; ok {
		otp.ConsumedAt = arg.ConsumedAt
		m.Otps[uid.String()] = otp
	}
	return nil
}

func (m *MockCoursesQuerier) InvalidateExistingBuyerOTPs(ctx context.Context, arg coursesdb.InvalidateExistingBuyerOTPsParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, o := range m.Otps {
		if o.TenantID == arg.TenantID && o.Phone == arg.Phone && !o.ConsumedAt.Valid {
			o.ConsumedAt = arg.ConsumedAt
			m.Otps[k] = o
		}
	}
	return nil
}

func (m *MockCoursesQuerier) CreateCourse(ctx context.Context, arg coursesdb.CreateCourseParams) (coursesdb.Course, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	c := coursesdb.Course{
		ID:             pgtype.UUID{Bytes: id, Valid: true},
		TenantID:       arg.TenantID,
		Title:          arg.Title,
		Slug:           arg.Slug,
		Description:    arg.Description,
		InstructorName: arg.InstructorName,
		PriceRials:     arg.PriceRials,
		CoverMediaID:   arg.CoverMediaID,
		Status:         arg.Status,
		PublishedAt:    arg.PublishedAt,
		CreatedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	m.Courses[id.String()] = c
	return c, nil
}

func (m *MockCoursesQuerier) GetCourseByID(ctx context.Context, arg coursesdb.GetCourseByIDParams) (coursesdb.Course, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	c, ok := m.Courses[id.String()]
	if !ok || c.TenantID != arg.TenantID {
		return coursesdb.Course{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *MockCoursesQuerier) GetCourseByIDForUpdate(ctx context.Context, arg coursesdb.GetCourseByIDForUpdateParams) (coursesdb.Course, error) {
	return m.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *MockCoursesQuerier) GetCourseBySlug(ctx context.Context, arg coursesdb.GetCourseBySlugParams) (coursesdb.Course, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.Courses {
		if c.TenantID == arg.TenantID && c.Slug == arg.Slug {
			return c, nil
		}
	}
	return coursesdb.Course{}, pgx.ErrNoRows
}

func (m *MockCoursesQuerier) ListPublishedCourses(ctx context.Context, tenantID pgtype.UUID) ([]coursesdb.Course, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []coursesdb.Course
	for _, c := range m.Courses {
		if c.TenantID == tenantID && c.Status == "published" {
			res = append(res, c)
		}
	}
	return res, nil
}

func (m *MockCoursesQuerier) ListAdminCourses(ctx context.Context, tenantID pgtype.UUID) ([]coursesdb.Course, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []coursesdb.Course
	for _, c := range m.Courses {
		if c.TenantID == tenantID {
			res = append(res, c)
		}
	}
	return res, nil
}

func (m *MockCoursesQuerier) UpdateCourse(ctx context.Context, arg coursesdb.UpdateCourseParams) (coursesdb.Course, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	c, ok := m.Courses[id.String()]
	if !ok || c.TenantID != arg.TenantID {
		return coursesdb.Course{}, pgx.ErrNoRows
	}
	c.Title = arg.Title
	c.Slug = arg.Slug
	c.Description = arg.Description
	c.InstructorName = arg.InstructorName
	c.PriceRials = arg.PriceRials
	c.CoverMediaID = arg.CoverMediaID
	c.Status = arg.Status
	c.PublishedAt = arg.PublishedAt
	c.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.Courses[id.String()] = c
	return c, nil
}

func (m *MockCoursesQuerier) SoftDeleteCourse(ctx context.Context, arg coursesdb.SoftDeleteCourseParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	delete(m.Courses, id.String())
	return nil
}

func (m *MockCoursesQuerier) CreateCourseLesson(ctx context.Context, arg coursesdb.CreateCourseLessonParams) (coursesdb.CourseLesson, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	l := coursesdb.CourseLesson{
		ID:              pgtype.UUID{Bytes: id, Valid: true},
		TenantID:        arg.TenantID,
		CourseID:        arg.CourseID,
		Title:           arg.Title,
		Description:     arg.Description,
		DisplayOrder:    arg.DisplayOrder,
		MediaAssetID:    arg.MediaAssetID,
		DurationSeconds: arg.DurationSeconds,
		IsFreePreview:   arg.IsFreePreview,
		CreatedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	m.Lessons[id.String()] = l
	return l, nil
}

func (m *MockCoursesQuerier) ListCourseLessons(ctx context.Context, arg coursesdb.ListCourseLessonsParams) ([]coursesdb.CourseLesson, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []coursesdb.CourseLesson
	for _, l := range m.Lessons {
		if l.TenantID == arg.TenantID && l.CourseID == arg.CourseID {
			res = append(res, l)
		}
	}
	return res, nil
}

func (m *MockCoursesQuerier) GetCourseLessonByID(ctx context.Context, arg coursesdb.GetCourseLessonByIDParams) (coursesdb.CourseLesson, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	l, ok := m.Lessons[id.String()]
	if !ok || l.TenantID != arg.TenantID || l.CourseID != arg.CourseID {
		return coursesdb.CourseLesson{}, pgx.ErrNoRows
	}
	return l, nil
}

func (m *MockCoursesQuerier) UpdateCourseLesson(ctx context.Context, arg coursesdb.UpdateCourseLessonParams) (coursesdb.CourseLesson, error) {
	return coursesdb.CourseLesson{}, nil
}

func (m *MockCoursesQuerier) SoftDeleteCourseLesson(ctx context.Context, arg coursesdb.SoftDeleteCourseLessonParams) error {
	return nil
}

func (m *MockCoursesQuerier) CreateCourseEntitlement(ctx context.Context, arg coursesdb.CreateCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	ent := coursesdb.CourseEntitlement{
		ID:                  pgtype.UUID{Bytes: id, Valid: true},
		TenantID:            arg.TenantID,
		BuyerID:             arg.BuyerID,
		CourseID:            arg.CourseID,
		PurchaseID:          arg.PurchaseID,
		Status:              arg.Status,
		CourseTitleSnapshot: arg.CourseTitleSnapshot,
		PriceRialsSnapshot:  arg.PriceRialsSnapshot,
		GrantedAt:           arg.GrantedAt,
		CreatedAt:           pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:           pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	m.Entitlements[id.String()] = ent
	return ent, nil
}

func (m *MockCoursesQuerier) GetActiveCourseEntitlement(ctx context.Context, arg coursesdb.GetActiveCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ent := range m.Entitlements {
		if ent.TenantID == arg.TenantID && ent.BuyerID == arg.BuyerID && ent.CourseID == arg.CourseID && ent.Status == "active" {
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func (m *MockCoursesQuerier) GetCourseEntitlementByBuyerAndCourse(ctx context.Context, arg coursesdb.GetCourseEntitlementByBuyerAndCourseParams) (coursesdb.CourseEntitlement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ent := range m.Entitlements {
		if ent.TenantID == arg.TenantID && ent.BuyerID == arg.BuyerID && ent.CourseID == arg.CourseID {
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func (m *MockCoursesQuerier) ListBuyerCourseEntitlements(ctx context.Context, arg coursesdb.ListBuyerCourseEntitlementsParams) ([]coursesdb.ListBuyerCourseEntitlementsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []coursesdb.ListBuyerCourseEntitlementsRow
	for _, ent := range m.Entitlements {
		if ent.TenantID == arg.TenantID && ent.BuyerID == arg.BuyerID && ent.Status == "active" {
			var courseID uuid.UUID
			copy(courseID[:], ent.CourseID.Bytes[:])
			c := m.Courses[courseID.String()]
			res = append(res, coursesdb.ListBuyerCourseEntitlementsRow{
				ID:                  ent.ID,
				TenantID:            ent.TenantID,
				BuyerID:             ent.BuyerID,
				CourseID:            ent.CourseID,
				PurchaseID:          ent.PurchaseID,
				Status:              ent.Status,
				CourseTitleSnapshot: ent.CourseTitleSnapshot,
				PriceRialsSnapshot:  ent.PriceRialsSnapshot,
				GrantedAt:           ent.GrantedAt,
				RevokedAt:           ent.RevokedAt,
				RevocationReason:    ent.RevocationReason,
				CreatedAt:           ent.CreatedAt,
				UpdatedAt:           ent.UpdatedAt,
				CurrentCourseTitle:  c.Title,
				CourseSlug:          c.Slug,
				CoverMediaID:        c.CoverMediaID,
				CourseStatus:        c.Status,
			})
		}
	}
	return res, nil
}

func (m *MockCoursesQuerier) ListCourseEntitlementsForAdmin(ctx context.Context, arg coursesdb.ListCourseEntitlementsForAdminParams) ([]coursesdb.ListCourseEntitlementsForAdminRow, error) {
	return nil, nil
}

func (m *MockCoursesQuerier) RevokeCourseEntitlement(ctx context.Context, arg coursesdb.RevokeCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	return coursesdb.CourseEntitlement{}, nil
}

func (m *MockCoursesQuerier) GetCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.GetCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ent := range m.Entitlements {
		if ent.TenantID == arg.TenantID && ent.PurchaseID == arg.PurchaseID {
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func (m *MockCoursesQuerier) ActivateCourseEntitlement(ctx context.Context, arg coursesdb.ActivateCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	ent, ok := m.Entitlements[id.String()]
	if !ok || ent.TenantID != arg.TenantID {
		return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
	}
	ent.Status = "active"
	ent.GrantedAt = arg.GrantedAt
	m.Entitlements[id.String()] = ent
	return ent, nil
}

func (m *MockCoursesQuerier) RevokeCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.RevokeCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for idStr, ent := range m.Entitlements {
		if ent.TenantID == arg.TenantID && ent.PurchaseID == arg.PurchaseID && ent.Status == "active" {
			ent.Status = "revoked"
			ent.RevokedAt = arg.RevokedAt
			ent.RevocationReason = arg.RevocationReason
			m.Entitlements[idStr] = ent
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}
