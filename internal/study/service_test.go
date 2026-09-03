package study

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flashcards/internal/progress"
)

func TestSessionChoosesTypedWhenListIsTooShort(t *testing.T) {
	list := List{Cards: []Card{{ID: "a", Front: "one", Back: "een"}, {ID: "b", Front: "two", Back: "twee"}, {ID: "c", Front: "three", Back: "drie"}}}

	session, err := NewSession(list, "mixed", 0)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}
	if session.Mode != "typed" {
		t.Fatalf("expected typed fallback, got %q", session.Mode)
	}
}

func TestMixedModeEnabledForLargerDeck(t *testing.T) {
	list := List{Cards: []Card{{ID: "a", Front: "one", Back: "een"}, {ID: "b", Front: "two", Back: "twee"}, {ID: "c", Front: "three", Back: "drie"}, {ID: "d", Front: "four", Back: "vier"}}}

	session, err := NewSession(list, "mixed", 0)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}
	if session.Mode != "mixed" {
		t.Fatalf("expected mixed mode, got %q", session.Mode)
	}
}

func TestNewSessionRandomizesCardOrder(t *testing.T) {
	cards := make([]Card, 8)
	for index := range cards {
		cards[index] = Card{ID: string(rune('a' + index))}
	}

	first, err := NewSession(List{Cards: cards}, "typed", 0)
	if err != nil {
		t.Fatalf("new first session failed: %v", err)
	}
	firstOrder := make([]string, len(first.Queue))
	for index, card := range first.Queue {
		firstOrder[index] = card.ID
	}

	for attempt := 0; attempt < 20; attempt++ {
		other, err := NewSession(List{Cards: cards}, "typed", 0)
		if err != nil {
			t.Fatalf("new session failed: %v", err)
		}
		for index, card := range other.Queue {
			if card.ID != firstOrder[index] {
				return
			}
		}
	}
	t.Fatalf("session order never changed from %#v", firstOrder)
}

func TestMixedSessionStoresSelectedDifficulty(t *testing.T) {
	list := List{Cards: []Card{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}}
	session, err := NewSessionWithDifficulty(list, "mixed", "hard", 0)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}
	if session.Difficulty != "hard" {
		t.Fatalf("difficulty = %q, want hard", session.Difficulty)
	}
}

func TestBidirectionalSessionGeneratesAndGradesBothDirections(t *testing.T) {
	list := List{Cards: []Card{{ID: "a", Front: "bonjour", Back: "goedendag"}}}
	session, err := NewBidirectionalSession(list, "typed", "ok", 0, DefaultMixSettings())
	if err != nil {
		t.Fatalf("new bidirectional session: %v", err)
	}
	session.PromptSides["a"] = "b"
	prompt, answer := PromptAndAnswer(session.Queue[0], session.PromptSides["a"])
	if prompt != "goedendag" || answer != "bonjour" {
		t.Fatalf("prompt/answer = %q/%q", prompt, answer)
	}
	if err := session.GradeCurrent("bonjour", "normalized"); err != nil {
		t.Fatalf("grade reverse direction: %v", err)
	}
	if session.Results[0].PromptSide != "b" || !session.Results[0].Correct {
		t.Fatalf("result = %#v", session.Results[0])
	}
}

func TestMixedSessionRejectsUnknownDifficulty(t *testing.T) {
	list := List{Cards: []Card{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}}
	if _, err := NewSessionWithDifficulty(list, "mixed", "unknown", 0); err == nil {
		t.Fatal("expected invalid difficulty error")
	}
}

func TestMixedSessionAppliesDifficultyPercentage(t *testing.T) {
	cards := make([]Card, 10)
	for index := range cards {
		cards[index].ID = string(rune('a' + index))
		cards[index].Back = string(rune('A' + index))
	}

	for _, testCase := range []struct {
		difficulty string
		want       int
	}{
		{difficulty: "easy", want: 8},
		{difficulty: "ok", want: 4},
		{difficulty: "hard", want: 1},
	} {
		session, err := NewSessionWithDifficulty(List{Cards: cards}, "mixed", testCase.difficulty, 0)
		if err != nil {
			t.Fatalf("new session failed: %v", err)
		}
		got := 0
		for _, questionMode := range session.QuestionModes {
			if questionMode == "multiple_choice" {
				got++
			}
		}
		if got != testCase.want {
			t.Fatalf("%s multiple choice count = %d, want %d", testCase.difficulty, got, testCase.want)
		}
	}
}

