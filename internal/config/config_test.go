package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		"agent.vless-reality.example.yaml",
	}
	for _, name := range patterns {
		cfg, err := LoadAgent(filepath.Join(repositoryRoot, "configs", name))
		if err != nil {
			t.Fatalf("%s is invalid: %v", name, err)
		}
		if name == "agent.vless-reality.example.yaml" &&
			(cfg.OperationsStateDir != "/var/lib/hyfleet-agent-ops" ||
				cfg.BackupDir != "/var/lib/hyfleet-backups" ||
				cfg.OperationsSocketPath != "/run/hyfleet-agent-ops.sock" ||
				!strings.HasSuffix(filepath.ToSlash(cfg.StatePath), "/var/lib/hyfleet-agent/agent-state.json") ||
				!strings.HasSuffix(filepath.ToSlash(cfg.LocalDatabasePath), "/var/lib/hyfleet-agent/agent.db")) {
			t.Fatalf("%s does not match the standard systemd paths: %#v", name, cfg)
		}
	}
}

func TestLoadVLESSRealityAgentUsesBoundedLocalPaths(t *testing.T) {
	validBody := `
server_url: https://panel.example.com
node_name: reality-node
adapter_type: sing_box_vless_reality
core_name: sing-box
service_unit: hyfleet-sing-box-reality.service
state_path: state.json
`
	cfg, err := LoadAgent(writeConfig(t, "reality.yaml", validBody))
	if err != nil {
		t.Fatalf("LoadAgent() error = %v", err)
	}
	if cfg.CoreConfigPath != "/etc/sing-box/hyfleet-reality.json" ||
		cfg.SingBoxBinaryPath != "/usr/bin/sing-box" ||
		cfg.RealityIdentityPath != "/var/lib/hyfleet-agent-ops/reality-hyfleet-sing-box-reality.json" ||
		cfg.OperationsStateDir != "/var/lib/hyfleet-agent-ops" ||
		cfg.BackupDir != "/var/lib/hyfleet-backups" {
		t.Fatalf("unexpected Reality config: %#v", cfg)
	}
	for name, addition := range map[string]string{
		"binary":               "sing_box_binary_path: /tmp/sing-box\n",
		"identity":             "reality_identity_path: /etc/sing-box/private.json\n",
		"core":                 "core_config_path: /etc/hysteria/config.yaml\n",
		"relative state dir":   "operations_state_dir: var/lib/hyfleet-agent-ops-lab\n",
		"unclean state dir":    "operations_state_dir: /var/lib/hyfleet-agent-ops/../hyfleet-agent-ops-lab\n",
		"nested state dir":     "operations_state_dir: /var/lib/hyfleet-agent-ops/lab\n",
		"other lab state dir":  "operations_state_dir: /var/lib/hyfleet-agent-ops-test-lab\n",
		"relative backup dir":  "backup_dir: var/lib/hyfleet-backups-lab\n",
		"unclean backup dir":   "backup_dir: /var/lib/hyfleet-backups/../hyfleet-backups-lab\n",
		"nested backup dir":    "backup_dir: /var/lib/hyfleet-backups/lab\n",
		"other lab backup dir": "backup_dir: /var/lib/hyfleet-backups-test-lab\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadAgent(writeConfig(t, "invalid-reality.yaml", validBody+addition)); err == nil {
				t.Fatal("LoadAgent() accepted an unbounded Reality path")
			}
		})
	}
}

func TestLoadVLESSRealityAgentRejectsUnsupportedParallelLabPaths(t *testing.T) {
	body := `
server_url: https://panel.example.com
node_name: reality-lab
adapter_type: sing_box_vless_reality
core_name: sing-box
service_unit: hyfleet-sing-box-reality-lab.service
core_config_path: /etc/sing-box/hyfleet-reality-lab.json
state_path: /var/lib/hyfleet-agent-lab/agent-state.json
operations_state_dir: /var/lib/hyfleet-agent-ops-lab
backup_dir: /var/lib/hyfleet-backups-lab
`
	if _, err := LoadAgent(writeConfig(t, "reality-lab.yaml", body)); err == nil {
		t.Fatal("LoadAgent() accepted a parallel lab layout without packaged systemd support")
	}
}

