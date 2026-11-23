package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

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
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}

func (m *mockAuthQuerier) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	var u uuid.UUID
	copy(u[:], id.Bytes[:])
	identity, ok := m.identities[u.String()]
	if !ok {
		return authdb.PanelIdentity{}, pgx.ErrNoRows
	}
	return identity, nil
}

func (m *mockAuthQuerier) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
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
	s, ok := m.sessions[tokenHash]
	if !ok {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	if s.ExpiresAt.Time.Before(time.Now()) {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return s, nil
}

func (m *mockAuthQuerier) DeletePanelSession(ctx context.Context, tokenHash string) error {
	delete(m.sessions, tokenHash)
	return nil
}

func (m *mockAuthQuerier) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	var u uuid.UUID
	copy(u[:], userID.Bytes[:])
	var result []authdb.ListMembershipsByUserIDRow
	for _, mem := range m.memberships {
		var memUser uuid.UUID
		copy(memUser[:], mem.UserID.Bytes[:])
		if memUser == u {
			result = append(result, authdb.ListMembershipsByUserIDRow{
				ID:         mem.ID,
				TenantID:   mem.TenantID,
				UserID:     mem.UserID,
				Role:       mem.Role,
				TenantName: "Clinic Test",
				TenantSlug: "clinic-test",
			})
		}
	}
	return result, nil
}

func (m *mockAuthQuerier) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	var u, t uuid.UUID
	copy(u[:], arg.UserID.Bytes[:])
	copy(t[:], arg.TenantID.Bytes[:])
	key := u.String() + ":" + t.String()
	mem, ok := m.memberships[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return mem, nil
}

func (m *mockAuthQuerier) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	return authdb.Membership{}, nil
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func TestPasswordHashing(t *testing.T) {
	password := "SecretClinicPass123"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	valid, err := auth.CheckPassword(password, hash)
	if err != nil || !valid {
		t.Fatalf("expected password to be valid, got valid=%v, err=%v", valid, err)
	}

	invalid, err := auth.CheckPassword("WrongPassword", hash)
	if err != nil || invalid {
		t.Fatalf("expected wrong password to fail, got valid=%v, err=%v", invalid, err)
	}
}

func TestIndependentSessions(t *testing.T) {
	mock := newMockAuthQuerier()
	service := auth.NewService(mock)

	user1ID := uuid.New()
	hash1, _ := auth.HashPassword("pass1")
	mock.identities[user1ID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(user1ID),
		Email:        "user1@clinic.ir",
		PasswordHash: hash1,
		FullName:     "Doctor One",
		Status:       "active",
	}

	user2ID := uuid.New()
	hash2, _ := auth.HashPassword("pass2")
	mock.identities[user2ID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(user2ID),
		Email:        "user2@clinic.ir",
		PasswordHash: hash2,
		FullName:     "Doctor Two",
		Status:       "active",
	}

	ctx := context.Background()

	// 1. Login user 1
	sess1, err := service.Login(ctx, "user1@clinic.ir", "pass1")
	if err != nil {
		t.Fatalf("login user 1 failed: %v", err)
	}

	// 2. Login user 2
	sess2, err := service.Login(ctx, "user2@clinic.ir", "pass2")
	if err != nil {
		t.Fatalf("login user 2 failed: %v", err)
	}

	if sess1.Token == sess2.Token {
		t.Fatal("sessions tokens must be distinct")
	}

	// 3. Validate sessions independently
	u1, err := service.ValidateSession(ctx, sess1.Token)
	if err != nil || u1.ID != user1ID {
		t.Fatalf("expected user1, got %v", u1)
	}

	u2, err := service.ValidateSession(ctx, sess2.Token)
	if err != nil || u2.ID != user2ID {
		t.Fatalf("expected user2, got %v", u2)
	}

	// 4. Logout user 1 -> user 2 must remain valid
	err = service.Logout(ctx, sess1.Token)
	if err != nil {
		t.Fatalf("logout user 1 failed: %v", err)
	}

	_, err = service.ValidateSession(ctx, sess1.Token)
	if err != auth.ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired for user 1, got %v", err)
	}

	u2StillValid, err := service.ValidateSession(ctx, sess2.Token)
	if err != nil || u2StillValid.ID != user2ID {
		t.Fatalf("user 2 session should still be valid after user 1 logout, got %v", err)
	}
}

