package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusDisabled = "disabled"
)

var pinPattern = regexp.MustCompile(`^\d{6}$`)

// Student is the persisted record for a student account.
type Student = Kid

// Kid is the persisted record for a kid account.
type Kid struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	PinHash       string    `json:"pin_hash"`
	MustChangePIN bool      `json:"must_change_pin,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	ApprovedAt    time.Time `json:"approved_at,omitempty"`
	DisabledAt    time.Time `json:"disabled_at,omitempty"`
}

// StudentStore persists student records in a JSON file.
type StudentStore = Store

// Store persists kid records in a JSON file.
type Store struct {
	mu   sync.Mutex
	path string
	kids []Kid
}

func NewStore(path string) (*Store, error) {
	store := &Store{path: path}
	if err := store.load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, err
	}
	return store, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.kids)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.kids, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *Store) Add(kid Kid) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.kids {
		if strings.EqualFold(existing.Username, kid.Username) {
			return fmt.Errorf("username already taken")
		}
	}
	s.kids = append(s.kids, kid)
	return s.save()
}

func (s *Store) Update(kid Kid) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.kids {
		if existing.ID == kid.ID {
			s.kids[i] = kid
			return s.save()
		}
	}
	return fmt.Errorf("kid not found")
}

// Delete removes a kid record after an explicit admin rejection.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, kid := range s.kids {
		if kid.ID != id {
			continue
		}
		s.kids = append(s.kids[:index], s.kids[index+1:]...)
		return s.save()
	}
	return fmt.Errorf("kid not found")
}

func (s *Store) GetByUsername(username string) (*Kid, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.kids {
		if strings.EqualFold(s.kids[i].Username, username) {
			kid := s.kids[i]
			return &kid, true
		}
	}
	return nil, false
}

func (s *Store) List() []Kid {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Kid, len(s.kids))
	copy(out, s.kids)
	return out
}

func (s *Store) Get(id string) (*Kid, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.kids {
		if s.kids[i].ID == id {
			kid := s.kids[i]
			return &kid, true
		}
	}
	return nil, false
}

// Service handles account lifecycle rules.
type Service struct {
	store      *Store
	mailSender func(to string, subject string, body string) error
	lockout    map[string]time.Time
	lockoutMu  sync.Mutex
}

func NewService(store *Store, mailSender func(string, string, string) error) *Service {
	return &Service{
		store:      store,
		mailSender: mailSender,
		lockout:    map[string]time.Time{},
	}
}

func (s *Service) Signup(username, pin string) (*Kid, error) {
	if !pinPattern.MatchString(pin) {
		return nil, errors.New("vul een 6-cijferige pincode in")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("gebruikersnaam is verplicht")
	}
	if _, ok := s.store.GetByUsername(username); ok {
		return nil, errors.New("gebruikersnaam is al in gebruik")
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	kid := Kid{
		ID:        id,
		Username:  username,
		PinHash:   string(hash),
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.Add(kid); err != nil {
		return nil, err
	}
	if s.mailSender != nil {
		_ = s.mailSender("admin@example.com", "Nieuwe aanmelding", fmt.Sprintf("Nieuwe accountaanvraag voor %s", username))
	}
	return &kid, nil
}

func (s *Service) Login(username, pin string) (*Kid, error) {
	if s.isLocked(username) {
		return nil, errors.New("account tijdelijk geblokkeerd")
	}
	kid, ok := s.store.GetByUsername(username)
	if !ok {
		return nil, errors.New("onjuiste gebruikersnaam of pincode")
	}
	if kid.Status == StatusPending {
		return nil, errors.New("account wacht op goedkeuring")
	}
	if kid.Status == StatusDisabled {
		return nil, errors.New("account is uitgeschakeld")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(kid.PinHash), []byte(pin)); err != nil {
		s.recordFailure(username)
		return nil, errors.New("onjuiste gebruikersnaam of pincode")
	}
	s.clearFailure(username)
	return kid, nil
}

func (s *Service) Approve(id string) error {
	kid, ok := s.store.Get(id)
	if !ok {
		return fmt.Errorf("kid not found")
	}
	kid.Status = StatusApproved
	kid.ApprovedAt = time.Now().UTC()
	return s.store.Update(*kid)
}

func (s *Service) ApproveByUsername(username string) error {
	kid, ok := s.store.GetByUsername(username)
	if !ok {
		return fmt.Errorf("kid not found")
	}
	return s.Approve(kid.ID)
}

func (s *Service) Disable(id string) error {
	kid, ok := s.store.Get(id)
	if !ok {
		return fmt.Errorf("kid not found")
	}
	kid.Status = StatusDisabled
	kid.DisabledAt = time.Now().UTC()
	return s.store.Update(*kid)
}

// Enable restores an explicitly disabled kid account to approved status.
func (s *Service) Enable(id string) error {
	kid, ok := s.store.Get(id)
	if !ok {
		return fmt.Errorf("kid not found")
	}
	if kid.Status != StatusDisabled {
		return fmt.Errorf("kid is not disabled")
	}
	kid.Status = StatusApproved
	kid.DisabledAt = time.Time{}
	return s.store.Update(*kid)
}

// Reject permanently deletes a pending account after explicit admin action.
func (s *Service) Reject(id string) error {
	kid, ok := s.store.Get(id)
	if !ok {
		return fmt.Errorf("kid not found")
	}
	if kid.Status != StatusPending {
		return fmt.Errorf("only pending kids can be rejected")
	}
	return s.store.Delete(id)
}

func (s *Service) CreateApprovedKid(username, pin string) (*Kid, error) {
	if !pinPattern.MatchString(pin) {
		return nil, errors.New("vul een 6-cijferige pincode in")
	}
	if _, ok := s.store.GetByUsername(username); ok {
		return nil, errors.New("gebruikersnaam is al in gebruik")
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	pw, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	kid := Kid{
		ID:         id,
		Username:   username,
		PinHash:    string(pw),
		Status:     StatusApproved,
		CreatedAt:  time.Now().UTC(),
		ApprovedAt: time.Now().UTC(),
	}
	if err := s.store.Add(kid); err != nil {
		return nil, err
	}
	return &kid, nil
}

// ResetPIN replaces a student's PIN and requires a change at the next login.
func (s *Service) ResetPIN(id, temporaryPIN string) error {
	if !pinPattern.MatchString(temporaryPIN) {
		return errors.New("vul een 6-cijferige tijdelijke pincode in")
	}
	kid, ok := s.store.Get(id)
	if !ok {
		return fmt.Errorf("kid not found")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(temporaryPIN), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	kid.PinHash = string(hash)
	kid.MustChangePIN = true
	s.clearFailure(kid.Username)
	return s.store.Update(*kid)
}

// ChangePIN verifies the temporary PIN and stores the student's chosen replacement.
func (s *Service) ChangePIN(id, currentPIN, newPIN string) error {
	if !pinPattern.MatchString(newPIN) {
		return errors.New("vul een 6-cijferige pincode in")
	}
	kid, ok := s.store.Get(id)
	if !ok || bcrypt.CompareHashAndPassword([]byte(kid.PinHash), []byte(currentPIN)) != nil {
		return errors.New("onjuiste tijdelijke pincode")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPIN), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	kid.PinHash = string(hash)
	kid.MustChangePIN = false
	s.clearFailure(kid.Username)
	return s.store.Update(*kid)
}

// MustChangePIN reports whether the student must replace a temporary PIN.
func (s *Service) MustChangePIN(id string) bool {
	kid, ok := s.store.Get(id)
	return ok && kid.MustChangePIN
}

func (s *Service) isLocked(username string) bool {
	s.lockoutMu.Lock()
	defer s.lockoutMu.Unlock()
	if when, ok := s.lockout[username]; ok && time.Now().Before(when) {
		return true
	}
	return false
}

func (s *Service) recordFailure(username string) {
	s.lockoutMu.Lock()
	defer s.lockoutMu.Unlock()
	if s.lockout[username].IsZero() {
		s.lockout[username] = time.Now().Add(5 * time.Minute)
		return
	}
	if time.Now().After(s.lockout[username]) {
		s.lockout[username] = time.Now().Add(5 * time.Minute)
		return
	}
	if time.Until(s.lockout[username]) < 0 {
		s.lockout[username] = time.Now().Add(5 * time.Minute)
	}
}

func (s *Service) clearFailure(username string) {
	s.lockoutMu.Lock()
	defer s.lockoutMu.Unlock()
	delete(s.lockout, username)
}

func (s *Service) lockoutUntil(username string) time.Time {
	s.lockoutMu.Lock()
	defer s.lockoutMu.Unlock()
	return s.lockout[username]
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func init() {
	_ = os.MkdirAll("data", 0o755)
}

func main() {}
