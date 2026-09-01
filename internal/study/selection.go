package study

import (
	"sort"

	"flashcards/internal/content"
	"flashcards/internal/progress"
)

// SelectList chooses the saved last-used list when available, else the latest list.
func SelectList(references []content.ListReference, kidProgress progress.KidProgress) (content.ListReference, bool) {
	for _, reference := range references {
		if reference.Subject == kidProgress.LastSubject && reference.Period == kidProgress.LastPeriod {
			return reference, true
		}
	}
	if len(references) == 0 {
		return content.ListReference{}, false
	}
	sorted := append([]content.ListReference(nil), references...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt) })
	return sorted[0], true
}

// RememberListSelection records the source list chosen when a session starts.
func RememberListSelection(kidProgress *progress.KidProgress, subject, period string) {
	kidProgress.LastSubject = subject
	kidProgress.LastPeriod = period
}
