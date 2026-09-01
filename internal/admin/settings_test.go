package admin

import (
	"path/filepath"
	"testing"
	"time"

	"flashcards/internal/auth"
	"flashcards/internal/progress"
)

func TestSettingsStoreUsesAndPersistsDefaults(t *testing.T) {
	store := NewSettingsStore(filepath.Join(t.TempDir(), "settings.json"))
	got, err := store.Load()
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if got != (MixedSessionSettings{Easy: 80, OK: 40, Hard: 5}) {
		t.Fatalf("defaults = %#v", got)
	}

	want := MixedSessionSettings{Easy: 75, OK: 35, Hard: 10}
	if err := store.Save(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err = store.Load()
	if err != nil {
		t.Fatalf("load saved settings: %v", err)
	}
	if got != want {
		t.Fatalf("settings = %#v, want %#v", got, want)
	}
}

func TestServiceRejectsUnauthorizedAndInvalidUpdates(t *testing.T) {
	store := NewSettingsStore(filepath.Join(t.TempDir(), "settings.json"))
	service := NewService(store)
	if err := service.UpdateMixedSessionSettings(false, DefaultMixedSessionSettings()); err == nil {
		t.Fatal("expected unauthorized update to fail")
	}
	if err := service.UpdateMixedSessionSettings(true, MixedSessionSettings{Easy: 101, OK: 40, Hard: 5}); err == nil {
		t.Fatal("expected invalid percentage to fail")
	}
	if err := service.UpdateMixedSessionSettings(true, MixedSessionSettings{Easy: 0, OK: 100, Hard: 5}); err != nil {
		t.Fatalf("expected valid boundary percentages: %v", err)
	}
}

func TestProgressDashboardSummariesAggregateMultipleKids(t *testing.T) {
	store := progress.NewStore(filepath.Join(t.TempDir(), "progress"))
	if err := store.Save("kid-a", progress.KidProgress{Sessions: []progress.SessionSummary{{Subject: "Frans", CardsSeen: 10, CorrectFirstTry: 8, EndedAt: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)}}}); err != nil {
		t.Fatalf("save kid a progress: %v", err)
	}
	if err := store.Save("kid-b", progress.KidProgress{Sessions: []progress.SessionSummary{{Subject: "Geschiedenis", CardsSeen: 4, CorrectFirstTry: 3, EndedAt: time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)}}}); err != nil {
		t.Fatalf("save kid b progress: %v", err)
	}

	service := NewProgressDashboardService(store)
	summaries, err := service.Summaries(true, []auth.Kid{{ID: "kid-b", Username: "Bram"}, {ID: "kid-a", Username: "Anna"}})
	if err != nil {
		t.Fatalf("summaries failed: %v", err)
	}
	if len(summaries) != 2 || summaries[0].Username != "Anna" || summaries[0].Subjects[0].Accuracy != 0.8 {
		t.Fatalf("summaries = %#v", summaries)
	}
	if summaries[1].RecentActivity.IsZero() || summaries[1].Subjects[0].Subject != "Geschiedenis" {
		t.Fatalf("second summary = %#v", summaries[1])
	}
}

func TestProgressDashboardSummariesRequireAdminAuthentication(t *testing.T) {
	service := NewProgressDashboardService(progress.NewStore(t.TempDir()))
	if _, err := service.Summaries(false, nil); err == nil {
		t.Fatal("expected unauthenticated dashboard request to fail")
	}
}
