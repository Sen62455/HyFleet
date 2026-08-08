package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Server struct {
	Listen             string
	PublicURL          string
	DatabasePath       string
	MasterKeyFile      string
	BootstrapToken     string
	CookieSecure       bool
	SessionLifetime    time.Duration
	SessionIdleTimeout time.Duration
	StaleAfter         time.Duration
	OfflineAfter       time.Duration
}

type serverFile struct {
	Listen             string `yaml:"listen"`
	PublicURL          string `yaml:"public_url"`
	DatabasePath       string `yaml:"database_path"`
	MasterKeyFile      string `yaml:"master_key_file"`
	CookieSecure       *bool  `yaml:"cookie_secure"`
	SessionLifetime    string `yaml:"session_lifetime"`
	SessionIdleTimeout string `yaml:"session_idle_timeout"`
	StaleAfter         string `yaml:"stale_after"`
	OfflineAfter       string `yaml:"offline_after"`
}

func LoadServer(path string) (Server, error) {
	file := serverFile{
		Listen:             "127.0.0.1:8080",
		PublicURL:          "http://127.0.0.1:8080",
		DatabasePath:       "data/server.db",
		MasterKeyFile:      "data/master.key",
		SessionLifetime:    "12h",
		SessionIdleTimeout: "30m",
		StaleAfter:         "45s",
		OfflineAfter:       "90s",
	}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Server{}, fmt.Errorf("read server config: %w", err)
		}
		if err := decodeYAML(data, &file); err != nil {
			return Server{}, fmt.Errorf("parse server config: %w", err)
		}
	}
	if value := os.Getenv("HYFLEET_LISTEN"); value != "" {
		file.Listen = value
	}
	if value := os.Getenv("HYFLEET_PUBLIC_URL"); value != "" {
		file.PublicURL = value
	}
	if value := os.Getenv("HYFLEET_DATABASE_PATH"); value != "" {
		file.DatabasePath = value
	}
	if value := os.Getenv("HYFLEET_MASTER_KEY_FILE"); value != "" {
		file.MasterKeyFile = value
	}
	publicURL, err := url.Parse(file.PublicURL)
	if err != nil || publicURL.Scheme == "" || publicURL.Host == "" {
		return Server{}, errors.New("public_url must be an absolute http or https URL")
	}
	if publicURL.Scheme != "http" && publicURL.Scheme != "https" {
		return Server{}, errors.New("public_url scheme must be http or https")
	}
	if publicURL.User != nil || (publicURL.Path != "" && publicURL.Path != "/") ||
		publicURL.RawQuery != "" || publicURL.Fragment != "" {
		return Server{}, errors.New("public_url must be an origin without credentials, path, query, or fragment")
	}
	cookieSecure := publicURL.Scheme == "https"
	if file.CookieSecure != nil {
		cookieSecure = *file.CookieSecure
	}
	if publicURL.Scheme == "https" && !cookieSecure {
		return Server{}, errors.New("cookie_secure cannot be false for an https public_url")
	}
	lifetime, err := parseDuration("session_lifetime", file.SessionLifetime, 5*time.Minute, 7*24*time.Hour)
	if err != nil {
		return Server{}, err
	}
	idle, err := parseDuration("session_idle_timeout", file.SessionIdleTimeout, time.Minute, lifetime)
	if err != nil {
		return Server{}, err
	}
	stale, err := parseDuration("stale_after", file.StaleAfter, 15*time.Second, 30*time.Minute)
	if err != nil {
		return Server{}, err
	}
	offline, err := parseDuration("offline_after", file.OfflineAfter, stale, time.Hour)
	if err != nil {
		return Server{}, err
	}
	base := "."
	if path != "" {
		base = filepath.Dir(path)
	}
	return Server{
		Listen:             file.Listen,
		PublicURL:          publicURL.String(),
		DatabasePath:       resolvePath(base, file.DatabasePath),
		MasterKeyFile:      resolvePath(base, file.MasterKeyFile),
		BootstrapToken:     os.Getenv("HYFLEET_BOOTSTRAP_TOKEN"),
		CookieSecure:       cookieSecure,
		SessionLifetime:    lifetime,
		SessionIdleTimeout: idle,
		StaleAfter:         stale,
		OfflineAfter:       offline,
	}, nil
}

func parseDuration(name, value string, minimum, maximum time.Duration) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if duration < minimum || duration > maximum {
		return 0, fmt.Errorf("%s must be between %s and %s", name, minimum, maximum)
	}
	return duration, nil
}

func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}
