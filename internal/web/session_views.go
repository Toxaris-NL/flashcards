package web

import (
	"html/template"
	"net/http"

	"flashcards/internal/content"
)

var sessionSelectionTemplate = template.Must(template.ParseFS(staticFiles, "static/session_selection.html"))

func renderSessionSelection(response http.ResponseWriter, csrfToken string, references []content.ListReference, selected content.ListReference) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		CSRFToken  string
		References []content.ListReference
		Selected   content.ListReference
	}{CSRFToken: csrfToken, References: references, Selected: selected}
	if err := sessionSelectionTemplate.Execute(response, data); err != nil {
		http.Error(response, "sessiekeuze laden mislukt", http.StatusInternalServerError)
	}
}
