package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"flashcards/internal/config"

	"golang.org/x/crypto/bcrypt"
)

const adminSessionCookie = "flashcards_admin"

// Session contains verified admin-session state.
type Session struct {
	ExpiresAt time.Time `json:"expires_at"`
	CSRFToken string    `json:"csrf_token"`
}

// SessionManager authenticates configured admin credentials and signs sessions.
type SessionManager struct {
	mu     sync.RWMutex
	config config.Config
	now    func() time.Time
}

// NewSessionManager creates a manager using the system clock.
func NewSessionManager(config config.Config) *SessionManager {
	return &SessionManager{config: config, now: time.Now}
}

// Login verifies credentials and creates a signed session cookie.
func (m *SessionManager) Login(username, password string) (*http.Cookie, Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !constantTimeEqual(username, m.config.AdminUsername) || bcrypt.CompareHashAndPassword([]byte(m.config.AdminPasswordHash), []byte(password)) != nil {
		return nil, Session{}, false
	}
	csrfToken, err := randomToken()
	if err != nil {
		return nil, Session{}, false
	}
	session := Session{ExpiresAt: m.now().Add(8 * time.Hour).UTC(), CSRFToken: csrfToken}
	value, err := m.sign(session)
	if err != nil {
		return nil, Session{}, false
	}
	return &http.Cookie{
		Name:     adminSessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	}, session, true
}

// ChangePassword verifies the current password and persists a new one.
func (m *SessionManager) ChangePassword(currentPassword, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if bcrypt.CompareHashAndPassword([]byte(m.config.AdminPasswordHash), []byte(currentPassword)) != nil {
		return errors.New("current admin password is incorrect")
	}
	hash, err := config.UpdateAdminPassword(m.config.ConfigPath, newPassword)
	if err != nil {
		return err
	}
	m.config.AdminPasswordHash = hash
	return nil
}

// Authenticate verifies the request's signed, unexpired admin session.
func (m *SessionManager) Authenticate(request *http.Request) (Session, bool) {
	cookie, err := request.Cookie(adminSessionCookie)
	if err != nil {
		return Session{}, false
	}
	session, err := m.verify(cookie.Value)
	if err != nil || !session.ExpiresAt.After(m.now()) {
		return Session{}, false
	}
	return session, true
}

// VerifyCSRF reports whether a request carries the session's form token.
func VerifyCSRF(request *http.Request, session Session) bool {
	if err := request.ParseForm(); err != nil {
		return false
	}
	return constantTimeEqual(request.FormValue("csrf_token"), session.CSRFToken)
}

func (m *SessionManager) sign(session Session) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(m.config.SessionSecret))
	_, _ = mac.Write([]byte(encodedPayload))
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (m *SessionManager) verify(value string) (Session, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return Session{}, http.ErrNoCookie
	}
	mac := hmac.New(sha256.New, []byte(m.config.SessionSecret))
	_, _ = mac.Write([]byte(parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return Session{}, http.ErrNoCookie
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Session{}, err
	}
	var session Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func constantTimeEqual(left, right string) bool {
	leftHash := sha256.Sum256([]byte(left))
	rightHash := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
