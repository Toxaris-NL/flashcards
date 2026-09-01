package admin

import (
	"errors"
	"sort"
	"time"

	"flashcards/internal/auth"
	"flashcards/internal/progress"
)

// KidProgressSummary is the admin's progress view for one kid.
type KidProgressSummary struct {
	KidID          string
	Username       string
	RecentActivity time.Time
	Subjects       []progress.SubjectSummary
}

// ProgressDashboardService provides admin-only cross-kid progress summaries.
type ProgressDashboardService struct {
	progressStore *progress.Store
}

// NewProgressDashboardService creates an admin progress summary service.
func NewProgressDashboardService(progressStore *progress.Store) *ProgressDashboardService {
	return &ProgressDashboardService{progressStore: progressStore}
}

// Summaries returns recent activity and per-subject accuracy for every kid.
func (s *ProgressDashboardService) Summaries(authenticated bool, kids []auth.Kid) ([]KidProgressSummary, error) {
	if !authenticated {
		return nil, errors.New("admin authentication required")
	}

	summaries := make([]KidProgressSummary, 0, len(kids))
	for _, kid := range kids {
		kidProgress, err := s.progressStore.Load(kid.ID)
		if err != nil {
			return nil, err
		}
		dashboard := progress.KidDashboard(kidProgress)
		summary := KidProgressSummary{
			KidID:    kid.ID,
			Username: kid.Username,
			Subjects: dashboard.Subjects,
		}
		if len(dashboard.RecentSessions) > 0 {
			summary.RecentActivity = dashboard.RecentSessions[0].EndedAt
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Username < summaries[j].Username
	})
	return summaries, nil
}
