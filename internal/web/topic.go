package web

import (
	"errors"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"flashcards/internal/auth"
	"flashcards/internal/content"
	"flashcards/internal/progress"
	"flashcards/internal/review"
	"flashcards/internal/study"
)

// TopicDependencies supplies the authenticated content service for kid-owned routes.
type TopicDependencies struct {
	Sessions *auth.KidSessionManager
	Content  *content.Service
	Progress *progress.Store
}

// NewTopicHandler serves kid-owned topic editor pages.
func NewTopicHandler(dependencies TopicDependencies) http.Handler {
	router := chi.NewRouter()
	router.Group(func(protected chi.Router) {
		protected.Use(requireKid(dependencies.Sessions))
		protected.Get("/student/sessions/new", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			references, err := dependencies.Content.ListReferences(session.KidID)
			if err != nil {
				http.Error(response, "lijsten laden mislukt", http.StatusInternalServerError)
				return
			}
			kidProgress, err := dependencies.Progress.Load(session.KidID)
			if err != nil {
				http.Error(response, "voortgang laden mislukt", http.StatusInternalServerError)
				return
			}
			selected, _ := study.SelectList(references, kidProgress)
			renderSessionSelection(response, session.CSRFToken, references, selected)
		})
		protected.Post("/student/sessions/select", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			references, err := dependencies.Content.ListReferences(session.KidID)
			if err != nil {
				http.Error(response, "lijsten laden mislukt", http.StatusInternalServerError)
				return
			}
			valid := false
			for _, reference := range references {
				if reference.Subject == request.FormValue("subject") && reference.Period == request.FormValue("period") {
					valid = true
				}
			}
			if !valid {
				http.Error(response, "ongeldige lijstkeuze", http.StatusBadRequest)
				return
			}
			kidProgress, err := dependencies.Progress.Load(session.KidID)
			if err != nil {
				http.Error(response, "voortgang laden mislukt", http.StatusInternalServerError)
				return
			}
			study.RememberListSelection(&kidProgress, request.FormValue("subject"), request.FormValue("period"))
			if err := dependencies.Progress.Save(session.KidID, kidProgress); err != nil {
				http.Error(response, "lijstkeuze opslaan mislukt", http.StatusInternalServerError)
				return
			}
			mode, difficulty := requestedSessionOptions(request)
			http.Redirect(response, request, "/student/study?mode="+mode+"&difficulty="+difficulty, http.StatusSeeOther)
		}))
		protected.Get("/student/study", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			kidProgress, err := dependencies.Progress.Load(session.KidID)
			if err != nil {
				http.Error(response, "voortgang laden mislukt", http.StatusInternalServerError)
				return
			}
			list, err := dependencies.Content.LoadList(session.KidID, kidProgress.LastSubject, kidProgress.LastPeriod)
			if err != nil || len(list.Cards) == 0 {
				http.NotFound(response, request)
				return
			}
			cards := make([]study.Card, 0, len(list.Cards))
			for _, card := range list.Cards {
				cards = append(cards, study.Card{ID: card.ID, Front: card.Front, Back: card.Back})
			}
			mode, difficulty := requestedSessionOptions(request)
			studySession, err := study.NewBidirectionalSession(study.List{Subject: list.Subject, Period: list.Period, Cards: cards}, mode, difficulty, 0, study.DefaultMixSettings())
			if err != nil {
				http.Error(response, "sessie starten mislukt", http.StatusBadRequest)
				return
			}
			studyCards := make([]StudySessionCard, 0, len(studySession.Queue))
			for _, sessionCard := range studySession.Queue {
				card := list.Cards[0]
				for _, sourceCard := range list.Cards {
					if sourceCard.ID == sessionCard.ID {
						card = sourceCard
						break
					}
				}
				frontLanguage := content.EffectiveSideLanguage(card, list, "a")
				backLanguage := content.EffectiveSideLanguage(card, list, "b")
				_, subjectIsLanguage := content.LanguageCode(list.Subject)
				if frontLanguage == "" && subjectIsLanguage {
					frontLanguage, _ = content.LanguageCode(list.Subject)
				}
				if studySession.PromptSides[card.ID] == "b" {
					card.Front, card.Back = card.Back, card.Front
					frontLanguage, backLanguage = backLanguage, frontLanguage
				}
				studyCard := StudySessionCard{ID: card.ID, Front: card.Front, Back: card.Back, IsLanguageCard: card.IsLanguageCard || subjectIsLanguage || frontLanguage != "" || backLanguage != "", FrontLanguage: frontLanguage, BackLanguage: backLanguage, QuestionMode: studySession.QuestionModes[card.ID]}
				if studyCard.QuestionMode == "multiple_choice" {
					studyCard.Choices = multipleChoiceAnswers(sessionCard, cards, studySession.PromptSides[card.ID])
				}
				studyCards = append(studyCards, studyCard)
			}
			renderStudyCard(response, StudyCard{Subject: list.Subject, Period: list.Period, CSRFToken: session.CSRFToken, Cards: studyCards})
		})
		protected.Post("/student/study/complete", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			cardsSeen, correctFirstTry, totalAttempts, err := studyCompletionFromRequest(request)
			if err != nil {
				http.Error(response, "ongeldige sessiegegevens", http.StatusBadRequest)
				return
			}
			summary := progress.SessionSummary{ID: strconv.FormatInt(time.Now().UnixNano(), 10), Subject: request.FormValue("subject"), Period: request.FormValue("period"), StartedAt: time.Now().UTC(), EndedAt: time.Now().UTC(), Mode: request.FormValue("mode"), CardsSeen: cardsSeen, CorrectFirstTry: correctFirstTry, TotalAttempts: totalAttempts}
			if err := dependencies.Progress.AppendSession(session.KidID, summary); err != nil {
				http.Error(response, "sessie opslaan mislukt", http.StatusInternalServerError)
				return
			}
			response.WriteHeader(http.StatusNoContent)
		}))
		protected.Get("/student/topics/edit", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			list, err := dependencies.Content.LoadList(session.KidID, request.URL.Query().Get("subject"), request.URL.Query().Get("period"))
			if err != nil {
				http.NotFound(response, request)
				return
			}
			renderTopicEditor(response, topicEditorData{Labels: TopicLabelsNL, CSRFToken: session.CSRFToken, List: list})
		})
		protected.Post("/student/topics/import", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			preview, err := content.ParseCSVPreviewForSubject(request.FormValue("data"), request.FormValue("subject"))
			if err != nil {
				http.Error(response, "import lezen mislukt", http.StatusBadRequest)
				return
			}
			writeJSON(response, preview)
		}))
		protected.Post("/student/topics", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			list, err := topicListFromRequest(request)
			if err != nil {
				http.Error(response, "onderwerp opslaan mislukt", http.StatusBadRequest)
				return
			}
			oldList, oldErr := dependencies.Content.LoadList(session.KidID, list.Subject, list.Period)
			if oldErr != nil && !isNotFound(oldErr) {
				http.Error(response, "onderwerp laden mislukt", http.StatusInternalServerError)
				return
			}
			if err := dependencies.Content.SaveList(session.KidID, list); err != nil {
				http.Error(response, "onderwerp opslaan mislukt", http.StatusBadRequest)
				return
			}
			if oldList != nil && dependencies.Progress != nil {
				if err := resetChangedPairs(dependencies.Progress, session.KidID, *oldList, list); err != nil {
					http.Error(response, "voortgang bijwerken mislukt", http.StatusInternalServerError)
					return
				}
			}
			http.Redirect(response, request, "/student/topics/edit?subject="+list.Subject+"&period="+list.Period, http.StatusSeeOther)
		}))
		protected.Post("/student/topics/delete", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			list, err := dependencies.Content.LoadList(session.KidID, request.FormValue("subject"), request.FormValue("period"))
			if err != nil {
				http.Error(response, "onderwerp verwijderen mislukt", http.StatusBadRequest)
				return
			}
			if err := dependencies.Content.DeleteList(session.KidID, request.FormValue("subject"), request.FormValue("period")); err != nil {
				http.Error(response, "onderwerp verwijderen mislukt", http.StatusBadRequest)
				return
			}
			if dependencies.Progress != nil {
				kidProgress, err := dependencies.Progress.Load(session.KidID)
				if err != nil {
					http.Error(response, "voortgang bijwerken mislukt", http.StatusInternalServerError)
					return
				}
				for _, card := range list.Cards {
					review.ResetPairDirections(&kidProgress, card.ID)
				}
				if err := dependencies.Progress.Save(session.KidID, kidProgress); err != nil {
					http.Error(response, "voortgang bijwerken mislukt", http.StatusInternalServerError)
					return
				}
			}
			http.Redirect(response, request, "/student/sessions/new", http.StatusSeeOther)
		}))
		protected.Get("/student/topics/new", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			renderTopicEditor(response, topicEditorData{Labels: TopicLabelsNL, CSRFToken: session.CSRFToken})
		})
		protected.Get("/kid/topics/new", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			renderTopicEditor(response, topicEditorData{Labels: TopicLabelsNL, CSRFToken: session.CSRFToken})
		})
		protected.Get("/kid/sessions/new", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			references, err := dependencies.Content.ListReferences(session.KidID)
			if err != nil {
				http.Error(response, "lijsten laden mislukt", http.StatusInternalServerError)
				return
			}
			kidProgress, err := dependencies.Progress.Load(session.KidID)
			if err != nil {
				http.Error(response, "voortgang laden mislukt", http.StatusInternalServerError)
				return
			}
			selected, _ := study.SelectList(references, kidProgress)
			renderSessionSelection(response, session.CSRFToken, references, selected)
		})
		protected.Post("/kid/sessions/select", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			references, err := dependencies.Content.ListReferences(session.KidID)
			if err != nil {
				http.Error(response, "lijsten laden mislukt", http.StatusInternalServerError)
				return
			}
			valid := false
			for _, reference := range references {
				if reference.Subject == request.FormValue("subject") && reference.Period == request.FormValue("period") {
					valid = true
				}
			}
			if !valid {
				http.Error(response, "ongeldige lijstkeuze", http.StatusBadRequest)
				return
			}
			kidProgress, err := dependencies.Progress.Load(session.KidID)
			if err != nil {
				http.Error(response, "voortgang laden mislukt", http.StatusInternalServerError)
				return
			}
			study.RememberListSelection(&kidProgress, request.FormValue("subject"), request.FormValue("period"))
			if err := dependencies.Progress.Save(session.KidID, kidProgress); err != nil {
				http.Error(response, "lijstkeuze opslaan mislukt", http.StatusInternalServerError)
				return
			}
			http.Redirect(response, request, "/kid/study", http.StatusSeeOther)
		}))
		protected.Get("/kid/study", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			kidProgress, err := dependencies.Progress.Load(session.KidID)
			if err != nil {
				http.Error(response, "voortgang laden mislukt", http.StatusInternalServerError)
				return
			}
			list, err := dependencies.Content.LoadList(session.KidID, kidProgress.LastSubject, kidProgress.LastPeriod)
			if err != nil || len(list.Cards) == 0 {
				http.NotFound(response, request)
				return
			}
			cards := make([]study.Card, 0, len(list.Cards))
			for _, card := range list.Cards {
				cards = append(cards, study.Card{ID: card.ID, Front: card.Front, Back: card.Back})
			}
			studySession, err := study.NewBidirectionalSession(study.List{Subject: list.Subject, Period: list.Period, Cards: cards}, "typed", "ok", 0, study.DefaultMixSettings())
			if err != nil {
				http.Error(response, "sessie starten mislukt", http.StatusBadRequest)
				return
			}
			card := list.Cards[0]
			if studySession.PromptSides[card.ID] == "b" {
				card.Front, card.Back = card.Back, card.Front
				card.SideALanguage, card.SideBLanguage = content.EffectiveSideLanguage(card, list, "b"), content.EffectiveSideLanguage(card, list, "a")
			}
			renderStudyCard(response, StudyCard{Front: card.Front, Back: card.Back, IsLanguageCard: content.EffectiveSideLanguage(card, list, "a") != "" || content.EffectiveSideLanguage(card, list, "b") != "", FrontLanguage: content.EffectiveSideLanguage(card, list, "a"), BackLanguage: content.EffectiveSideLanguage(card, list, "b")})
		})
		protected.Get("/kid/topics/edit", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			list, err := dependencies.Content.LoadList(session.KidID, request.URL.Query().Get("subject"), request.URL.Query().Get("period"))
			if err != nil {
				http.NotFound(response, request)
				return
			}
			renderTopicEditor(response, topicEditorData{Labels: TopicLabelsNL, CSRFToken: session.CSRFToken, List: list})
		})
		protected.Post("/kid/topics", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			list, err := topicListFromRequest(request)
			if err != nil {
				http.Error(response, "onderwerp opslaan mislukt", http.StatusBadRequest)
				return
			}
			oldList, oldErr := dependencies.Content.LoadList(session.KidID, list.Subject, list.Period)
			if oldErr != nil && !isNotFound(oldErr) {
				http.Error(response, "onderwerp laden mislukt", http.StatusInternalServerError)
				return
			}
			if err := dependencies.Content.SaveList(session.KidID, list); err != nil {
				http.Error(response, "onderwerp opslaan mislukt", http.StatusBadRequest)
				return
			}
			if oldList != nil && dependencies.Progress != nil {
				if err := resetChangedPairs(dependencies.Progress, session.KidID, *oldList, list); err != nil {
					http.Error(response, "voortgang bijwerken mislukt", http.StatusInternalServerError)
					return
				}
			}
			http.Redirect(response, request, "/kid/topics/edit?subject="+list.Subject+"&period="+list.Period, http.StatusSeeOther)
		}))
		protected.Post("/kid/topics/import", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			var preview content.ImportPreview
			var err error
			if request.FormValue("format") == "json" {
				preview, err = content.ParseJSONPreviewForSubject(request.FormValue("data"), request.FormValue("subject"))
			} else {
				preview, err = content.ParseCSVPreviewForSubject(request.FormValue("data"), request.FormValue("subject"))
			}
			if err != nil {
				http.Error(response, "import lezen mislukt", http.StatusBadRequest)
				return
			}
			writeJSON(response, preview)
		}))
		protected.Post("/kid/topics/delete", kidCSRFHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request, session auth.KidSession) {
			list, err := dependencies.Content.LoadList(session.KidID, request.FormValue("subject"), request.FormValue("period"))
			if err != nil {
				http.Error(response, "onderwerp verwijderen mislukt", http.StatusBadRequest)
				return
			}
			if err := dependencies.Content.DeleteList(session.KidID, request.FormValue("subject"), request.FormValue("period")); err != nil {
				http.Error(response, "onderwerp verwijderen mislukt", http.StatusBadRequest)
				return
			}
			if dependencies.Progress != nil {
				kidProgress, err := dependencies.Progress.Load(session.KidID)
				if err != nil {
					http.Error(response, "voortgang bijwerken mislukt", http.StatusInternalServerError)
					return
				}
				for _, card := range list.Cards {
					review.ResetPairDirections(&kidProgress, card.ID)
				}
				if err := dependencies.Progress.Save(session.KidID, kidProgress); err != nil {
					http.Error(response, "voortgang bijwerken mislukt", http.StatusInternalServerError)
					return
				}
			}
			response.WriteHeader(http.StatusNoContent)
		}))
	})
	return router
}

