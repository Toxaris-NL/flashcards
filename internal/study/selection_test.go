package study

import (
	"testing"
	"time"

	"flashcards/internal/content"
	"flashcards/internal/progress"
)

func TestSelectListUsesSavedSelectionOrMostRecentlyUpdatedFallback(t *testing.T) {
	references := []content.ListReference{
		{Subject: "Frans", Period: "Hoofdstuk 4", UpdatedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)},
		{Subject: "Duits", Period: "Hoofdstuk 2", UpdatedAt: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)},
	}
	selected, ok := SelectList(references, progress.KidProgress{LastSubject: "Frans", LastPeriod: "Hoofdstuk 4"})
	if !ok || selected.Subject != "Frans" {
		t.Fatalf("saved selection = %#v, ok = %t", selected, ok)
	}
	selected, ok = SelectList(references, progress.KidProgress{LastSubject: "Weg", LastPeriod: "Oud"})
	if !ok || selected.Subject != "Duits" {
		t.Fatalf("fallback selection = %#v, ok = %t", selected, ok)
	}
}

func TestRememberListSelection(t *testing.T) {
	kidProgress := progress.KidProgress{}
	RememberListSelection(&kidProgress, "Frans", "Hoofdstuk 4")
	if kidProgress.LastSubject != "Frans" || kidProgress.LastPeriod != "Hoofdstuk 4" {
		t.Fatalf("selection = %#v", kidProgress)
	}
}
