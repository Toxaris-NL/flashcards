package config

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestLoadReadsTOMLAndEnvironmentOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flashcards.toml")
	hash, err := bcrypt.GenerateFromPassword([]byte("toml-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	contents := "[server]\nlisten_addr = ':9090'\n[data]\ndir = '/data'\n[admin]\nusername = 'sander'\npassword_hash = '" + string(hash) + "'\nsession_secret = 'toml-secret'\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	environmentHash, err := bcrypt.GenerateFromPassword([]byte("environment-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate environment hash: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD_HASH", string(environmentHash))

	config, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.ListenAddr != ":9090" || config.DataDir != "/data" || config.AdminUsername != "sander" || config.AdminPasswordHash != string(environmentHash) || config.SessionSecret != "toml-secret" {
		t.Fatalf("config = %#v", config)
	}
}

func TestLoadRejectsMissingRequiredValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flashcards.toml")
	if err := os.WriteFile(path, []byte("[server]\nlisten_addr = ':8080'\n[data]\ndir = '/data'\n[admin]\nusername = 'sander'\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected incomplete config to fail")
	}
}

func TestLoadRejectsPlaintextAdminPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flashcards.toml")
	contents := "[server]\nlisten_addr = ':8080'\n[data]\ndir = '/data'\n[admin]\nusername = 'sander'\npassword = 'plaintext'\nsession_secret = 'secret'\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected plaintext password configuration to fail")
	}
}

func TestLoadCreatesDefaultConfigWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flashcards.toml")
	config, err := Load(path)
	if err != nil {
		t.Fatalf("load config with missing file: %v", err)
	}
	if config.ListenAddr != ":8080" {
		t.Fatalf("listen addr = %q, want :8080", config.ListenAddr)
	}
	if config.DataDir != "data" {
		t.Fatalf("data dir = %q, want data", config.DataDir)
	}
	if config.AdminUsername == "" || config.AdminPasswordHash == "" || config.SessionSecret == "" {
		t.Fatal("expected generated default admin credentials to be populated")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(config.AdminPasswordHash), []byte("example-password")); err != nil {
		t.Fatalf("expected generated default password hash to match example-password: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected default config file to be created at %s: %v", path, err)
	}
}

func TestUpdateAdminPasswordWritesUsableHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flashcards.toml")
	if _, err := Load(path); err != nil {
		t.Fatalf("create config: %v", err)
	}
	hash, err := UpdateAdminPassword(path, "new-password")
	if err != nil {
		t.Fatalf("update password: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("new-password")); err != nil {
		t.Fatalf("new hash does not match password: %v", err)
	}
	updated, err := Load(path)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(updated.AdminPasswordHash), []byte("new-password")) != nil {
		t.Fatalf("updated config = %#v, err = %v", updated, err)
	}
}