func kidCSRFHandler(sessions *auth.KidSessionManager, next func(http.ResponseWriter, *http.Request, auth.KidSession)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		session, ok := sessions.Authenticate(request)
		if !ok || !auth.VerifyKidCSRF(request, session) {
			http.Error(response, "ongeldige formulierbeveiliging", http.StatusForbidden)
			return
		}
		next(response, request, session)
	}
}

func topicListFromRequest(request *http.Request) (content.List, error) {
	if err := request.ParseForm(); err != nil {
		return content.List{}, err
	}
	list := content.List{
		Subject:              strings.TrimSpace(request.FormValue("subject")),
		Period:               strings.TrimSpace(request.FormValue("period")),
		SideADefaultLanguage: request.FormValue("side_a_default_language"),
		SideBDefaultLanguage: request.FormValue("side_b_default_language"),
	}
	if language, isLanguageSubject := content.LanguageCode(list.Subject); isLanguageSubject && list.SideADefaultLanguage == "" {
		list.SideADefaultLanguage = language
	}
	sideAs, sideBs := request.Form["side_a"], request.Form["side_b"]
	ids, sideALanguages, sideBLanguages := request.Form["id"], request.Form["side_a_language"], request.Form["side_b_language"]
	if len(sideAs) != len(sideBs) || list.Subject == "" || list.Period == "" {
		return content.List{}, http.ErrMissingFile
	}
	for index := range sideAs {
		if strings.TrimSpace(sideAs[index]) == "" && strings.TrimSpace(sideBs[index]) == "" {
			continue
		}
		card := content.Card{Front: strings.TrimSpace(sideAs[index]), Back: strings.TrimSpace(sideBs[index])}
		if index < len(ids) {
			card.ID = ids[index]
		}
		if index < len(sideALanguages) {
			card.SideALanguage = sideALanguages[index]
		}
		if index < len(sideBLanguages) {
			card.SideBLanguage = sideBLanguages[index]
		}
		list.Cards = append(list.Cards, card)
	}
	return list, nil
}

