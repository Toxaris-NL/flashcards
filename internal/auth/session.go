package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const kidSessionCookie = "flashcards_kid"

// StudentSession contains the verified identity for a student-owned request.
type StudentSession = KidSession

// KidSession contains the verified identity for a kid-owned request.
type KidSession struct {
	KidID     string    `json:"kid_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CSRFToken string    `json:"csrf_token"`
}

// StudentSessionManager issues and validates signed student session cookies.
type StudentSessionManager = KidSessionManager

// KidSessionManager issues and validates signed kid session cookies.
type KidSessionManager struct {
	service *Service
	secret  []byte
	now     func() time.Time
}

// NewStudentSessionManager creates a student-session manager using the supplied signing secret.
func NewStudentSessionManager(service *Service, secret string) *StudentSessionManager {
	return NewKidSessionManager(service, secret)
}

// NewKidSessionManager creates a kid-session manager using the supplied signing secret.
func NewKidSessionManager(service *Service, secret string) *KidSessionManager {
	return &KidSessionManager{service: service, secret: []byte(secret), now: time.Now}
}

// Signup creates a new kid account pending approval.
func (m *KidSessionManager) Signup(username, pin string) (*Kid, error) {
	return m.service.Signup(username, pin)
}

// Login validates approved kid credentials and issues a signed session cookie.
func (m *KidSessionManager) Login(username, pin string) (*http.Cookie, KidSession, error) {
	kid, err := m.service.Login(username, pin)
	if err != nil {
		return nil, KidSession{}, err
	}
	csrfToken, err := randomCSRFToken()
	if err != nil {
		return nil, KidSession{}, err
	}
	session := KidSession{KidID: kid.ID, ExpiresAt: m.now().Add(8 * time.Hour).UTC(), CSRFToken: csrfToken}
	value, err := m.sign(session)
	if err != nil {
		return nil, KidSession{}, err
	}
	return &http.Cookie{Name: kidSessionCookie, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: session.ExpiresAt}, session, nil
}

// MustChangePIN reports whether a signed-in student must replace a temporary PIN.
func (m *KidSessionManager) MustChangePIN(kidID string) bool {
	return m.service.MustChangePIN(kidID)
}

// ChangePIN replaces a student's temporary PIN with their chosen PIN.
func (m *KidSessionManager) ChangePIN(kidID, currentPIN, newPIN string) error {
	return m.service.ChangePIN(kidID, currentPIN, newPIN)
}

// VerifyKidCSRF reports whether a request carries the current session's form token.
func VerifyKidCSRF(request *http.Request, session KidSession) bool {
	if err := request.ParseForm(); err != nil {
		return false
	}
	leftHash := sha256.Sum256([]byte(request.FormValue("csrf_token")))
	rightHash := sha256.Sum256([]byte(session.CSRFToken))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

// Authenticate verifies a request's signed, unexpired kid session.
func (m *KidSessionManager) Authenticate(request *http.Request) (KidSession, bool) {
	cookie, err := request.Cookie(kidSessionCookie)
	if err != nil {
		return KidSession{}, false
	}
	session, err := m.verify(cookie.Value)
	if err != nil || session.KidID == "" || !session.ExpiresAt.After(m.now()) {
		return KidSession{}, false
	}
	return session, true
}

func (m *KidSessionManager) sign(session KidSession) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (m *KidSessionManager) verify(value string) (KidSession, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return KidSession{}, http.ErrNoCookie
	}
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return KidSession{}, http.ErrNoCookie
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return KidSession{}, err
	}
	var session KidSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return KidSession{}, err
	}
	return session, nil
}

func randomCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
