package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/blog"
	"psychology/api/internal/blog/blogdb"
	"psychology/api/internal/booking"
	"psychology/api/internal/booking/bookingdb"
	"psychology/api/internal/courses"
	"psychology/api/internal/courses/coursesdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/media"
	"psychology/api/internal/media/mediadb"
	"psychology/api/internal/payments"
	"psychology/api/internal/payments/paymentsdb"
	"psychology/api/internal/platform/cache"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/retention"
	"psychology/api/internal/reports"
	"psychology/api/internal/reports/reportsdb"
	"psychology/api/internal/settings"
	"psychology/api/internal/workshops"
	"psychology/api/internal/workshops/workshopsdb"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mediaAssetValidatorAdapter struct {
	store *mediadb.Queries
}

func (a *mediaAssetValidatorAdapter) GetMediaAsset(ctx context.Context, tenantID, mediaAssetID uuid.UUID) (*courses.MediaAssetInfo, error) {
	asset, err := a.store.GetMediaAssetByID(ctx, mediadb.GetMediaAssetByIDParams{
		TenantID: pgtype.UUID{Bytes: tenantID, Valid: true},
		ID:       pgtype.UUID{Bytes: mediaAssetID, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	var tid, id uuid.UUID
	copy(tid[:], asset.TenantID.Bytes[:])
	copy(id[:], asset.ID.Bytes[:])
	return &courses.MediaAssetInfo{
		ID:               id,
		TenantID:         tid,
		AccessClass:      asset.AccessClass,
		ValidationStatus: asset.ValidationStatus,
		MimeType:         asset.MimeType,
		SizeBytes:        asset.SizeBytes,
	}, nil
}

func setupRouter(
	authHandler *auth.Handler,
	bookingHandler *booking.Handler,
	coursesHandler *courses.Handler,
	reportsHandler *reports.Handler,
	settingsHandler *settings.Handler,
	workshopsHandler *workshops.Handler,
	paymentsHandler *payments.Handler,
	blogHandler *blog.Handler,
	mediaHandler *media.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Unified standard: All API routes mounted under /v1 (AP-01 / PSY-5)
	r.Route("/v1", func(v1 chi.Router) {
		v1.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "ok",
			})
		})

		if authHandler != nil {
			authHandler.MountRoutes(v1)
		}
		if bookingHandler != nil {
			bookingHandler.MountRoutes(v1)
		}
		if coursesHandler != nil {
			coursesHandler.MountRoutes(v1)
		}
		if settingsHandler != nil {
			settingsHandler.MountRoutes(v1)
		}
		if workshopsHandler != nil {
			workshopsHandler.MountRoutes(v1)
		}
		if paymentsHandler != nil {
			paymentsHandler.MountRoutes(v1)
		}
		if blogHandler != nil {
			blogHandler.MountRoutes(v1)
		}
		if mediaHandler != nil {
			mediaHandler.MountRoutes(v1)
		}
		if reportsHandler != nil {
			reportsHandler.MountRoutes(v1)
		}
	})

	// Also support root callbacks for external payment gateways if contacted at /callbacks directly
	if paymentsHandler != nil {
		paymentsHandler.MountRoutes(r)
	}

	return r
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	var (
		authHandler      *auth.Handler
		bookingHandler   *booking.Handler
		coursesHandler   *courses.Handler
		reportsHandler   *reports.Handler
		settingsHandler  *settings.Handler
		workshopsHandler *workshops.Handler
		paymentsHandler  *payments.Handler
		blogHandler      *blog.Handler
		mediaHandler     *media.Handler
		pool             *pgxpool.Pool
	)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		var err error
		pool, err = pgxpool.New(ctx, dbURL)
		if err != nil {
			logger.Error("failed to connect to database", "error", err)
			os.Exit(1)
		}
		defer pool.Close()

		store := db.NewStore(pool)
		domainResolver := domain.NewResolver(domaindb.New(pool))
		authService := auth.NewServiceWithStore(authdb.New(pool), store)
		authHandler = auth.NewHandler(authService)

		settingsService := settings.NewServiceWithStore(store)
		settingsHandler = settings.NewHandler(settingsService, domainResolver, authService)

		workshopsStore := workshopsdb.New(pool)
		workshopsService := workshops.NewService(workshopsStore)
		workshopsHandler = workshops.NewHandler(workshopsService, domainResolver, authService)

		var paymentProvider payments.PaymentProvider = payments.NewMockPaymentProvider()
		if merchantID := os.Getenv("ZARINPAL_MERCHANT_ID"); merchantID != "" {
			paymentProvider = payments.NewZarinPalProvider(merchantID, os.Getenv("ZARINPAL_SANDBOX") == "true", http.DefaultClient)
		}

		paymentsStore := paymentsdb.New(pool)
		coursesStore := coursesdb.New(pool)
		paymentsService := payments.NewService(paymentsStore, paymentProvider, workshopsStore, coursesStore)
		paymentsHandler = payments.NewHandler(paymentsService, domainResolver, authService)
		workshopsHandler.SetPaymentService(paymentsService)

		blogStore := blogdb.New(pool)
		blogService := blog.NewService(blogStore)
		blogHandler = blog.NewHandler(blogService, domainResolver, authService)

		var storageProvider media.StorageProvider = media.NewMockStorageProvider()
		if s3Endpoint := os.Getenv("S3_ENDPOINT"); s3Endpoint != "" {
			storageProvider = media.NewS3StorageProvider(media.S3Config{
				Endpoint:        s3Endpoint,
				Region:          os.Getenv("S3_REGION"),
				Bucket:          os.Getenv("S3_BUCKET"),
				AccessKeyID:     os.Getenv("S3_ACCESS_KEY"),
				SecretAccessKey: os.Getenv("S3_SECRET_KEY"),
			})
		}
		mediaStore := mediadb.New(pool)
		mediaService := media.NewService(mediaStore, storageProvider)
		mediaHandler = media.NewHandler(mediaService, domainResolver, authService)

		bookingStore := bookingdb.New(pool)
		bookingService := booking.NewService(bookingStore)
		bookingHandler = booking.NewHandler(bookingService, domainResolver, authService)

		otpSender := courses.NewMockOTPSender()
		buyerSvc := courses.NewBuyerService(coursesStore, otpSender, mediaService)
		mediaValidator := &mediaAssetValidatorAdapter{store: mediaStore}
		adminSvc := courses.NewAdminCourseService(coursesStore, mediaValidator)
		coursesHandler = courses.NewHandler(buyerSvc, adminSvc, domainResolver, authService, paymentsService)

		reportsStore := reportsdb.New(pool)
		reportsService := reports.NewService(reportsStore)
		reportsHandler = reports.NewHandler(reportsService, domainResolver, authService)

		// Start background Outbox cache invalidation worker (AP-11 / PSY-5.3 / PSY-5.14)
		var workerDB cache.OutboxDBTX = pool
		if workerURL := os.Getenv("WORKER_DATABASE_URL"); workerURL != "" {
			if wPool, err := pgxpool.New(ctx, workerURL); err == nil {
				workerDB = wPool
				defer wPool.Close()
			} else {
				logger.Warn("failed to connect to worker database, falling back to app pool", "error", err)
			}
		}
		outboxPurger := cache.NewArvanCloudPurger(os.Getenv("ARVANCLOUD_API_KEY"), nil)
		outboxWorker := cache.NewOutboxWorker(workerDB, outboxPurger)
		go func() {
			logger.Info("starting background outbox cache invalidation worker")
			outboxWorker.Run(ctx, 3*time.Second)
		}()

		// Start background Data Retention cleaner (90-day anonymization & OTP/session cleanup)
		retentionCleaner := retention.NewCleaner(pool)
		go func() {
			logger.Info("starting background data retention cleaner")
			retentionCleaner.RunPeriodic(ctx, 1*time.Hour, retention.DefaultRetentionDays)
		}()
	}

	router := setupRouter(
		authHandler,
		bookingHandler,
		coursesHandler,
		reportsHandler,
		settingsHandler,
		workshopsHandler,
		paymentsHandler,
		blogHandler,
		mediaHandler,
	)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting psychology API server with unified /v1 routing and standard timeouts", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		logger.Error("server failed to start", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		logger.Info("shutting down psychology API server gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server forced to shutdown", "error", err)
			os.Exit(1)
		}
		logger.Info("psychology API server shutdown cleanly")
	}
}