func TestLoadVLESSRealityAgentRejectsMixedOrUnmanagedDeploymentTuple(t *testing.T) {
	validBody := `
server_url: https://panel.example.com
node_name: reality-node
adapter_type: sing_box_vless_reality
core_name: sing-box
service_unit: hyfleet-sing-box-reality.service
core_config_path: /etc/sing-box/hyfleet-reality.json
operations_state_dir: /var/lib/hyfleet-agent-ops
backup_dir: /var/lib/hyfleet-backups
reality_identity_path: /var/lib/hyfleet-agent-ops/reality-hyfleet-sing-box-reality.json
state_path: state.json
`
	for name, replacement := range map[string]string{
		"other unit":      strings.Replace(validBody, "hyfleet-sing-box-reality.service", "sing-box.service", 1),
		"other config":    strings.Replace(validBody, "/etc/sing-box/hyfleet-reality.json", "/etc/sing-box/config.json", 1),
		"other identity":  strings.Replace(validBody, "reality-hyfleet-sing-box-reality.json", "reality-other.json", 1),
		"lab unit only":   strings.Replace(validBody, "hyfleet-sing-box-reality.service", "hyfleet-sing-box-reality-lab.service", 1),
		"lab config only": strings.Replace(validBody, "hyfleet-reality.json", "hyfleet-reality-lab.json", 1),
		"lab state only":  strings.Replace(validBody, "/var/lib/hyfleet-agent-ops\n", "/var/lib/hyfleet-agent-ops-lab\n", 1),
		"lab backup only": strings.Replace(validBody, "/var/lib/hyfleet-backups\n", "/var/lib/hyfleet-backups-lab\n", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadAgent(writeConfig(t, "invalid-tuple.yaml", replacement)); err == nil {
				t.Fatal("LoadAgent() accepted a mixed or unmanaged Reality deployment tuple")
			}
		})
	}
}

func TestLoadAgentRejectsRealityHelperPathsForOtherAdapters(t *testing.T) {
	body := `
server_url: https://panel.example.com
node_name: native-node
adapter_type: native_hysteria2
core_name: hysteria
service_unit: hysteria-server.service
state_path: state.json
`
	for name, addition := range map[string]string{
		"operations state": "operations_state_dir: /var/lib/hyfleet-agent-ops\n",
		"backup":           "backup_dir: /var/lib/hyfleet-backups\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadAgent(writeConfig(t, "invalid-native.yaml", body+addition)); err == nil {
				t.Fatal("LoadAgent() accepted a Reality helper path for another adapter")
			}
		})
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
	if cfg.TelemetryEvery != time.Minute {
		t.Fatalf("TelemetryEvery = %s, want 1m", cfg.TelemetryEvery)
	}

	for name, replacement := range map[string]string{
		"http":        strings.Replace(validBody, "https://", "http://", 1),
		"url path":    strings.Replace(validBody, "panel.example.com", "panel.example.com/base", 1),
		"unit":        strings.Replace(validBody, "hysteria-server.service", "../bad service", 1),
		"unit option": strings.Replace(validBody, "hysteria-server.service", "--help", 1),
		"config path": strings.Replace(validBody, "state_path: state.json", "state_path: state.json\ncore_config_path: /etc/other/config.yaml", 1),
		"unknown":     validBody + "heartbeat_typo: 10s\n",
		"telemetry":   validBody + "telemetry_every: 5s\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, "invalid.yaml", replacement)
			if _, err := LoadAgent(path); err == nil {
				t.Fatal("LoadAgent() accepted invalid configuration")
			}
		})
	}
}

func TestLoadSUIAgentRequiresLoopbackAPI(t *testing.T) {
	validBody := `
server_url: https://panel.example.com
node_name: sui-node
adapter_type: s_ui
core_name: sing-box
service_unit: s-ui.service
state_path: state.json
s_ui_api_url: http://127.0.0.1:2095/app/apiv2
local_database_path: agent.db
`
	t.Setenv("HYFLEET_SUI_TOKEN", "local-only-token")
	cfg, err := LoadAgent(writeConfig(t, "sui.yaml", validBody))
	if err != nil {
		t.Fatalf("LoadAgent() error = %v", err)
	}
	if cfg.SUIAPIURL != "http://127.0.0.1:2095/app/apiv2" ||
		cfg.SUIToken != "local-only-token" || !filepath.IsAbs(cfg.LocalDatabasePath) {
		t.Fatalf("unexpected S-UI config: %#v", cfg)
	}
	for name, replacement := range map[string]string{
		"public host": strings.Replace(validBody, "127.0.0.1", "panel.example.com", 1),
		"https":       strings.Replace(validBody, "http://", "https://", 1),
		"wrong path":  strings.Replace(validBody, "/app/apiv2", "/app/api", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadAgent(writeConfig(t, "invalid-sui.yaml", replacement)); err == nil {
				t.Fatal("LoadAgent() accepted a non-loopback S-UI API")
			}
		})
	}
}
