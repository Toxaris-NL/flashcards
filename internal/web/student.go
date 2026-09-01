package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"flashcards/internal/auth"
)

var studentLandingTemplate = template.Must(template.ParseFS(staticFiles, "static/student_landing.html"))
var studentLoginTemplate = template.Must(template.ParseFS(staticFiles, "static/student_login.html"))
var studentSignupTemplate = template.Must(template.ParseFS(staticFiles, "static/student_signup.html"))
var studentPINTemplate = template.Must(template.ParseFS(staticFiles, "static/student_pin.html"))

// NewStudentHandler provides student login and session-protected routes.
func NewStudentHandler(sessions *auth.StudentSessionManager) http.Handler {
	router := chi.NewRouter()
	router.Get("/", func(response http.ResponseWriter, request *http.Request) {
		renderStudentTemplate(response, studentLandingTemplate, struct {
			Error   string
			Success string
		}{Error: request.URL.Query().Get("error"), Success: request.URL.Query().Get("success")})
	})
	router.Get("/student/login", func(response http.ResponseWriter, request *http.Request) {
		renderStudentTemplate(response, studentLoginTemplate, struct {
			Error   string
			Success string
		}{Error: request.URL.Query().Get("error"), Success: request.URL.Query().Get("success")})
	})
	router.Get("/kid/login", func(response http.ResponseWriter, request *http.Request) {
		renderStudentTemplate(response, studentLoginTemplate, struct {
			Error   string
			Success string
		}{Error: request.URL.Query().Get("error"), Success: request.URL.Query().Get("success")})
	})
	router.Post("/student/login", func(response http.ResponseWriter, request *http.Request) {
		cookie, session, err := sessions.Login(request.FormValue("username"), request.FormValue("pin"))
		if err != nil {
			http.Redirect(response, request, "/student/login?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		http.SetCookie(response, cookie)
		if sessions.MustChangePIN(session.KidID) {
			http.Redirect(response, request, "/student/pin", http.StatusSeeOther)
			return
		}
		http.Redirect(response, request, "/student/sessions/new", http.StatusSeeOther)
	})
	router.Post("/kid/login", func(response http.ResponseWriter, request *http.Request) {
		cookie, session, err := sessions.Login(request.FormValue("username"), request.FormValue("pin"))
		if err != nil {
			http.Redirect(response, request, "/student/login?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		http.SetCookie(response, cookie)
		if sessions.MustChangePIN(session.KidID) {
			http.Redirect(response, request, "/student/pin", http.StatusSeeOther)
			return
		}
		http.Redirect(response, request, "/kid/sessions/new", http.StatusSeeOther)
	})
	router.Get("/student/signup", func(response http.ResponseWriter, request *http.Request) {
		renderStudentTemplate(response, studentSignupTemplate, struct {
			Error   string
			Success string
		}{Error: request.URL.Query().Get("error"), Success: request.URL.Query().Get("success")})
	})
	router.Get("/kid/signup", func(response http.ResponseWriter, request *http.Request) {
		renderStudentTemplate(response, studentSignupTemplate, struct {
			Error   string
			Success string
		}{Error: request.URL.Query().Get("error"), Success: request.URL.Query().Get("success")})
	})
	router.Post("/student/signup", func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			http.Redirect(response, request, "/student/signup?error="+url.QueryEscape("formulier is ongeldig"), http.StatusSeeOther)
			return
		}
		student, err := sessions.Signup(request.FormValue("username"), request.FormValue("pin"))
		if err != nil {
			http.Redirect(response, request, "/student/signup?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		_ = student
		http.Redirect(response, request, "/student/login?success="+url.QueryEscape("Aanvraag ontvangen. Wacht op goedkeuring."), http.StatusSeeOther)
	})
	router.Post("/kid/signup", func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			http.Redirect(response, request, "/student/signup?error="+url.QueryEscape("formulier is ongeldig"), http.StatusSeeOther)
			return
		}
		student, err := sessions.Signup(request.FormValue("username"), request.FormValue("pin"))
		if err != nil {
			http.Redirect(response, request, "/student/signup?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		_ = student
		http.Redirect(response, request, "/student/login?success="+url.QueryEscape("Aanvraag ontvangen. Wacht op goedkeuring."), http.StatusSeeOther)
	})
	router.Group(func(protected chi.Router) {
		protected.Use(requireKid(sessions))
		protected.Get("/student/pin", func(response http.ResponseWriter, request *http.Request) {
			renderStudentTemplate(response, studentPINTemplate, struct{ Error string }{Error: request.URL.Query().Get("error")})
		})
		protected.Post("/student/pin", func(response http.ResponseWriter, request *http.Request) {
			session, _ := sessions.Authenticate(request)
			if err := sessions.ChangePIN(session.KidID, request.FormValue("temporary_pin"), request.FormValue("new_pin")); err != nil {
				http.Redirect(response, request, "/student/pin?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
				return
			}
			http.Redirect(response, request, "/student/sessions/new", http.StatusSeeOther)
		})
		protected.Get("/student/me", func(response http.ResponseWriter, request *http.Request) {
			session, _ := sessions.Authenticate(request)
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]string{"kid_id": session.KidID})
		})
		protected.Get("/kid/me", func(response http.ResponseWriter, request *http.Request) {
			session, _ := sessions.Authenticate(request)
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]string{"kid_id": session.KidID})
		})
	})
	return router
}

func renderStudentTemplate(response http.ResponseWriter, page *template.Template, data any) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(response, data); err != nil {
		http.Error(response, "studentpagina laden mislukt", http.StatusInternalServerError)
	}
}

func requireStudent(sessions *auth.StudentSessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if _, ok := sessions.Authenticate(request); !ok {
				http.Error(response, "kindtoegang vereist", http.StatusForbidden)
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func requireKid(sessions *auth.KidSessionManager) func(http.Handler) http.Handler {
	return requireStudent(sessions)
}
