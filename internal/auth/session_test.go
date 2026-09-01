package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestKidSessionManagerAcceptsApprovedKid(t *testing.T) {
	service := newApprovedKidService(t)
	manager := NewKidSessionManager(service, "session-secret")
	cookie, session, err := manager.Login("mia", "112233")
	if err != nil || session.KidID == "" || session.CSRFToken == "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("login = %#v, %#v, %v", cookie, session, err)
	}
	request := httptest.NewRequest(http.MethodGet, "/topics", nil)
	request.AddCookie(cookie)
	if got, ok := manager.Authenticate(request); !ok || got.KidID != session.KidID {
		t.Fatalf("session = %#v, ok = %t", got, ok)
	}
}

func TestVerifyKidCSRFRejectsMissingAndInvalidTokens(t *testing.T) {
	session := KidSession{CSRFToken: "expected-token"}
	missing := httptest.NewRequest(http.MethodPost, "/kid/topics", nil)
	if VerifyKidCSRF(missing, session) {
		t.Fatal("missing CSRF token must be rejected")
	}
	invalid := httptest.NewRequest(http.MethodPost, "/kid/topics", nil)
	invalid.Form = url.Values{"csrf_token": {"wrong-token"}}
	if VerifyKidCSRF(invalid, session) {
		t.Fatal("invalid CSRF token must be rejected")
	}
	valid := httptest.NewRequest(http.MethodPost, "/kid/topics", nil)
	valid.Form = url.Values{"csrf_token": {"expected-token"}}
	if !VerifyKidCSRF(valid, session) {
		t.Fatal("valid CSRF token must be accepted")
	}
}

func TestKidSessionManagerRejectsInvalidAndTamperedSessions(t *testing.T) {
	service := newApprovedKidService(t)
	manager := NewKidSessionManager(service, "session-secret")
	if _, _, err := manager.Login("mia", "wrong"); err == nil {
		t.Fatal("invalid PIN must be rejected")
	}
	manager = NewKidSessionManager(newApprovedKidService(t), "session-secret")
	cookie, _, err := manager.Login("mia", "112233")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/topics", nil)
	request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value + "tampered"})
	if _, ok := manager.Authenticate(request); ok {
		t.Fatal("tampered session must be rejected")
	}
}

func TestKidSessionManagerRejectsExpiredAndPendingKids(t *testing.T) {
	store := MustNewStore(t.TempDir() + "/kids.json")
	service := NewService(store, nil)
	if _, err := service.Signup("pending", "112233"); err != nil {
		t.Fatalf("signup: %v", err)
	}
	manager := NewKidSessionManager(service, "session-secret")
	if _, _, err := manager.Login("pending", "112233"); err == nil {
		t.Fatal("pending kid must be rejected")
	}
	approved := newApprovedKidService(t)
	manager = NewKidSessionManager(approved, "session-secret")
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	cookie, _, err := manager.Login("mia", "112233")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	manager.now = func() time.Time { return now.Add(9 * time.Hour) }
	request := httptest.NewRequest(http.MethodGet, "/topics", nil)
	request.AddCookie(cookie)
	if _, ok := manager.Authenticate(request); ok {
		t.Fatal("expired session must be rejected")
	}
}

func newApprovedKidService(t *testing.T) *Service {
	t.Helper()
	store := MustNewStore(t.TempDir() + "/kids.json")
	service := NewService(store, nil)
	if _, err := service.CreateApprovedKid("mia", "112233"); err != nil {
		t.Fatalf("create kid: %v", err)
	}
	return service
}
