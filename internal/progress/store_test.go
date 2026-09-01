package progress

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSavesAndLoadsReviewState(t *testing.T) {
	store := NewStore(t.TempDir())
	want := KidProgress{
		ListSessions: map[string]int{"french-week-1": 3},
		CardState: map[string]CardState{
			"card-1": {ConsecutiveCorrect: 2, NextEligibleSession: 7, LastReviewedSession: 3},
		},
	}

	if err := store.Save("kid-1", want); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := store.Load("kid-1")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got.ListSessions["french-week-1"] != 3 {
		t.Fatalf("list sessions = %d, want 3", got.ListSessions["french-week-1"])
	}
	if got.CardState["card-1"] != want.CardState["card-1"] {
		t.Fatalf("card state = %#v, want %#v", got.CardState["card-1"], want.CardState["card-1"])
	}
}

func TestStoreSavesAndLoadsSessionSummaries(t *testing.T) {
	store := NewStore(t.TempDir())
	want := SessionSummary{
		ID: "session-1", Subject: "Frans", Period: "Hoofdstuk 4", Mode: "mixed",
		StartedAt: time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 8, 31, 10, 5, 0, 0, time.UTC),
		CardsSeen: 20, CorrectFirstTry: 15, TotalAttempts: 24,
	}
	if err := store.Save("kid-1", KidProgress{Sessions: []SessionSummary{want}}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := store.Load("kid-1")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(got.Sessions) != 1 || got.Sessions[0] != want {
		t.Fatalf("sessions = %#v, want %#v", got.Sessions, want)
	}
}

func TestStoreAppendsSessionWithoutReplacingReviewState(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Save("kid-1", KidProgress{CardState: map[string]CardState{"card-1": {ConsecutiveCorrect: 2}}}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	if err := store.AppendSession("kid-1", SessionSummary{ID: "session-1", CardsSeen: 1}); err != nil {
		t.Fatalf("append session: %v", err)
	}
	got, err := store.Load("kid-1")
	if err != nil {
		t.Fatalf("load progress: %v", err)
	}
	if len(got.Sessions) != 1 || got.CardState["card-1"].ConsecutiveCorrect != 2 {
		t.Fatalf("progress after append = %#v", got)
	}
}

func TestRecentListHistoryFiltersSortsAndLimitsSessions(t *testing.T) {
	day := func(number int) time.Time {
		return time.Date(2026, 8, number, 12, 0, 0, 0, time.UTC)
	}
	kidProgress := KidProgress{Sessions: []SessionSummary{
		{ID: "old", Subject: "Frans", Period: "Hoofdstuk 4", EndedAt: day(1)},
		{ID: "other-period", Subject: "Frans", Period: "Hoofdstuk 5", EndedAt: day(5)},
		{ID: "middle", Subject: "Frans", Period: "Hoofdstuk 4", EndedAt: day(2)},
		{ID: "new", Subject: "Frans", Period: "Hoofdstuk 4", EndedAt: day(4)},
		{ID: "newer", Subject: "Frans", Period: "Hoofdstuk 4", EndedAt: day(6)},
	}}

	history := RecentListHistory(kidProgress, "Frans", "Hoofdstuk 4")
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3", len(history))
	}
	want := []string{"newer", "new", "middle"}
	for index, id := range want {
		if history[index].ID != id {
			t.Fatalf("history = %#v, want ids %#v", history, want)
		}
	}
}

func TestKidDashboardAggregatesSubjectsMasteryAndRecentSessions(t *testing.T) {
	day := func(number int) time.Time {
		return time.Date(2026, 8, number, 12, 0, 0, 0, time.UTC)
	}
	kidProgress := KidProgress{
		CardState: map[string]CardState{
			"mastered": {ConsecutiveCorrect: 4},
			"learning": {ConsecutiveCorrect: 3},
		},
		Sessions: []SessionSummary{
			{ID: "older", Subject: "Frans", CardsSeen: 10, CorrectFirstTry: 8, EndedAt: day(1)},
			{ID: "newer", Subject: "Frans", CardsSeen: 5, CorrectFirstTry: 5, EndedAt: day(3)},
			{ID: "history", Subject: "Geschiedenis", CardsSeen: 4, CorrectFirstTry: 2, EndedAt: day(2)},
		},
	}

	dashboard := KidDashboard(kidProgress)
	if dashboard.MasteredCards != 1 {
		t.Fatalf("mastered cards = %d, want 1", dashboard.MasteredCards)
	}
	if len(dashboard.Subjects) != 2 || dashboard.Subjects[0].Subject != "Frans" {
		t.Fatalf("subjects = %#v", dashboard.Subjects)
	}
	french := dashboard.Subjects[0]
	if french.CardsSeen != 15 || french.CorrectFirstTry != 13 || french.Accuracy != 13.0/15.0 {
		t.Fatalf("french summary = %#v", french)
	}
	if dashboard.RecentSessions[0].ID != "newer" {
		t.Fatalf("recent sessions = %#v", dashboard.RecentSessions)
	}
}

func TestProgressLabelsAreCentralizedInDutch(t *testing.T) {
	for _, key := range []string{"accuracy", "cards_mastered", "date", "mode", "recent_activity", "recent_sessions", "score", "subject_progress"} {
		if LabelsNL[key] == "" {
			t.Fatalf("missing Dutch progress label %q", key)
		}
	}
}

func TestStoreLoadsExistingProgressWithoutReviewFields(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kid-1.json"), []byte(`{"sessions":[]}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := NewStore(root).Load("kid-1")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got.ListSessions == nil || got.CardState == nil || got.Sessions == nil {
		t.Fatal("expected initialized progress fields for existing progress")
	}
}