func resetChangedPairs(store *progress.Store, kidID string, oldList, newList content.List) error {
	newCards := make(map[string]content.Card, len(newList.Cards))
	for _, card := range newList.Cards {
		newCards[card.ID] = card
	}
	kidProgress, err := store.Load(kidID)
	if err != nil {
		return err
	}
	for _, oldCard := range oldList.Cards {
		newCard, found := newCards[oldCard.ID]
		if !found || oldCard.Front != newCard.Front || oldCard.Back != newCard.Back {
			review.ResetPairDirections(&kidProgress, oldCard.ID)
		}
	}
	return store.Save(kidID, kidProgress)
}

func isNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func requestedSessionOptions(request *http.Request) (string, string) {
	mode := request.FormValue("mode")
	if mode != "typed" && mode != "mixed" {
		mode = "mixed"
	}
	difficulty := request.FormValue("difficulty")
	if difficulty != "easy" && difficulty != "hard" && difficulty != "ok" {
		difficulty = "ok"
	}
	return mode, difficulty
}

func multipleChoiceAnswers(card study.Card, cards []study.Card, promptSide string) []string {
	_, answer := study.PromptAndAnswer(card, promptSide)
	choices := []string{answer}
	seen := map[string]bool{answer: true}
	for _, candidate := range cards {
		_, distractor := study.PromptAndAnswer(candidate, promptSide)
		if candidate.ID == card.ID || seen[distractor] || distractor == "" {
			continue
		}
		choices = append(choices, distractor)
		seen[distractor] = true
		if len(choices) == 4 {
			break
		}
	}
	rand.Shuffle(len(choices), func(left, right int) { choices[left], choices[right] = choices[right], choices[left] })
	return choices
}

func studyCompletionFromRequest(request *http.Request) (int, int, int, error) {
	cardsSeen, err := strconv.Atoi(request.FormValue("cards_seen"))
	if err != nil || cardsSeen < 1 {
		return 0, 0, 0, http.ErrNotSupported
	}
	correctFirstTry, err := strconv.Atoi(request.FormValue("correct_first_try"))
	if err != nil || correctFirstTry < 0 || correctFirstTry > cardsSeen {
		return 0, 0, 0, http.ErrNotSupported
	}
	totalAttempts, err := strconv.Atoi(request.FormValue("total_attempts"))
	if err != nil || totalAttempts < cardsSeen {
		return 0, 0, 0, http.ErrNotSupported
	}
	return cardsSeen, correctFirstTry, totalAttempts, nil
}
