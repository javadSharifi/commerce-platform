package courses

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"psychology/api/internal/courses/coursesdb"
	"psychology/api/internal/education"
	"psychology/api/internal/media"
	"psychology/api/internal/platform/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrCourseNotFound              = errors.New("courses: دوره آموزشی یافت نشد")
	ErrLessonNotFound              = errors.New("courses: جلسه آموزشی یافت نشد")
	ErrCannotPublishWithoutLessons = errors.New("courses: حداقل یک جلسه آموزشی برای انتشار دوره الزامی است")
	ErrInvalidCoursePrice          = errors.New("courses: مبلغ دوره نمی‌تواند منفی باشد")
	ErrCourseArchived              = errors.New("courses: دوره بایگانی شده است و امکان انتشار مجدد ندارد")
	ErrDisallowedMediaAccessClass  = errors.New("courses: فقط فایل‌های با کلاس دسترسی خصوصی دوره (course_private) مجاز به پیوند هستند و فایل‌های عمومی یا بلاگ پذیرفته نمی‌شوند")
	ErrMediaCrossTenantForbidden   = errors.New("courses: پیوند به فایل رسانه متعلق به کلینیک دیگر غیرمجاز است")
	ErrMediaNotVerified            = errors.New("courses: فایل رسانه انتخاب‌شده هنوز تایید نهایی نشده است")
)

type MediaAssetInfo struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	AccessClass      string
	ValidationStatus string
	MimeType         string
	SizeBytes        int64
}

type MediaAssetValidator interface {
	GetMediaAsset(ctx context.Context, tenantID, mediaAssetID uuid.UUID) (*MediaAssetInfo, error)
}

