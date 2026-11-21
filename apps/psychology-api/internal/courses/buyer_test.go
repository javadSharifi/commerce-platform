package courses_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"psychology/api/internal/courses"
	"psychology/api/internal/courses/coursesdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockCoursesQuerier struct {
	accounts     map[string]coursesdb.BuyerAccount
	sessions     map[string]coursesdb.BuyerSession
	otps         map[string]coursesdb.BuyerOtpCode
	courses      map[string]coursesdb.Course
	lessons      map[string]coursesdb.CourseLesson
	entitlements map[string]coursesdb.CourseEntitlement
}

func newMockCoursesQuerier() *mockCoursesQuerier {
	return &mockCoursesQuerier{
		accounts:     make(map[string]coursesdb.BuyerAccount),
		sessions:     make(map[string]coursesdb.BuyerSession),
		otps:         make(map[string]coursesdb.BuyerOtpCode),
		courses:      make(map[string]coursesdb.Course),
		lessons:      make(map[string]coursesdb.CourseLesson),
		entitlements: make(map[string]coursesdb.CourseEntitlement),
	}
}

func (m *mockCoursesQuerier) CreateBuyerAccount(ctx context.Context, arg coursesdb.CreateBuyerAccountParams) (coursesdb.BuyerAccount, error) {
	id := uuid.New()
	acc := coursesdb.BuyerAccount{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		TenantID:  arg.TenantID,
		Phone:     arg.Phone,
		FullName:  arg.FullName,
		Status:    arg.Status,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	key := id.String()
	m.accounts[key] = acc
	return acc, nil
}

func (m *mockCoursesQuerier) GetBuyerAccountByID(ctx context.Context, arg coursesdb.GetBuyerAccountByIDParams) (coursesdb.BuyerAccount, error) {
	var targetID uuid.UUID
	copy(targetID[:], arg.ID.Bytes[:])
	acc, ok := m.accounts[targetID.String()]
	if !ok || acc.TenantID != arg.TenantID {
		return coursesdb.BuyerAccount{}, pgx.ErrNoRows
	}
	return acc, nil
}

func (m *mockCoursesQuerier) GetBuyerAccountByPhone(ctx context.Context, arg coursesdb.GetBuyerAccountByPhoneParams) (coursesdb.BuyerAccount, error) {
	for _, acc := range m.accounts {
		if acc.TenantID == arg.TenantID && acc.Phone == arg.Phone {
			return acc, nil
		}
	}
	return coursesdb.BuyerAccount{}, pgx.ErrNoRows
}

func (m *mockCoursesQuerier) UpdateBuyerAccount(ctx context.Context, arg coursesdb.UpdateBuyerAccountParams) (coursesdb.BuyerAccount, error) {
	var targetID uuid.UUID
	copy(targetID[:], arg.ID.Bytes[:])
	acc, ok := m.accounts[targetID.String()]
	if !ok || acc.TenantID != arg.TenantID {
		return coursesdb.BuyerAccount{}, pgx.ErrNoRows
	}
	acc.FullName = arg.FullName
	acc.Status = arg.Status
	acc.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.accounts[targetID.String()] = acc
	return acc, nil
}

func (m *mockCoursesQuerier) CreateBuyerSession(ctx context.Context, arg coursesdb.CreateBuyerSessionParams) (coursesdb.BuyerSession, error) {
	id := uuid.New()
	sess := coursesdb.BuyerSession{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		TenantID:  arg.TenantID,
		BuyerID:   arg.BuyerID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.sessions[arg.TokenHash] = sess
	return sess, nil
}

func (m *mockCoursesQuerier) GetBuyerSessionByTokenHash(ctx context.Context, arg coursesdb.GetBuyerSessionByTokenHashParams) (coursesdb.GetBuyerSessionByTokenHashRow, error) {
	sess, ok := m.sessions[arg.TokenHash]
	if !ok || sess.TenantID != arg.TenantID {
		return coursesdb.GetBuyerSessionByTokenHashRow{}, pgx.ErrNoRows
	}
	if sess.ExpiresAt.Time.Before(time.Now()) {
		return coursesdb.GetBuyerSessionByTokenHashRow{}, pgx.ErrNoRows
	}

	var buyerID uuid.UUID
	copy(buyerID[:], sess.BuyerID.Bytes[:])
	acc, ok := m.accounts[buyerID.String()]
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

func (m *mockCoursesQuerier) DeleteBuyerSession(ctx context.Context, arg coursesdb.DeleteBuyerSessionParams) error {
	for k, v := range m.sessions {
		if k == arg.TokenHash && v.TenantID == arg.TenantID {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *mockCoursesQuerier) DeleteBuyerSessionsByBuyerID(ctx context.Context, arg coursesdb.DeleteBuyerSessionsByBuyerIDParams) error {
	for k, v := range m.sessions {
		if v.TenantID == arg.TenantID && v.BuyerID == arg.BuyerID {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *mockCoursesQuerier) CreateBuyerOTPCode(ctx context.Context, arg coursesdb.CreateBuyerOTPCodeParams) (coursesdb.BuyerOtpCode, error) {
	id := uuid.New()
	otp := coursesdb.BuyerOtpCode{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		TenantID:  arg.TenantID,
		Phone:     arg.Phone,
		CodeHash:  arg.CodeHash,
		ExpiresAt: arg.ExpiresAt,
		Attempts:  0,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.otps[id.String()] = otp
	return otp, nil
}

func (m *mockCoursesQuerier) GetLatestActiveBuyerOTP(ctx context.Context, arg coursesdb.GetLatestActiveBuyerOTPParams) (coursesdb.BuyerOtpCode, error) {
	var latest coursesdb.BuyerOtpCode
	found := false
	for _, o := range m.otps {
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

func (m *mockCoursesQuerier) IncrementBuyerOTPAttempts(ctx context.Context, id pgtype.UUID) error {
	var uid uuid.UUID
	copy(uid[:], id.Bytes[:])
	if otp, ok := m.otps[uid.String()]; ok {
		otp.Attempts++
		m.otps[uid.String()] = otp
	}
	return nil
}

func (m *mockCoursesQuerier) ConsumeBuyerOTP(ctx context.Context, arg coursesdb.ConsumeBuyerOTPParams) error {
	var uid uuid.UUID
	copy(uid[:], arg.ID.Bytes[:])
	if otp, ok := m.otps[uid.String()]; ok {
		otp.ConsumedAt = arg.ConsumedAt
		m.otps[uid.String()] = otp
	}
	return nil
}

func (m *mockCoursesQuerier) InvalidateExistingBuyerOTPs(ctx context.Context, arg coursesdb.InvalidateExistingBuyerOTPsParams) error {
	for k, o := range m.otps {
		if o.TenantID == arg.TenantID && o.Phone == arg.Phone && !o.ConsumedAt.Valid {
			o.ConsumedAt = arg.ConsumedAt
			m.otps[k] = o
		}
	}
	return nil
}

func (m *mockCoursesQuerier) CreateCourse(ctx context.Context, arg coursesdb.CreateCourseParams) (coursesdb.Course, error) {
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
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.courses[id.String()] = c
	return c, nil
}

func (m *mockCoursesQuerier) GetCourseByID(ctx context.Context, arg coursesdb.GetCourseByIDParams) (coursesdb.Course, error) {
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	c, ok := m.courses[id.String()]
	if !ok || c.TenantID != arg.TenantID {
		return coursesdb.Course{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *mockCoursesQuerier) GetCourseByIDForUpdate(ctx context.Context, arg coursesdb.GetCourseByIDForUpdateParams) (coursesdb.Course, error) {
	return m.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *mockCoursesQuerier) GetCourseBySlug(ctx context.Context, arg coursesdb.GetCourseBySlugParams) (coursesdb.Course, error) {
	for _, c := range m.courses {
		if c.TenantID == arg.TenantID && c.Slug == arg.Slug {
			return c, nil
		}
	}
	return coursesdb.Course{}, pgx.ErrNoRows
}

func (m *mockCoursesQuerier) ListPublishedCourses(ctx context.Context, tenantID pgtype.UUID) ([]coursesdb.Course, error) {
	var res []coursesdb.Course
	for _, c := range m.courses {
		if c.TenantID == tenantID && c.Status == "published" {
			res = append(res, c)
		}
	}
	return res, nil
}

func (m *mockCoursesQuerier) ListAdminCourses(ctx context.Context, tenantID pgtype.UUID) ([]coursesdb.Course, error) {
	var res []coursesdb.Course
	for _, c := range m.courses {
		if c.TenantID == tenantID {
			res = append(res, c)
		}
	}
	return res, nil
}

func (m *mockCoursesQuerier) UpdateCourse(ctx context.Context, arg coursesdb.UpdateCourseParams) (coursesdb.Course, error) {
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	c, ok := m.courses[id.String()]
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
	c.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.courses[id.String()] = c
	return c, nil
}

func (m *mockCoursesQuerier) SoftDeleteCourse(ctx context.Context, arg coursesdb.SoftDeleteCourseParams) error {
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	delete(m.courses, id.String())
	return nil
}

func (m *mockCoursesQuerier) CreateCourseLesson(ctx context.Context, arg coursesdb.CreateCourseLessonParams) (coursesdb.CourseLesson, error) {
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
		CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.lessons[id.String()] = l
	return l, nil
}

func (m *mockCoursesQuerier) ListCourseLessons(ctx context.Context, arg coursesdb.ListCourseLessonsParams) ([]coursesdb.CourseLesson, error) {
	var res []coursesdb.CourseLesson
	for _, l := range m.lessons {
		if l.TenantID == arg.TenantID && l.CourseID == arg.CourseID {
			res = append(res, l)
		}
	}
	return res, nil
}

func (m *mockCoursesQuerier) GetCourseLessonByID(ctx context.Context, arg coursesdb.GetCourseLessonByIDParams) (coursesdb.CourseLesson, error) {
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	l, ok := m.lessons[id.String()]
	if !ok || l.TenantID != arg.TenantID || l.CourseID != arg.CourseID {
		return coursesdb.CourseLesson{}, pgx.ErrNoRows
	}
	return l, nil
}

func (m *mockCoursesQuerier) UpdateCourseLesson(ctx context.Context, arg coursesdb.UpdateCourseLessonParams) (coursesdb.CourseLesson, error) {
	return coursesdb.CourseLesson{}, nil
}

func (m *mockCoursesQuerier) SoftDeleteCourseLesson(ctx context.Context, arg coursesdb.SoftDeleteCourseLessonParams) error {
	return nil
}

func (m *mockCoursesQuerier) CreateCourseEntitlement(ctx context.Context, arg coursesdb.CreateCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
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
		CreatedAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.entitlements[id.String()] = ent
	return ent, nil
}

func (m *mockCoursesQuerier) GetActiveCourseEntitlement(ctx context.Context, arg coursesdb.GetActiveCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	for _, ent := range m.entitlements {
		if ent.TenantID == arg.TenantID && ent.BuyerID == arg.BuyerID && ent.CourseID == arg.CourseID && ent.Status == "active" {
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func (m *mockCoursesQuerier) GetCourseEntitlementByBuyerAndCourse(ctx context.Context, arg coursesdb.GetCourseEntitlementByBuyerAndCourseParams) (coursesdb.CourseEntitlement, error) {
	for _, ent := range m.entitlements {
		if ent.TenantID == arg.TenantID && ent.BuyerID == arg.BuyerID && ent.CourseID == arg.CourseID {
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func (m *mockCoursesQuerier) ListBuyerCourseEntitlements(ctx context.Context, arg coursesdb.ListBuyerCourseEntitlementsParams) ([]coursesdb.ListBuyerCourseEntitlementsRow, error) {
	var res []coursesdb.ListBuyerCourseEntitlementsRow
	for _, ent := range m.entitlements {
		if ent.TenantID == arg.TenantID && ent.BuyerID == arg.BuyerID && ent.Status == "active" {
			var courseID uuid.UUID
			copy(courseID[:], ent.CourseID.Bytes[:])
			c := m.courses[courseID.String()]
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

func (m *mockCoursesQuerier) ListCourseEntitlementsForAdmin(ctx context.Context, arg coursesdb.ListCourseEntitlementsForAdminParams) ([]coursesdb.ListCourseEntitlementsForAdminRow, error) {
	return nil, nil
}

func (m *mockCoursesQuerier) RevokeCourseEntitlement(ctx context.Context, arg coursesdb.RevokeCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	return coursesdb.CourseEntitlement{}, nil
}

func (m *mockCoursesQuerier) GetCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.GetCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error) {
	for _, ent := range m.entitlements {
		if ent.TenantID == arg.TenantID && ent.PurchaseID == arg.PurchaseID {
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func (m *mockCoursesQuerier) ActivateCourseEntitlement(ctx context.Context, arg coursesdb.ActivateCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	var id uuid.UUID
	copy(id[:], arg.ID.Bytes[:])
	ent, ok := m.entitlements[id.String()]
	if !ok || ent.TenantID != arg.TenantID {
		return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
	}
	ent.Status = "active"
	ent.GrantedAt = arg.GrantedAt
	m.entitlements[id.String()] = ent
	return ent, nil
}

func (m *mockCoursesQuerier) RevokeCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.RevokeCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error) {
	for idStr, ent := range m.entitlements {
		if ent.TenantID == arg.TenantID && ent.PurchaseID == arg.PurchaseID && ent.Status == "active" {
			ent.Status = "revoked"
			ent.RevokedAt = arg.RevokedAt
			ent.RevocationReason = arg.RevocationReason
			m.entitlements[idStr] = ent
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

type mockDomainQuerier struct {
	domains map[string]uuid.UUID
}

func (m *mockDomainQuerier) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	tid, ok := m.domains[host]
	if !ok {
		return domaindb.Domain{}, domain.ErrDomainNotFound
	}
	return domaindb.Domain{
		TenantID:   pgtype.UUID{Bytes: tid, Valid: true},
		Host:       host,
		IsVerified: true,
	}, nil
}

// ------------------------------------------------------------------------------------
// Tests
// ------------------------------------------------------------------------------------

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"09123456789", "09123456789", false},
		{"+989123456789", "09123456789", false},
		{"00989123456789", "09123456789", false},
		{"989123456789", "09123456789", false},
		{"9123456789", "09123456789", false},
		{"۰۹۱۲۳۴۵۶۷۸۹", "09123456789", false},
		{"۰۹۱۲-۳۴۵-۶۷۸۹", "09123456789", false},
		{" ۰۹۱۲ ۳۴۵ ۶۷۸۹ ", "09123456789", false},
		{"08123456789", "", true},
		{"0912345678", "", true},
		{"091234567890", "", true},
		{"", "", true},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := courses.NormalizePhone(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizePhone(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if got != tt.expected {
				t.Fatalf("NormalizePhone(%q) = %q, want = %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSendAndVerifyOTP_RegistrationAndLogin(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	q := newMockCoursesQuerier()
	sender := courses.NewMockOTPSender()
	svc := courses.NewBuyerService(q, sender)

	// 1. Send OTP
	phone := "09123456789"
	res, err := svc.SendOTP(ctx, tenantID, phone)
	if err != nil {
		t.Fatalf("SendOTP failed: %v", err)
	}
	if res.Phone != phone || res.ExpiresInSeconds != 120 {
		t.Fatalf("Unexpected SendOTP result: %+v", res)
	}

	code := sender.GetLastCode(phone)
	if len(code) != 6 {
		t.Fatalf("Invalid generated OTP code: %s", code)
	}

	// 2. Verify OTP (Registration with full name)
	verifyRes, err := svc.VerifyOTP(ctx, tenantID, phone, code, "سارا احمدی")
	if err != nil {
		t.Fatalf("VerifyOTP failed: %v", err)
	}
	if verifyRes.SessionToken == "" {
		t.Fatal("Expected non-empty session token")
	}
	if verifyRes.Account.Phone != phone || verifyRes.Account.FullName != "سارا احمدی" {
		t.Fatalf("Unexpected account info: %+v", verifyRes.Account)
	}
	buyerID := verifyRes.Account.ID

	// 3. Second Send OTP (Login for existing account)
	_, err = svc.SendOTP(ctx, tenantID, phone)
	if err != nil {
		t.Fatalf("Second SendOTP failed: %v", err)
	}
	code2 := sender.GetLastCode(phone)

	loginRes, err := svc.VerifyOTP(ctx, tenantID, phone, code2, "")
	if err != nil {
		t.Fatalf("Second VerifyOTP failed: %v", err)
	}
	if loginRes.Account.ID != buyerID {
		t.Fatalf("Expected same buyer ID across logins (%s vs %s)", buyerID, loginRes.Account.ID)
	}
	if loginRes.Account.FullName != "سارا احمدی" {
		t.Fatalf("Full name was not preserved: %s", loginRes.Account.FullName)
	}
}

func TestOTP_WrongCodeAndMaxAttemptsLockout(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	q := newMockCoursesQuerier()
	sender := courses.NewMockOTPSender()
	svc := courses.NewBuyerService(q, sender)

	phone := "09121112233"
	_, err := svc.SendOTP(ctx, tenantID, phone)
	if err != nil {
		t.Fatalf("SendOTP failed: %v", err)
	}
	correctCode := sender.GetLastCode(phone)

	// 5 wrong attempts
	for i := 1; i <= 5; i++ {
		_, err := svc.VerifyOTP(ctx, tenantID, phone, "000000", "")
		if err != courses.ErrOTPInvalid {
			t.Fatalf("Attempt %d expected ErrOTPInvalid, got: %v", i, err)
		}
	}

	// 6th attempt with correct code should be locked out
	_, err = svc.VerifyOTP(ctx, tenantID, phone, correctCode, "")
	if err != courses.ErrOTPExceededAttempts {
		t.Fatalf("Expected ErrOTPExceededAttempts after 5 fails, got: %v", err)
	}
}

func TestOTP_ReplayPrevention(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	q := newMockCoursesQuerier()
	sender := courses.NewMockOTPSender()
	svc := courses.NewBuyerService(q, sender)

	phone := "09129998877"
	_, err := svc.SendOTP(ctx, tenantID, phone)
	if err != nil {
		t.Fatalf("SendOTP failed: %v", err)
	}
	code := sender.GetLastCode(phone)

	// Verify once -> succeeds and consumes OTP
	_, err = svc.VerifyOTP(ctx, tenantID, phone, code, "")
	if err != nil {
		t.Fatalf("First VerifyOTP failed: %v", err)
	}

	// Replay exact same code -> must fail
	_, err = svc.VerifyOTP(ctx, tenantID, phone, code, "")
	if err != courses.ErrOTPNotFoundOrExpired {
		t.Fatalf("Expected ErrOTPNotFoundOrExpired on replay, got: %v", err)
	}
}

func TestOTP_ExpiredCode(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	q := newMockCoursesQuerier()
	sender := courses.NewMockOTPSender()
	svc := courses.NewBuyerService(q, sender)

	fakeNow := time.Now()
	svc.SetNowFunc(func() time.Time { return fakeNow })

	phone := "09124445566"
	_, err := svc.SendOTP(ctx, tenantID, phone)
	if err != nil {
		t.Fatalf("SendOTP failed: %v", err)
	}
	code := sender.GetLastCode(phone)

	// Advance time past 120s TTL
	fakeNow = fakeNow.Add(121 * time.Second)

	_, err = svc.VerifyOTP(ctx, tenantID, phone, code, "")
	if err != courses.ErrOTPNotFoundOrExpired {
		t.Fatalf("Expected ErrOTPNotFoundOrExpired for expired code, got: %v", err)
	}
}

func TestBuyerSession_AuthenticateAndExpiryAndLogout(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	q := newMockCoursesQuerier()
	sender := courses.NewMockOTPSender()
	svc := courses.NewBuyerService(q, sender)

	phone := "09123334455"
	_, _ = svc.SendOTP(ctx, tenantID, phone)
	code := sender.GetLastCode(phone)

	res, err := svc.VerifyOTP(ctx, tenantID, phone, code, "کاربر تستی")
	if err != nil {
		t.Fatalf("VerifyOTP failed: %v", err)
	}
	rawToken := res.SessionToken

	// 1. Authenticate with valid session
	buyer, err := svc.AuthenticateSession(ctx, tenantID, rawToken)
	if err != nil {
		t.Fatalf("AuthenticateSession failed: %v", err)
	}
	if buyer.Phone != phone {
		t.Fatalf("Expected phone %s, got: %s", phone, buyer.Phone)
	}

	// 2. Logout
	if err := svc.Logout(ctx, tenantID, rawToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// 3. Authenticate after logout fails
	_, err = svc.AuthenticateSession(ctx, tenantID, rawToken)
	if err != courses.ErrInvalidBuyerSession {
		t.Fatalf("Expected ErrInvalidBuyerSession after logout, got: %v", err)
	}
}

func TestTenantScoping_StrictBuyerIsolation(t *testing.T) {
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()
	q := newMockCoursesQuerier()
	sender := courses.NewMockOTPSender()
	svc := courses.NewBuyerService(q, sender)

	phone := "09120001122"

	// 1. User registers in Tenant A
	_, err := svc.SendOTP(ctx, tenantA, phone)
	if err != nil {
		t.Fatalf("Tenant A SendOTP failed: %v", err)
	}
	codeA := sender.GetLastCode(phone)
	resA, err := svc.VerifyOTP(ctx, tenantA, phone, codeA, "کاربر کلینیک الف")
	if err != nil {
		t.Fatalf("Tenant A VerifyOTP failed: %v", err)
	}

	// 2. Same phone registers in Tenant B
	_, err = svc.SendOTP(ctx, tenantB, phone)
	if err != nil {
		t.Fatalf("Tenant B SendOTP failed: %v", err)
	}
	codeB := sender.GetLastCode(phone)
	resB, err := svc.VerifyOTP(ctx, tenantB, phone, codeB, "کاربر کلینیک ب")
	if err != nil {
		t.Fatalf("Tenant B VerifyOTP failed: %v", err)
	}

	// Invariant 1: Distinct accounts per tenant
	if resA.Account.ID == resB.Account.ID {
		t.Fatalf("Tenant isolation broken! Expected different account IDs, got identical: %s", resA.Account.ID)
	}

	// Invariant 2: Session token from Tenant A presented to Tenant B is rejected
	_, err = svc.AuthenticateSession(ctx, tenantB, resA.SessionToken)
	if err != courses.ErrInvalidBuyerSession {
		t.Fatalf("Expected ErrInvalidBuyerSession when using Tenant A session in Tenant B, got: %v", err)
	}

	// Invariant 3: Session token from Tenant B presented to Tenant A is rejected
	_, err = svc.AuthenticateSession(ctx, tenantA, resB.SessionToken)
	if err != courses.ErrInvalidBuyerSession {
		t.Fatalf("Expected ErrInvalidBuyerSession when using Tenant B session in Tenant A, got: %v", err)
	}
}

func TestPurchaseWithCurrentAccount_And_OTPRecovery(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	q := newMockCoursesQuerier()
	sender := courses.NewMockOTPSender()
	svc := courses.NewBuyerService(q, sender)

	// 1. Setup course in catalog
	courseID := uuid.New()
	q.courses[courseID.String()] = coursesdb.Course{
		ID:          pgtype.UUID{Bytes: courseID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantID, Valid: true},
		Title:       "مدیریت استرس",
		Slug:        "stress-management",
		PriceRials:  12000000,
		Status:      "published",
		PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	// 2. Buyer registers
	phone := "09125556677"
	_, _ = svc.SendOTP(ctx, tenantID, phone)
	code := sender.GetLastCode(phone)
	regRes, err := svc.VerifyOTP(ctx, tenantID, phone, code, "علی حسینی")
	if err != nil {
		t.Fatalf("Registration failed: %v", err)
	}
	initialBuyerID := regRes.Account.ID

	// 3. Purchase: grant course entitlement
	purchaseID := uuid.New()
	ent, err := svc.GrantCourseEntitlement(ctx, tenantID, initialBuyerID, courseID, &purchaseID, "مدیریت استرس", 12000000)
	if err != nil {
		t.Fatalf("GrantCourseEntitlement failed: %v", err)
	}
	if ent.Status != "active" {
		t.Fatalf("Expected active status, got: %s", ent.Status)
	}

	// 4. Verify my courses list
	myCourses, err := svc.GetMyCourses(ctx, tenantID, initialBuyerID)
	if err != nil {
		t.Fatalf("GetMyCourses failed: %v", err)
	}
	if len(myCourses) != 1 || myCourses[0].CourseTitleSnapshot != "مدیریت استرس" {
		t.Fatalf("Unexpected courses list: %+v", myCourses)
	}

	// 5. Invalidate / logout session
	_ = svc.Logout(ctx, tenantID, regRes.SessionToken)

	// 6. OTP Recovery / Re-login with the same phone
	_, err = svc.SendOTP(ctx, tenantID, phone)
	if err != nil {
		t.Fatalf("Recovery SendOTP failed: %v", err)
	}
	recoveryCode := sender.GetLastCode(phone)

	recoveryRes, err := svc.VerifyOTP(ctx, tenantID, phone, recoveryCode, "")
	if err != nil {
		t.Fatalf("Recovery VerifyOTP failed: %v", err)
	}

	// Invariant: Same account recovered
	if recoveryRes.Account.ID != initialBuyerID {
		t.Fatalf("Account ID changed during recovery: %s vs %s", initialBuyerID, recoveryRes.Account.ID)
	}

	// Invariant: Entitlements preserved
	recoveredCourses, err := svc.GetMyCourses(ctx, tenantID, recoveryRes.Account.ID)
	if err != nil {
		t.Fatalf("GetMyCourses after recovery failed: %v", err)
	}
	if len(recoveredCourses) != 1 || recoveredCourses[0].CourseTitleSnapshot != "مدیریت استرس" {
		t.Fatalf("Entitlements lost after recovery! Got: %+v", recoveredCourses)
	}

	// Invariant: Replay of recovery OTP fails
	_, err = svc.VerifyOTP(ctx, tenantID, phone, recoveryCode, "")
	if err != courses.ErrOTPNotFoundOrExpired {
		t.Fatalf("Expected ErrOTPNotFoundOrExpired on replay of recovery code, got: %v", err)
	}
}

func TestHTTPHandlers(t *testing.T) {
	tenantID := uuid.New()
	host := "clinic-test.ir"

	q := newMockCoursesQuerier()
	sender := courses.NewMockOTPSender()
	svc := courses.NewBuyerService(q, sender)

	domQ := &mockDomainQuerier{
		domains: map[string]uuid.UUID{
			host: tenantID,
		},
	}
	resolver := domain.NewResolver(domQ)

	handler := courses.NewHandler(svc, nil, resolver, nil)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// 1. Send OTP via HTTP
	sendBody, _ := json.Marshal(map[string]string{
		"phone": "09121234567",
	})
	req := httptest.NewRequest("POST", "/account/auth/otp/send", bytes.NewReader(sendBody))
	req.Host = host
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /account/auth/otp/send code = %d, body = %s", rec.Code, rec.Body.String())
	}

	code := sender.GetLastCode("09121234567")
	if code == "" {
		t.Fatal("Expected OTP to be sent via sender")
	}

	// 2. Verify OTP via HTTP
	verifyBody, _ := json.Marshal(map[string]string{
		"phone":     "09121234567",
		"code":      code,
		"full_name": "مریم احمدی",
	})
	req = httptest.NewRequest("POST", "/account/auth/otp/verify", bytes.NewReader(verifyBody))
	req.Host = host
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /account/auth/otp/verify code = %d, body = %s", rec.Code, rec.Body.String())
	}

	var verifyRes courses.BuyerSessionInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &verifyRes); err != nil {
		t.Fatalf("Failed to parse verify response: %v", err)
	}
	sessionToken := verifyRes.SessionToken
	if sessionToken == "" {
		t.Fatal("Expected session token in verify response")
	}

	// Check cookie
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "buyer_session_token" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value != sessionToken {
		t.Fatal("Expected buyer_session_token cookie to be set")
	}

	// 3. GET /account/auth/me with Bearer token
	req = httptest.NewRequest("GET", "/account/auth/me", nil)
	req.Host = host
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /account/auth/me code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var meRes map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &meRes)
	if meRes["phone"] != "09121234567" || meRes["full_name"] != "مریم احمدی" {
		t.Fatalf("Unexpected me response: %+v", meRes)
	}

	// 4. GET /account/courses with Bearer token (cache-control no-store check)
	req = httptest.NewRequest("GET", "/account/courses", nil)
	req.Host = host
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /account/courses code = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains([]byte(rec.Header().Get("Cache-Control")), []byte("no-store")) {
		t.Fatalf("Expected Cache-Control: no-store on account courses, got: %s", rec.Header().Get("Cache-Control"))
	}

	// 5. POST /account/auth/logout
	req = httptest.NewRequest("POST", "/account/auth/logout", nil)
	req.Host = host
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /account/auth/logout code = %d, body = %s", rec.Code, rec.Body.String())
	}

	// 6. Subsequent request should be 401 Unauthorized
	req = httptest.NewRequest("GET", "/account/auth/me", nil)
	req.Host = host
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 after logout, got: %d", rec.Code)
	}
}
