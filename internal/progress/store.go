// Package progress persists per-kid study and review state.
package progress

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// CardState records a card's session-based review eligibility.
type CardState struct {
	ConsecutiveCorrect  int `json:"consecutive_correct"`
	NextEligibleSession int `json:"next_eligible_session"`
	LastReviewedSession int `json:"last_reviewed_session"`
}

// SessionSummary records one completed or stopped study session.
type SessionSummary struct {
	ID              string    `json:"id"`
	Subject         string    `json:"subject"`
	Period          string    `json:"period"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
	Mode            string    `json:"mode"`
	CardsSeen       int       `json:"cards_seen"`
	CorrectFirstTry int       `json:"correct_first_try"`
	TotalAttempts   int       `json:"total_attempts"`
}

// SubjectSummary is one subject's aggregated study performance.
type SubjectSummary struct {
	Subject         string
	CardsSeen       int
	CorrectFirstTry int
	Accuracy        float64
}

// Dashboard summarizes a kid's progress across all subjects.
type Dashboard struct {
	Subjects       []SubjectSummary
	MasteredCards  int
	RecentSessions []SessionSummary
}

// KidProgress is persisted separately from source card content.
type KidProgress struct {
	ListSessions map[string]int       `json:"list_sessions,omitempty"`
	CardState    map[string]CardState `json:"card_state,omitempty"`
	Sessions     []SessionSummary     `json:"sessions,omitempty"`
	LastSubject  string               `json:"last_subject,omitempty"`
	LastPeriod   string               `json:"last_period,omitempty"`
}

// Store persists each kid's progress in a separate JSON file.
type Store struct {
	mu   sync.Mutex
	root string
}

// NewStore creates a progress store rooted at the supplied directory.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// Load returns an empty progress record when the kid has no saved progress.
func (s *Store) Load(kidID string) (KidProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path(kidID))
	if errors.Is(err, os.ErrNotExist) {
		return newKidProgress(), nil
	}
	if err != nil {
		return KidProgress{}, err
	}

	progress := newKidProgress()
	if len(data) > 0 {
		if err := json.Unmarshal(data, &progress); err != nil {
			return KidProgress{}, err
		}
	}
	if progress.ListSessions == nil {
		progress.ListSessions = map[string]int{}
	}
	if progress.CardState == nil {
		progress.CardState = map[string]CardState{}
	}
	if progress.Sessions == nil {
		progress.Sessions = []SessionSummary{}
	}
	return progress, nil
}

// Save writes a kid's progress atomically enough for the single-binary process.
func (s *Store) Save(kidID string, progress KidProgress) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(kidID), data, 0o600)
}

// AppendSession appends an answered study session without replacing review state.
func (s *Store) AppendSession(kidID string, summary SessionSummary) error {
	if summary.CardsSeen == 0 {
		return errors.New("session must contain at least one answer")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	progress := newKidProgress()
	data, err := os.ReadFile(s.path(kidID))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &progress); err != nil {
			return err
		}
	}
	progress.Sessions = append(progress.Sessions, summary)
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	data, err = json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(kidID), data, 0o600)
}

// RecentListHistory returns the three most recent summaries for a subject and period.
func RecentListHistory(kidProgress KidProgress, subject, period string) []SessionSummary {
	history := make([]SessionSummary, 0, 3)
	for _, summary := range kidProgress.Sessions {
		if summary.Subject == subject && summary.Period == period {
			history = append(history, summary)
		}
	}
	sort.SliceStable(history, func(i, j int) bool {
		return history[i].EndedAt.After(history[j].EndedAt)
	})
	if len(history) > 3 {
		history = history[:3]
	}
	return history
}

// KidDashboard derives subject accuracy, mastered cards, and recent sessions.
func KidDashboard(kidProgress KidProgress) Dashboard {
	bySubject := map[string]SubjectSummary{}
	for _, session := range kidProgress.Sessions {
		summary := bySubject[session.Subject]
		summary.Subject = session.Subject
		summary.CardsSeen += session.CardsSeen
		summary.CorrectFirstTry += session.CorrectFirstTry
		bySubject[session.Subject] = summary
	}
	dashboard := Dashboard{RecentSessions: append([]SessionSummary(nil), kidProgress.Sessions...)}
	for _, state := range kidProgress.CardState {
		if state.ConsecutiveCorrect >= 4 {
			dashboard.MasteredCards++
		}
	}
	for _, summary := range bySubject {
		if summary.CardsSeen > 0 {
			summary.Accuracy = float64(summary.CorrectFirstTry) / float64(summary.CardsSeen)
		}
		dashboard.Subjects = append(dashboard.Subjects, summary)
	}
	sort.Slice(dashboard.Subjects, func(i, j int) bool {
		return dashboard.Subjects[i].Subject < dashboard.Subjects[j].Subject
	})
	sort.SliceStable(dashboard.RecentSessions, func(i, j int) bool {
		return dashboard.RecentSessions[i].EndedAt.After(dashboard.RecentSessions[j].EndedAt)
	})
	return dashboard
}

func (s *Store) path(kidID string) string {
	return filepath.Join(s.root, kidID+".json")
}

func newKidProgress() KidProgress {
	return KidProgress{
		ListSessions: map[string]int{},
		CardState:    map[string]CardState{},
		Sessions:     []SessionSummary{},
	}
}
