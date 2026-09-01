package web

import (
	"html/template"
	"net/http"

	adminservice "flashcards/internal/admin"
)

var adminLoginTemplate = template.Must(template.ParseFS(staticFiles, "static/admin_login.html"))
var adminDashboardTemplate = template.Must(template.ParseFS(staticFiles, "static/admin_dashboard.html"))

// AdminLabelsNL contains all user-facing text for the admin views.
var AdminLabelsNL = map[string]string{
	"admin":          "Beheer",
	"approve":        "Goedkeuren",
	"create_student": "Student toevoegen",
	"disable":        "Uitschakelen",
	"enable":         "Inschakelen",
	"error":          "Er ging iets mis",
	"easy":           "Makkelijk",
	"hard":           "Moeilijk",
	"students":       "Studenten",
	"login":          "Aanmelden",
	"password":       "Wachtwoord",
	"pin":            "Pincode",
	"progress":       "Voortgang",
	"reject":         "Afwijzen",
	"save":           "Opslaan",
	"settings":       "Instellingen",
	"username":       "Gebruikersnaam",
}

type adminDashboardData struct {
	Labels    map[string]string
	CSRFToken string
	Error     string
	Kids      []kidView
	Progress  []adminservice.KidProgressSummary
	Settings  adminservice.MixedSessionSettings
}

func renderAdminLogin(response http.ResponseWriter, request *http.Request) {
	renderAdminTemplate(response, adminLoginTemplate, struct {
		Labels map[string]string
		Error  string
	}{Labels: AdminLabelsNL, Error: request.URL.Query().Get("error")})
}

func renderAdminDashboard(response http.ResponseWriter, request *http.Request, dependencies AdminDependencies, session adminservice.Session) {
	settings, err := dependencies.SettingsStore.Load()
	if err != nil {
		http.Error(response, "instellingen laden mislukt", http.StatusInternalServerError)
		return
	}
	progress, err := dependencies.ProgressDashboard.Summaries(true, dependencies.AuthStore.List())
	if err != nil {
		http.Error(response, "voortgang laden mislukt", http.StatusInternalServerError)
		return
	}
	renderAdminTemplate(response, adminDashboardTemplate, adminDashboardData{
		Labels: AdminLabelsNL, CSRFToken: session.CSRFToken, Error: request.URL.Query().Get("error"), Kids: kidViews(dependencies.AuthStore.List()), Progress: progress, Settings: settings,
	})
}

func renderAdminTemplate(response http.ResponseWriter, page *template.Template, data any) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(response, data); err != nil {
		http.Error(response, "beheerpagina laden mislukt", http.StatusInternalServerError)
	}
}
