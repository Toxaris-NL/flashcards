package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"flashcards/internal/admin"
	"flashcards/internal/auth"
	"flashcards/internal/config"
	"flashcards/internal/content"
	"flashcards/internal/progress"
	"flashcards/internal/web"
)

func main() {
	configuration, err := config.Load(configPath())
	if err != nil {
		slog.Error("configuratie laden mislukt", "error", err)
		os.Exit(1)
	}
	handler, err := applicationHandler(configuration, configuration.DataDir)
	if err != nil {
		slog.Error("toepassing starten mislukt", "error", err)
		os.Exit(1)
	}
	slog.Info("flashcards server started", "address", configuration.ListenAddr)
	if err := http.ListenAndServe(configuration.ListenAddr, handler); err != nil {
		slog.Error("flashcards server stopped", "error", err)
	}
}

func applicationHandler(configuration config.Config, dataDir string) (http.Handler, error) {
	authStore, err := auth.NewStore(filepath.Join(dataDir, "kids.json"))
	if err != nil {
		return nil, err
	}
	settingsStore := admin.NewSettingsStore(filepath.Join(dataDir, "admin-settings.json"))
	adminHandler := web.NewAdminManagementHandler(web.AdminDependencies{
		Sessions:          admin.NewSessionManager(configuration),
		AuthService:       auth.NewService(authStore, nil),
		AuthStore:         authStore,
		ProgressDashboard: admin.NewProgressDashboardService(progress.NewStore(filepath.Join(dataDir, "progress"))),
		SettingsStore:     settingsStore,
		SettingsService:   admin.NewService(settingsStore),
	})
	studyHandler := web.NewStudyHandler(web.StudyCard{Subject: "Frans", Front: "bonjour", Back: "hallo", IsLanguageCard: true})
	studentSessions := auth.NewStudentSessionManager(auth.NewService(authStore, nil), configuration.SessionSecret)
	topicHandler := web.NewTopicHandler(web.TopicDependencies{
		Sessions: studentSessions,
		Content:  content.NewService(filepath.Join(dataDir, "content")),
		Progress: progress.NewStore(filepath.Join(dataDir, "progress")),
	})
	studentHandler := web.NewStudentHandler(studentSessions)
	router := chi.NewRouter()
	router.Handle("/", studentHandler)
	router.Handle("/study", studyHandler)
	router.Handle("/static/*", studyHandler)
	router.Handle("/admin", adminHandler)
	router.Handle("/admin/*", adminHandler)
	router.Handle("/student/login", studentHandler)
	router.Handle("/student/signup", studentHandler)
	router.Handle("/student/*", topicHandler)
	router.Handle("/kid/login", studentHandler)
	router.Handle("/kid/signup", studentHandler)
	router.Handle("/kid/*", topicHandler)
	return router, nil
}

func configPath() string {
	if path := os.Getenv("FLASHCARDS_CONFIG"); path != "" {
		return path
	}
	return "flashcards.toml"
}
