package study

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"flashcards/internal/progress"
	"flashcards/internal/review"
)

// Card describes a study card.
type Card struct {
	ID    string
	Front string
	Back  string
}

// List is a deck the kid is studying.
type List struct {
	Subject       string
	Period        string
	Cards         []Card
	PassThreshold int
}

// Progress tracks per-session results.
type Progress struct {
	TotalCorrect  int
	TotalAnswered int
	CurrentIndex  int
	TotalCards    int
}

// Session is a single study run.
type Session struct {
	ID            string
	Subject       string
	Period        string
	StartedAt     time.Time
	Mode          string
	Difficulty    string
	Queue         []Card
	QuestionModes map[string]string
	PromptSides   map[string]string
	Progress      Progress
	Results       []review.Result
	TimerSec      int
}

// MixSettings controls the multiple-choice share for each mixed-session difficulty.
type MixSettings struct {
	Easy int
	OK   int
	Hard int
}

// DefaultMixSettings returns the configured v1 difficulty defaults.
func DefaultMixSettings() MixSettings {
	return MixSettings{Easy: 80, OK: 40, Hard: 5}
}

// NewSession initializes a session with the chosen mode.
func NewSession(list List, mode string, timerSec int) (*Session, error) {
	return NewSessionWithDifficulty(list, mode, "ok", timerSec)
}

// NewSessionWithDifficulty initializes a session with its mixed-mode difficulty.
func NewSessionWithDifficulty(list List, mode, difficulty string, timerSec int) (*Session, error) {
	return NewSessionWithSettings(list, mode, difficulty, timerSec, DefaultMixSettings())
}

// NewSessionWithSettings initializes a session using the supplied mix settings.
func NewSessionWithSettings(list List, mode, difficulty string, timerSec int, settings MixSettings) (*Session, error) {
	if len(list.Cards) == 0 {
		return nil, fmt.Errorf("no cards in list")
	}
	if !settings.valid() {
		return nil, fmt.Errorf("invalid mixed-session settings")
	}
	sessionID, err := newSessionID()
	if err != nil {
		return nil, err
	}
	startedAt := time.Now().UTC()

	if mode == "mixed" && len(list.Cards) >= 4 {
		if !validDifficulty(difficulty) {
			return nil, fmt.Errorf("invalid session difficulty")
		}
		queue := append([]Card(nil), list.Cards...)
		sides := promptSides(queue, false)
		return &Session{ID: sessionID, Subject: list.Subject, Period: list.Period, StartedAt: startedAt, Mode: "mixed", Difficulty: difficulty, Queue: queue, QuestionModes: mixedQuestionModes(queue, sides, settings.percentage(difficulty)), PromptSides: sides, Progress: Progress{TotalCards: len(list.Cards)}, TimerSec: timerSec}, nil
	}

	queue := append([]Card(nil), list.Cards...)
	return &Session{ID: sessionID, Subject: list.Subject, Period: list.Period, StartedAt: startedAt, Mode: "typed", Queue: queue, QuestionModes: typedQuestionModes(queue), PromptSides: promptSides(queue, false), Progress: Progress{TotalCards: len(list.Cards)}, TimerSec: timerSec}, nil
}

// NewBidirectionalSession creates a session that independently chooses each pair's prompt side.
func NewBidirectionalSession(list List, mode, difficulty string, timerSec int, settings MixSettings) (*Session, error) {
	session, err := NewSessionWithSettings(list, mode, difficulty, timerSec, settings)
	if err != nil {
		return nil, err
	}
	session.PromptSides = promptSides(session.Queue, true)
	return session, nil
}

// NewScheduledSession applies session-based review ordering after the pass threshold.
func NewScheduledSession(list List, mode string, timerSec, completedSessions int, states map[string]progress.CardState) (*Session, error) {
	session, err := NewSession(list, mode, timerSec)
	if err != nil {
		return nil, err
	}
	threshold := list.PassThreshold
	if threshold == 0 {
		threshold = 2
	}
	if completedSessions < threshold {
		return session, nil
	}

	cardIDs := make([]string, 0, len(list.Cards))
	byID := make(map[string]Card, len(list.Cards))
	for _, card := range list.Cards {
		cardIDs = append(cardIDs, card.ID)
		byID[card.ID] = card
	}
	selectedIDs := review.SelectCardIDs(cardIDs, states, completedSessions)
	session.Queue = make([]Card, 0, len(selectedIDs))
	for _, cardID := range selectedIDs {
		session.Queue = append(session.Queue, byID[cardID])
	}
	return session, nil
}