func TestCrossTenantIsolationAndRoleEnforcement(t *testing.T) {
	mock := newMockAuthQuerier()
	service := auth.NewService(mock)

	userAID := uuid.New()
	hashA, _ := auth.HashPassword("passwordA")
	mock.identities[userAID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(userAID),
		Email:        "userA@clinic.ir",
		PasswordHash: hashA,
		FullName:     "Doctor A",
		Status:       "active",
	}

	tenant1ID := uuid.New()
	tenant2ID := uuid.New()

	// User A is receptionist in Tenant 1 only
	mock.memberships[userAID.String()+":"+tenant1ID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenant1ID),
		UserID:   toPgUUID(userAID),
		Role:     "receptionist",
	}

	sessA, err := service.Login(context.Background(), "userA@clinic.ir", "passwordA")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Setup protected router
	r := chi.NewRouter()
	r.Use(auth.Authenticate(service))
	r.Use(auth.RequireTenantMembership(service))

	r.Get("/admin/receptionist-only", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	})

	r.With(auth.RequireRole("owner")).Get("/admin/owner-only", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	})

	// Scenario 1: User A attempts to access Tenant 2 (Unauthorized cross-tenant attempt)
	req1 := httptest.NewRequest(http.MethodGet, "/admin/receptionist-only", nil)
	req1.Header.Set("Authorization", "Bearer "+sessA.Token)
	req1.Header.Set("X-Tenant-ID", tenant2ID.String())
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-tenant access to Tenant 2, got %d", rr1.Code)
	}

	// Scenario 2: User A accesses Tenant 1 with appropriate role -> 200 OK
	req2 := httptest.NewRequest(http.MethodGet, "/admin/receptionist-only", nil)
	req2.Header.Set("Authorization", "Bearer "+sessA.Token)
	req2.Header.Set("X-Tenant-ID", tenant1ID.String())
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Tenant 1 receptionist access, got %d: %s", rr2.Code, rr2.Body.String())
	}

	// Scenario 3: User A attempts owner-only action in Tenant 1 (Role restriction) -> 403
	req3 := httptest.NewRequest(http.MethodGet, "/admin/owner-only", nil)
	req3.Header.Set("Authorization", "Bearer "+sessA.Token)
	req3.Header.Set("X-Tenant-ID", tenant1ID.String())
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for owner-only endpoint, got %d", rr3.Code)
	}
}

func TestCSRFProtection(t *testing.T) {
	r := chi.NewRouter()
	r.Use(auth.CSRFProtection())
	r.Post("/admin/mutate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})

	// 1. Cross-site attempt -> Blocked
	reqCross := httptest.NewRequest(http.MethodPost, "/admin/mutate", bytes.NewBufferString(`{}`))
	reqCross.Header.Set("Sec-Fetch-Site", "cross-site")
	reqCross.Header.Set("Content-Type", "application/json")
	rrCross := httptest.NewRecorder()
	r.ServeHTTP(rrCross, reqCross)
	if rrCross.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-site mutating request, got %d", rrCross.Code)
	}

	// 2. Non-JSON attempt -> Blocked
	reqNonJSON := httptest.NewRequest(http.MethodPost, "/admin/mutate", bytes.NewBufferString(`foo=bar`))
	reqNonJSON.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrNonJSON := httptest.NewRecorder()
	r.ServeHTTP(rrNonJSON, reqNonJSON)
	if rrNonJSON.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 for non-JSON mutating request, got %d", rrNonJSON.Code)
	}

	// 3. Valid JSON request -> Allowed
	reqValid := httptest.NewRequest(http.MethodPost, "/admin/mutate", bytes.NewBufferString(`{"key":"value"}`))
	reqValid.Header.Set("Content-Type", "application/json")
	rrValid := httptest.NewRecorder()
	r.ServeHTTP(rrValid, reqValid)
	if rrValid.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid JSON request, got %d", rrValid.Code)
	}
}

