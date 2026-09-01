package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adminservice "flashcards/internal/admin"
	"flashcards/internal/auth"
	"flashcards/internal/config"
	"flashcards/internal/progress"

	"golang.org/x/crypto/bcrypt"
)

func TestAdminHandlerRefusesUnauthenticatedAndKidRequests(t *testing.T) {
	handler := NewAdminHandler(adminSessions())
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/admin", nil),
		httptest.NewRequest(http.MethodGet, "/admin", nil),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", response.Code)
		}
	}
}

func TestAdminHandlerCreatesSessionForValidLogin(t *testing.T) {
	handler := NewAdminHandler(adminSessions())
	form := url.Values{"username": {"sander"}, "password": {"secret"}}
	loginRequest := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303", loginResponse.Code)
	}
	cookie := loginResponse.Result().Cookies()[0]
	adminRequest := httptest.NewRequest(http.MethodGet, "/admin", nil)
	adminRequest.AddCookie(cookie)
	adminResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin status = %d, want 200", adminResponse.Code)
	}
}

func TestAdminHandlerRejectsInvalidLogin(t *testing.T) {
	handler := NewAdminHandler(adminSessions())
	request := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader("username=sander&password=wrong"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("login status = %d, want 403", response.Code)
	}
}

func TestAdminManagementHandlersManageKidsWithCSRF(t *testing.T) {
	store := mustAuthStore(t)
	authService := auth.NewService(store, nil)
	pending, err := authService.Signup("pending", "112233")
	if err != nil {
		t.Fatalf("signup pending kid: %v", err)
	}
	handler := NewAdminManagementHandler(AdminDependencies{Sessions: adminSessions(), AuthStore: store, AuthService: authService})
	cookie, csrfToken := adminLogin(t, handler)

	listRequest := httptest.NewRequest(http.MethodGet, "/admin/kids", nil)
	listRequest.AddCookie(cookie)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	var kids []kidView
	if err := json.NewDecoder(listResponse.Body).Decode(&kids); err != nil || len(kids) != 1 || kids[0].Username != "pending" {
		t.Fatalf("kid list = %#v, err = %v", kids, err)
	}

	approveRequest := csrfRequest(http.MethodPost, "/admin/kids/"+pending.ID+"/approve", csrfToken, cookie, nil)
	approveResponse := httptest.NewRecorder()
	handler.ServeHTTP(approveResponse, approveRequest)
	if approveResponse.Code != http.StatusNoContent {
		t.Fatalf("approve status = %d", approveResponse.Code)
	}
	if err := authService.Disable(pending.ID); err != nil {
		t.Fatalf("disable directly: %v", err)
	}
	enableRequest := csrfRequest(http.MethodPost, "/admin/kids/"+pending.ID+"/enable", csrfToken, cookie, nil)
	enableResponse := httptest.NewRecorder()
	handler.ServeHTTP(enableResponse, enableRequest)
	if enableResponse.Code != http.StatusNoContent {
		t.Fatalf("enable status = %d", enableResponse.Code)
	}

	createForm := url.Values{"username": {"new-kid"}, "pin": {"445566"}}
	createRequest := csrfRequest(http.MethodPost, "/admin/kids", csrfToken, cookie, createForm)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create status = %d", createResponse.Code)
	}
}

