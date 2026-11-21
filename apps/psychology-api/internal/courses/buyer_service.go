package courses

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"psychology/api/internal/courses/coursesdb"
	"psychology/api/internal/media"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidPhone         = errors.New("courses: شماره موبایل وارد شده معتبر نمی‌باشد (فرمت صحیح: ۰۹xxxxxxxxx)")
	ErrOTPNotFoundOrExpired = errors.New("courses: کد تأیید یافت نشد یا منقضی شده است")
	ErrOTPExceededAttempts  = errors.New("courses: تعداد تلاش‌های ناموفق بیش از حد مجاز است. لطفاً مجدداً کد دریافت کنید")
	ErrOTPInvalid           = errors.New("courses: کد وارد شده صحیح نمی‌باشد")
	ErrBuyerBlocked         = errors.New("courses: حساب کاربری شما مسدود شده است")
	ErrInvalidBuyerSession  = errors.New("courses: نشست کاربری نامعتبر یا منقضی شده است")
	ErrAccountNotFound      = errors.New("courses: حساب کاربری یافت نشد")
	ErrNotPurchased         = errors.New("courses: شما به این دوره دسترسی ندارید")
	ErrEntitlementRevoked   = errors.New("courses: دسترسی شما به این دوره لغو یا مسترد شده است")
	ErrCourseNotPublished   = errors.New("courses: دوره آموزشی در وضعیت انتشار نمی‌باشد")
	ErrLessonNotFreePreview = errors.New("courses: این جلسه پیش‌نمایش رایگان ندارد")
)

var iranianPhoneRegex = regexp.MustCompile(`^(?:\+98|0098|98|0)?(9\d{9})$`)

// NormalizePhone normalizes an Iranian mobile phone number to 09XXXXXXXXX format.
func NormalizePhone(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	// Replace Persian/Arabic numerals with Latin
	replacer := strings.NewReplacer(
		"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4",
		"۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
		"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4",
		"٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
	)
	cleaned := replacer.Replace(trimmed)
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")

	matches := iranianPhoneRegex.FindStringSubmatch(cleaned)
	if len(matches) != 2 {
		return "", ErrInvalidPhone
	}
	return "0" + matches[1], nil
}

func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func GenerateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func GenerateRandomOTPCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

type OTPSender interface {
	SendOTP(ctx context.Context, phone, code string) error
}

type MockOTPSender struct {
	mu       sync.RWMutex
	lastCode map[string]string
}

func NewMockOTPSender() *MockOTPSender {
	return &MockOTPSender{
		lastCode: make(map[string]string),
	}
}

func (m *MockOTPSender) SendOTP(ctx context.Context, phone, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastCode[phone] = code
	return nil
}

func (m *MockOTPSender) GetLastCode(phone string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastCode[phone]
}

type BuyerAccount struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Phone     string    `json:"phone"`
	FullName  string    `json:"full_name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BuyerSessionInfo struct {
	SessionToken string       `json:"session_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
	Account      BuyerAccount `json:"account"`
}