func TestAuthHTTPHandlers(t *testing.T) {
	mock := newMockAuthQuerier()
	service := auth.NewService(mock)
	handler := auth.NewHandler(service)

	userAID := uuid.New()
	hashA, _ := auth.HashPassword("securepass")
	mock.identities[userAID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(userAID),
		Email:        "admin@clinic.ir",
		PasswordHash: hashA,
		FullName:     "Doctor Admin",
		Status:       "active",
	}

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// 1. Login with wrong password -> 401
	bodyWrong, _ := json.Marshal(map[string]string{
		"email":    "admin@clinic.ir",
		"password": "wrong",
	})
	reqWrong := httptest.NewRequest(http.MethodPost, "/admin/auth/login", bytes.NewBuffer(bodyWrong))
	reqWrong.Header.Set("Content-Type", "application/json")
	rrWrong := httptest.NewRecorder()
	r.ServeHTTP(rrWrong, reqWrong)
	if rrWrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", rrWrong.Code)
	}

	// 2. Login with correct password -> 200 + session cookie
	bodyOk, _ := json.Marshal(map[string]string{
		"email":    "admin@clinic.ir",
		"password": "securepass",
	})
	reqOk := httptest.NewRequest(http.MethodPost, "/admin/auth/login", bytes.NewBuffer(bodyOk))
	reqOk.Header.Set("Content-Type", "application/json")
	rrOk := httptest.NewRecorder()
	r.ServeHTTP(rrOk, reqOk)
	if rrOk.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct login, got %d: %s", rrOk.Code, rrOk.Body.String())
	}

	cookies := rrOk.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("expected session_token cookie to be set")
	}

	// 3. Call /admin/auth/me with cookie -> 200
	reqMe := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
	reqMe.AddCookie(sessionCookie)
	rrMe := httptest.NewRecorder()
	r.ServeHTTP(rrMe, reqMe)
	if rrMe.Code != http.StatusOK {
		t.Fatalf("expected 200 for /admin/auth/me, got %d", rrMe.Code)
	}

	// 4. Logout -> 200 + cleared cookie
	reqLogout := httptest.NewRequest(http.MethodPost, "/admin/auth/logout", nil)
	reqLogout.AddCookie(sessionCookie)
	rrLogout := httptest.NewRecorder()
	r.ServeHTTP(rrLogout, reqLogout)
	if rrLogout.Code != http.StatusOK {
		t.Fatalf("expected 200 for logout, got %d", rrLogout.Code)
	}
}

func TestAuthCookie_SecureFlag(t *testing.T) {
	q := newMockAuthQuerier()
	pass, _ := auth.HashPassword("SecretPassword123!")
	q.identities["doc@test.ir"] = authdb.PanelIdentity{
		ID:           toPgUUID(uuid.New()),
		Email:        "doc@test.ir",
		PasswordHash: pass,
		Status:       "active",
	}

	// 1. Non-production / dev -> Secure: false
	hDev := auth.NewHandler(auth.NewService(q))
	hDev.SetCookieSecure(false)
	rDev := chi.NewRouter()
	hDev.MountRoutes(rDev)

	reqDev := httptest.NewRequest(http.MethodPost, "/admin/auth/login", strings.NewReader(`{"email":"doc@test.ir","password":"SecretPassword123!"}`))
	reqDev.Header.Set("Content-Type", "application/json")
	rrDev := httptest.NewRecorder()
	rDev.ServeHTTP(rrDev, reqDev)

	var devCookie *http.Cookie
	for _, c := range rrDev.Result().Cookies() {
		if c.Name == "session_token" {
			devCookie = c
			break
		}
	}
	if devCookie == nil {
		t.Fatal("expected dev cookie to be set")
	}
	if devCookie.Secure {
		t.Errorf("expected dev cookie to have Secure=false, got true")
	}

	// 2. Production -> Secure: true
	hProd := auth.NewHandler(auth.NewService(q))
	hProd.SetCookieSecure(true)
	rProd := chi.NewRouter()
	hProd.MountRoutes(rProd)

	reqProd := httptest.NewRequest(http.MethodPost, "/admin/auth/login", strings.NewReader(`{"email":"doc@test.ir","password":"SecretPassword123!"}`))
	reqProd.Header.Set("Content-Type", "application/json")
	rrProd := httptest.NewRecorder()
	rProd.ServeHTTP(rrProd, reqProd)

	var prodCookie *http.Cookie
	for _, c := range rrProd.Result().Cookies() {
		if c.Name == "session_token" {
			prodCookie = c
			break
		}
	}
	if prodCookie == nil {
		t.Fatal("expected prod cookie to be set")
	}
	if !prodCookie.Secure {
		t.Errorf("expected prod cookie to have Secure=true, got false")
	}
}
