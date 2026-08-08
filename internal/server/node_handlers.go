package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hyfleet/hyfleet/internal/cryptoutil"
	"github.com/hyfleet/hyfleet/internal/store"
)

type nodeRequest struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Region      string `json:"region"`
	AdapterType string `json:"adapter_type"`
	Enabled     *bool  `json:"enabled"`
}

type nodeResponse struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Provider            string     `json:"provider"`
	Region              string     `json:"region"`
	AdapterType         string     `json:"adapter_type"`
	Enabled             bool       `json:"enabled"`
	Status              string     `json:"status"`
	StatusReason        string     `json:"status_reason"`
	DesiredVersion      int64      `json:"desired_version"`
	AppliedVersion      int64      `json:"applied_version"`
	AgentInstallationID string     `json:"agent_installation_id,omitempty"`
	AgentVersion        string     `json:"agent_version"`
	ProtocolVersion     int        `json:"protocol_version"`
	OSName              string     `json:"os_name"`
	OSVersion           string     `json:"os_version"`
	Architecture        string     `json:"architecture"`
	CoreName            string     `json:"core_name"`
	CoreVersion         string     `json:"core_version"`
	CoreRunning         bool       `json:"core_running"`
	UptimeSeconds       int64      `json:"uptime_seconds"`
	CPUPercent          float64    `json:"cpu_percent"`
	MemoryUsedBytes     int64      `json:"memory_used_bytes"`
	MemoryTotalBytes    int64      `json:"memory_total_bytes"`
	DiskUsedBytes       int64      `json:"disk_used_bytes"`
	DiskTotalBytes      int64      `json:"disk_total_bytes"`
	NetworkRXBPS        int64      `json:"network_rx_bps"`
	NetworkTXBPS        int64      `json:"network_tx_bps"`
	Load1               float64    `json:"load_1"`
	Load5               float64    `json:"load_5"`
	Load15              float64    `json:"load_15"`
	LastSeenAt          *time.Time `json:"last_seen_at"`
	LastAppliedAt       *time.Time `json:"last_applied_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (a *App) handleListNodes(response http.ResponseWriter, request *http.Request) {
	nodes, err := a.store.ListNodes(request.Context())
	if err != nil {
		a.writeError(response, request, http.StatusInternalServerError, "nodes_read_failed", "could not read nodes")
		return
	}
	result := make([]nodeResponse, 0, len(nodes))
	now := time.Now().UTC()
	for _, node := range nodes {
		result = append(result, a.presentNode(node, now))
	}
	writeJSON(response, http.StatusOK, map[string]any{"nodes": result})
}

func (a *App) handleGetNode(response http.ResponseWriter, request *http.Request) {
	node, err := a.store.GetNode(request.Context(), chi.URLParam(request, "nodeID"))
	if errors.Is(err, store.ErrNotFound) {
		a.writeError(response, request, http.StatusNotFound, "node_not_found", "node not found")
		return
	}
	if err != nil {
		a.writeError(response, request, http.StatusInternalServerError, "node_read_failed", "could not read node")
		return
	}
	writeJSON(response, http.StatusOK, a.presentNode(node, time.Now().UTC()))
}

func (a *App) handleCreateNode(response http.ResponseWriter, request *http.Request) {
	var input nodeRequest
	if err := decodeJSON(response, request, &input, 32*1024); err != nil {
		a.writeError(response, request, http.StatusBadRequest, "invalid_request", "invalid node request")
		return
	}
	input = normalizeNodeRequest(input)
	if validationMessage := validateNodeRequest(input); validationMessage != "" {
		a.writeError(response, request, http.StatusUnprocessableEntity, "validation_failed", validationMessage)
		return
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	now := time.Now().UTC()
	node, err := a.store.CreateNode(request.Context(), store.NewNode{
		ID:          cryptoutil.NewID(),
		Name:        input.Name,
		Provider:    input.Provider,
		Region:      input.Region,
		AdapterType: input.AdapterType,
		Enabled:     enabled,
		Now:         now,
	})
	if err != nil {
		a.writeError(response, request, http.StatusConflict, "node_conflict", "a node with that name already exists")
		return
	}
	writeJSON(response, http.StatusCreated, a.presentNode(node, now))
}

func (a *App) handleUpdateNode(response http.ResponseWriter, request *http.Request) {
	var input nodeRequest
	if err := decodeJSON(response, request, &input, 32*1024); err != nil {
		a.writeError(response, request, http.StatusBadRequest, "invalid_request", "invalid node request")
		return
	}
	input = normalizeNodeRequest(input)
	if validationMessage := validateNodeRequest(input); validationMessage != "" || input.Enabled == nil {
		if validationMessage == "" {
			validationMessage = "enabled is required"
		}
		a.writeError(response, request, http.StatusUnprocessableEntity, "validation_failed", validationMessage)
		return
	}
	now := time.Now().UTC()
	node, err := a.store.UpdateNode(request.Context(), chi.URLParam(request, "nodeID"), store.UpdateNode{
		Name:        input.Name,
		Provider:    input.Provider,
		Region:      input.Region,
		AdapterType: input.AdapterType,
		Enabled:     *input.Enabled,
		Now:         now,
	})
	if errors.Is(err, store.ErrNotFound) {
		a.writeError(response, request, http.StatusNotFound, "node_not_found", "node not found")
		return
	}
	if err != nil {
		a.writeError(response, request, http.StatusConflict, "node_conflict", "a node with that name already exists")
		return
	}
	writeJSON(response, http.StatusOK, a.presentNode(node, now))
}

func (a *App) handleArchiveNode(response http.ResponseWriter, request *http.Request) {
	err := a.store.ArchiveNode(request.Context(), chi.URLParam(request, "nodeID"), time.Now().UTC())
	if errors.Is(err, store.ErrNotFound) {
		a.writeError(response, request, http.StatusNotFound, "node_not_found", "node not found")
		return
	}
	if err != nil {
		a.writeError(response, request, http.StatusInternalServerError, "node_archive_failed", "could not archive node")
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (a *App) handleEnrollmentToken(response http.ResponseWriter, request *http.Request) {
	session := sessionFromContext(request.Context())
	token, err := a.store.CreateEnrollmentToken(
		request.Context(), chi.URLParam(request, "nodeID"), session.AdminID,
		time.Now().UTC(), 10*time.Minute,
	)
	if errors.Is(err, store.ErrNotFound) {
		a.writeError(response, request, http.StatusNotFound, "node_not_found", "node not found")
		return
	}
	if err != nil {
		a.writeError(response, request, http.StatusInternalServerError, "enrollment_token_failed", "could not create enrollment token")
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{
		"node_id":          token.NodeID,
		"enrollment_token": token.Token,
		"expires_at":       token.ExpiresAt,
	})
}

func normalizeNodeRequest(input nodeRequest) nodeRequest {
	input.Name = strings.TrimSpace(input.Name)
	input.Provider = strings.TrimSpace(input.Provider)
	input.Region = strings.TrimSpace(input.Region)
	input.AdapterType = strings.TrimSpace(input.AdapterType)
	return input
}

func validateNodeRequest(input nodeRequest) string {
	if len(input.Name) < 2 || len(input.Name) > 64 {
		return "node name must be between 2 and 64 characters"
	}
	if len(input.Provider) > 64 || len(input.Region) > 64 {
		return "provider and region must be at most 64 characters"
	}
	switch input.AdapterType {
	case "native_hysteria2", "standalone_sing_box", "s_ui":
		return ""
	default:
		return "unsupported adapter type"
	}
}

func (a *App) presentNode(node store.Node, now time.Time) nodeResponse {
	status := node.Status
	if !node.Enabled {
		status = "disabled"
	} else if node.LastSeenAt != nil {
		age := now.Sub(*node.LastSeenAt)
		if age >= a.config.OfflineAfter {
			status = "offline"
		} else if age >= a.config.StaleAfter {
			status = "stale"
		}
	}
	return nodeResponse{
		ID: node.ID, Name: node.Name, Provider: node.Provider, Region: node.Region,
		AdapterType: node.AdapterType, Enabled: node.Enabled, Status: status,
		StatusReason: node.StatusReason, DesiredVersion: node.DesiredVersion,
		AppliedVersion: node.AppliedVersion, AgentInstallationID: node.AgentInstallationID,
		AgentVersion: node.AgentVersion, ProtocolVersion: node.ProtocolVersion,
		OSName: node.OSName, OSVersion: node.OSVersion, Architecture: node.Architecture,
		CoreName: node.CoreName, CoreVersion: node.CoreVersion, CoreRunning: node.CoreRunning,
		UptimeSeconds: node.UptimeSeconds, CPUPercent: node.CPUPercent,
		MemoryUsedBytes: node.MemoryUsedBytes, MemoryTotalBytes: node.MemoryTotalBytes,
		DiskUsedBytes: node.DiskUsedBytes, DiskTotalBytes: node.DiskTotalBytes,
		NetworkRXBPS: node.NetworkRXBPS, NetworkTXBPS: node.NetworkTXBPS,
		Load1: node.Load1, Load5: node.Load5, Load15: node.Load15,
		LastSeenAt: node.LastSeenAt, LastAppliedAt: node.LastAppliedAt,
		CreatedAt: node.CreatedAt, UpdatedAt: node.UpdatedAt,
	}
}
