package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	adminservice "flashcards/internal/admin"
	"flashcards/internal/auth"
)

// NewAdminHandler provides login and authenticated admin routes.
func NewAdminHandler(sessions *adminservice.SessionManager) http.Handler {
	router := chi.NewRouter()
	router.Post("/admin/login", func(response http.ResponseWriter, request *http.Request) {
		username := request.FormValue("username")
		password := request.FormValue("password")
		cookie, _, ok := sessions.Login(username, password)
		if !ok {
			http.Error(response, "aanmelden mislukt", http.StatusForbidden)
			return
		}
		http.SetCookie(response, cookie)
		http.Redirect(response, request, "/admin", http.StatusSeeOther)
	})
	router.Group(func(protected chi.Router) {
		protected.Use(requireAdmin(sessions))
		protected.Get("/admin", func(response http.ResponseWriter, request *http.Request) {
			response.WriteHeader(http.StatusOK)
		})
	})
	return router
}

// AdminDependencies supplies the services used by protected management routes.
type AdminDependencies struct {
	Sessions          *adminservice.SessionManager
	AuthService       *auth.Service
	AuthStore         *auth.Store
	ProgressDashboard *adminservice.ProgressDashboardService
	SettingsStore     *adminservice.SettingsStore
	SettingsService   *adminservice.Service
}

// NewAdminManagementHandler adds authenticated kid-management routes.
func NewAdminManagementHandler(dependencies AdminDependencies) http.Handler {
	router := chi.NewRouter()
	router.Get("/admin/login", renderAdminLogin)
	router.Post("/admin/login", loginHandler(dependencies.Sessions))
	router.Group(func(protected chi.Router) {
		protected.Use(requireAdmin(dependencies.Sessions))
		protected.Get("/admin", func(response http.ResponseWriter, request *http.Request) {
			session, _ := dependencies.Sessions.Authenticate(request)
			renderAdminDashboard(response, request, dependencies, session)
		})
		protected.Get("/admin/kids", func(response http.ResponseWriter, request *http.Request) {
			writeJSON(response, kidViews(dependencies.AuthStore.List()))
		})
		protected.Post("/admin/kids", csrfHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request) {
			kid, err := dependencies.AuthService.CreateApprovedKid(request.FormValue("username"), request.FormValue("pin"))
			if err != nil {
				adminFailure(response, request, "/admin", "Student aanmaken mislukt", http.StatusBadRequest)
				return
			}
			if wantsHTML(request) {
				http.Redirect(response, request, "/admin", http.StatusSeeOther)
				return
			}
			writeJSON(response, toKidView(*kid))
		}))
		protected.Post("/admin/kids/{id}/approve", csrfHandler(dependencies.Sessions, kidAction(dependencies.AuthService.Approve)))
		protected.Post("/admin/kids/{id}/reject", csrfHandler(dependencies.Sessions, kidAction(dependencies.AuthService.Reject)))
		protected.Post("/admin/kids/{id}/disable", csrfHandler(dependencies.Sessions, kidAction(dependencies.AuthService.Disable)))
		protected.Post("/admin/kids/{id}/enable", csrfHandler(dependencies.Sessions, kidAction(dependencies.AuthService.Enable)))
		protected.Post("/admin/kids/{id}/reset-pin", csrfHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request) {
			if err := dependencies.AuthService.ResetPIN(chi.URLParam(request, "id"), request.FormValue("temporary_pin")); err != nil {
				adminFailure(response, request, "/admin", "Pincode resetten mislukt", http.StatusBadRequest)
				return
			}
			http.Redirect(response, request, "/admin", http.StatusSeeOther)
		}))
		protected.Post("/admin/password", csrfHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request) {
			if err := dependencies.Sessions.ChangePassword(request.FormValue("current_password"), request.FormValue("new_password")); err != nil {
				adminFailure(response, request, "/admin", "Wachtwoord wijzigen mislukt", http.StatusBadRequest)
				return
			}
			http.Redirect(response, request, "/admin", http.StatusSeeOther)
		}))
		protected.Get("/admin/progress", func(response http.ResponseWriter, request *http.Request) {
			summaries, err := dependencies.ProgressDashboard.Summaries(true, dependencies.AuthStore.List())
			if err != nil {
				http.Error(response, "voortgang laden mislukt", http.StatusInternalServerError)
				return
			}
			writeJSON(response, summaries)
		})
		protected.Get("/admin/settings", func(response http.ResponseWriter, request *http.Request) {
			settings, err := dependencies.SettingsStore.Load()
			if err != nil {
				http.Error(response, "instellingen laden mislukt", http.StatusInternalServerError)
				return
			}
			writeJSON(response, settings)
		})
		protected.Post("/admin/settings", csrfHandler(dependencies.Sessions, func(response http.ResponseWriter, request *http.Request) {
			settings, err := settingsFromRequest(request)
			if err != nil {
				adminFailure(response, request, "/admin", "Ongeldige instellingen", http.StatusBadRequest)
				return
			}
			if err := dependencies.SettingsService.UpdateMixedSessionSettings(true, settings); err != nil {
				adminFailure(response, request, "/admin", "Instellingen opslaan mislukt", http.StatusBadRequest)
				return
			}
			if wantsHTML(request) {
				http.Redirect(response, request, "/admin", http.StatusSeeOther)
				return
			}
			response.WriteHeader(http.StatusNoContent)
		}))
	})
	return router
}

