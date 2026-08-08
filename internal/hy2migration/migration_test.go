package hy2migration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

const sampleConfig = `# server comment
listen: :443
tls:
  cert: /etc/hysteria/cert.pem
  key: /etc/hysteria/key.pem
auth:
  type: password
  password: old-secret
masquerade:
  type: string
  string:
    content: hello
`

func TestRewriteAuthPreservesConfigAndRemovesPassword(t *testing.T) {
	endpoint := "http://127.0.0.1:18081/hysteria/auth"
	output, err := RewriteAuth([]byte(sampleConfig), endpoint)
	if err != nil {
		t.Fatalf("RewriteAuth() error = %v", err)
	}
	text := string(output)
	if strings.Contains(text, "old-secret") || !strings.Contains(text, "# server comment") {
		t.Fatalf("unexpected rewritten YAML:\n%s", text)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode rewritten YAML: %v", err)
	}
	if decoded["listen"] != ":443" || decoded["masquerade"] == nil || decoded["tls"] == nil {
		t.Fatalf("unrelated config was not preserved: %#v", decoded)
	}
	auth := decoded["auth"].(map[string]any)
	if auth["type"] != "http" || auth["http"].(map[string]any)["url"] != endpoint {
		t.Fatalf("unexpected auth config: %#v", auth)
	}
}

func TestRewriteAuthRejectsRemoteEndpoint(t *testing.T) {
	if _, err := RewriteAuth([]byte(sampleConfig), "http://192.0.2.10/auth"); err == nil {
		t.Fatal("RewriteAuth() accepted a non-loopback endpoint")
	}
}

func TestRewriteRuntimeAddsLoopbackTrafficStats(t *testing.T) {
	endpoint := "http://127.0.0.1:18081/hysteria/auth"
	output, err := RewriteRuntime([]byte(sampleConfig), endpoint, "127.0.0.1:18082", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("RewriteRuntime() error = %v", err)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode rewritten runtime YAML: %v", err)
	}
	stats, ok := decoded["trafficStats"].(map[string]any)
	if !ok || stats["listen"] != "127.0.0.1:18082" ||
		stats["secret"] != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected trafficStats config: %#v", decoded["trafficStats"])
	}
	if strings.Contains(string(output), "old-secret") {
		t.Fatal("old password remained in rewritten runtime config")
	}
}

func TestProbeStatsEndpointRequiresSecretAndJSON(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/traffic" || request.Header.Get("Authorization") != secret {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = response.Write([]byte(`{}`))
	}))
	defer server.Close()
	listen := strings.TrimPrefix(server.URL, "http://")
	if err := ProbeStatsEndpoint(context.Background(), listen, secret, time.Second); err != nil {
		t.Fatalf("ProbeStatsEndpoint() error = %v", err)
	}
	if err := ProbeStatsEndpoint(context.Background(), listen, secret+"bad", time.Second); err == nil {
		t.Fatal("ProbeStatsEndpoint() accepted the wrong secret")
	}
}

func TestApplyMigratesAfterProbeAndCreatesPrivateBackup(t *testing.T) {
	server := validProbeServer(t)
	defer server.Close()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(sampleConfig), 0o640); err != nil {
		t.Fatal(err)
	}
	manager := &fakeServiceManager{active: true}
	result, err := Apply(context.Background(), Options{
		ConfigPath: configPath,
		AuthURL:    server.URL + "/hysteria/auth",
		Service:    "hysteria-server.service",
		Timeout:    3 * time.Second,
	}, manager)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !result.Changed || result.BackupPath == "" || manager.restarts != 1 {
		t.Fatalf("unexpected result = %#v, restarts = %d", result, manager.restarts)
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil || string(backup) != sampleConfig {
		t.Fatalf("backup = %q, error = %v", backup, err)
	}
	info, err := os.Stat(result.BackupPath)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("backup mode = %v, error = %v", info.Mode().Perm(), err)
	}
	migrated, _ := os.ReadFile(configPath)
	if strings.Contains(string(migrated), "old-secret") || !strings.Contains(string(migrated), "type: http") {
		t.Fatalf("config was not migrated:\n%s", migrated)
	}
}

func TestApplyRestoresOriginalWhenRestartFails(t *testing.T) {
	server := validProbeServer(t)
	defer server.Close()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(sampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := &fakeServiceManager{active: true, restartErrors: []error{fmt.Errorf("new config failed"), nil}}
	result, err := Apply(context.Background(), Options{
		ConfigPath: configPath,
		AuthURL:    server.URL + "/hysteria/auth",
		Service:    "hysteria-server.service",
		Timeout:    3 * time.Second,
	}, manager)
	if err == nil || !strings.Contains(err.Error(), "old config was restored") {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.BackupPath == "" || manager.restarts != 2 {
		t.Fatalf("unexpected result = %#v, restarts = %d", result, manager.restarts)
	}
	restored, readErr := os.ReadFile(configPath)
	if readErr != nil || string(restored) != sampleConfig {
		t.Fatalf("original config was not restored: %v\n%s", readErr, restored)
	}
}

func TestProbeRequiresDeniedHTTP200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`{"ok":false}`))
	}))
	defer server.Close()
	if err := ProbeAuthEndpoint(context.Background(), server.URL+"/auth", time.Second); err == nil {
		t.Fatal("ProbeAuthEndpoint() accepted a non-200 response")
	}
}

func validProbeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/hysteria/auth" {
			t.Errorf("unexpected probe request: %s %s", request.Method, request.URL.Path)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"ok":false}`))
	}))
}

type fakeServiceManager struct {
	active        bool
	restarts      int
	restartErrors []error
}

func (manager *fakeServiceManager) Restart(_ context.Context, _ string) error {
	manager.restarts++
	if len(manager.restartErrors) == 0 {
		return nil
	}
	errorValue := manager.restartErrors[0]
	manager.restartErrors = manager.restartErrors[1:]
	return errorValue
}

func (manager *fakeServiceManager) IsActive(_ context.Context, _ string) (bool, error) {
	return manager.active, nil
}