func TestMixedSessionFallsBackToTypedModesForShortLists(t *testing.T) {
	list := List{Cards: []Card{{ID: "a"}, {ID: "b"}, {ID: "c"}}}
	session, err := NewSessionWithDifficulty(list, "mixed", "easy", 0)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}
	for _, questionMode := range session.QuestionModes {
		if questionMode != "typed" {
			t.Fatalf("question mode = %q, want typed", questionMode)
		}
	}
}

func TestMixedSessionFallsBackToTypedWhenCardLacksDistractors(t *testing.T) {
	list := List{Cards: []Card{
		{ID: "a", Back: "same"},
		{ID: "b", Back: "same"},
		{ID: "c", Back: "same"},
		{ID: "d", Back: "same"},
	}}
	session, err := NewSessionWithDifficulty(list, "mixed", "easy", 0)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}
	for _, questionMode := range session.QuestionModes {
		if questionMode != "typed" {
			t.Fatalf("question mode = %q, want typed", questionMode)
		}
	}
}

func TestDirectionSpecificDistractorsUseOppositePairSide(t *testing.T) {
	cards := []Card{
		{ID: "a", Front: "a", Back: "same"},
		{ID: "b", Front: "b", Back: "same"},
		{ID: "c", Front: "c", Back: "same"},
		{ID: "d", Front: "d", Back: "same"},
	}
	if HasEnoughDistractors(cards[0], cards, "a") {
		t.Fatal("same opposite-side values must not form distractors")
	}
	if !HasEnoughDistractors(cards[0], cards, "b") {
		t.Fatal("distinct front values must form reverse-direction distractors")
	}
}

func TestScheduledSessionUsesPlainOrderBeforePassThreshold(t *testing.T) {
	list := List{PassThreshold: 2, Cards: []Card{{ID: "a"}, {ID: "b"}}}
	session, err := NewScheduledSession(list, "typed", 0, 1, map[string]progress.CardState{"a": {NextEligibleSession: 10}})
	if err != nil {
		t.Fatalf("new scheduled session failed: %v", err)
	}
	if len(session.Queue) != 2 || !containsCardIDs(session.Queue, "a", "b") {
		t.Fatalf("unexpected pre-threshold queue: %#v", session.Queue)
	}
}

func TestScheduledSessionUsesEligibleCardsAfterPassThreshold(t *testing.T) {
	list := List{PassThreshold: 2, Cards: []Card{{ID: "a"}, {ID: "b"}}}
	states := map[string]progress.CardState{
		"a": {NextEligibleSession: 5, LastReviewedSession: 1},
		"b": {NextEligibleSession: 2, LastReviewedSession: 2},
	}
	session, err := NewScheduledSession(list, "typed", 0, 2, states)
	if err != nil {
		t.Fatalf("new scheduled session failed: %v", err)
	}
	if len(session.Queue) != 1 || session.Queue[0].ID != "b" {
		t.Fatalf("unexpected scheduled queue: %#v", session.Queue)
	}
}

func TestNormalizedMatchingIgnoresCaseAndWhitespace(t *testing.T) {
	if !MatchesAnswer("  Café  ", "café", "normalized") {
		t.Fatal("expected normalized match to ignore case and whitespace")
	}
	if MatchesAnswer("Café", "cafe", "normalized") {
		t.Fatal("expected accent-preserving comparison to fail for cafe vs café")
	}
}

func TestStrictMatchingRequiresExactString(t *testing.T) {
	if MatchesAnswer("Café", "café", "strict") {
		t.Fatal("expected strict match to require exact casing and spacing")
	}
}

func TestWrongAnswerRequeuesCurrentCard(t *testing.T) {
	list := List{Cards: []Card{{ID: "a", Front: "one", Back: "een"}, {ID: "b", Front: "two", Back: "twee"}, {ID: "c", Front: "three", Back: "drie"}}}
	session, err := NewSession(list, "typed", 0)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}

	if err := session.GradeCurrent("fout", "typed"); err != nil {
		t.Fatalf("grade failed: %v", err)
	}
	if len(session.Queue) != 3 {
		t.Fatalf("expected 3 cards in queue after requeue, got %d", len(session.Queue))
	}
	if session.Queue[0].ID == "a" {
		t.Fatal("expected wrong answer card to move later in the queue")
	}
}

