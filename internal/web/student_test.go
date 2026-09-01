package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"flashcards/internal/auth"

	"golang.org/x/crypto/bcrypt"
)

func TestStudentHandlerUsesSignedSessionIdentity(t *testing.T) {
	sessions := kidSessions(t)
	handler := NewStudentHandler(sessions)
	login := httptest.NewRequest(http.MethodPost, "/kid/login", strings.NewReader(url.Values{"username": {"mia"}, "pin": {"112233"}}.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", loginResponse.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/kid/me", nil)
	request.AddCookie(loginResponse.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "kid-1") {
		t.Fatalf("identity response = %d: %s", response.Code, response.Body.String())
	}
}

func TestStudentHandlerRejectsInvalidAndMissingSessions(t *testing.T) {
	sessions := kidSessions(t)
	handler := NewStudentHandler(sessions)
	missing := httptest.NewRequest(http.MethodGet, "/kid/me", nil)
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusForbidden {
		t.Fatalf("missing status = %d, want 403", missingResponse.Code)
	}
	cookie, _, err := sessions.Login("mia", "112233")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	tampered := httptest.NewRequest(http.MethodGet, "/kid/me", nil)
	tampered.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value + "tampered"})
	tamperedResponse := httptest.NewRecorder()
	handler.ServeHTTP(tamperedResponse, tampered)
	if tamperedResponse.Code != http.StatusForbidden {
		t.Fatalf("tampered status = %d, want 403", tamperedResponse.Code)
	}
}

func TestStudentLoginRedirectsToSeriesSelection(t *testing.T) {
	sessions := kidSessions(t)
	handler := NewStudentHandler(sessions)
	login := httptest.NewRequest(http.MethodPost, "/student/login", strings.NewReader(url.Values{"username": {"mia"}, "pin": {"112233"}}.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, login)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", response.Code)
	}
	if got := response.Result().Header.Get("Location"); got != "/student/sessions/new" {
		t.Fatalf("redirect = %q, want %q", got, "/student/sessions/new")
	}
}

func kidSessions(t *testing.T) *auth.KidSessionManager {
	t.Helper()
	store, err := auth.NewStore(t.TempDir() + "/kids.json")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Add(auth.Kid{ID: "kid-1", Username: "mia", PinHash: kidPINHash(t), Status: auth.StatusApproved}); err != nil {
		t.Fatalf("add kid: %v", err)
	}
	return auth.NewKidSessionManager(auth.NewService(store, nil), "session-secret")
}

func kidPINHash(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("112233"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate PIN hash: %v", err)
	}
	return string(hash)
}