// MatchesAnswer compares typed responses according to list-level typing mode.
func MatchesAnswer(answer, correct, matchMode string) bool {
	if matchMode == "strict" {
		return answer == correct
	}
	return strings.TrimSpace(strings.ToLower(answer)) == strings.TrimSpace(strings.ToLower(correct))
}

// GradeCurrent marks the current card as correct or incorrect and requeues it if necessary.
func (s *Session) GradeCurrent(answer, matchMode string) error {
	if len(s.Queue) == 0 {
		return fmt.Errorf("no cards left")
	}
	current := s.Queue[0]
	s.Queue = s.Queue[1:]
	s.Progress.TotalAnswered++
	promptSide := s.PromptSides[current.ID]
	_, expectedAnswer := PromptAndAnswer(current, promptSide)
	if MatchesAnswer(answer, expectedAnswer, matchMode) {
		s.Progress.TotalCorrect++
		s.Results = append(s.Results, review.Result{CardID: current.ID, PromptSide: promptSide, Correct: true})
		return nil
	}
	s.Results = append(s.Results, review.Result{CardID: current.ID, PromptSide: promptSide, Correct: false})
	if len(s.Queue) == 0 {
		s.Queue = append(s.Queue, current)
		return nil
	}
	insertAt := 1 + rand.Intn(len(s.Queue))
	if insertAt > len(s.Queue) {
		insertAt = len(s.Queue)
	}
	requeued := append([]Card{}, s.Queue[:insertAt]...)
	requeued = append(requeued, current)
	requeued = append(requeued, s.Queue[insertAt:]...)
	s.Queue = requeued
	return nil
}

// Summary returns an appendable record when the session has at least one answer.
func (s *Session) Summary(endedAt time.Time) (progress.SessionSummary, bool) {
	if len(s.Results) == 0 {
		return progress.SessionSummary{}, false
	}
	seen := make(map[string]bool)
	correctFirstTry := 0
	for _, result := range s.Results {
		if seen[result.CardID] {
			continue
		}
		seen[result.CardID] = true
		if result.Correct {
			correctFirstTry++
		}
	}
	return progress.SessionSummary{
		ID:              s.ID,
		Subject:         s.Subject,
		Period:          s.Period,
		StartedAt:       s.StartedAt,
		EndedAt:         endedAt.UTC(),
		Mode:            s.Mode,
		CardsSeen:       len(seen),
		CorrectFirstTry: correctFirstTry,
		TotalAttempts:   len(s.Results),
	}, true
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func newSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := cryptorand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func validDifficulty(difficulty string) bool {
	return difficulty == "easy" || difficulty == "ok" || difficulty == "hard"
}

func (s MixSettings) valid() bool {
	return validPercentage(s.Easy) && validPercentage(s.OK) && validPercentage(s.Hard)
}

func (s MixSettings) percentage(difficulty string) int {
	switch difficulty {
	case "easy":
		return s.Easy
	case "hard":
		return s.Hard
	default:
		return s.OK
	}
}

func validPercentage(percentage int) bool {
	return percentage >= 0 && percentage <= 100
}

func mixedQuestionModes(cards []Card, sides map[string]string, percentage int) map[string]string {
	modes := typedQuestionModes(cards)
	count := int(math.Round(float64(len(cards)*percentage) / 100))
	indices := rand.Perm(len(cards))
	for _, index := range indices[:count] {
		if HasEnoughDistractors(cards[index], cards, sides[cards[index].ID]) {
			modes[cards[index].ID] = "multiple_choice"
		}
	}
	return modes
}

func typedQuestionModes(cards []Card) map[string]string {
	modes := make(map[string]string, len(cards))
	for _, card := range cards {
		modes[card.ID] = "typed"
	}
	return modes
}

func promptSides(cards []Card, randomize bool) map[string]string {
	sides := make(map[string]string, len(cards))
	for _, card := range cards {
		side := "a"
		if randomize && rand.Intn(2) == 1 {
			side = "b"
		}
		sides[card.ID] = side
	}
	return sides
}

// PromptAndAnswer returns the visible prompt and required opposite side for a pair direction.
func PromptAndAnswer(card Card, promptSide string) (string, string) {
	if promptSide == "b" {
		return card.Back, card.Front
	}
	return card.Front, card.Back
}

// HasEnoughDistractors checks the opposite side values for one prompt direction.
func HasEnoughDistractors(card Card, cards []Card, promptSide string) bool {
	distractors := make(map[string]struct{})
	_, answer := PromptAndAnswer(card, promptSide)
	for _, candidate := range cards {
		_, candidateAnswer := PromptAndAnswer(candidate, promptSide)
		if candidate.ID == card.ID || candidateAnswer == "" || candidateAnswer == answer {
			continue
		}
		distractors[candidateAnswer] = struct{}{}
	}
	return len(distractors) >= 3
}
