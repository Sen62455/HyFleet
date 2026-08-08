package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hyfleet/hyfleet/internal/buildinfo"
	"github.com/hyfleet/hyfleet/internal/config"
	"github.com/hyfleet/hyfleet/internal/cryptoutil"
	"github.com/hyfleet/hyfleet/internal/protocol"
)

type Agent struct {
	config    config.Agent
	logger    *slog.Logger
	client    *http.Client
	collector Collector
	state     State
}

func New(cfg config.Agent, logger *slog.Logger) (*Agent, error) {
	state, err := LoadState(cfg.StatePath)
	if err != nil {
		return nil, err
	}
	if err := SaveState(cfg.StatePath, state); err != nil {
		return nil, err
	}
	return &Agent{
		config:    cfg,
		logger:    logger,
		client:    &http.Client{Timeout: 20 * time.Second},
		collector: NewCollector(),
		state:     state,
	}, nil
}

func (agent *Agent) Run(ctx context.Context) error {
	if agent.state.NodeCredential == "" {
		if agent.config.EnrollmentToken == "" {
			return errors.New("Agent is not enrolled and HYFLEET_ENROLLMENT_TOKEN is empty")
		}
		if err := agent.enrollWithBackoff(ctx); err != nil {
			return err
		}
	}
	agent.logger.Info("Agent started",
		"node_id", agent.state.NodeID,
		"installation_id", agent.state.InstallationID,
		"adapter", agent.config.AdapterType,
	)
	if err := agent.sendPendingAck(ctx); err != nil {
		agent.logger.Warn("pending desired acknowledgement failed", "error", err)
	}
	if _, err := agent.heartbeat(ctx); err != nil {
		agent.logger.Warn("initial heartbeat failed", "error", err)
	}
	if err := agent.pollDesired(ctx); err != nil {
		agent.logger.Warn("initial desired-state poll failed", "error", err)
	}
	heartbeatTimer := time.NewTimer(jitter(agent.config.HeartbeatEvery))
	desiredTimer := time.NewTimer(jitter(agent.config.DesiredEvery))
	defer heartbeatTimer.Stop()
	defer desiredTimer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeatTimer.C:
			if _, err := agent.heartbeat(ctx); err != nil {
				agent.logger.Warn("heartbeat failed", "error", err)
			}
			heartbeatTimer.Reset(jitter(agent.config.HeartbeatEvery))
		case <-desiredTimer.C:
			if err := agent.sendPendingAck(ctx); err != nil {
				agent.logger.Warn("desired acknowledgement retry failed", "error", err)
			} else if err := agent.pollDesired(ctx); err != nil {
				agent.logger.Warn("desired-state poll failed", "error", err)
			}
			desiredTimer.Reset(jitter(agent.config.DesiredEvery))
		}
	}
}

func (agent *Agent) enrollWithBackoff(ctx context.Context) error {
	delay := time.Second
	for {
		if err := agent.enroll(ctx); err == nil {
			return nil
		} else {
			agent.logger.Warn("enrollment attempt failed", "error", err, "retry_in", delay)
		}
		timer := time.NewTimer(jitter(delay))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < 2*time.Minute {
			delay *= 2
			if delay > 2*time.Minute {
				delay = 2 * time.Minute
			}
		}
	}
}

func (agent *Agent) enroll(ctx context.Context) error {
	if agent.state.PendingEnrollmentRequestID == "" {
		agent.state.PendingEnrollmentRequestID = cryptoutil.NewID()
		if err := SaveState(agent.config.StatePath, agent.state); err != nil {
			return err
		}
	}
	facts := agent.collector.Facts()
	request := protocol.EnrollRequest{
		EnrollmentToken: agent.config.EnrollmentToken,
		InstallationID:  agent.state.InstallationID,
		RequestID:       agent.state.PendingEnrollmentRequestID,
		AgentVersion:    buildinfo.Version,
		OS:              facts.OS,
		OSVersion:       facts.OSVersion,
		Architecture:    facts.Architecture,
		Capabilities:    []string{"host_metrics", "desired_state_v1", "read_only_foundation"},
		Adapter: protocol.EnrollmentAdapter{
			Type:     agent.config.AdapterType,
			CoreName: agent.config.CoreName,
		},
	}
	var result protocol.EnrollResponse
	status, err := agent.doJSON(ctx, http.MethodPost, "/agent/v1/enroll", request,
		agent.state.PendingEnrollmentRequestID, false, &result)
	if err != nil {
		return err
	}
	if status != http.StatusOK || result.Protocol != protocol.MajorVersion ||
		result.NodeID == "" || result.NodeCredential == "" {
		return fmt.Errorf("server returned invalid enrollment response (status %d)", status)
	}
	agent.state.NodeID = result.NodeID
	agent.state.NodeCredential = result.NodeCredential
	agent.state.PendingEnrollmentRequestID = ""
	return SaveState(agent.config.StatePath, agent.state)
}

