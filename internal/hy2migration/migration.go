package hy2migration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	defaultTimeout       = 12 * time.Second
	stableServiceWindow  = 2 * time.Second
	maxAuthResponseBytes = 4096
	maxStatsProbeBytes   = 2 * 1024 * 1024
)

var serviceUnitPattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)

type Options struct {
	ConfigPath  string
	AuthURL     string
	StatsListen string
	StatsSecret string
	Service     string
	Timeout     time.Duration
}

type Result struct {
	Changed    bool
	BackupPath string
}

type ServiceManager interface {
	Restart(context.Context, string) error
	IsActive(context.Context, string) (bool, error)
}

type SystemdManager struct{}

func (SystemdManager) Restart(ctx context.Context, unit string) error {
	output, err := exec.CommandContext(ctx, "systemctl", "restart", unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %w: %s", unit, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (SystemdManager) IsActive(ctx context.Context, unit string) (bool, error) {
	command := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit)
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return false, nil
		}
		return false, fmt.Errorf("systemctl is-active %s: %w", unit, err)
	}
	return true, nil
}

func Apply(ctx context.Context, options Options, manager ServiceManager) (Result, error) {
	options, err := validateOptions(options)
	if err != nil {
		return Result{}, err
	}
	if manager == nil {
		return Result{}, errors.New("service manager is required")
	}
	if err := ProbeAuthEndpoint(ctx, options.AuthURL, options.Timeout); err != nil {
		return Result{}, fmt.Errorf("auth endpoint preflight failed: %w", err)
	}

	original, info, err := readRegularFile(options.ConfigPath)
	if err != nil {
		return Result{}, err
	}
	updated, err := RewriteRuntime(original, options.AuthURL, options.StatsListen, options.StatsSecret)
	if err != nil {
		return Result{}, fmt.Errorf("rewrite Hysteria config: %w", err)
	}
	if bytes.Equal(original, updated) {
		if options.StatsListen != "" {
			if err := ProbeStatsEndpoint(ctx, options.StatsListen, options.StatsSecret, options.Timeout); err != nil {
				return Result{}, fmt.Errorf("traffic stats endpoint probe failed: %w", err)
			}
		}
		return Result{Changed: false}, nil
	}

	backupPath, err := writeBackup(options.ConfigPath, original)
	if err != nil {
		return Result{}, err
	}
	if err := writeAtomic(options.ConfigPath, updated, info); err != nil {
		return Result{BackupPath: backupPath}, fmt.Errorf("install migrated config: %w", err)
	}
	applyErr := restartAndWait(ctx, manager, options.Service, options.Timeout)
	if applyErr == nil && options.StatsListen != "" {
		applyErr = ProbeStatsEndpoint(ctx, options.StatsListen, options.StatsSecret, options.Timeout)
	}
	if applyErr == nil {
		return Result{Changed: true, BackupPath: backupPath}, nil
	} else {
		migrationErr := applyErr
		if restoreErr := writeAtomic(options.ConfigPath, original, info); restoreErr != nil {
			return Result{BackupPath: backupPath}, fmt.Errorf(
				"migrated service failed (%v); automatic config restore also failed: %w",
				migrationErr, restoreErr,
			)
		}
		if restoreErr := restartAndWait(ctx, manager, options.Service, options.Timeout); restoreErr != nil {
			return Result{BackupPath: backupPath}, fmt.Errorf(
				"migrated service failed (%v); old config was restored but service restart failed: %w",
				migrationErr, restoreErr,
			)
		}
		return Result{BackupPath: backupPath}, fmt.Errorf(
			"migrated service failed and the old config was restored: %w", migrationErr,
		)
	}
}

