package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var systemdUnitPattern = regexp.MustCompile(`^[A-Za-z0-9_.@:-]+$`)

type Agent struct {
	ServerURL       string
	EnrollmentToken string
	StatePath       string
	NodeName        string
	AdapterType     string
	CoreName        string
	ServiceUnit     string
	HeartbeatEvery  time.Duration
	DesiredEvery    time.Duration
	AllowHTTP       bool
}

type agentFile struct {
	ServerURL          string `yaml:"server_url"`
	EnrollmentTokenEnv string `yaml:"enrollment_token_env"`
	StatePath          string `yaml:"state_path"`
	NodeName           string `yaml:"node_name"`
	AdapterType        string `yaml:"adapter_type"`
	CoreName           string `yaml:"core_name"`
	ServiceUnit        string `yaml:"service_unit"`
	HeartbeatEvery     string `yaml:"heartbeat_every"`
	DesiredEvery       string `yaml:"desired_every"`
	AllowHTTP          bool   `yaml:"allow_insecure_http"`
}

func LoadAgent(path string) (Agent, error) {
	if path == "" {
		return Agent{}, errors.New("agent config path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Agent{}, fmt.Errorf("read agent config: %w", err)
	}
	file := agentFile{
		EnrollmentTokenEnv: "HYFLEET_ENROLLMENT_TOKEN",
		StatePath:          "../var/agent-state.json",
		HeartbeatEvery:     "15s",
		DesiredEvery:       "10s",
	}
	if err := decodeYAML(data, &file); err != nil {
		return Agent{}, fmt.Errorf("parse agent config: %w", err)
	}
	serverURL, err := url.Parse(file.ServerURL)
	if err != nil || serverURL.Scheme == "" || serverURL.Host == "" {
		return Agent{}, errors.New("server_url must be an absolute URL")
	}
	if serverURL.Scheme != "https" && !(file.AllowHTTP && serverURL.Scheme == "http") {
		return Agent{}, errors.New("server_url must use https unless allow_insecure_http is explicitly enabled")
	}
	if serverURL.User != nil || (serverURL.Path != "" && serverURL.Path != "/") ||
		serverURL.RawQuery != "" || serverURL.Fragment != "" {
		return Agent{}, errors.New("server_url must be an origin without credentials, path, query, or fragment")
	}
	file.NodeName = strings.TrimSpace(file.NodeName)
	file.CoreName = strings.TrimSpace(file.CoreName)
	file.ServiceUnit = strings.TrimSpace(file.ServiceUnit)
	if file.NodeName == "" || len(file.NodeName) > 64 {
		return Agent{}, errors.New("node_name is required")
	}
	if file.CoreName == "" || len(file.CoreName) > 64 {
		return Agent{}, errors.New("core_name is required and must be at most 64 characters")
	}
	if !systemdUnitPattern.MatchString(file.ServiceUnit) || len(file.ServiceUnit) > 128 {
		return Agent{}, errors.New("service_unit must be a simple systemd unit name")
	}
	if !environmentNamePattern.MatchString(file.EnrollmentTokenEnv) {
		return Agent{}, errors.New("enrollment_token_env is not a valid environment variable name")
	}
	switch file.AdapterType {
	case "native_hysteria2", "standalone_sing_box", "s_ui":
	default:
		return Agent{}, errors.New("unsupported adapter_type")
	}
	heartbeat, err := parseDuration("heartbeat_every", file.HeartbeatEvery, 5*time.Second, 5*time.Minute)
	if err != nil {
		return Agent{}, err
	}
	desired, err := parseDuration("desired_every", file.DesiredEvery, 5*time.Second, 5*time.Minute)
	if err != nil {
		return Agent{}, err
	}
	return Agent{
		ServerURL:       serverURL.String(),
		EnrollmentToken: os.Getenv(file.EnrollmentTokenEnv),
		StatePath:       resolvePath(filepath.Dir(path), file.StatePath),
		NodeName:        file.NodeName,
		AdapterType:     file.AdapterType,
		CoreName:        file.CoreName,
		ServiceUnit:     file.ServiceUnit,
		HeartbeatEvery:  heartbeat,
		DesiredEvery:    desired,
		AllowHTTP:       file.AllowHTTP,
	}, nil
}
