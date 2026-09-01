// Package web serves the minimal static study-card surface.
package web

import (
	"embed"
	"html/template"
	"io"
	"mime"
	"net/http"
	"path"

	"github.com/go-chi/chi/v5"

	"flashcards/internal/content"
)

//go:embed static/*
var staticFiles embed.FS

var studyTemplate = template.Must(template.ParseFS(staticFiles, "static/study.html"))

// StudyCard is the card content rendered by the study page.
type StudyCard struct {
	Subject        string
	Period         string
	CSRFToken      string
	Front          string
	Back           string
	IsLanguageCard bool
	LanguageCode   string
	FrontLanguage  string
	BackLanguage   string
	Cards          []StudySessionCard
}

// StudySessionCard is one prompt and answer pair presented during a study session.
type StudySessionCard struct {
	ID             string
	Front          string
	Back           string
	IsLanguageCard bool
	FrontLanguage  string
	BackLanguage   string
	QuestionMode   string
	Choices        []string
}

// NewStudyHandler serves a reusable study-card page and its static assets.
func NewStudyHandler(card StudyCard) http.Handler {
	if card.LanguageCode == "" {
		card.LanguageCode, _ = content.LanguageCode(card.Subject)
	}
	if card.FrontLanguage == "" {
		card.FrontLanguage = card.LanguageCode
	}
	if card.BackLanguage == "" {
		card.BackLanguage = card.LanguageCode
	}
	router := chi.NewRouter()
	router.Get("/study", func(response http.ResponseWriter, request *http.Request) {
		renderStudyCard(response, card)
	})
	router.Get("/static/{file}", func(response http.ResponseWriter, request *http.Request) {
		filename := chi.URLParam(request, "file")
		file, err := staticFiles.Open("static/" + filename)
		if err != nil {
			http.NotFound(response, request)
			return
		}
		defer file.Close()
		if contentType := mime.TypeByExtension(path.Ext(filename)); contentType != "" {
			response.Header().Set("Content-Type", contentType)
		}
		response.Header().Set("Cache-Control", "no-cache")
		_, _ = io.Copy(response, file)
	})
	return router
}

func renderStudyCard(response http.ResponseWriter, card StudyCard) {
	if len(card.Cards) == 0 {
		card.Cards = []StudySessionCard{{
			Front:          card.Front,
			Back:           card.Back,
			IsLanguageCard: card.IsLanguageCard,
			FrontLanguage:  card.FrontLanguage,
			BackLanguage:   card.BackLanguage,
		}}
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := studyTemplate.Execute(response, card); err != nil {
		http.Error(response, "kan studiekaart niet tonen", http.StatusInternalServerError)
	}
}
