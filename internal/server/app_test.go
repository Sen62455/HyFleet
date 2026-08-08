package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hyfleet/hyfleet/internal/config"
	"github.com/hyfleet/hyfleet/internal/cryptoutil"
	"github.com/hyfleet/hyfleet/internal/protocol"
	"github.com/hyfleet/hyfleet/internal/store"
)

const testPassword = "correct horse battery staple"

type testApp struct {
	handler http.Handler
	store   *store.Store
	cookie  *http.Cookie
	csrf    string
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	database, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "hyfleet.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	cfg := config.Server{
		PublicURL:          "http://hyfleet.test",
		BootstrapToken:     "test-bootstrap-token-with-enough-entropy",
		SessionLifetime:    12 * time.Hour,
		SessionIdleTimeout: 30 * time.Minute,
		StaleAfter:         45 * time.Second,
		OfflineAfter:       90 * time.Second,
	}
	application, err := New(cfg, database, bytes.Repeat([]byte{0x42}, 32), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler, err := application.Handler()
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	return &testApp{handler: handler, store: database}
}

func (app *testApp) request(t *testing.T, method, path string, body any, csrf, origin string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, "http://hyfleet.test"+path, reader)
	request.RemoteAddr = "192.0.2.20:40200"
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if app.cookie != nil {
		request.AddCookie(app.cookie)
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	return response
}

func (app *testApp) bootstrap(t *testing.T) {
	t.Helper()
	response := app.request(t, http.MethodPost, "/api/v1/setup/bootstrap", map[string]any{
		"bootstrap_token": "test-bootstrap-token-with-enough-entropy",
		"username":        "admin",
		"password":        testPassword,
	}, "", "http://hyfleet.test")
	requireStatus(t, response, http.StatusOK)
	var session sessionResponse
	decodeResponse(t, response, &session)
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("bootstrap cookies = %d, want 1", len(cookies))
	}
	app.cookie = cookies[0]
	app.csrf = session.CSRFToken
}

func requireStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, want, response.Body.String())
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, response.Body.String())
	}
}

func TestAdminAndNodeLifecycle(t *testing.T) {
	app := newTestApp(t)
	status := app.request(t, http.MethodGet, "/api/v1/setup/status", nil, "", "")
	requireStatus(t, status, http.StatusOK)
	var setup map[string]bool
	decodeResponse(t, status, &setup)
	if !setup["setup_required"] || !setup["bootstrap_token_configured"] {
		t.Fatalf("unexpected setup status: %#v", setup)
	}

	rejected := app.request(t, http.MethodPost, "/api/v1/setup/bootstrap", map[string]any{
		"bootstrap_token": "test-bootstrap-token-with-enough-entropy",
		"username":        "admin", "password": testPassword,
	}, "", "https://attacker.example")
	requireStatus(t, rejected, http.StatusForbidden)

	app.bootstrap(t)
	if !app.cookie.HttpOnly || app.cookie.SameSite != http.SameSiteStrictMode || app.cookie.Secure {
		t.Fatalf("unexpected session cookie attributes: %#v", app.cookie)
	}
	second := app.request(t, http.MethodPost, "/api/v1/setup/bootstrap", map[string]any{
		"bootstrap_token": "test-bootstrap-token-with-enough-entropy",
		"username":        "other", "password": testPassword,
	}, "", "")
	requireStatus(t, second, http.StatusConflict)

	withoutCSRF := app.request(t, http.MethodPost, "/api/v1/nodes", map[string]any{
		"name": "LisaHost", "provider": "Lisa", "region": "US",
		"adapter_type": "native_hysteria2",
	}, "", "")
	requireStatus(t, withoutCSRF, http.StatusForbidden)

	created := app.request(t, http.MethodPost, "/api/v1/nodes", map[string]any{
		"name": "LisaHost", "provider": "Lisa", "region": "US",
		"adapter_type": "native_hysteria2",
	}, app.csrf, "http://hyfleet.test")
	requireStatus(t, created, http.StatusCreated)
	var node nodeResponse
	decodeResponse(t, created, &node)
	if node.Name != "LisaHost" || node.Status != "pending" || node.DesiredVersion != 1 || !node.Enabled {
		t.Fatalf("unexpected node: %#v", node)
	}

	duplicate := app.request(t, http.MethodPost, "/api/v1/nodes", map[string]any{
		"name": "lisahost", "provider": "Other", "region": "US",
		"adapter_type": "native_hysteria2",
	}, app.csrf, "")
	requireStatus(t, duplicate, http.StatusConflict)

	updated := app.request(t, http.MethodPut, "/api/v1/nodes/"+node.ID, map[string]any{
		"name": "LisaHost", "provider": "Lisa", "region": "Los Angeles",
		"adapter_type": "native_hysteria2", "enabled": false,
	}, app.csrf, "")
	requireStatus(t, updated, http.StatusOK)
	decodeResponse(t, updated, &node)
	if node.Enabled || node.Status != "disabled" || node.DesiredVersion != 2 {
		t.Fatalf("unexpected updated node: %#v", node)
	}

	token := app.request(t, http.MethodPost, "/api/v1/nodes/"+node.ID+"/enrollment-token", map[string]any{}, app.csrf, "")
	requireStatus(t, token, http.StatusCreated)
	var enrollment map[string]any
	decodeResponse(t, token, &enrollment)
	if enrollment["enrollment_token"] == "" {
		t.Fatal("enrollment token is empty")
	}

	archived := app.request(t, http.MethodDelete, "/api/v1/nodes/"+node.ID, nil, app.csrf, "")
	requireStatus(t, archived, http.StatusNoContent)
	listed := app.request(t, http.MethodGet, "/api/v1/nodes", nil, "", "")
	requireStatus(t, listed, http.StatusOK)
	var list struct {
		Nodes []nodeResponse `json:"nodes"`
	}
	decodeResponse(t, listed, &list)
	if len(list.Nodes) != 0 {
		t.Fatalf("nodes after archive = %d, want 0", len(list.Nodes))
	}
	recreated := app.request(t, http.MethodPost, "/api/v1/nodes", map[string]any{
		"name": "lisahost", "provider": "Replacement", "region": "Hong Kong",
		"adapter_type": "s_ui",
	}, app.csrf, "")
	requireStatus(t, recreated, http.StatusCreated)
	var replacement nodeResponse
	decodeResponse(t, recreated, &replacement)
	if replacement.ID == node.ID || replacement.Name != "lisahost" || replacement.AdapterType != "s_ui" {
		t.Fatalf("unexpected replacement node: %#v", replacement)
	}

	logout := app.request(t, http.MethodPost, "/api/v1/auth/logout", map[string]any{}, app.csrf, "")
	requireStatus(t, logout, http.StatusNoContent)
	session := app.request(t, http.MethodGet, "/api/v1/auth/session", nil, "", "")
	requireStatus(t, session, http.StatusUnauthorized)
	login := app.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "ADMIN", "password": testPassword,
	}, "", "http://hyfleet.test")
	requireStatus(t, login, http.StatusOK)
}

