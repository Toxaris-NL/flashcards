// Package review calculates session-based card eligibility.
package review

import (
	"sort"
	"strings"

	"flashcards/internal/progress"
)

// Result records the outcome of one card answer in a session.
type Result struct {
	CardID     string
	PromptSide string
	Correct    bool
}

// DirectionKey identifies one independently scheduled direction of a pair.
func DirectionKey(cardID, promptSide string) string {
	if promptSide == "" {
		return cardID
	}
	return cardID + ":" + strings.ToLower(promptSide)
}

// ResetPairDirections removes current review state for both directions of a pair.
func ResetPairDirections(kidProgress *progress.KidProgress, cardID string) {
	delete(kidProgress.CardState, DirectionKey(cardID, "a"))
	delete(kidProgress.CardState, DirectionKey(cardID, "b"))
}

// UpdateState returns the review state after a result in a completed session.
func UpdateState(state progress.CardState, correct bool, completedSessions int) progress.CardState {
	state.LastReviewedSession = completedSessions
	if !correct {
		state.ConsecutiveCorrect = 0
		state.NextEligibleSession = completedSessions
		return state
	}

	state.ConsecutiveCorrect++
	state.NextEligibleSession = completedSessions + interval(state.ConsecutiveCorrect)
	return state
}

// IsEligible reports whether a card can be used at the start of a session.
func IsEligible(state progress.CardState, completedSessions int) bool {
	return state.NextEligibleSession <= completedSessions
}

// SelectCardIDs returns eligible cards first, or a least-recently-reviewed fallback.
func SelectCardIDs(cardIDs []string, states map[string]progress.CardState, completedSessions int) []string {
	selected := make([]string, 0, len(cardIDs))
	for _, cardID := range cardIDs {
		if IsEligible(states[cardID], completedSessions) {
			selected = append(selected, cardID)
		}
	}
	if len(selected) == 0 {
		selected = append(selected, cardIDs...)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		left := states[selected[i]]
		right := states[selected[j]]
		if left.LastReviewedSession == right.LastReviewedSession {
			return selected[i] < selected[j]
		}
		return left.LastReviewedSession < right.LastReviewedSession
	})
	return selected
}

// CompleteSession records results and advances review state after an answered session.
func CompleteSession(kidProgress *progress.KidProgress, listID string, results []Result) bool {
	if len(results) == 0 {
		return false
	}
	if kidProgress.ListSessions == nil {
		kidProgress.ListSessions = map[string]int{}
	}
	if kidProgress.CardState == nil {
		kidProgress.CardState = map[string]progress.CardState{}
	}
	completedSessions := kidProgress.ListSessions[listID] + 1
	kidProgress.ListSessions[listID] = completedSessions
	for _, result := range results {
		key := DirectionKey(result.CardID, result.PromptSide)
		kidProgress.CardState[key] = UpdateState(kidProgress.CardState[key], result.Correct, completedSessions)
	}
	return true
}

func interval(consecutiveCorrect int) int {
	switch {
	case consecutiveCorrect <= 1:
		return 2
	case consecutiveCorrect == 2:
		return 4
	default:
		return 8
	}
}
