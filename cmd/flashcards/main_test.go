package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"flashcards/internal/config"

	"golang.org/x/crypto/bcrypt"
)

func TestApplicationHandlerServesConfiguredAdminAndStudyRoutes(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	handler, err := applicationHandler(config.Config{ListenAddr: ":8080", DataDir: t.TempDir(), AdminUsername: "sander", AdminPasswordHash: string(hash), SessionSecret: "session-secret"}, t.TempDir())
	if err != nil {
		t.Fatalf("application handler: %v", err)
	}
	for _, testCase := range []struct {
		path string
		want int
	}{
		{path: "/", want: http.StatusOK},
		{path: "/study", want: http.StatusOK},
		{path: "/admin/login", want: http.StatusOK},
		{path: "/admin", want: http.StatusForbidden},
		{path: "/kid/signup", want: http.StatusOK},
		{path: "/kid/topics/new", want: http.StatusForbidden},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, testCase.path, nil))
		if response.Code != testCase.want {
			t.Fatalf("%s status = %d, want %d", testCase.path, response.Code, testCase.want)
		}
	}
}

func TestExampleConfigurationLoads(t *testing.T) {
	path := filepath.Join("..", "..", "flashcards.example.toml")
	if _, err := config.Load(path); err != nil {
		t.Fatalf("example configuration must load: %v", err)
	}
}

func TestMountedStyleConfigurationConstructsApplication(t *testing.T) {
	dataDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "flashcards.toml")
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	contents := "[server]\nlisten_addr = ':8080'\n[data]\ndir = '" + dataDir + "'\n[admin]\nusername = 'sander'\npassword_hash = '" + string(hash) + "'\nsession_secret = 'session-secret'\n"
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write mounted configuration: %v", err)
	}
	configuration, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load mounted configuration: %v", err)
	}
	handler, err := applicationHandler(configuration, configuration.DataDir)
	if err != nil {
		t.Fatalf("construct application: %v", err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/study", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("study status = %d, want 200", response.Code)
	}
	if err := os.WriteFile(filepath.Join(configuration.DataDir, "write-check"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("configured data directory must be writable: %v", err)
	}
}