type AdminCourseItem struct {
	Topic          string     `json:"topic,omitempty"`
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Description    string     `json:"description"`
	InstructorName string     `json:"instructor_name"`
	PriceRials     int64      `json:"price_rials"`
	CoverMediaID   *uuid.UUID `json:"cover_media_id,omitempty"`
	Status         string     `json:"status"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	LessonsCount   int        `json:"lessons_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AdminLessonItem struct {
	ID              uuid.UUID `json:"id"`
	TenantID        uuid.UUID `json:"tenant_id"`
	CourseID        uuid.UUID `json:"course_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	DisplayOrder    int       `json:"display_order"`
	MediaAssetID    uuid.UUID `json:"media_asset_id"`
	DurationSeconds int       `json:"duration_seconds"`
	IsFreePreview   bool      `json:"is_free_preview"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AdminCourseEntitlementItem struct {
	ID                  uuid.UUID  `json:"id"`
	TenantID            uuid.UUID  `json:"tenant_id"`
	BuyerID             uuid.UUID  `json:"buyer_id"`
	BuyerPhone          string     `json:"buyer_phone"`
	BuyerName           string     `json:"buyer_name"`
	CourseID            uuid.UUID  `json:"course_id"`
	PurchaseID          *uuid.UUID `json:"purchase_id,omitempty"`
	Status              string     `json:"status"`
	CourseTitleSnapshot string     `json:"course_title_snapshot"`
	PriceRialsSnapshot  int64      `json:"price_rials_snapshot"`
	GrantedAt           time.Time  `json:"granted_at"`
}

type CreateCourseInput struct {
	Topic          string     `json:"topic,omitempty"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Description    string     `json:"description"`
	InstructorName string     `json:"instructor_name"`
	PriceRials     int64      `json:"price_rials"`
	CoverMediaID   *uuid.UUID `json:"cover_media_id,omitempty"`
}

type UpdateCourseInput struct {
	Topic          *string    `json:"topic,omitempty"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Description    *string    `json:"description"`
	InstructorName *string    `json:"instructor_name"`
	PriceRials     *int64     `json:"price_rials"`
	CoverMediaID   *uuid.UUID `json:"cover_media_id,omitempty"`
}

type CreateLessonInput struct {
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	DisplayOrder    int       `json:"display_order"`
	MediaAssetID    uuid.UUID `json:"media_asset_id"`
	DurationSeconds int       `json:"duration_seconds"`
	IsFreePreview   bool      `json:"is_free_preview"`
}

type PublicCourseItem struct {
	Topic          string     `json:"topic,omitempty"`
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Description    string     `json:"description"`
	InstructorName string     `json:"instructor_name"`
	PriceRials     int64      `json:"price_rials"`
	CoverMediaID   *uuid.UUID `json:"cover_media_id,omitempty"`
	Status         string     `json:"status"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	LessonsCount   int        `json:"lessons_count"`
}

type PublicLessonItem struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	DisplayOrder    int32     `json:"display_order"`
	DurationSeconds int32     `json:"duration_seconds"`
	IsFreePreview   bool      `json:"is_free_preview"`
}

type PublicCourseDetail struct {
	Topic          string             `json:"topic,omitempty"`
	ID             uuid.UUID          `json:"id"`
	Title          string             `json:"title"`
	Slug           string             `json:"slug"`
	Description    string             `json:"description"`
	InstructorName string             `json:"instructor_name"`
	PriceRials     int64              `json:"price_rials"`
	CoverMediaID   *uuid.UUID         `json:"cover_media_id,omitempty"`
	Status         string             `json:"status"`
	PublishedAt    *time.Time         `json:"published_at,omitempty"`
	Lessons        []PublicLessonItem `json:"lessons"`
}

type UpdateLessonInput struct {
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	DisplayOrder    int       `json:"display_order"`
	MediaAssetID    uuid.UUID `json:"media_asset_id"`
	DurationSeconds int       `json:"duration_seconds"`
	IsFreePreview   bool      `json:"is_free_preview"`
}

type AdminCourseService struct {
	catalogStore   *db.Store
	queries        Querier
	mediaValidator MediaAssetValidator
	nowFunc        func() time.Time
}

func NewAdminCourseService(q Querier, mv MediaAssetValidator) *AdminCourseService {
	return &AdminCourseService{
		queries:        q,
		mediaValidator: mv,
		nowFunc:        time.Now,
	}
}

func (s *AdminCourseService) SetNowFunc(f func() time.Time) {
	s.nowFunc = f
}

func (s *AdminCourseService) ListCourses(ctx context.Context, tenantID uuid.UUID, statusFilter string) ([]AdminCourseItem, error) {
	return db.ReadTenant(ctx, s.catalogStore, tenantID, s.queries,
		func(tx pgx.Tx) Querier { return coursesdb.New(tx) },
		func(q Querier) ([]AdminCourseItem, error) { return s.listCourses(ctx, q, tenantID, statusFilter) },
	)
}

func (s *AdminCourseService) listCourses(ctx context.Context, q Querier, tenantID uuid.UUID, statusFilter string) ([]AdminCourseItem, error) {
	coursesList, err := q.ListAdminCourses(ctx, toPgUUID(tenantID))
	if err != nil {
		return nil, fmt.Errorf("list admin courses: %w", err)
	}

	result := make([]AdminCourseItem, 0, len(coursesList))
	for _, c := range coursesList {
		if statusFilter != "" && statusFilter != "all" && c.Status != statusFilter {
			continue
		}
		item := AdminCourseItem{
			ID:             fromPgUUID(c.ID),
			TenantID:       fromPgUUID(c.TenantID),
			Title:          c.Title,
			Topic:          c.Topic,
			Slug:           c.Slug,
			Description:    c.Description,
			InstructorName: c.InstructorName,
			PriceRials:     c.PriceRials,
			Status:         c.Status,
			CreatedAt:      c.CreatedAt.Time,
			UpdatedAt:      c.UpdatedAt.Time,
		}
		if c.CoverMediaID.Valid {
			cid := fromPgUUID(c.CoverMediaID)
			item.CoverMediaID = &cid
		}
		if c.PublishedAt.Valid {
			item.PublishedAt = &c.PublishedAt.Time
		}

		// Count lessons
		lessons, err := q.ListCourseLessons(ctx, coursesdb.ListCourseLessonsParams{
			TenantID: c.TenantID,
			CourseID: c.ID,
		})
		if err == nil {
			item.LessonsCount = len(lessons)
		}

		result = append(result, item)
	}
	return result, nil
}

func (s *AdminCourseService) GetCourse(ctx context.Context, tenantID, courseID uuid.UUID) (*AdminCourseItem, error) {
	return db.ReadTenant(ctx, s.catalogStore, tenantID, s.queries,
		func(tx pgx.Tx) Querier { return coursesdb.New(tx) },
		func(q Querier) (*AdminCourseItem, error) { return s.getCourse(ctx, q, tenantID, courseID) },
	)
}

func (s *AdminCourseService) getCourse(ctx context.Context, q Querier, tenantID, courseID uuid.UUID) (*AdminCourseItem, error) {
	c, err := q.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(courseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, fmt.Errorf("get course: %w", err)
	}

	item := &AdminCourseItem{
		ID:             fromPgUUID(c.ID),
		TenantID:       fromPgUUID(c.TenantID),
		Title:          c.Title,
		Topic:          c.Topic,
		Slug:           c.Slug,
		Description:    c.Description,
		InstructorName: c.InstructorName,
		PriceRials:     c.PriceRials,
		Status:         c.Status,
		CreatedAt:      c.CreatedAt.Time,
		UpdatedAt:      c.UpdatedAt.Time,
	}
	if c.CoverMediaID.Valid {
		cid := fromPgUUID(c.CoverMediaID)
		item.CoverMediaID = &cid
	}
	if c.PublishedAt.Valid {
		item.PublishedAt = &c.PublishedAt.Time
	}

	lessons, err := q.ListCourseLessons(ctx, coursesdb.ListCourseLessonsParams{
		TenantID: c.TenantID,
		CourseID: c.ID,
	})
	if err == nil {
		item.LessonsCount = len(lessons)
	}

	return item, nil
}

func (s *AdminCourseService) CreateCourse(ctx context.Context, tenantID uuid.UUID, in CreateCourseInput) (*AdminCourseItem, error) {
	if s.catalogStore == nil {
		return s.createCourse(ctx, s.queries, tenantID, in)
	}
	return education.WriteCatalog(ctx, s.catalogStore, tenantID, func(tx pgx.Tx) (*AdminCourseItem, error) {
		return s.createCourse(ctx, coursesdb.New(tx), tenantID, in)
	})
}

func (s *AdminCourseService) createCourse(ctx context.Context, q Querier, tenantID uuid.UUID, in CreateCourseInput) (*AdminCourseItem, error) {
	topic, err := education.NormalizeTopic(in.Topic)
	if err != nil {
		return nil, err
	}

	if in.PriceRials < 0 {
		return nil, ErrInvalidCoursePrice
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, errors.New("courses: عنوان دوره الزامی است")
	}
	slug := strings.TrimSpace(strings.ToLower(in.Slug))
	if slug == "" {
		slug = strings.ReplaceAll(title, " ", "-")
	}

	var coverID pgtype.UUID
	if in.CoverMediaID != nil {
		coverID = toPgUUID(*in.CoverMediaID)
	}

	c, err := q.CreateCourse(ctx, coursesdb.CreateCourseParams{
		TenantID:       toPgUUID(tenantID),
		Topic:          topic,
		Title:          title,
		Slug:           slug,
		Description:    in.Description,
		InstructorName: in.InstructorName,
		PriceRials:     in.PriceRials,
		CoverMediaID:   coverID,
		Status:         "draft",
		PublishedAt:    pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("create course: %w", err)
	}

	item := &AdminCourseItem{
		ID:             fromPgUUID(c.ID),
		TenantID:       fromPgUUID(c.TenantID),
		Title:          c.Title,
		Topic:          c.Topic,
		Slug:           c.Slug,
		Description:    c.Description,
		InstructorName: c.InstructorName,
		PriceRials:     c.PriceRials,
		Status:         c.Status,
		LessonsCount:   0,
		CreatedAt:      c.CreatedAt.Time,
		UpdatedAt:      c.UpdatedAt.Time,
	}
	if in.CoverMediaID != nil {
		item.CoverMediaID = in.CoverMediaID
	}
	return item, nil
}

func (s *AdminCourseService) UpdateCourse(ctx context.Context, tenantID, courseID uuid.UUID, in UpdateCourseInput) (*AdminCourseItem, error) {
	if s.catalogStore == nil {
		return s.updateCourse(ctx, s.queries, nil, tenantID, courseID, in)
	}
	return education.WriteCatalog(ctx, s.catalogStore, tenantID, func(tx pgx.Tx) (*AdminCourseItem, error) {
		return s.updateCourse(ctx, coursesdb.New(tx), tx, tenantID, courseID, in)
	})
}

func (s *AdminCourseService) updateCourse(ctx context.Context, q Querier, tx pgx.Tx, tenantID, courseID uuid.UUID, in UpdateCourseInput) (*AdminCourseItem, error) {
	var topic pgtype.Text
	if in.Topic != nil {
		value, err := education.NormalizeTopic(*in.Topic)
		if err != nil {
			return nil, err
		}
		topic = pgtype.Text{String: value, Valid: true}
	}

	if in.PriceRials != nil && *in.PriceRials < 0 {
		return nil, ErrInvalidCoursePrice
	}

	existing, err := q.GetCourseByIDForUpdate(ctx, coursesdb.GetCourseByIDForUpdateParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(courseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, err
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = existing.Title
	}
	slug := strings.TrimSpace(strings.ToLower(in.Slug))
	if slug == "" {
		slug = existing.Slug
	}

	description := existing.Description
	if in.Description != nil {
		description = *in.Description
	}
	instructor := existing.InstructorName
	if in.InstructorName != nil {
		instructor = *in.InstructorName
	}
	price := existing.PriceRials
	if in.PriceRials != nil {
		price = *in.PriceRials
	}
	coverID := existing.CoverMediaID
	if in.CoverMediaID != nil {
		coverID = toPgUUID(*in.CoverMediaID)
	}

	c, err := q.UpdateCourse(ctx, coursesdb.UpdateCourseParams{
		TenantID:       toPgUUID(tenantID),
		ID:             toPgUUID(courseID),
		Title:          title,
		Slug:           slug,
		Description:    description,
		InstructorName: instructor,
		PriceRials:     price,
		CoverMediaID:   coverID,
		Status:         existing.Status,
		PublishedAt:    existing.PublishedAt,
		Topic:          topic,
	})
	if err != nil {
		return nil, fmt.Errorf("update course: %w", err)
	}

	if tx != nil && c.Status == "published" {
		paths := []string{"/", "/courses", "/courses/" + url.PathEscape(c.Slug)}
		if existing.Slug != c.Slug {
			paths = append(paths, "/courses/"+url.PathEscape(existing.Slug))
		}
		if err := education.InvalidateCatalog(ctx, tx, tenantID, paths...); err != nil {
			return nil, err
		}
	}
	return s.getCourse(ctx, q, tenantID, courseID)
}

func (s *AdminCourseService) DeleteCourse(ctx context.Context, tenantID, courseID uuid.UUID) error {
	return s.queries.SoftDeleteCourse(ctx, coursesdb.SoftDeleteCourseParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(courseID),
	})
}

func (s *AdminCourseService) PublishCourse(ctx context.Context, tenantID, courseID uuid.UUID) (*AdminCourseItem, error) {
	existing, err := s.queries.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(courseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, err
	}

	if existing.Status == "archived" {
		return nil, ErrCourseArchived
	}

	if existing.PriceRials < 0 {
		return nil, ErrInvalidCoursePrice
	}

	// Invariant: At least 1 prepared lesson is strictly required to publish a course!
	lessons, err := s.queries.ListCourseLessons(ctx, coursesdb.ListCourseLessonsParams{
		TenantID: toPgUUID(tenantID),
		CourseID: toPgUUID(courseID),
	})
	if err != nil {
		return nil, fmt.Errorf("list course lessons for publish: %w", err)
	}
	if len(lessons) == 0 {
		return nil, ErrCannotPublishWithoutLessons
	}

	now := s.nowFunc()
	c, err := s.queries.UpdateCourse(ctx, coursesdb.UpdateCourseParams{
		TenantID:       toPgUUID(tenantID),
		ID:             toPgUUID(courseID),
		Title:          existing.Title,
		Slug:           existing.Slug,
		Description:    existing.Description,
		InstructorName: existing.InstructorName,
		PriceRials:     existing.PriceRials,
		CoverMediaID:   existing.CoverMediaID,
		Status:         "published",
		PublishedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("publish course: %w", err)
	}

	return s.GetCourse(ctx, tenantID, fromPgUUID(c.ID))
}

func (s *AdminCourseService) ArchiveCourse(ctx context.Context, tenantID, courseID uuid.UUID) (*AdminCourseItem, error) {
	existing, err := s.queries.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(courseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, err
	}

	c, err := s.queries.UpdateCourse(ctx, coursesdb.UpdateCourseParams{
		TenantID:       toPgUUID(tenantID),
		ID:             toPgUUID(courseID),
		Title:          existing.Title,
		Slug:           existing.Slug,
		Description:    existing.Description,
		InstructorName: existing.InstructorName,
		PriceRials:     existing.PriceRials,
		CoverMediaID:   existing.CoverMediaID,
		Status:         "archived",
		PublishedAt:    existing.PublishedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("archive course: %w", err)
	}

	return s.GetCourse(ctx, tenantID, fromPgUUID(c.ID))
}

func (s *AdminCourseService) ListLessons(ctx context.Context, tenantID, courseID uuid.UUID) ([]AdminLessonItem, error) {
	lessons, err := s.queries.ListCourseLessons(ctx, coursesdb.ListCourseLessonsParams{
		TenantID: toPgUUID(tenantID),
		CourseID: toPgUUID(courseID),
	})
	if err != nil {
		return nil, fmt.Errorf("list course lessons: %w", err)
	}

	result := make([]AdminLessonItem, 0, len(lessons))
	for _, l := range lessons {
		result = append(result, AdminLessonItem{
			ID:              fromPgUUID(l.ID),
			TenantID:        fromPgUUID(l.TenantID),
			CourseID:        fromPgUUID(l.CourseID),
			Title:           l.Title,
			Description:     l.Description,
			DisplayOrder:    int(l.DisplayOrder),
			MediaAssetID:    fromPgUUID(l.MediaAssetID),
			DurationSeconds: int(l.DurationSeconds),
			IsFreePreview:   l.IsFreePreview,
			CreatedAt:       l.CreatedAt.Time,
			UpdatedAt:       l.UpdatedAt.Time,
		})
	}
	return result, nil
}

func (s *AdminCourseService) GetLesson(ctx context.Context, tenantID, courseID, lessonID uuid.UUID) (*AdminLessonItem, error) {
	l, err := s.queries.GetCourseLessonByID(ctx, coursesdb.GetCourseLessonByIDParams{
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

	return &AdminLessonItem{
		ID:              fromPgUUID(l.ID),
		TenantID:        fromPgUUID(l.TenantID),
		CourseID:        fromPgUUID(l.CourseID),
		Title:           l.Title,
		Description:     l.Description,
		DisplayOrder:    int(l.DisplayOrder),
		MediaAssetID:    fromPgUUID(l.MediaAssetID),
		DurationSeconds: int(l.DurationSeconds),
		IsFreePreview:   l.IsFreePreview,
		CreatedAt:       l.CreatedAt.Time,
		UpdatedAt:       l.UpdatedAt.Time,
	}, nil
}

func (s *AdminCourseService) CreateLesson(ctx context.Context, tenantID, courseID uuid.UUID, in CreateLessonInput) (*AdminLessonItem, error) {
	// Verify course exists
	_, err := s.queries.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(courseID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, err
	}

	// Validate Media Asset separation & access class
	if s.mediaValidator != nil {
		media, err := s.mediaValidator.GetMediaAsset(ctx, tenantID, in.MediaAssetID)
		if err != nil {
			return nil, err
		}
		if media.TenantID != tenantID {
			return nil, ErrMediaCrossTenantForbidden
		}
		if media.AccessClass != "course_private" {
			return nil, ErrDisallowedMediaAccessClass
		}
		if media.ValidationStatus != "verified" {
			return nil, ErrMediaNotVerified
		}
	}

	order := in.DisplayOrder
	if order <= 0 {
		order = 1
	}

	l, err := s.queries.CreateCourseLesson(ctx, coursesdb.CreateCourseLessonParams{
		TenantID:        toPgUUID(tenantID),
		CourseID:        toPgUUID(courseID),
		Title:           in.Title,
		Description:     in.Description,
		DisplayOrder:    int32(order),
		MediaAssetID:    toPgUUID(in.MediaAssetID),
		DurationSeconds: int32(in.DurationSeconds),
		IsFreePreview:   in.IsFreePreview,
	})
	if err != nil {
		return nil, fmt.Errorf("create course lesson: %w", err)
	}

	return &AdminLessonItem{
		ID:              fromPgUUID(l.ID),
		TenantID:        fromPgUUID(l.TenantID),
		CourseID:        fromPgUUID(l.CourseID),
		Title:           l.Title,
		Description:     l.Description,
		DisplayOrder:    int(l.DisplayOrder),
		MediaAssetID:    fromPgUUID(l.MediaAssetID),
		DurationSeconds: int(l.DurationSeconds),
		IsFreePreview:   l.IsFreePreview,
		CreatedAt:       l.CreatedAt.Time,
		UpdatedAt:       l.UpdatedAt.Time,
	}, nil
}

func (s *AdminCourseService) UpdateLesson(ctx context.Context, tenantID, courseID, lessonID uuid.UUID, in UpdateLessonInput) (*AdminLessonItem, error) {
	// Validate Media Asset if changing
	if s.mediaValidator != nil && in.MediaAssetID != uuid.Nil {
		media, err := s.mediaValidator.GetMediaAsset(ctx, tenantID, in.MediaAssetID)
		if err != nil {
			return nil, err
		}
		if media.TenantID != tenantID {
			return nil, ErrMediaCrossTenantForbidden
		}
		if media.AccessClass != "course_private" {
			return nil, ErrDisallowedMediaAccessClass
		}
		if media.ValidationStatus != "verified" {
			return nil, ErrMediaNotVerified
		}
	}

	l, err := s.queries.UpdateCourseLesson(ctx, coursesdb.UpdateCourseLessonParams{
		TenantID:        toPgUUID(tenantID),
		CourseID:        toPgUUID(courseID),
		ID:              toPgUUID(lessonID),
		Title:           in.Title,
		Description:     in.Description,
		DisplayOrder:    int32(in.DisplayOrder),
		MediaAssetID:    toPgUUID(in.MediaAssetID),
		DurationSeconds: int32(in.DurationSeconds),
		IsFreePreview:   in.IsFreePreview,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLessonNotFound
		}
		return nil, fmt.Errorf("update course lesson: %w", err)
	}

	return &AdminLessonItem{
		ID:              fromPgUUID(l.ID),
		TenantID:        fromPgUUID(l.TenantID),
		CourseID:        fromPgUUID(l.CourseID),
		Title:           l.Title,
		Description:     l.Description,
		DisplayOrder:    int(l.DisplayOrder),
		MediaAssetID:    fromPgUUID(l.MediaAssetID),
		DurationSeconds: int(l.DurationSeconds),
		IsFreePreview:   l.IsFreePreview,
		CreatedAt:       l.CreatedAt.Time,
		UpdatedAt:       l.UpdatedAt.Time,
	}, nil
}

func (s *AdminCourseService) DeleteLesson(ctx context.Context, tenantID, courseID, lessonID uuid.UUID) error {
	return s.queries.SoftDeleteCourseLesson(ctx, coursesdb.SoftDeleteCourseLessonParams{
		TenantID: toPgUUID(tenantID),
		CourseID: toPgUUID(courseID),
		ID:       toPgUUID(lessonID),
	})
}

func (s *AdminCourseService) ListEntitlements(ctx context.Context, tenantID, courseID uuid.UUID) ([]AdminCourseEntitlementItem, error) {
	rows, err := s.queries.ListCourseEntitlementsForAdmin(ctx, coursesdb.ListCourseEntitlementsForAdminParams{
		TenantID: toPgUUID(tenantID),
		CourseID: toPgUUID(courseID),
	})
	if err != nil {
		return nil, fmt.Errorf("list course entitlements for admin: %w", err)
	}

	result := make([]AdminCourseEntitlementItem, 0, len(rows))
	for _, r := range rows {
		item := AdminCourseEntitlementItem{
			ID:                  fromPgUUID(r.ID),
			TenantID:            fromPgUUID(r.TenantID),
			BuyerID:             fromPgUUID(r.BuyerID),
			BuyerPhone:          r.BuyerPhone,
			BuyerName:           r.BuyerName,
			CourseID:            fromPgUUID(r.CourseID),
			Status:              r.Status,
			CourseTitleSnapshot: r.CourseTitleSnapshot,
			PriceRialsSnapshot:  r.PriceRialsSnapshot,
			GrantedAt:           r.GrantedAt.Time,
		}
		if r.PurchaseID.Valid {
			pid := fromPgUUID(r.PurchaseID)
			item.PurchaseID = &pid
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *AdminCourseService) WithCatalogStore(store *db.Store) *AdminCourseService {
	s.catalogStore = store
	return s
}

type publicCatalog struct {
	Querier
	tx pgx.Tx
}

func bindPublicCatalog(tx pgx.Tx) publicCatalog {
	return publicCatalog{Querier: coursesdb.New(tx), tx: tx}
}

func (q publicCatalog) cover(ctx context.Context, tenantID uuid.UUID, id pgtype.UUID) (*uuid.UUID, error) {
	if !id.Valid {
		return nil, nil
	}
	coverID := fromPgUUID(id)
	if q.tx == nil {
		return &coverID, nil
	}
	err := media.ValidatePublicImage(ctx, q.tx, tenantID, coverID)
	switch {
	case err == nil:
		return &coverID, nil
	case errors.Is(err, media.ErrMediaNotFound), errors.Is(err, media.ErrMediaNotPublic),
		errors.Is(err, media.ErrMediaNotVerified), errors.Is(err, media.ErrInvalidMimeType),
		errors.Is(err, media.ErrFileSizeExceeded):
		return nil, nil
	default:
		return nil, fmt.Errorf("validate public course cover: %w", err)
	}
}

func (s *AdminCourseService) ListPublishedCourses(ctx context.Context, tenantID uuid.UUID) ([]PublicCourseItem, error) {
	return db.ReadTenant(ctx, s.catalogStore, tenantID, publicCatalog{Querier: s.queries}, bindPublicCatalog,
		func(q publicCatalog) ([]PublicCourseItem, error) {
			if tenantID == uuid.Nil {
				return nil, errors.New("courses: tenant_id cannot be nil")
			}
			courses, err := q.ListPublishedCourses(ctx, toPgUUID(tenantID))
			if err != nil {
				return nil, err
			}
			res := make([]PublicCourseItem, 0, len(courses))
			for _, c := range courses {
				coverID, err := q.cover(ctx, tenantID, c.CoverMediaID)
				if err != nil {
					return nil, err
				}
				var pubAt *time.Time
				if c.PublishedAt.Valid {
					t := c.PublishedAt.Time
					pubAt = &t
				}
				lessons, err := q.ListCourseLessons(ctx, coursesdb.ListCourseLessonsParams{
					TenantID: toPgUUID(tenantID),
					CourseID: c.ID,
				})
				if err != nil {
					return nil, err
				}
				res = append(res, PublicCourseItem{
					ID:             fromPgUUID(c.ID),
					Title:          c.Title,
					Topic:          c.Topic,
					Slug:           c.Slug,
					Description:    c.Description,
					InstructorName: c.InstructorName,
					PriceRials:     c.PriceRials,
					CoverMediaID:   coverID,
					Status:         c.Status,
					PublishedAt:    pubAt,
					LessonsCount:   len(lessons),
				})
			}
			return res, nil
		},
	)
}

func (s *AdminCourseService) GetPublicCourseBySlug(ctx context.Context, tenantID uuid.UUID, slug string) (*PublicCourseDetail, error) {
	return db.ReadTenant(ctx, s.catalogStore, tenantID, publicCatalog{Querier: s.queries}, bindPublicCatalog,
		func(q publicCatalog) (*PublicCourseDetail, error) {
			if tenantID == uuid.Nil {
				return nil, errors.New("courses: tenant_id cannot be nil")
			}
			trimmed := strings.TrimSpace(slug)
			if trimmed == "" {
				return nil, ErrCourseNotFound
			}
			course, err := q.GetCourseBySlug(ctx, coursesdb.GetCourseBySlugParams{
				TenantID: toPgUUID(tenantID),
				Slug:     trimmed,
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

			lessons, err := q.ListCourseLessons(ctx, coursesdb.ListCourseLessonsParams{
				TenantID: toPgUUID(tenantID),
				CourseID: course.ID,
			})
			if err != nil {
				return nil, err
			}

			publicLessons := make([]PublicLessonItem, 0, len(lessons))
			for _, l := range lessons {
				publicLessons = append(publicLessons, PublicLessonItem{
					ID:              fromPgUUID(l.ID),
					Title:           l.Title,
					Description:     l.Description,
					DisplayOrder:    l.DisplayOrder,
					DurationSeconds: l.DurationSeconds,
					IsFreePreview:   l.IsFreePreview,
				})
			}

			coverID, err := q.cover(ctx, tenantID, course.CoverMediaID)
			if err != nil {
				return nil, err
			}
			var pubAt *time.Time
			if course.PublishedAt.Valid {
				t := course.PublishedAt.Time
				pubAt = &t
			}

			return &PublicCourseDetail{
				ID:             fromPgUUID(course.ID),
				Title:          course.Title,
				Topic:          course.Topic,
				Slug:           course.Slug,
				Description:    course.Description,
				InstructorName: course.InstructorName,
				PriceRials:     course.PriceRials,
				CoverMediaID:   coverID,
				Status:         course.Status,
				PublishedAt:    pubAt,
				Lessons:        publicLessons,
			}, nil
		},
	)
}
