// Package config loads application configuration from TOML and environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/crypto/bcrypt"
)

// Config contains the credentials and signing secret required by the application.
type Config struct {
	ListenAddr        string
	DataDir           string
	AdminUsername     string
	AdminPasswordHash string
	SessionSecret     string
	ConfigPath        string
}

type fileConfig struct {
	Server struct {
		ListenAddr string `toml:"listen_addr"`
	} `toml:"server"`
	Admin struct {
		Username      string `toml:"username"`
		PasswordHash  string `toml:"password_hash"`
		Password      string `toml:"password"`
		SessionSecret string `toml:"session_secret"`
	} `toml:"admin"`
	Data struct {
		Dir string `toml:"dir"`
	} `toml:"data"`
}

// Load reads TOML configuration and applies environment overrides.
func Load(path string) (Config, error) {
	var file fileConfig
	if path != "" {
		if err := ensureDefaultConfig(path); err != nil {
			return Config{}, err
		}
		if _, err := toml.DecodeFile(path, &file); err != nil {
			return Config{}, err
		}
	}
	if file.Admin.Password != "" {
		return Config{}, errors.New("admin.password is not supported; use admin.password_hash")
	}
	config := Config{
		ListenAddr:        environmentValue("SERVER_LISTEN_ADDR", file.Server.ListenAddr),
		DataDir:           environmentValue("DATA_DIR", file.Data.Dir),
		AdminUsername:     environmentValue("ADMIN_USERNAME", file.Admin.Username),
		AdminPasswordHash: environmentValue("ADMIN_PASSWORD_HASH", file.Admin.PasswordHash),
		SessionSecret:     environmentValue("SESSION_SECRET", file.Admin.SessionSecret),
		ConfigPath:        path,
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

// UpdateAdminPassword stores a bcrypt hash for a new admin password in TOML.
func UpdateAdminPassword(path, password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("admin password must be at least 8 characters")
	}
	if path == "" {
		return "", errors.New("admin config path is required")
	}
	var file fileConfig
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	file.Admin.PasswordHash = string(hash)
	output, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer output.Close()
	if err := toml.NewEncoder(output).Encode(file); err != nil {
		return "", err
	}
	return string(hash), nil
}

func ensureDefaultConfig(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	contents := fmt.Sprintf("# Generated default config. Update the admin password hash and session secret before using this in production.\n[server]\nlisten_addr = \"%s\"\n\n[admin]\nusername = \"%s\"\n# Replace with a bcrypt hash for the admin password.\npassword_hash = \"%s\"\nsession_secret = \"%s\"\n\n[data]\ndir = \"%s\"\n",
		":8080",
		"admin",
		"$2a$10$N0jldfHlXh4e9GUVBZ87behz9gyqu6Pi6W5kMfhx8p001jTQoKA0q",
		"replace-with-a-long-random-secret",
		"data",
	)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return err
	}
	return nil
}

// Validate rejects incomplete admin authentication configuration.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return errors.New("server.listen_addr is required")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return errors.New("data.dir is required")
	}
	if strings.TrimSpace(c.AdminUsername) == "" {
		return errors.New("admin.username is required")
	}
	if _, err := bcrypt.Cost([]byte(c.AdminPasswordHash)); err != nil {
		return errors.New("admin.password_hash must be a valid bcrypt hash")
	}
	if c.SessionSecret == "" {
		return errors.New("admin.session_secret is required")
	}
	return nil
}

func environmentValue(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