func TestAdminManagementHandlerRejectsPendingKidAndMissingCSRF(t *testing.T) {
	store := mustAuthStore(t)
	authService := auth.NewService(store, nil)
	pending, err := authService.Signup("pending", "112233")
	if err != nil {
		t.Fatalf("signup pending kid: %v", err)
	}
	handler := NewAdminManagementHandler(AdminDependencies{Sessions: adminSessions(), AuthStore: store, AuthService: authService})
	cookie, csrfToken := adminLogin(t, handler)
	missingToken := httptest.NewRequest(http.MethodPost, "/admin/kids/"+pending.ID+"/reject", nil)
	missingToken.AddCookie(cookie)
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missingToken)
	if missingResponse.Code != http.StatusForbidden {
		t.Fatalf("missing token status = %d", missingResponse.Code)
	}
	rejectRequest := csrfRequest(http.MethodPost, "/admin/kids/"+pending.ID+"/reject", csrfToken, cookie, nil)
	rejectResponse := httptest.NewRecorder()
	handler.ServeHTTP(rejectResponse, rejectRequest)
	if rejectResponse.Code != http.StatusNoContent {
		t.Fatalf("reject status = %d", rejectResponse.Code)
	}
	if _, ok := store.Get(pending.ID); ok {
		t.Fatal("rejected pending kid must be deleted")
	}
}

func TestAdminManagementHandlerExposesProgressAndSettings(t *testing.T) {
	dependencies := adminDependencies(t)
	if err := dependencies.AuthStore.Add(auth.Kid{ID: "kid-1", Username: "mia", Status: auth.StatusApproved}); err != nil {
		t.Fatalf("add kid: %v", err)
	}
	progressStore := progress.NewStore(filepath.Join(t.TempDir(), "progress"))
	if err := progressStore.Save("kid-1", progress.KidProgress{Sessions: []progress.SessionSummary{{Subject: "Frans", CardsSeen: 5, CorrectFirstTry: 4, EndedAt: time.Now().UTC()}}}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	dependencies.ProgressDashboard = adminservice.NewProgressDashboardService(progressStore)
	handler := NewAdminManagementHandler(dependencies)
	cookie, csrfToken := adminLogin(t, handler)

	progressRequest := httptest.NewRequest(http.MethodGet, "/admin/progress", nil)
	progressRequest.AddCookie(cookie)
	progressResponse := httptest.NewRecorder()
	handler.ServeHTTP(progressResponse, progressRequest)
	if progressResponse.Code != http.StatusOK || !strings.Contains(progressResponse.Body.String(), "mia") {
		t.Fatalf("progress response = %d: %s", progressResponse.Code, progressResponse.Body.String())
	}

	settingsForm := url.Values{"easy": {"75"}, "ok": {"30"}, "hard": {"5"}}
	settingsRequest := csrfRequest(http.MethodPost, "/admin/settings", csrfToken, cookie, settingsForm)
	settingsResponse := httptest.NewRecorder()
	handler.ServeHTTP(settingsResponse, settingsRequest)
	if settingsResponse.Code != http.StatusNoContent {
		t.Fatalf("settings status = %d", settingsResponse.Code)
	}
	settings, err := dependencies.SettingsStore.Load()
	if err != nil || settings.Easy != 75 || settings.OK != 30 || settings.Hard != 5 {
		t.Fatalf("settings = %#v, err = %v", settings, err)
	}
}

func TestAdminViewsRenderLoginAndDashboardData(t *testing.T) {
	dependencies := adminDependencies(t)
	if err := dependencies.AuthStore.Add(auth.Kid{ID: "kid-1", Username: "mia", Status: auth.StatusApproved}); err != nil {
		t.Fatalf("add kid: %v", err)
	}
	progressStore := progress.NewStore(filepath.Join(t.TempDir(), "progress"))
	if err := progressStore.Save("kid-1", progress.KidProgress{Sessions: []progress.SessionSummary{{Subject: "Frans", CardsSeen: 5, CorrectFirstTry: 4}}}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	dependencies.ProgressDashboard = adminservice.NewProgressDashboardService(progressStore)
	handler := NewAdminManagementHandler(dependencies)

	loginRequest := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK || !strings.Contains(loginResponse.Body.String(), "Aanmelden") {
		t.Fatalf("login page = %d: %s", loginResponse.Code, loginResponse.Body.String())
	}

	cookie, csrfToken := adminLogin(t, handler)
	dashboardRequest := httptest.NewRequest(http.MethodGet, "/admin", nil)
	dashboardRequest.AddCookie(cookie)
	dashboardResponse := httptest.NewRecorder()
	handler.ServeHTTP(dashboardResponse, dashboardRequest)
	body := dashboardResponse.Body.String()
	if dashboardResponse.Code != http.StatusOK || !strings.Contains(body, "Studenten") || !strings.Contains(body, "mia") || !strings.Contains(body, csrfToken) || !strings.Contains(body, "Instellingen") {
		t.Fatalf("dashboard page = %d: %s", dashboardResponse.Code, body)
	}
}

func TestAdminHTMLFormsRedirectAndRenderValidationFailures(t *testing.T) {
	dependencies := adminDependencies(t)
	dependencies.ProgressDashboard = adminservice.NewProgressDashboardService(progress.NewStore(filepath.Join(t.TempDir(), "progress")))
	handler := NewAdminManagementHandler(dependencies)

	invalidLogin := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader("username=sander&password=wrong"))
	invalidLogin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidLogin.Header.Set("Accept", "text/html")
	invalidLoginResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidLoginResponse, invalidLogin)
	if invalidLoginResponse.Code != http.StatusSeeOther || !strings.Contains(invalidLoginResponse.Header().Get("Location"), "error=") {
		t.Fatalf("invalid login response = %d: %s", invalidLoginResponse.Code, invalidLoginResponse.Header().Get("Location"))
	}

	cookie, csrfToken := adminLogin(t, handler)
	invalidCreate := csrfRequest(http.MethodPost, "/admin/kids", csrfToken, cookie, url.Values{"username": {"mia"}, "pin": {"bad"}})
	invalidCreate.Header.Set("Accept", "text/html")
	invalidCreateResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidCreateResponse, invalidCreate)
	if invalidCreateResponse.Code != http.StatusSeeOther {
		t.Fatalf("invalid create response = %d", invalidCreateResponse.Code)
	}
	dashboardRequest := httptest.NewRequest(http.MethodGet, invalidCreateResponse.Header().Get("Location"), nil)
	dashboardRequest.AddCookie(cookie)
	dashboardResponse := httptest.NewRecorder()
	handler.ServeHTTP(dashboardResponse, dashboardRequest)
	if !strings.Contains(dashboardResponse.Body.String(), "Student aanmaken mislukt") {
		t.Fatalf("dashboard error response = %s", dashboardResponse.Body.String())
	}
}