func Rollback(
	ctx context.Context,
	configPath, backupPath, service string,
	timeout time.Duration,
	manager ServiceManager,
) (Result, error) {
	options, err := validateOptions(Options{
		ConfigPath: configPath,
		AuthURL:    "http://127.0.0.1/unused",
		Service:    service,
		Timeout:    timeout,
	})
	if err != nil {
		return Result{}, err
	}
	if manager == nil {
		return Result{}, errors.New("service manager is required")
	}
	if !filepath.IsAbs(backupPath) {
		return Result{}, errors.New("backup path must be absolute")
	}

	current, info, err := readRegularFile(options.ConfigPath)
	if err != nil {
		return Result{}, err
	}
	backup, _, err := readRegularFile(backupPath)
	if err != nil {
		return Result{}, fmt.Errorf("read rollback backup: %w", err)
	}
	if _, err := parseRootMapping(backup); err != nil {
		return Result{}, fmt.Errorf("rollback backup is not valid YAML: %w", err)
	}
	if bytes.Equal(current, backup) {
		return Result{Changed: false}, nil
	}

	safeguardPath, err := writeBackup(options.ConfigPath, current)
	if err != nil {
		return Result{}, fmt.Errorf("back up current config before rollback: %w", err)
	}
	if err := writeAtomic(options.ConfigPath, backup, info); err != nil {
		return Result{BackupPath: safeguardPath}, fmt.Errorf("install rollback config: %w", err)
	}
	if err := restartAndWait(ctx, manager, options.Service, options.Timeout); err == nil {
		return Result{Changed: true, BackupPath: safeguardPath}, nil
	} else {
		rollbackErr := err
		if restoreErr := writeAtomic(options.ConfigPath, current, info); restoreErr != nil {
			return Result{BackupPath: safeguardPath}, fmt.Errorf(
				"rollback service failed (%v); restoring the pre-rollback config also failed: %w",
				rollbackErr, restoreErr,
			)
		}
		if restoreErr := restartAndWait(ctx, manager, options.Service, options.Timeout); restoreErr != nil {
			return Result{BackupPath: safeguardPath}, fmt.Errorf(
				"rollback service failed (%v); pre-rollback config restored but service restart failed: %w",
				rollbackErr, restoreErr,
			)
		}
		return Result{BackupPath: safeguardPath}, fmt.Errorf(
			"rollback service failed and the pre-rollback config was restored: %w", rollbackErr,
		)
	}
}

func RewriteAuth(input []byte, authURL string) ([]byte, error) {
	return RewriteRuntime(input, authURL, "", "")
}

func RewriteRuntime(input []byte, authURL, statsListen, statsSecret string) ([]byte, error) {
	if err := validateAuthURL(authURL); err != nil {
		return nil, err
	}
	if (statsListen == "") != (statsSecret == "") {
		return nil, errors.New("traffic stats listen address and secret must be configured together")
	}
	if statsListen != "" {
		if err := validateStatsOptions(statsListen, statsSecret); err != nil {
			return nil, err
		}
	}
	document, err := parseRootMapping(input)
	if err != nil {
		return nil, err
	}
	root := document.Content[0]
	auth := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	auth.Content = append(auth.Content,
		scalarNode("type"), scalarNode("http"),
		scalarNode("http"), &yaml.Node{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
			Content: []*yaml.Node{
				scalarNode("url"), scalarNode(authURL),
			},
		},
	)

	setRootMapping(root, "auth", auth)
	if statsListen != "" {
		stats := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		stats.Content = append(stats.Content,
			scalarNode("listen"), scalarNode(statsListen),
			scalarNode("secret"), scalarNode(statsSecret),
		)
		setRootMapping(root, "trafficStats", stats)
	}

	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("finish YAML encoding: %w", err)
	}
	if err := validateRewrittenRuntime(output.Bytes(), authURL, statsListen, statsSecret); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func setRootMapping(root *yaml.Node, key string, value *yaml.Node) {
	for index := 0; index < len(root.Content); index += 2 {
		if root.Content[index].Value == key {
			root.Content[index+1] = value
			return
		}
	}
	root.Content = append(root.Content, scalarNode(key), value)
}

func ProbeAuthEndpoint(ctx context.Context, endpoint string, timeout time.Duration) error {
	if err := validateAuthURL(endpoint); err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	body := bytes.NewBufferString(`{"addr":"127.0.0.1:1","auth":"hyfleet-migration-probe-invalid","tx":0}`)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxAuthResponseBytes+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}
	if len(responseBody) > maxAuthResponseBytes {
		return errors.New("auth response is too large")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("auth endpoint returned HTTP %d, want 200", response.StatusCode)
	}
	var payload struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return fmt.Errorf("decode auth response: %w", err)
	}
	if payload.OK {
		return errors.New("auth endpoint unexpectedly accepted an invalid probe credential")
	}
	return nil
}