type kidView struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Status   string `json:"status"`
}

func loginHandler(sessions *adminservice.SessionManager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		cookie, _, ok := sessions.Login(request.FormValue("username"), request.FormValue("password"))
		if !ok {
			adminFailure(response, request, "/admin/login", "Aanmelden mislukt", http.StatusForbidden)
			return
		}
		http.SetCookie(response, cookie)
		http.Redirect(response, request, "/admin", http.StatusSeeOther)
	}
}

func csrfHandler(sessions *adminservice.SessionManager, next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		session, ok := sessions.Authenticate(request)
		if !ok || !adminservice.VerifyCSRF(request, session) {
			adminFailure(response, request, "/admin", "Ongeldige formulierbeveiliging", http.StatusForbidden)
			return
		}
		next(response, request)
	}
}

func kidAction(action func(string) error) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if err := action(chi.URLParam(request, "id")); err != nil {
			adminFailure(response, request, "/admin", "Kind bijwerken mislukt", http.StatusBadRequest)
			return
		}
		if wantsHTML(request) {
			http.Redirect(response, request, "/admin", http.StatusSeeOther)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	}
}

func kidViews(kids []auth.Kid) []kidView {
	views := make([]kidView, 0, len(kids))
	for _, kid := range kids {
		views = append(views, toKidView(kid))
	}
	return views
}

func toKidView(kid auth.Kid) kidView {
	return kidView{ID: kid.ID, Username: kid.Username, Status: kid.Status}
}

func writeJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(value)
}

func settingsFromRequest(request *http.Request) (adminservice.MixedSessionSettings, error) {
	easy, err := strconv.Atoi(request.FormValue("easy"))
	if err != nil {
		return adminservice.MixedSessionSettings{}, err
	}
	ok, err := strconv.Atoi(request.FormValue("ok"))
	if err != nil {
		return adminservice.MixedSessionSettings{}, err
	}
	hard, err := strconv.Atoi(request.FormValue("hard"))
	if err != nil {
		return adminservice.MixedSessionSettings{}, err
	}
	return adminservice.MixedSessionSettings{Easy: easy, OK: ok, Hard: hard}, nil
}

func wantsHTML(request *http.Request) bool {
	return strings.Contains(request.Header.Get("Accept"), "text/html")
}

func adminFailure(response http.ResponseWriter, request *http.Request, target, message string, status int) {
	if wantsHTML(request) {
		http.Redirect(response, request, target+"?error="+url.QueryEscape(message), http.StatusSeeOther)
		return
	}
	http.Error(response, strings.ToLower(message), status)
}

func requireAdmin(sessions *adminservice.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if _, ok := sessions.Authenticate(request); !ok {
				http.Error(response, "beheerderstoegang vereist", http.StatusForbidden)
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}