func adminSessions() *adminservice.SessionManager {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return adminservice.NewSessionManager(config.Config{ListenAddr: ":8080", DataDir: "data", AdminUsername: "sander", AdminPasswordHash: string(hash), SessionSecret: "session-secret"})
}

func adminLogin(t *testing.T, handler http.Handler) (*http.Cookie, string) {
	t.Helper()
	cookie, session, ok := adminSessions().Login("sander", "secret")
	if !ok {
		t.Fatal("create admin session")
	}
	return cookie, session.CSRFToken
}

func csrfRequest(method, target, csrfToken string, cookie *http.Cookie, form url.Values) *http.Request {
	if form == nil {
		form = url.Values{}
	}
	form.Set("csrf_token", csrfToken)
	request := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	return request
}

func mustAuthStore(t *testing.T) *auth.Store {
	t.Helper()
	store, err := auth.NewStore(filepath.Join(t.TempDir(), "kids.json"))
	if err != nil {
		t.Fatalf("new auth store: %v", err)
	}
	return store
}

func adminDependencies(t *testing.T) AdminDependencies {
	t.Helper()
	settingsStore := adminservice.NewSettingsStore(filepath.Join(t.TempDir(), "settings.json"))
	return AdminDependencies{
		Sessions:        adminSessions(),
		AuthStore:       mustAuthStore(t),
		SettingsStore:   settingsStore,
		SettingsService: adminservice.NewService(settingsStore),
	}
}
