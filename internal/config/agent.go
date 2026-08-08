package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var systemdUnitPattern = regexp.MustCompile(`^[A-Za-z0-9_.@:-]+$`)

type Agent struct {
	ServerURL           string
	EnrollmentToken     string
	StatePath           string
	NodeName            string
	AdapterType         string
	CoreName            string
	ServiceUnit         string
	HeartbeatEvery      time.Duration
	DesiredEvery        time.Duration
	AllowHTTP           bool
	AuthListen          string
	AuthPath            string
	AuthCachePath       string
	TrafficStatsURL     string
	TrafficStatsSecret  string
	TrafficDatabasePath string
	LocalDatabasePath   string
	TrafficEvery        time.Duration
	SUIAPIURL           string
	SUIToken            string
}

type agentFile struct {
	ServerURL             string `yaml:"server_url"`
	EnrollmentTokenEnv    string `yaml:"enrollment_token_env"`
	StatePath             string `yaml:"state_path"`
	NodeName              string `yaml:"node_name"`
	AdapterType           string `yaml:"adapter_type"`
	CoreName              string `yaml:"core_name"`
	ServiceUnit           string `yaml:"service_unit"`
	HeartbeatEvery        string `yaml:"heartbeat_every"`
	DesiredEvery          string `yaml:"desired_every"`
	AllowHTTP             bool   `yaml:"allow_insecure_http"`
	AuthListen            string `yaml:"auth_listen"`
	AuthPath              string `yaml:"auth_path"`
	AuthCachePath         string `yaml:"auth_cache_path"`
	TrafficStatsURL       string `yaml:"traffic_stats_url"`
	TrafficStatsSecretEnv string `yaml:"traffic_stats_secret_env"`
	TrafficDatabasePath   string `yaml:"traffic_database_path"`
	LocalDatabasePath     string `yaml:"local_database_path"`
	TrafficEvery          string `yaml:"traffic_every"`
	SUIAPIURL             string `yaml:"s_ui_api_url"`
	SUITokenEnv           string `yaml:"s_ui_token_env"`
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
		EnrollmentTokenEnv:    "HYFLEET_ENROLLMENT_TOKEN",
		StatePath:             "../var/agent-state.json",
		HeartbeatEvery:        "15s",
		DesiredEvery:          "10s",
		TrafficStatsSecretEnv: "HYFLEET_HY2_STATS_SECRET",
		TrafficEvery:          "30s",
		SUITokenEnv:           "HYFLEET_SUI_TOKEN",
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
	statePath := resolvePath(filepath.Dir(path), file.StatePath)
	if file.AdapterType == "native_hysteria2" {
		if file.AuthListen == "" {
			file.AuthListen = "127.0.0.1:18081"
		}
		if file.AuthPath == "" {
			file.AuthPath = "/hysteria/auth"
		}
		if file.AuthCachePath == "" {
			file.AuthCachePath = filepath.Join(filepath.Dir(statePath), "auth-cache.json")
		}
		if file.TrafficStatsURL == "" {
			file.TrafficStatsURL = "http://127.0.0.1:18082"
		}
		if file.TrafficDatabasePath == "" {
			file.TrafficDatabasePath = filepath.Join(filepath.Dir(statePath), "agent.db")
		}
		if file.LocalDatabasePath == "" {
			file.LocalDatabasePath = file.TrafficDatabasePath
		}
		if err := validateLoopbackListener(file.AuthListen); err != nil {
			return Agent{}, err
		}
		if !strings.HasPrefix(file.AuthPath, "/") || strings.ContainsAny(file.AuthPath, "?#") ||
			len(file.AuthPath) > 128 {
			return Agent{}, errors.New("auth_path must be an absolute HTTP path without query or fragment")
		}
		if err := validateLoopbackHTTPOrigin(file.TrafficStatsURL); err != nil {
			return Agent{}, err
		}
		if !environmentNamePattern.MatchString(file.TrafficStatsSecretEnv) {
			return Agent{}, errors.New("traffic_stats_secret_env is not a valid environment variable name")
		}
	}
	if file.AdapterType == "s_ui" {
		if file.SUIAPIURL == "" {
			file.SUIAPIURL = "http://127.0.0.1:2095/app/apiv2"
		}
		if file.LocalDatabasePath == "" {
			file.LocalDatabasePath = filepath.Join(filepath.Dir(statePath), "agent.db")
		}
		if err := validateLoopbackSUIAPIURL(file.SUIAPIURL); err != nil {
			return Agent{}, err
		}
		if !environmentNamePattern.MatchString(file.SUITokenEnv) {
			return Agent{}, errors.New("s_ui_token_env is not a valid environment variable name")
		}
	}
	heartbeat, err := parseDuration("heartbeat_every", file.HeartbeatEvery, 5*time.Second, 5*time.Minute)
	if err != nil {
		return Agent{}, err
	}
	desired, err := parseDuration("desired_every", file.DesiredEvery, 5*time.Second, 5*time.Minute)
	if err != nil {
		return Agent{}, err
	}
	traffic, err := parseDuration("traffic_every", file.TrafficEvery, 10*time.Second, 5*time.Minute)
	if err != nil {
		return Agent{}, err
	}
	return Agent{
		ServerURL:           serverURL.String(),
		EnrollmentToken:     os.Getenv(file.EnrollmentTokenEnv),
		StatePath:           statePath,
		NodeName:            file.NodeName,
		AdapterType:         file.AdapterType,
		CoreName:            file.CoreName,
		ServiceUnit:         file.ServiceUnit,
		HeartbeatEvery:      heartbeat,
		DesiredEvery:        desired,
		AllowHTTP:           file.AllowHTTP,
		AuthListen:          file.AuthListen,
		AuthPath:            file.AuthPath,
		AuthCachePath:       resolveOptionalPath(filepath.Dir(path), file.AuthCachePath),
		TrafficStatsURL:     file.TrafficStatsURL,
		TrafficStatsSecret:  os.Getenv(file.TrafficStatsSecretEnv),
		TrafficDatabasePath: resolveOptionalPath(filepath.Dir(path), file.TrafficDatabasePath),
		LocalDatabasePath:   resolveOptionalPath(filepath.Dir(path), file.LocalDatabasePath),
		TrafficEvery:        traffic,
		SUIAPIURL:           strings.TrimRight(file.SUIAPIURL, "/"),
		SUIToken:            os.Getenv(file.SUITokenEnv),
	}, nil
}

func validateLoopbackSUIAPIURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("s_ui_api_url must be a plain HTTP loopback URL")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return errors.New("s_ui_api_url must use a literal loopback IP")
	}
	_, port, err := net.SplitHostPort(parsed.Host)
	if err != nil || port == "" {
		return errors.New("s_ui_api_url must include a TCP port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("s_ui_api_url port must be between 1 and 65535")
	}
	if !strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), "/apiv2") {
		return errors.New("s_ui_api_url path must end with /apiv2")
	}
	return nil
}

func validateLoopbackHTTPOrigin(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("traffic_stats_url must be a plain HTTP loopback origin")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return errors.New("traffic_stats_url must use a literal loopback IP")
	}
	_, port, err := net.SplitHostPort(parsed.Host)
	if err != nil || port == "" {
		return errors.New("traffic_stats_url must include a TCP port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("traffic_stats_url port must be between 1 and 65535")
	}
	return nil
}

func validateLoopbackListener(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return errors.New("auth_listen must be a loopback IP and TCP port")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("auth_listen must use a literal loopback IP")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("auth_listen port must be between 1 and 65535")
	}
	return nil
}

func resolveOptionalPath(base, path string) string {
	if path == "" {
		return ""
	}
	return resolvePath(base, path)
}
