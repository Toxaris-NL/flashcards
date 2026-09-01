package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"flashcards/internal/config"

	"golang.org/x/crypto/bcrypt"
)

func TestSessionManagerIssuesAndAuthenticatesSignedCookie(t *testing.T) {
	manager := NewSessionManager(testConfig(t))
	cookie, session, ok := manager.Login("sander", "secret")
	if !ok || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || session.CSRFToken == "" {
		t.Fatalf("login result = %#v, %#v, %t", cookie, session, ok)
	}
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.AddCookie(cookie)
	if got, ok := manager.Authenticate(request); !ok || got.CSRFToken != session.CSRFToken {
		t.Fatalf("authenticated session = %#v, %t", got, ok)
	}
}

func TestSessionManagerRejectsInvalidAndTamperedCredentials(t *testing.T) {
	manager := NewSessionManager(testConfig(t))
	if _, _, ok := manager.Login("sander", "wrong"); ok {
		t.Fatal("invalid credentials must not create a session")
	}
	cookie, _, _ := manager.Login("sander", "secret")
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value + "tampered"})
	if _, ok := manager.Authenticate(request); ok {
		t.Fatal("tampered session must be rejected")
	}
}

func TestSessionManagerRejectsExpiredCookie(t *testing.T) {
	currentTime := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	manager := NewSessionManager(testConfig(t))
	manager.now = func() time.Time { return currentTime }
	cookie, _, ok := manager.Login("sander", "secret")
	if !ok {
		t.Fatal("expected valid login")
	}
	manager.now = func() time.Time { return currentTime.Add(9 * time.Hour) }
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.AddCookie(cookie)
	if _, ok := manager.Authenticate(request); ok {
		t.Fatal("expired session must be rejected")
	}
}

func TestVerifyCSRFRejectsMissingAndInvalidTokens(t *testing.T) {
	session := Session{CSRFToken: "expected-token"}
	missing := httptest.NewRequest(http.MethodPost, "/admin/kids", nil)
	if VerifyCSRF(missing, session) {
		t.Fatal("missing CSRF token must be rejected")
	}
	invalid := httptest.NewRequest(http.MethodPost, "/admin/kids", nil)
	invalid.Form = url.Values{"csrf_token": {"wrong-token"}}
	if VerifyCSRF(invalid, session) {
		t.Fatal("invalid CSRF token must be rejected")
	}
	valid := httptest.NewRequest(http.MethodPost, "/admin/kids", nil)
	valid.Form = url.Values{"csrf_token": {"expected-token"}}
	if !VerifyCSRF(valid, session) {
		t.Fatal("valid CSRF token must be accepted")
	}
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	return config.Config{ListenAddr: ":8080", DataDir: t.TempDir(), AdminUsername: "sander", AdminPasswordHash: string(hash), SessionSecret: "session-secret"}
}
