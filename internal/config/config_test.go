package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func writeConfig(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func TestCheckedInConfigurationExamples(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	t.Setenv("HYFLEET_BOOTSTRAP_TOKEN", "test-token")
	if _, err := LoadServer(filepath.Join(repositoryRoot, "configs", "server.example.yaml")); err != nil {
		t.Fatalf("server example is invalid: %v", err)
	}
	patterns := []string{
		"agent.native-hysteria2.example.yaml",
		"agent.standalone-sing-box.example.yaml",
		"agent.s-ui.example.yaml",
	}
	for _, name := range patterns {
		if _, err := LoadAgent(filepath.Join(repositoryRoot, "configs", name)); err != nil {
			t.Fatalf("%s is invalid: %v", name, err)
		}
	}
}

func TestRepositoryYAMLParses(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	err := filepath.WalkDir(repositoryRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if entry.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var document yaml.Node
		if err := yaml.Unmarshal(data, &document); err != nil {
			t.Errorf("parse %s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository YAML: %v", err)
	}
}

func TestLoadServerRejectsUnknownFieldAndURLPath(t *testing.T) {
	t.Setenv("HYFLEET_BOOTSTRAP_TOKEN", "test-token")
	unknown := writeConfig(t, "server-unknown.yaml", "public_url: https://panel.example.com\nlisten_typo: ':8080'\n")
	if _, err := LoadServer(unknown); err == nil || !strings.Contains(err.Error(), "field listen_typo not found") {
		t.Fatalf("LoadServer(unknown) error = %v", err)
	}
	withPath := writeConfig(t, "server-path.yaml", "public_url: https://panel.example.com/hyfleet\n")
	if _, err := LoadServer(withPath); err == nil || !strings.Contains(err.Error(), "must be an origin") {
		t.Fatalf("LoadServer(path) error = %v", err)
	}
}

func TestLoadServerResolvesRelativeStatePaths(t *testing.T) {
	path := writeConfig(t, "server.yaml", `
listen: 127.0.0.1:8080
public_url: https://panel.example.com
database_path: state/server.db
master_key_file: state/master.key
`)
	cfg, err := LoadServer(path)
	if err != nil {
		t.Fatalf("LoadServer() error = %v", err)
	}
	if !filepath.IsAbs(cfg.DatabasePath) || filepath.Base(cfg.MasterKeyFile) != "master.key" || !cfg.CookieSecure {
		t.Fatalf("unexpected server config: %#v", cfg)
	}
}

func TestLoadAgentSecurityValidation(t *testing.T) {
	validBody := `
server_url: https://panel.example.com
node_name: test-node
adapter_type: native_hysteria2
core_name: hysteria
service_unit: hysteria-server.service
state_path: state.json
`
	path := writeConfig(t, "agent.yaml", validBody)
	cfg, err := LoadAgent(path)
	if err != nil {
		t.Fatalf("LoadAgent() error = %v", err)
	}
	if cfg.AdapterType != "native_hysteria2" || !filepath.IsAbs(cfg.StatePath) {
		t.Fatalf("unexpected Agent config: %#v", cfg)
	}

	for name, replacement := range map[string]string{
		"http":     strings.Replace(validBody, "https://", "http://", 1),
		"url path": strings.Replace(validBody, "panel.example.com", "panel.example.com/base", 1),
		"unit":     strings.Replace(validBody, "hysteria-server.service", "../bad service", 1),
		"unknown":  validBody + "heartbeat_typo: 10s\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, "invalid.yaml", replacement)
			if _, err := LoadAgent(path); err == nil {
				t.Fatal("LoadAgent() accepted invalid configuration")
			}
		})
	}
}