func (agent *Agent) heartbeat(ctx context.Context) (int64, error) {
	metrics, err := agent.collector.Sample(ctx)
	if err != nil {
		return 0, err
	}
	request := protocol.HeartbeatRequest{
		InstallationID: agent.state.InstallationID,
		AppliedVersion: agent.state.AppliedVersion,
		Agent: protocol.AgentInfo{
			Version:  buildinfo.Version,
			Protocol: protocol.MajorVersion,
		},
		Core: protocol.CoreInfo{
			Name:    agent.config.CoreName,
			Running: agent.collector.ServiceRunning(ctx, agent.config.ServiceUnit),
		},
		Host:      metrics,
		SampledAt: time.Now().UTC(),
	}
	var result protocol.HeartbeatResponse
	status, err := agent.doJSON(ctx, http.MethodPost, "/agent/v1/heartbeat", request,
		cryptoutil.NewID(), true, &result)
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("heartbeat returned status %d", status)
	}
	return result.DesiredVersion, nil
}

func (agent *Agent) pollDesired(ctx context.Context) error {
	endpoint := "/agent/v1/desired?after=" + strconv.FormatInt(agent.state.AppliedVersion, 10)
	var result protocol.DesiredEnvelope
	status, err := agent.doJSON(ctx, http.MethodGet, endpoint, nil, cryptoutil.NewID(), true, &result)
	if err != nil {
		return err
	}
	if status == http.StatusNoContent {
		return nil
	}
	if status != http.StatusOK {
		return fmt.Errorf("desired-state poll returned status %d", status)
	}
	if result.Snapshot.NodeID != agent.state.NodeID || result.Snapshot.Adapter != agent.config.AdapterType ||
		result.Snapshot.Version <= agent.state.AppliedVersion || result.Snapshot.SchemaVersion != 1 {
		return errors.New("desired snapshot identity or version is invalid")
	}
	canonical, err := json.Marshal(result.Snapshot)
	if err != nil {
		return fmt.Errorf("encode desired snapshot for verification: %w", err)
	}
	hash := sha256.Sum256(canonical)
	encodedHash := base64.RawURLEncoding.EncodeToString(hash[:])
	if encodedHash != result.SHA256 {
		return errors.New("desired snapshot hash mismatch")
	}
	if len(result.Snapshot.Users) != 0 {
		return agent.ackFailed(ctx, result, "foundation_read_only", "Phase 1 Agent refuses user configuration")
	}
	agent.state.AppliedVersion = result.Snapshot.Version
	agent.state.AppliedSnapshotHash = result.SHA256
	agent.state.PendingAckVersion = result.Snapshot.Version
	agent.state.PendingAckHash = result.SHA256
	if err := SaveState(agent.config.StatePath, agent.state); err != nil {
		return err
	}
	return agent.sendPendingAck(ctx)
}

func (agent *Agent) ackFailed(ctx context.Context, desired protocol.DesiredEnvelope, code, message string) error {
	request := protocol.DesiredAckRequest{
		Status:       "failed",
		SnapshotHash: desired.SHA256,
		Adapter:      agent.config.AdapterType,
		ErrorCode:    code,
		Message:      message,
	}
	status, err := agent.doJSON(ctx, http.MethodPost,
		"/agent/v1/desired/"+strconv.FormatInt(desired.Snapshot.Version, 10)+"/ack",
		request, cryptoutil.NewID(), true, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return fmt.Errorf("failed acknowledgement returned status %d", status)
	}
	return nil
}

func (agent *Agent) sendPendingAck(ctx context.Context) error {
	if agent.state.PendingAckVersion == 0 {
		return nil
	}
	request := protocol.DesiredAckRequest{
		Status:       "applied",
		SnapshotHash: agent.state.PendingAckHash,
		Adapter:      agent.config.AdapterType,
	}
	status, err := agent.doJSON(ctx, http.MethodPost,
		"/agent/v1/desired/"+strconv.FormatInt(agent.state.PendingAckVersion, 10)+"/ack",
		request, cryptoutil.NewID(), true, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return fmt.Errorf("desired acknowledgement returned status %d", status)
	}
	agent.state.PendingAckVersion = 0
	agent.state.PendingAckHash = ""
	return SaveState(agent.config.StatePath, agent.state)
}

func (agent *Agent) doJSON(
	ctx context.Context,
	method, endpoint string,
	body any,
	requestID string,
	authenticated bool,
	destination any,
) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	base, err := url.Parse(agent.config.ServerURL)
	if err != nil {
		return 0, err
	}
	relative, err := url.Parse(endpoint)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, method, base.ResolveReference(relative).String(), reader)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-HyFleet-Protocol", strconv.Itoa(protocol.MajorVersion))
	request.Header.Set("X-HyFleet-Agent", buildinfo.Version)
	request.Header.Set("X-Request-ID", requestID)
	request.Header.Set("User-Agent", "hyfleet-agent/"+buildinfo.Version)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		request.Header.Set("Authorization", "Bearer "+agent.state.NodeCredential)
	}
	response, err := agent.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, 2*1024*1024)
	if response.StatusCode >= 400 {
		var apiError protocol.ErrorResponse
		if err := json.NewDecoder(limited).Decode(&apiError); err == nil && apiError.Error.Code != "" {
			return response.StatusCode, fmt.Errorf("server rejected request: %s", apiError.Error.Code)
		}
		return response.StatusCode, fmt.Errorf("server rejected request with status %d", response.StatusCode)
	}
	if destination != nil && response.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(limited).Decode(destination); err != nil {
			return response.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}
	return response.StatusCode, nil
}

func jitter(base time.Duration) time.Duration {
	if base <= 0 {
		return time.Second
	}
	spread := base / 10
	if spread <= 0 {
		return base
	}
	return base - spread + time.Duration(rand.Int64N(int64(spread*2)+1))
}
