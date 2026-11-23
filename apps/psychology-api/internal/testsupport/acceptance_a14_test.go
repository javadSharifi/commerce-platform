package testsupport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/settings"
	"psychology/api/internal/settings/settingsdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TestAcceptance_PSY_A14_PublicSiteRedesign_TwoHosts verifies acceptance scenario PSY-A14 in Go:
// 1. Two separate verified clinics/hosts: Host A (sharifi-clinic.ir) and Host B (moradi-center.ir).
// 2. Host A publishes the new "editorial" template with "calm-sage" palette.
// 3. Host B publishes "clinical" template with "warm-sand" palette.
// 4. Verification of Go validation:
//    - "editorial" template is accepted with version locked to "1.0.0".
//    - Uncatalogued templates are rejected with 400 Bad Request.
// 5. Strict Multi-Tenant Isolation:
//    - Host A's appearance reflects editorial + calm-sage.
//    - Host B's appearance remains clinical + warm-sand.
//    - Zero cross-tenant contamination between Host A and Host B.
func TestAcceptance_PSY_A14_PublicSiteRedesign_TwoHosts(t *testing.T) {
	hostA := "sharifi-clinic.ir"
	hostB := "moradi-center.ir"

	tenantAID := uuid.MustParse("11111111-aaaa-1111-aaaa-111111111111")
	tenantBID := uuid.MustParse("22222222-bbbb-2222-bbbb-222222222222")

	ownerAID := uuid.MustParse("33333333-aaaa-3333-aaaa-333333333333")
	ownerBID := uuid.MustParse("55555555-bbbb-5555-bbbb-555555555555")

	mockStore := newFullMockStore()

	// Seed Tenant A
	mockStore.clinics[tenantAID.String()] = "کلینیک تخصصی روان‌شناسی شریفی"
	mockStore.domains[hostA] = domaindb.Domain{
		Host:       hostA,
		TenantID:   toPgUUID(tenantAID),
		IsVerified: true,
	}
	mockStore.settings[tenantAID.String()] = settingsdb.SiteSetting{
		TenantID:        toPgUUID(tenantAID),
		Title:           "کلینیک تخصصی روان‌شناسی شریفی",
		Description:     "ارائه خدمات روان‌درمانی فردی، زوج‌درمانی و کارگاه‌های تخصصی",
		Phone:           "۰۲۱-۲۲۸۴۹۱۰۳",
		Address:         "تهران، خیابان شریعتی، بالاتر از ظفر",
		PrimaryColor:    "#2D4F43",
		TemplateID:      "minimal",
		TemplateVersion: "1.0.0",
		ThemeRadius:     "md",
		ThemeScale:      "md",
		PaletteID:       "calm-sage",
	}

	hashPass, _ := auth.HashPassword("secret123456")
	mockStore.identities[ownerAID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerAID),
		Email:        "owner@sharifi-clinic.ir",
		PasswordHash: hashPass,
		FullName:     "دکتر محمد جواد شریفی",
		Status:       "active",
	}
	mockStore.memberships[ownerAID.String()+":"+tenantAID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantAID),
		UserID:   toPgUUID(ownerAID),
		Role:     "owner",
	}

	// Seed Tenant B
	mockStore.clinics[tenantBID.String()] = "مرکز مشاوره و روان‌شناسی مرادی"
	mockStore.domains[hostB] = domaindb.Domain{
		Host:       hostB,
		TenantID:   toPgUUID(tenantBID),
		IsVerified: true,
	}
	mockStore.settings[tenantBID.String()] = settingsdb.SiteSetting{
		TenantID:        toPgUUID(tenantBID),
		Title:           "مرکز مشاوره و روان‌شناسی مرادی",
		Description:     "مرکز جامع سلامت روان و خدمات روان‌شناختی",
		Phone:           "۰۲۱-۸۸۸۸۴۴۴۴",
		Address:         "تهران، سعادت‌آباد",
		PrimaryColor:    "#0284c7",
		TemplateID:      "clinical",
		TemplateVersion: "1.0.0",
		ThemeRadius:     "lg",
		ThemeScale:      "sm",
		PaletteID:       "warm-sand",
	}

	mockStore.identities[ownerBID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerBID),
		Email:        "owner@moradi-center.ir",
		PasswordHash: hashPass,
		FullName:     "دکتر رضا مرادی",
		Status:       "active",
	}
	mockStore.memberships[ownerBID.String()+":"+tenantBID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantBID),
		UserID:   toPgUUID(ownerBID),
		Role:     "owner",
	}

	domainResolver := domain.NewResolver(mockStore)
	authService := auth.NewService(mockStore)
	settingsService := settings.NewService(mockStore)
	authHandler := auth.NewHandler(authService)
	settingsHandler := settings.NewHandler(settingsService, domainResolver, authService)

	// Designate ownerAID as appearance owner for A, ownerBID for B
	settingsHandler.SetAppearanceOwnerUserID(ownerAID)

	apiRouter := chi.NewRouter()
	apiRouter.Route("/v1", func(r chi.Router) {
		authHandler.MountRoutes(r)
		settingsHandler.MountRoutes(r)
	})

	// Login sessions
	sessOwnerA, err := authService.Login(context.Background(), "owner@sharifi-clinic.ir", "secret123456")
	if err != nil {
		t.Fatalf("failed to login owner A: %v", err)
	}

	// 1. Host A publishes new "editorial" template with "calm-sage"
	bodyPutA := map[string]interface{}{
		"template_id": "editorial",
		"palette_id":  "calm-sage",
	}
	bPutA, _ := json.Marshal(bodyPutA)
	reqPutA := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewReader(bPutA))
	reqPutA.Host = hostA
	reqPutA.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
	reqPutA.Header.Set("X-Tenant-ID", tenantAID.String())
	reqPutA.Header.Set("Content-Type", "application/json")
	recPutA := httptest.NewRecorder()
	apiRouter.ServeHTTP(recPutA, reqPutA)
	if recPutA.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when publishing editorial template on Tenant A, got %d: %s", recPutA.Code, recPutA.Body.String())
	}

	var respA map[string]interface{}
	if err := json.Unmarshal(recPutA.Body.Bytes(), &respA); err != nil {
		t.Fatalf("failed to decode appearance response: %v", err)
	}
	if respA["template_id"] != "editorial" {
		t.Fatalf("expected template_id 'editorial', got %v", respA["template_id"])
	}
	if respA["template_version"] != "1.0.0" {
		t.Fatalf("expected template_version '1.0.0', got %v", respA["template_version"])
	}
	if respA["palette_id"] != "calm-sage" {
		t.Fatalf("expected palette_id 'calm-sage', got %v", respA["palette_id"])
	}

	// 2. Verify Host A Public Site Profile reflects editorial + calm-sage
	reqPubA := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqPubA.Host = hostA
	recPubA := httptest.NewRecorder()
	apiRouter.ServeHTTP(recPubA, reqPubA)
	if recPubA.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Host A public site, got %d", recPubA.Code)
	}
	var pubA map[string]interface{}
	if err := json.Unmarshal(recPubA.Body.Bytes(), &pubA); err != nil {
		t.Fatalf("failed to parse Host A public site JSON: %v", err)
	}
	if pubA["template_id"] != "editorial" {
		t.Fatalf("expected Host A template_id 'editorial', got %v", pubA["template_id"])
	}
	if pubA["palette_id"] != "calm-sage" {
		t.Fatalf("expected Host A palette_id 'calm-sage', got %v", pubA["palette_id"])
	}
	if pubA["name"] != "کلینیک تخصصی روان‌شناسی شریفی" {
		t.Fatalf("expected Host A name 'کلینیک تخصصی روان‌شناسی شریفی', got %v", pubA["name"])
	}

	// 3. Verify Host B Public Site Profile remains untouched (clinical + warm-sand)
	reqPubB := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqPubB.Host = hostB
	recPubB := httptest.NewRecorder()
	apiRouter.ServeHTTP(recPubB, reqPubB)
	if recPubB.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Host B public site, got %d", recPubB.Code)
	}
	var pubB map[string]interface{}
	if err := json.Unmarshal(recPubB.Body.Bytes(), &pubB); err != nil {
		t.Fatalf("failed to parse Host B public site JSON: %v", err)
	}
	if pubB["template_id"] != "clinical" {
		t.Fatalf("expected Host B template_id 'clinical', got %v", pubB["template_id"])
	}
	if pubB["palette_id"] != "warm-sand" {
		t.Fatalf("expected Host B palette_id 'warm-sand', got %v", pubB["palette_id"])
	}
	if pubB["name"] != "مرکز مشاوره و روان‌شناسی مرادی" {
		t.Fatalf("expected Host B name 'مرکز مشاوره و روان‌شناسی مرادی', got %v", pubB["name"])
	}

	// 4. Verify rejection of uncatalogued template on Tenant A
	bodyInvalid := map[string]interface{}{
		"template_id": "nonexistent-template-999",
		"palette_id":  "calm-sage",
	}
	bInv, _ := json.Marshal(bodyInvalid)
	reqInv := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewReader(bInv))
	reqInv.Host = hostA
	reqInv.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
	reqInv.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInv.Header.Set("Content-Type", "application/json")
	recInv := httptest.NewRecorder()
	apiRouter.ServeHTTP(recInv, reqInv)
	if recInv.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for uncatalogued template, got %d", recInv.Code)
	}

	// 5. Verify internal settings isolation in store
	settingA := mockStore.settings[tenantAID.String()]
	if settingA.TemplateID != "editorial" || settingA.PaletteID != "calm-sage" {
		t.Fatalf("expected store setting A to have editorial/calm-sage, got %s/%s", settingA.TemplateID, settingA.PaletteID)
	}
	settingB := mockStore.settings[tenantBID.String()]
	if settingB.TemplateID != "clinical" || settingB.PaletteID != "warm-sand" {
		t.Fatalf("expected store setting B to remain clinical/warm-sand, got %s/%s", settingB.TemplateID, settingB.PaletteID)
	}
}