func ProbeStatsEndpoint(ctx context.Context, listen, secret string, timeout time.Duration) error {
	if err := validateStatsOptions(listen, secret); err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, "http://"+listen+"/traffic", nil,
	)
	if err != nil {
		return fmt.Errorf("create traffic stats probe: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", secret)
	response, err := (&http.Client{Timeout: timeout}).Do(request)
	if err != nil {
		return fmt.Errorf("send traffic stats probe: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxStatsProbeBytes+1))
	if err != nil {
		return fmt.Errorf("read traffic stats probe: %w", err)
	}
	if len(payload) > maxStatsProbeBytes {
		return errors.New("traffic stats probe response is too large")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("traffic stats endpoint returned HTTP %d, want 200", response.StatusCode)
	}
	var counters map[string]json.RawMessage
	if err := json.Unmarshal(payload, &counters); err != nil || counters == nil {
		return errors.New("traffic stats endpoint returned invalid JSON")
	}
	return nil
}

func validateOptions(options Options) (Options, error) {
	if !filepath.IsAbs(options.ConfigPath) {
		return Options{}, errors.New("Hysteria config path must be absolute")
	}
	if err := validateAuthURL(options.AuthURL); err != nil {
		return Options{}, err
	}
	if (options.StatsListen == "") != (options.StatsSecret == "") {
		return Options{}, errors.New("traffic stats listen address and secret must be configured together")
	}
	if options.StatsListen != "" {
		if err := validateStatsOptions(options.StatsListen, options.StatsSecret); err != nil {
			return Options{}, err
		}
	}
	if !serviceUnitPattern.MatchString(options.Service) || !strings.HasSuffix(options.Service, ".service") {
		return Options{}, errors.New("invalid systemd service unit")
	}
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	if options.Timeout < stableServiceWindow {
		return Options{}, fmt.Errorf("timeout must be at least %s", stableServiceWindow)
	}
	return options, nil
}

func validateStatsOptions(listen, secret string) error {
	host, port, err := net.SplitHostPort(listen)
	if err != nil || port == "" {
		return errors.New("traffic stats listen address must include a loopback IP and TCP port")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("traffic stats listen address must use a literal loopback IP")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("traffic stats listen port must be between 1 and 65535")
	}
	if len(secret) < 16 || len(secret) > 256 || strings.TrimSpace(secret) != secret ||
		strings.ContainsAny(secret, "\r\n\x00") {
		return errors.New("traffic stats secret must be 16 to 256 non-whitespace characters")
	}
	return nil
}

func validateAuthURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse auth URL: %w", err)
	}
	if parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("auth URL must be a plain HTTP loopback URL without credentials, query, or fragment")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("auth URL host must be a literal loopback IP")
	}
	if parsed.Path == "" || parsed.Path[0] != '/' {
		return errors.New("auth URL must include an absolute path")
	}
	return nil
}

func parseRootMapping(input []byte) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(input, &document); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("Hysteria config root must be a YAML mapping")
	}
	return &document, nil
}

func validateRewrittenAuth(input []byte, authURL string) error {
	return validateRewrittenRuntime(input, authURL, "", "")
}

func validateRewrittenRuntime(input []byte, authURL, statsListen, statsSecret string) error {
	var decoded map[string]any
	if err := yaml.Unmarshal(input, &decoded); err != nil {
		return fmt.Errorf("validate rewritten YAML: %w", err)
	}
	auth, ok := decoded["auth"].(map[string]any)
	if !ok || auth["type"] != "http" {
		return errors.New("rewritten auth section is invalid")
	}
	httpConfig, ok := auth["http"].(map[string]any)
	if !ok || httpConfig["url"] != authURL {
		return errors.New("rewritten HTTP auth URL is invalid")
	}
	if _, exists := auth["password"]; exists {
		return errors.New("rewritten auth section still contains password authentication")
	}
	if statsListen != "" {
		stats, ok := decoded["trafficStats"].(map[string]any)
		if !ok || stats["listen"] != statsListen || stats["secret"] != statsSecret {
			return errors.New("rewritten traffic stats section is invalid")
		}
	}
	return nil
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func readRegularFile(path string) ([]byte, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s must be a regular file (symlinks are not accepted)", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, info, nil
}

func writeBackup(configPath string, data []byte) (string, error) {
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	path := configPath + ".hyfleet-backup-" + stamp
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create backup %s: %w", path, err)
	}
	failed := true
	defer func() {
		_ = file.Close()
		if failed {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("sync backup: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close backup: %w", err)
	}
	if err := syncParentDirectory(path); err != nil {
		return "", err
	}
	failed = false
	return path, nil
}

func writeAtomic(path string, data []byte, info os.FileInfo) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".hyfleet-hy2-config-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := applyOwnership(temporaryPath, info); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return err
	}
	return syncParentDirectory(path)
}

func restartAndWait(
	ctx context.Context,
	manager ServiceManager,
	service string,
	timeout time.Duration,
) error {
	restartCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := manager.Restart(restartCtx, service); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	var activeSince time.Time
	for time.Now().Before(deadline) {
		active, err := manager.IsActive(restartCtx, service)
		if err != nil {
			return err
		}
		if active {
			if activeSince.IsZero() {
				activeSince = time.Now()
			}
			if time.Since(activeSince) >= stableServiceWindow {
				return nil
			}
		} else {
			activeSince = time.Time{}
		}
		select {
		case <-restartCtx.Done():
			return restartCtx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("%s did not remain active for %s", service, stableServiceWindow)
}
