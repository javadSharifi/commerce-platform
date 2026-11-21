package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"psychology/api/internal/auth"
	"psychology/api/internal/booking"
	"psychology/api/internal/courses"
	"psychology/api/internal/reports"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func setupRouter(authHandler *auth.Handler, bookingHandler *booking.Handler, coursesHandler *courses.Handler, reportsHandler *reports.Handler) http.Handler {
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
	})

	if coursesHandler != nil {
		coursesHandler.MountRoutes(r)
	}

	if reportsHandler != nil {
		reportsHandler.MountRoutes(r)
	}

	return r
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := setupRouter(nil, nil, nil, nil)

	logger.Info("starting psychology API server", "port", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
