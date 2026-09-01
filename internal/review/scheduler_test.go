package review

import (
	"path/filepath"
	"testing"

	"flashcards/internal/progress"
)

func TestCorrectAnswersUseTwoFourThenEightSessionIntervals(t *testing.T) {
	state := progress.CardState{}
	state = UpdateState(state, true, 1)
	if state.NextEligibleSession != 3 || state.ConsecutiveCorrect != 1 {
		t.Fatalf("first correct state = %#v", state)
	}
	state = UpdateState(state, true, 3)
	if state.NextEligibleSession != 7 || state.ConsecutiveCorrect != 2 {
		t.Fatalf("second correct state = %#v", state)
	}
	state = UpdateState(state, true, 7)
	if state.NextEligibleSession != 15 || state.ConsecutiveCorrect != 3 {
		t.Fatalf("third correct state = %#v", state)
	}
	state = UpdateState(state, true, 15)
	if state.NextEligibleSession != 23 || state.ConsecutiveCorrect != 4 {
		t.Fatalf("later correct state = %#v", state)
	}
}

func TestIncorrectAnswerIsEligibleForNextSession(t *testing.T) {
	state := UpdateState(progress.CardState{ConsecutiveCorrect: 3, NextEligibleSession: 20}, false, 5)
	if state.ConsecutiveCorrect != 0 || state.NextEligibleSession != 5 {
		t.Fatalf("incorrect state = %#v", state)
	}
	if !IsEligible(state, 5) {
		t.Fatal("incorrect card should be eligible for the next session")
	}
}

func TestEligibilityWaitsForRequiredCompletedSessions(t *testing.T) {
	state := progress.CardState{NextEligibleSession: 5}
	if IsEligible(state, 4) {
		t.Fatal("card should not be eligible before enough sessions are completed")
	}
	if !IsEligible(state, 5) {
		t.Fatal("card should be eligible after enough sessions are completed")
	}
}

func TestSelectCardIDsPrioritizesEligibleCards(t *testing.T) {
	states := map[string]progress.CardState{
		"due":    {NextEligibleSession: 3, LastReviewedSession: 2},
		"future": {NextEligibleSession: 5, LastReviewedSession: 1},
	}
	got := SelectCardIDs([]string{"future", "due"}, states, 3)
	if len(got) != 1 || got[0] != "due" {
		t.Fatalf("selected = %#v, want only due card", got)
	}
}

func TestSelectCardIDsFallsBackToLeastRecentlyReviewed(t *testing.T) {
	states := map[string]progress.CardState{
		"recent": {NextEligibleSession: 10, LastReviewedSession: 5},
		"old":    {NextEligibleSession: 10, LastReviewedSession: 1},
		"middle": {NextEligibleSession: 10, LastReviewedSession: 3},
	}
	got := SelectCardIDs([]string{"recent", "middle", "old"}, states, 5)
	want := []string{"old", "middle", "recent"}
	for index, cardID := range want {
		if got[index] != cardID {
			t.Fatalf("selected = %#v, want %#v", got, want)
		}
	}
}

func TestCompleteSessionUpdatesAndPersistsReviewState(t *testing.T) {
	store := progress.NewStore(filepath.Join(t.TempDir(), "progress"))
	kidProgress := progress.KidProgress{}
	if !CompleteSession(&kidProgress, "french-week-1", []Result{{CardID: "correct", Correct: true}, {CardID: "wrong", Correct: false}}) {
		t.Fatal("expected answered session to be recorded")
	}
	if kidProgress.ListSessions["french-week-1"] != 1 {
		t.Fatalf("completed sessions = %d, want 1", kidProgress.ListSessions["french-week-1"])
	}
	if kidProgress.CardState["correct"].NextEligibleSession != 3 {
		t.Fatalf("correct state = %#v", kidProgress.CardState["correct"])
	}
	if kidProgress.CardState["wrong"].NextEligibleSession != 1 {
		t.Fatalf("wrong state = %#v", kidProgress.CardState["wrong"])
	}
	if err := store.Save("kid-1", kidProgress); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	loaded, err := store.Load("kid-1")
	if err != nil {
		t.Fatalf("load progress: %v", err)
	}
	if loaded.CardState["correct"] != kidProgress.CardState["correct"] {
		t.Fatalf("persisted state = %#v", loaded.CardState["correct"])
	}
}

func TestCompleteSessionDoesNotAdvanceUnansweredSession(t *testing.T) {
	kidProgress := progress.KidProgress{}
	if CompleteSession(&kidProgress, "french-week-1", nil) {
		t.Fatal("expected unanswered session not to be recorded")
	}
	if len(kidProgress.ListSessions) != 0 {
		t.Fatalf("list sessions = %#v, want empty", kidProgress.ListSessions)
	}
}

func TestDirectionSpecificReviewStateSchedulesPairDirectionsIndependently(t *testing.T) {
	kidProgress := progress.KidProgress{}
	if !CompleteSession(&kidProgress, "french-week-1", []Result{{CardID: "pair-1", PromptSide: "a", Correct: true}}) {
		t.Fatal("expected completed session")
	}
	if _, ok := kidProgress.CardState[DirectionKey("pair-1", "b")]; ok {
		t.Fatal("opposite direction state must remain independent")
	}
	for completed := 3; completed <= 11; completed += 2 {
		CompleteSession(&kidProgress, "french-week-1", []Result{{CardID: "pair-1", PromptSide: "a", Correct: true}})
	}
	state := kidProgress.CardState[DirectionKey("pair-1", "a")]
	if state.ConsecutiveCorrect < 4 {
		t.Fatalf("direction state = %#v", state)
	}
}

func TestResetPairDirectionsKeepsOtherReviewState(t *testing.T) {
	kidProgress := progress.KidProgress{CardState: map[string]progress.CardState{
		DirectionKey("pair-1", "a"): {},
		DirectionKey("pair-1", "b"): {},
		DirectionKey("pair-2", "a"): {},
	}}
	ResetPairDirections(&kidProgress, "pair-1")
	if len(kidProgress.CardState) != 1 {
		t.Fatalf("state = %#v", kidProgress.CardState)
	}
	if _, ok := kidProgress.CardState[DirectionKey("pair-2", "a")]; !ok {
		t.Fatal("unrelated review state must be retained")
	}
}
