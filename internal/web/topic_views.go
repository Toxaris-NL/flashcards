package web

import (
	"html/template"
	"net/http"

	"flashcards/internal/content"
)

var topicEditorTemplate = template.Must(template.ParseFS(staticFiles, "static/topic_editor.html"))

// TopicLabelsNL contains kid-facing topic editor text.
var TopicLabelsNL = map[string]string{
	"add_pair":     "Paar toevoegen",
	"delete_list":  "Lijst verwijderen",
	"edit_topic":   "Onderwerp bewerken",
	"new_topic":    "Nieuw onderwerp",
	"period":       "Periode",
	"save":         "Opslaan",
	"side_a":       "Zijde A",
	"side_b":       "Zijde B",
	"subject":      "Vak",
	"topic_editor": "Flashcard-editor",
}

type topicEditorData struct {
	Labels    map[string]string
	CSRFToken string
	List      *content.List
}

func renderTopicEditor(response http.ResponseWriter, data topicEditorData) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := topicEditorTemplate.Execute(response, data); err != nil {
		http.Error(response, "editor laden mislukt", http.StatusInternalServerError)
	}
}