func TestAgentEnrollmentHeartbeatAndDesiredState(t *testing.T) {
	app := newTestApp(t)
	app.bootstrap(t)
	created := app.request(t, http.MethodPost, "/api/v1/nodes", map[string]any{
		"name": "test-node", "provider": "local", "region": "test",
		"adapter_type": "native_hysteria2",
	}, app.csrf, "")
	requireStatus(t, created, http.StatusCreated)
	var node nodeResponse
	decodeResponse(t, created, &node)
	tokenResponse := app.request(t, http.MethodPost, "/api/v1/nodes/"+node.ID+"/enrollment-token", map[string]any{}, app.csrf, "")
	requireStatus(t, tokenResponse, http.StatusCreated)
	var token struct {
		EnrollmentToken string `json:"enrollment_token"`
	}
	decodeResponse(t, tokenResponse, &token)

	installationID := cryptoutil.NewID()
	requestID := cryptoutil.NewID()
	enrollBody := protocol.EnrollRequest{
		EnrollmentToken: token.EnrollmentToken,
		InstallationID:  installationID,
		RequestID:       requestID,
		AgentVersion:    "v0.1.0-test", OS: "linux", OSVersion: "24.04", Architecture: "amd64",
		Capabilities: []string{"host_metrics", "read_only_foundation"},
		Adapter:      protocol.EnrollmentAdapter{Type: "native_hysteria2", CoreName: "hysteria"},
	}
	enrolled := agentRequest(t, app.handler, http.MethodPost, "/agent/v1/enroll", enrollBody, "", requestID)
	requireStatus(t, enrolled, http.StatusOK)
	var credentials protocol.EnrollResponse
	decodeResponse(t, enrolled, &credentials)
	if credentials.NodeID != node.ID || credentials.NodeCredential == "" || credentials.Protocol != protocol.MajorVersion {
		t.Fatalf("unexpected enrollment: %#v", credentials)
	}

	replay := agentRequest(t, app.handler, http.MethodPost, "/agent/v1/enroll", enrollBody, "", requestID)
	requireStatus(t, replay, http.StatusOK)
	var replayed protocol.EnrollResponse
	decodeResponse(t, replay, &replayed)
	if replayed.NodeCredential != credentials.NodeCredential {
		t.Fatal("enrollment retry did not replay the original credential")
	}

	conflictingID := cryptoutil.NewID()
	conflicting := enrollBody
	conflicting.RequestID = conflictingID
	conflict := agentRequest(t, app.handler, http.MethodPost, "/agent/v1/enroll", conflicting, "", conflictingID)
	requireStatus(t, conflict, http.StatusConflict)

	now := time.Now().UTC().Truncate(time.Second)
	heartbeat := protocol.HeartbeatRequest{
		InstallationID: installationID,
		AppliedVersion: 0,
		Agent:          protocol.AgentInfo{Version: "v0.1.0-test", Protocol: protocol.MajorVersion},
		Core:           protocol.CoreInfo{Name: "hysteria", Version: "v2.12.0", Running: true},
		Host: protocol.HostMetrics{
			UptimeSeconds: 300, CPUPercent: 12.5,
			MemoryUsedBytes: 256 << 20, MemoryTotalBytes: 1024 << 20,
			DiskUsedBytes: 3 << 30, DiskTotalBytes: 20 << 30,
			NetworkRXBPS: 1200, NetworkTXBPS: 800, Load1: 0.1, Load5: 0.2, Load15: 0.3,
		},
		SampledAt: now,
	}
	beat := agentRequest(t, app.handler, http.MethodPost, "/agent/v1/heartbeat", heartbeat, credentials.NodeCredential, cryptoutil.NewID())
	requireStatus(t, beat, http.StatusOK)
	var beatResult protocol.HeartbeatResponse
	decodeResponse(t, beat, &beatResult)
	if beatResult.DesiredVersion != 1 {
		t.Fatalf("desired version = %d, want 1", beatResult.DesiredVersion)
	}
	storedNode, err := app.store.GetNode(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("GetNode() error = %v", err)
	}
	if storedNode.Status != "online" || !storedNode.CoreRunning || storedNode.MemoryUsedBytes != 256<<20 {
		t.Fatalf("heartbeat was not persisted: %#v", storedNode)
	}
	adapterChange := app.request(t, http.MethodPut, "/api/v1/nodes/"+node.ID, map[string]any{
		"name": "test-node", "provider": "local", "region": "test",
		"adapter_type": "s_ui", "enabled": true,
	}, app.csrf, "")
	requireStatus(t, adapterChange, http.StatusConflict)

	clearedReplay := agentRequest(t, app.handler, http.MethodPost, "/agent/v1/enroll", enrollBody, "", requestID)
	requireStatus(t, clearedReplay, http.StatusConflict)

	desired := agentRequest(t, app.handler, http.MethodGet, "/agent/v1/desired?after=0", nil, credentials.NodeCredential, cryptoutil.NewID())
	requireStatus(t, desired, http.StatusOK)
	var envelope protocol.DesiredEnvelope
	decodeResponse(t, desired, &envelope)
	if envelope.Snapshot.NodeID != node.ID || envelope.Snapshot.Version != 1 || len(envelope.Snapshot.Users) != 0 {
		t.Fatalf("unexpected desired state: %#v", envelope)
	}

	ack := protocol.DesiredAckRequest{Status: "applied", SnapshotHash: envelope.SHA256, Adapter: "native_hysteria2"}
	ackResponse := agentRequest(t, app.handler, http.MethodPost, "/agent/v1/desired/1/ack", ack, credentials.NodeCredential, cryptoutil.NewID())
	requireStatus(t, ackResponse, http.StatusNoContent)
	storedNode, err = app.store.GetNode(context.Background(), node.ID)
	if err != nil || storedNode.AppliedVersion != 1 || storedNode.LastAppliedAt == nil {
		t.Fatalf("desired acknowledgement not persisted: node=%#v err=%v", storedNode, err)
	}
	upToDate := agentRequest(t, app.handler, http.MethodGet, "/agent/v1/desired?after=1", nil, credentials.NodeCredential, cryptoutil.NewID())
	requireStatus(t, upToDate, http.StatusNoContent)

	var samples int
	if err := app.store.DB().QueryRow("SELECT COUNT(*) FROM node_metric_samples WHERE node_id = ?", node.ID).Scan(&samples); err != nil || samples != 1 {
		t.Fatalf("metric sample count = %d, err = %v", samples, err)
	}
}

func agentRequest(t *testing.T, handler http.Handler, method, path string, body any, credential, requestID string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, "http://hyfleet.test"+path, reader)
	request.RemoteAddr = "198.51.100.10:31200"
	request.Header.Set("X-HyFleet-Protocol", strconv.Itoa(protocol.MajorVersion))
	request.Header.Set("X-Request-ID", requestID)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestLoginRateLimit(t *testing.T) {
	app := newTestApp(t)
	app.bootstrap(t)
	for attempt := 1; attempt <= 9; attempt++ {
		response := app.request(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
			"username": "admin", "password": "definitely wrong",
		}, "", "")
		want := http.StatusUnauthorized
		if attempt == 9 {
			want = http.StatusTooManyRequests
		}
		requireStatus(t, response, want)
	}
}
