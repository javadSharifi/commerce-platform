package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"psychology/api/internal/booking/bookingdb"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"
)

func checkPublicBooking(t *testing.T, store *db.Store, tenantA, tenantB uuid.UUID, hostA, hostB string, serve func(http.ResponseWriter, *http.Request)) {
	t.Helper()
	ctx := context.Background()
	seed := func(tenant uuid.UUID, name string) (therapist, slot uuid.UUID) {
		t.Helper()
		err := store.WithTenantTx(tenantctx.With(ctx, tenant), func(tx pgx.Tx, id uuid.UUID) error {
			q := bookingdb.New(tx)
			th, err := q.CreateTherapist(ctx, bookingdb.CreateTherapistParams{TenantID: toPgUUID(id), Name: name, Slug: name, SessionDurationMinutes: 45, IsActive: true})
			if err != nil {
				return err
			}
			start := time.Now().UTC().Add(48 * time.Hour)
			sl, err := q.CreateAppointmentSlot(ctx, bookingdb.CreateAppointmentSlotParams{TenantID: toPgUUID(id), TherapistID: th.ID, StartsAt: pgtype.Timestamptz{Time: start, Valid: true}, EndsAt: pgtype.Timestamptz{Time: start.Add(45 * time.Minute), Valid: true}, Status: "available"})
			therapist, slot = uuid.UUID(th.ID.Bytes), uuid.UUID(sl.ID.Bytes)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	th, sl := seed(tenantA, "booking-alpha")
	_, foreign := seed(tenantB, "booking-beta")
	request := func(method, host, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = host
		req.Header.Set("X-Forwarded-Host", hostA)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("X-Guest-Lookup-Hash", token)
		}
		rec := httptest.NewRecorder()
		serve(rec, req)
		return rec
	}
	t.Run("public booking transaction and two host isolation", func(t *testing.T) {
		directory := request("GET", hostA, "/v1/public/therapists", "", "")
		if directory.Code != 200 || !strings.Contains(directory.Body.String(), "booking-alpha") || strings.Contains(directory.Body.String(), "booking-beta") {
			t.Fatalf("directory: %d %s", directory.Code, directory.Body.String())
		}
		route := "/v1/public/therapists/" + th.String() + "/slots"
		if res := request("GET", hostB, route, "", ""); res.Code != 404 {
			t.Errorf("foreign calendar: %d", res.Code)
		}
		if res := request("GET", hostA, route+"?from_date=2026-02-30", "", ""); res.Code != 400 {
			t.Errorf("invalid date: %d", res.Code)
		}
		if res := request("GET", hostA, route, "", ""); res.Code != 200 || !strings.Contains(res.Body.String(), sl.String()) {
			t.Fatalf("own slot: %d %s", res.Code, res.Body.String())
		}
		endpoint := "/v1/public/appointments"
		body := func(id uuid.UUID) string {
			return `{"therapist_id":"` + th.String() + `","slot_id":"` + id.String() + `","guest_name":"Test Guest","guest_phone":"09121234567","notes":"private test marker"}`
		}
		if res := request("POST", hostA, endpoint, body(foreign), ""); res.Code != 404 {
			t.Errorf("foreign slot claim: %d", res.Code)
		}
		var calls [2]*httptest.ResponseRecorder
		var wg sync.WaitGroup
		for i := range calls {
			wg.Add(1)
			go func() { defer wg.Done(); calls[i] = request("POST", hostA, endpoint, body(sl), "") }()
		}
		wg.Wait()
		if calls[0].Code != 201 {
			calls[0], calls[1] = calls[1], calls[0]
		}
		if calls[0].Code != 201 || calls[1].Code != 409 {
			t.Fatalf("expected one reservation and one conflict: %d %d", calls[0].Code, calls[1].Code)
		}
		var appt struct {
			ID    string
			Token string `json:"guest_lookup_hash"`
		}
		if err := json.Unmarshal(calls[0].Body.Bytes(), &appt); err != nil || appt.ID == "" || appt.Token == "" {
			t.Fatal("invalid appointment response")
		}
		cookies := calls[0].Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Path != "/appointments/"+appt.ID {
			t.Fatal("missing scoped tracking cookie")
		}
		statusPath := endpoint + "/" + appt.ID
		for _, tc := range []struct {
			host, token string
			status      int
		}{{hostA, "", 404}, {hostA, "wrong", 404}, {hostB, appt.Token, 404}, {hostA, appt.Token, 200}} {
			res := request("GET", tc.host, statusPath, "", tc.token)
			if res.Code != tc.status || !strings.Contains(res.Header().Get("Cache-Control"), "no-store") {
				t.Errorf("tracking status: %d expected %d", res.Code, tc.status)
			}
		}
		if res := request("POST", hostA, statusPath+"/cancel", "{}", appt.Token); res.Code != 200 || !strings.Contains(res.Body.String(), "cancelled") {
			t.Errorf("cancel: %d", res.Code)
		}
		if err := store.WithTenantTx(tenantctx.With(ctx, tenantA), func(tx pgx.Tx, id uuid.UUID) error {
			var notes, status string
			var count int
			err := tx.QueryRow(ctx, `SELECT a.notes,s.status,(SELECT count(*) FROM appointment_status_history h WHERE h.tenant_id=a.tenant_id AND h.appointment_id=a.id)
       FROM appointments a JOIN appointment_slots s ON s.id=a.slot_id AND s.tenant_id=a.tenant_id WHERE a.tenant_id=$1 AND a.id=$2`, id, appt.ID).Scan(&notes, &status, &count)
			if err == nil && (notes != "" || status != "available" || count != 2) {
				t.Errorf("expected no notes, available slot and two history entries: %q %d", status, count)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	})
}