type SendOTPResult struct {
	Phone            string `json:"phone"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	Message          string `json:"message"`
}

type BuyerCourseEntitlementItem struct {
	ID                  uuid.UUID  `json:"id"`
	CourseID            uuid.UUID  `json:"course_id"`
	CourseTitleSnapshot string     `json:"course_title_snapshot"`
	PriceRialsSnapshot  int64      `json:"price_rials_snapshot"`
	Status              string     `json:"status"`
	GrantedAt           time.Time  `json:"granted_at"`
	CurrentCourseTitle  string     `json:"current_course_title"`
	CourseSlug          string     `json:"course_slug"`
	CoverMediaID        *uuid.UUID `json:"cover_media_id,omitempty"`
	CourseStatus        string     `json:"course_status"`
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPgUUID(p pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], p.Bytes[:])
	return u
}

type Querier interface {
	ActivateCourseEntitlement(ctx context.Context, arg coursesdb.ActivateCourseEntitlementParams) (coursesdb.CourseEntitlement, error)
	ConsumeBuyerOTP(ctx context.Context, arg coursesdb.ConsumeBuyerOTPParams) error
	CreateBuyerAccount(ctx context.Context, arg coursesdb.CreateBuyerAccountParams) (coursesdb.BuyerAccount, error)
	CreateBuyerOTPCode(ctx context.Context, arg coursesdb.CreateBuyerOTPCodeParams) (coursesdb.BuyerOtpCode, error)
	CreateBuyerSession(ctx context.Context, arg coursesdb.CreateBuyerSessionParams) (coursesdb.BuyerSession, error)
	CreateCourse(ctx context.Context, arg coursesdb.CreateCourseParams) (coursesdb.Course, error)
	CreateCourseEntitlement(ctx context.Context, arg coursesdb.CreateCourseEntitlementParams) (coursesdb.CourseEntitlement, error)
	CreateCourseLesson(ctx context.Context, arg coursesdb.CreateCourseLessonParams) (coursesdb.CourseLesson, error)
	DeleteBuyerSession(ctx context.Context, arg coursesdb.DeleteBuyerSessionParams) error
	DeleteBuyerSessionsByBuyerID(ctx context.Context, arg coursesdb.DeleteBuyerSessionsByBuyerIDParams) error
	GetActiveCourseEntitlement(ctx context.Context, arg coursesdb.GetActiveCourseEntitlementParams) (coursesdb.CourseEntitlement, error)
	GetCourseEntitlementByBuyerAndCourse(ctx context.Context, arg coursesdb.GetCourseEntitlementByBuyerAndCourseParams) (coursesdb.CourseEntitlement, error)
	GetBuyerAccountByID(ctx context.Context, arg coursesdb.GetBuyerAccountByIDParams) (coursesdb.BuyerAccount, error)
	GetBuyerAccountByPhone(ctx context.Context, arg coursesdb.GetBuyerAccountByPhoneParams) (coursesdb.BuyerAccount, error)
	GetBuyerSessionByTokenHash(ctx context.Context, arg coursesdb.GetBuyerSessionByTokenHashParams) (coursesdb.GetBuyerSessionByTokenHashRow, error)
	GetCourseByID(ctx context.Context, arg coursesdb.GetCourseByIDParams) (coursesdb.Course, error)
	GetCourseByIDForUpdate(ctx context.Context, arg coursesdb.GetCourseByIDForUpdateParams) (coursesdb.Course, error)
	GetCourseBySlug(ctx context.Context, arg coursesdb.GetCourseBySlugParams) (coursesdb.Course, error)
	GetCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.GetCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error)
	GetCourseLessonByID(ctx context.Context, arg coursesdb.GetCourseLessonByIDParams) (coursesdb.CourseLesson, error)
	GetLatestActiveBuyerOTP(ctx context.Context, arg coursesdb.GetLatestActiveBuyerOTPParams) (coursesdb.BuyerOtpCode, error)
	IncrementBuyerOTPAttempts(ctx context.Context, id pgtype.UUID) error
	InvalidateExistingBuyerOTPs(ctx context.Context, arg coursesdb.InvalidateExistingBuyerOTPsParams) error
	ListAdminCourses(ctx context.Context, tenantID pgtype.UUID) ([]coursesdb.Course, error)
	ListBuyerCourseEntitlements(ctx context.Context, arg coursesdb.ListBuyerCourseEntitlementsParams) ([]coursesdb.ListBuyerCourseEntitlementsRow, error)
	ListCourseEntitlementsForAdmin(ctx context.Context, arg coursesdb.ListCourseEntitlementsForAdminParams) ([]coursesdb.ListCourseEntitlementsForAdminRow, error)
	ListCourseLessons(ctx context.Context, arg coursesdb.ListCourseLessonsParams) ([]coursesdb.CourseLesson, error)
	ListPublishedCourses(ctx context.Context, tenantID pgtype.UUID) ([]coursesdb.Course, error)
	RevokeCourseEntitlement(ctx context.Context, arg coursesdb.RevokeCourseEntitlementParams) (coursesdb.CourseEntitlement, error)
	RevokeCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.RevokeCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error)
	SoftDeleteCourse(ctx context.Context, arg coursesdb.SoftDeleteCourseParams) error
	SoftDeleteCourseLesson(ctx context.Context, arg coursesdb.SoftDeleteCourseLessonParams) error
	UpdateBuyerAccount(ctx context.Context, arg coursesdb.UpdateBuyerAccountParams) (coursesdb.BuyerAccount, error)
	UpdateCourse(ctx context.Context, arg coursesdb.UpdateCourseParams) (coursesdb.Course, error)
	UpdateCourseLesson(ctx context.Context, arg coursesdb.UpdateCourseLessonParams) (coursesdb.CourseLesson, error)
}

type LessonStreamOutput struct {
	StreamURL        string    `json:"stream_url"`
	ExpiresAt        time.Time `json:"expires_at"`
	ExpiresInSeconds int       `json:"expires_in_seconds,omitempty"`
	DurationSeconds  int32     `json:"duration_seconds"`
	MimeType         string    `json:"mime_type,omitempty"`
	SizeBytes        int64     `json:"size_bytes,omitempty"`
}

type MediaStreamSigner interface {
	GetCoursePrivateStreamURL(ctx context.Context, tenantID, assetID uuid.UUID) (*media.StreamURLOutput, error)
}

type BuyerService struct {
	queries     Querier
	otpSender   OTPSender
	mediaSigner MediaStreamSigner
	nowFunc     func() time.Time
}

func NewBuyerService(q Querier, sender OTPSender, ms ...MediaStreamSigner) *BuyerService {
	if sender == nil {
		sender = NewMockOTPSender()
	}
	s := &BuyerService{
		queries:   q,
		otpSender: sender,
		nowFunc:   time.Now,
	}
	if len(ms) > 0 {
		s.mediaSigner = ms[0]
	}
	return s
}

func (s *BuyerService) SetMediaSigner(ms MediaStreamSigner) {
	s.mediaSigner = ms
}

func (s *BuyerService) SetNowFunc(f func() time.Time) {
	s.nowFunc = f
}

func (s *BuyerService) SendOTP(ctx context.Context, tenantID uuid.UUID, rawPhone string) (*SendOTPResult, error) {
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return nil, err
	}

	now := s.nowFunc()
	// Invalidate any unconsumed codes for this phone & tenant
	_ = s.queries.InvalidateExistingBuyerOTPs(ctx, coursesdb.InvalidateExistingBuyerOTPsParams{
		TenantID:   toPgUUID(tenantID),
		Phone:      phone,
		ConsumedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})

	code, err := GenerateRandomOTPCode()
	if err != nil {
		return nil, fmt.Errorf("generate otp code: %w", err)
	}

	codeHash := HashToken(code)
	expiresAt := now.Add(120 * time.Second)

	_, err = s.queries.CreateBuyerOTPCode(ctx, coursesdb.CreateBuyerOTPCodeParams{
		TenantID:  toPgUUID(tenantID),
		Phone:     phone,
		CodeHash:  codeHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create otp record: %w", err)
	}

	if err := s.otpSender.SendOTP(ctx, phone, code); err != nil {
		return nil, fmt.Errorf("send otp delivery: %w", err)
	}

	return &SendOTPResult{
		Phone:            phone,
		ExpiresInSeconds: 120,
		Message:          "کد تأیید با موفقیت ارسال شد",
	}, nil
}

func (s *BuyerService) VerifyOTP(ctx context.Context, tenantID uuid.UUID, rawPhone, code, fullName string) (*BuyerSessionInfo, error) {
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return nil, err
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrOTPInvalid
	}

	now := s.nowFunc()
	otp, err := s.queries.GetLatestActiveBuyerOTP(ctx, coursesdb.GetLatestActiveBuyerOTPParams{
		TenantID:  toPgUUID(tenantID),
		Phone:     phone,
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOTPNotFoundOrExpired
		}
		return nil, fmt.Errorf("get active otp: %w", err)
	}

	if otp.Attempts >= 5 {
		return nil, ErrOTPExceededAttempts
	}

	providedHash := HashToken(code)
	if providedHash != otp.CodeHash {
		_ = s.queries.IncrementBuyerOTPAttempts(ctx, otp.ID)
		return nil, ErrOTPInvalid
	}

	// Consume OTP immediately to prevent replay
	if err := s.queries.ConsumeBuyerOTP(ctx, coursesdb.ConsumeBuyerOTPParams{
		ID:         otp.ID,
		ConsumedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		return nil, fmt.Errorf("consume otp: %w", err)
	}

	// Find or create buyer account
	account, err := s.queries.GetBuyerAccountByPhone(ctx, coursesdb.GetBuyerAccountByPhoneParams{
		TenantID: toPgUUID(tenantID),
		Phone:    phone,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Register new buyer account
			name := strings.TrimSpace(fullName)
			account, err = s.queries.CreateBuyerAccount(ctx, coursesdb.CreateBuyerAccountParams{
				TenantID: toPgUUID(tenantID),
				Phone:    phone,
				FullName: name,
				Status:   "active",
			})
			if err != nil {
				return nil, fmt.Errorf("create buyer account: %w", err)
			}
		} else {
			return nil, fmt.Errorf("find buyer account: %w", err)
		}
	} else {
		// Existing account: check status
		if account.Status != "active" {
			return nil, ErrBuyerBlocked
		}
		// Update full name if not set and provided
		if strings.TrimSpace(fullName) != "" && strings.TrimSpace(account.FullName) == "" {
			account, err = s.queries.UpdateBuyerAccount(ctx, coursesdb.UpdateBuyerAccountParams{
				TenantID: toPgUUID(tenantID),
				ID:       account.ID,
				FullName: strings.TrimSpace(fullName),
				Status:   account.Status,
			})
			if err != nil {
				return nil, fmt.Errorf("update buyer name: %w", err)
			}
		}
	}

	// Generate secure session token (32 bytes = 64 hex chars)
	rawSessionToken, err := GenerateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	tokenHash := HashToken(rawSessionToken)
	sessionExpiresAt := now.Add(30 * 24 * time.Hour) // 30-day session

	_, err = s.queries.CreateBuyerSession(ctx, coursesdb.CreateBuyerSessionParams{
		TenantID:  toPgUUID(tenantID),
		BuyerID:   account.ID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: sessionExpiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create buyer session: %w", err)
	}

	return &BuyerSessionInfo{
		SessionToken: rawSessionToken,
		ExpiresAt:    sessionExpiresAt,
		Account: BuyerAccount{
			ID:        fromPgUUID(account.ID),
			TenantID:  fromPgUUID(account.TenantID),
			Phone:     account.Phone,
			FullName:  account.FullName,
			Status:    account.Status,
			CreatedAt: account.CreatedAt.Time,
			UpdatedAt: account.UpdatedAt.Time,
		},
	}, nil
}

func (s *BuyerService) AuthenticateSession(ctx context.Context, tenantID uuid.UUID, rawToken string) (*BuyerAccount, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrInvalidBuyerSession
	}

	tokenHash := HashToken(rawToken)
	sess, err := s.queries.GetBuyerSessionByTokenHash(ctx, coursesdb.GetBuyerSessionByTokenHashParams{
		TenantID:  toPgUUID(tenantID),
		TokenHash: tokenHash,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidBuyerSession
		}
		return nil, fmt.Errorf("query buyer session: %w", err)
	}

	if sess.BuyerStatus != "active" {
		return nil, ErrBuyerBlocked
	}

	return &BuyerAccount{
		ID:        fromPgUUID(sess.BuyerID),
		TenantID:  fromPgUUID(sess.TenantID),
		Phone:     sess.Phone,
		FullName:  sess.FullName,
		Status:    sess.BuyerStatus,
		CreatedAt: sess.CreatedAt.Time,
		UpdatedAt: sess.ExpiresAt.Time,
	}, nil
}

func (s *BuyerService) Logout(ctx context.Context, tenantID uuid.UUID, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil
	}

	tokenHash := HashToken(rawToken)
	return s.queries.DeleteBuyerSession(ctx, coursesdb.DeleteBuyerSessionParams{
		TenantID:  toPgUUID(tenantID),
		TokenHash: tokenHash,
	})
}

func (s *BuyerService) GetMyCourses(ctx context.Context, tenantID, buyerID uuid.UUID) ([]BuyerCourseEntitlementItem, error) {
	rows, err := s.queries.ListBuyerCourseEntitlements(ctx, coursesdb.ListBuyerCourseEntitlementsParams{
		TenantID: toPgUUID(tenantID),
		BuyerID:  toPgUUID(buyerID),
	})
	if err != nil {
		return nil, fmt.Errorf("list buyer entitlements: %w", err)
	}

	result := make([]BuyerCourseEntitlementItem, 0, len(rows))
	for _, r := range rows {
		item := BuyerCourseEntitlementItem{
			ID:                  fromPgUUID(r.ID),
			CourseID:            fromPgUUID(r.CourseID),
			CourseTitleSnapshot: r.CourseTitleSnapshot,
			PriceRialsSnapshot:  r.PriceRialsSnapshot,
			Status:              r.Status,
			GrantedAt:           r.GrantedAt.Time,
			CurrentCourseTitle:  r.CurrentCourseTitle,
			CourseSlug:          r.CourseSlug,
			CourseStatus:        r.CourseStatus,
		}
		if r.CoverMediaID.Valid {
			cid := fromPgUUID(r.CoverMediaID)
			item.CoverMediaID = &cid
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *BuyerService) GrantCourseEntitlement(
	ctx context.Context,
	tenantID, buyerID, courseID uuid.UUID,
	purchaseID *uuid.UUID,
	courseTitleSnapshot string,
	priceRialsSnapshot int64,
) (*coursesdb.CourseEntitlement, error) {
	now := s.nowFunc()
	var pid pgtype.UUID
	if purchaseID != nil {
		pid = toPgUUID(*purchaseID)
	}

	ent, err := s.queries.CreateCourseEntitlement(ctx, coursesdb.CreateCourseEntitlementParams{
		TenantID:            toPgUUID(tenantID),
		BuyerID:             toPgUUID(buyerID),
		CourseID:            toPgUUID(courseID),
		PurchaseID:          pid,
		Status:              "active",
		CourseTitleSnapshot: courseTitleSnapshot,
		PriceRialsSnapshot:  priceRialsSnapshot,
		GrantedAt:           pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create course entitlement: %w", err)
	}
	return &ent, nil
}

// GetPrivateLessonStreamURL verifies buyer entitlement, course lifecycle, and lesson relations,
// then issues a signed streaming URL with a maximum 5-minute TTL.
// Tests must verify: without purchase, other account, other tenant, TTL expiry, and revoked entitlement.
func (s *BuyerService) GetPrivateLessonStreamURL(ctx context.Context, tenantID, buyerID, courseID, lessonID uuid.UUID) (*LessonStreamOutput, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("courses: tenant_id cannot be nil")
	}
	if buyerID == uuid.Nil {
		return nil, errors.New("courses: buyer_id cannot be nil")
	}
	if courseID == uuid.Nil {
		return nil, errors.New("courses: course_id cannot be nil")
	}
	if lessonID == uuid.Nil {
		return nil, errors.New("courses: lesson_id cannot be nil")
	}

	// 1. Check active entitlement for this buyer and course in this tenant
	_, err := s.queries.GetActiveCourseEntitlement(ctx, coursesdb.GetActiveCourseEntitlementParams{
		TenantID: toPgUUID(tenantID),
		BuyerID:  toPgUUID(buyerID),
		CourseID: toPgUUID(courseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Check if user had an entitlement that was revoked
			anyEnt, errEnt := s.queries.GetCourseEntitlementByBuyerAndCourse(ctx, coursesdb.GetCourseEntitlementByBuyerAndCourseParams{
				TenantID: toPgUUID(tenantID),
				BuyerID:  toPgUUID(buyerID),
				CourseID: toPgUUID(courseID),
			})
			if errEnt == nil && anyEnt.Status == "revoked" {
				return nil, ErrEntitlementRevoked
			}
			return nil, ErrNotPurchased
		}
		return nil, err
	}

	// 2. Check course
	course, err := s.queries.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(courseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, err
	}
	if course.Status == "draft" {
		return nil, ErrCourseNotPublished
	}
	// Note: "آرشیو دوره خرید تازه را می‌بندد و خریدار قبلی به جلسات موجود دسترسی دارد."
	// Therefore both "published" and "archived" allow streaming for existing entitled buyers.

	// 3. Check lesson
	lesson, err := s.queries.GetCourseLessonByID(ctx, coursesdb.GetCourseLessonByIDParams{
		TenantID: toPgUUID(tenantID),
		CourseID: toPgUUID(courseID),
		ID:       toPgUUID(lessonID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLessonNotFound
		}
		return nil, err
	}
	if fromPgUUID(lesson.CourseID) != courseID {
		return nil, ErrLessonNotFound
	}

	// 4. Generate signed private stream URL
	if s.mediaSigner == nil {
		return nil, errors.New("courses: media stream signer not configured")
	}
	streamOut, err := s.mediaSigner.GetCoursePrivateStreamURL(ctx, tenantID, fromPgUUID(lesson.MediaAssetID))
	if err != nil {
		return nil, fmt.Errorf("generate private stream url: %w", err)
	}

	now := s.nowFunc()
	expiresAt := now.Add(time.Duration(streamOut.ExpiresInSeconds) * time.Second)

	return &LessonStreamOutput{
		StreamURL:        streamOut.StreamURL,
		ExpiresAt:        expiresAt,
		ExpiresInSeconds: streamOut.ExpiresInSeconds,
		DurationSeconds:  lesson.DurationSeconds,
		MimeType:         streamOut.MimeType,
		SizeBytes:        streamOut.SizeBytes,
	}, nil
}

// GetPublicLessonPreviewStreamURL issues a signed streaming URL for a lesson marked as is_free_preview=true.
func (s *BuyerService) GetPublicLessonPreviewStreamURL(ctx context.Context, tenantID uuid.UUID, slug string, lessonID uuid.UUID) (*LessonStreamOutput, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("courses: tenant_id cannot be nil")
	}
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return nil, ErrCourseNotFound
	}
	if lessonID == uuid.Nil {
		return nil, errors.New("courses: lesson_id cannot be nil")
	}

	// 1. Resolve course by slug
	course, err := s.queries.GetCourseBySlug(ctx, coursesdb.GetCourseBySlugParams{
		TenantID: toPgUUID(tenantID),
		Slug:     trimmedSlug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, err
	}
	if course.Status != "published" {
		return nil, ErrCourseNotFound
	}

	// 2. Resolve lesson
	lesson, err := s.queries.GetCourseLessonByID(ctx, coursesdb.GetCourseLessonByIDParams{
		TenantID: toPgUUID(tenantID),
		CourseID: course.ID,
		ID:       toPgUUID(lessonID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLessonNotFound
		}
		return nil, err
	}
	if fromPgUUID(lesson.CourseID) != fromPgUUID(course.ID) {
		return nil, ErrLessonNotFound
	}
	if !lesson.IsFreePreview {
		return nil, ErrLessonNotFreePreview
	}

	// 3. Generate signed streaming URL
	if s.mediaSigner == nil {
		return nil, errors.New("courses: media stream signer not configured")
	}
	streamOut, err := s.mediaSigner.GetCoursePrivateStreamURL(ctx, tenantID, fromPgUUID(lesson.MediaAssetID))
	if err != nil {
		return nil, fmt.Errorf("generate preview stream url: %w", err)
	}

	now := s.nowFunc()
	expiresAt := now.Add(time.Duration(streamOut.ExpiresInSeconds) * time.Second)

	return &LessonStreamOutput{
		StreamURL:        streamOut.StreamURL,
		ExpiresAt:        expiresAt,
		ExpiresInSeconds: streamOut.ExpiresInSeconds,
		DurationSeconds:  lesson.DurationSeconds,
		MimeType:         streamOut.MimeType,
		SizeBytes:        streamOut.SizeBytes,
	}, nil
}