func TestTimePressureMarksExpiredQuestionIncorrect(t *testing.T) {
	list := List{Cards: []Card{{ID: "a", Front: "one", Back: "een"}, {ID: "b", Front: "two", Back: "twee"}}}
	session, err := NewSession(list, "typed", 10)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}

	if err := session.GradeCurrent("", "typed"); err != nil {
		t.Fatalf("grade failed: %v", err)
	}
	if session.Progress.TotalCorrect != 0 {
		t.Fatal("expected expired question to count as incorrect")
	}
	if len(strings.TrimSpace(session.Queue[0].ID)) == 0 {
		t.Fatal("expected queue to remain valid after timeout")
	}
}

func TestProgressTracksCorrectAnswerWithoutCardConfidence(t *testing.T) {
	list := List{Cards: []Card{{ID: "a", Front: "one", Back: "een"}, {ID: "b", Front: "two", Back: "twee"}}}
	session, err := NewSession(list, "typed", 0)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}

	if err := session.GradeCurrent(session.Queue[0].Back, "typed"); err != nil {
		t.Fatalf("grade failed: %v", err)
	}
	if session.Progress.TotalCorrect != 1 {
		t.Fatalf("expected one correct answer, got %d", session.Progress.TotalCorrect)
	}
}

func TestSessionRecordsAnswerResultsForReviewPersistence(t *testing.T) {
	list := List{Cards: []Card{{ID: "a", Back: "een"}, {ID: "b", Back: "twee"}}}
	session, err := NewSession(list, "typed", 0)
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}
	firstCardID := session.Queue[0].ID
	answer := session.Queue[0].Back
	if err := session.GradeCurrent(answer, "normalized"); err != nil {
		t.Fatalf("grade failed: %v", err)
	}
	if len(session.Results) != 1 || !session.Results[0].Correct || session.Results[0].CardID != firstCardID {
		t.Fatalf("results = %#v", session.Results)
	}
}

func containsCardIDs(cards []Card, wanted ...string) bool {
	seen := make(map[string]bool, len(cards))
	for _, card := range cards {
		seen[card.ID] = true
	}
	for _, cardID := range wanted {
		if !seen[cardID] {
			return false
		}
	}
	return true
}

func TestSessionSummaryLogsNormalAndEarlyStopSessions(t *testing.T) {
	store := progress.NewStore(filepath.Join(t.TempDir(), "progress"))
	list := List{Subject: "Frans", Period: "Hoofdstuk 4", Cards: []Card{{ID: "a", Back: "een"}, {ID: "b", Back: "twee"}}}

	early, err := NewSession(list, "typed", 0)
	if err != nil {
		t.Fatalf("new early session failed: %v", err)
	}
	if err := early.GradeCurrent(early.Queue[0].Back, "normalized"); err != nil {
		t.Fatalf("grade early session: %v", err)
	}
	earlySummary, ok := early.Summary(time.Now())
	if !ok || earlySummary.CardsSeen != 1 || earlySummary.CorrectFirstTry != 1 {
		t.Fatalf("early summary = %#v, ok = %t", earlySummary, ok)
	}
	if err := store.AppendSession("kid-1", earlySummary); err != nil {
		t.Fatalf("append early session: %v", err)
	}

	normal, err := NewSession(List{Subject: "Frans", Period: "Hoofdstuk 4", Cards: []Card{{ID: "c", Back: "drie"}}}, "typed", 0)
	if err != nil {
		t.Fatalf("new normal session failed: %v", err)
	}
	if err := normal.GradeCurrent("drie", "normalized"); err != nil {
		t.Fatalf("grade normal session: %v", err)
	}
	normalSummary, ok := normal.Summary(time.Now())
	if !ok || normalSummary.TotalAttempts != 1 {
		t.Fatalf("normal summary = %#v, ok = %t", normalSummary, ok)
	}
	if err := store.AppendSession("kid-1", normalSummary); err != nil {
		t.Fatalf("append normal session: %v", err)
	}
	stored, err := store.Load("kid-1")
	if err != nil {
		t.Fatalf("load logged sessions: %v", err)
	}
	if len(stored.Sessions) != 2 {
		t.Fatalf("stored sessions = %d, want 2", len(stored.Sessions))
	}
}
