package nodeops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hyfleet/hyfleet/internal/protocol"
)

const maxConfigBackupBytes = 8 * 1024 * 1024

var helperUnitPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]{0,127}$`)

type CommandFunc func(context.Context, string, ...string) ([]byte, error)

type Helper struct {
	ServiceUnit    string
	CoreConfigPath string
	BackupDir      string
	LedgerDir      string
	RunCommand     CommandFunc
	Now            func() time.Time
}

func NewHelper(serviceUnit, coreConfigPath string) (*Helper, error) {
	if !helperUnitPattern.MatchString(serviceUnit) {
		return nil, errors.New("invalid helper service unit")
	}
	if coreConfigPath != "" {
		if !pathpkg.IsAbs(coreConfigPath) || pathpkg.Clean(coreConfigPath) != coreConfigPath ||
			(!strings.HasPrefix(coreConfigPath, "/etc/hysteria/") &&
				!strings.HasPrefix(coreConfigPath, "/etc/sing-box/")) {
			return nil, errors.New("invalid helper core config path")
		}
	}
	return &Helper{
		ServiceUnit: serviceUnit, CoreConfigPath: coreConfigPath,
		BackupDir:  "/var/lib/hyfleet-backups",
		LedgerDir:  "/var/lib/hyfleet-agent-ops",
		RunCommand: runBoundedCommand,
		Now:        func() time.Time { return time.Now().UTC() },
	}, nil
}

func (helper *Helper) Serve(ctx context.Context, reader io.Reader, writer io.Writer) error {
	var request HelperRequest
	decoder := json.NewDecoder(io.LimitReader(reader, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode helper request: %w", err)
	}
	response := helper.Handle(ctx, request)
	if err := json.NewEncoder(writer).Encode(response); err != nil {
		return fmt.Errorf("encode helper response: %w", err)
	}
	return nil
}

func (helper *Helper) Handle(ctx context.Context, request HelperRequest) HelperResponse {
	operation := request.Operation
	if cached, ok := helper.loadResult(operation); ok {
		return cached
	}
	response := HelperResponse{
		Sequence: operation.Sequence, Status: "failed", CompletedAt: helper.now(),
	}
	if err := validateHelperOperation(operation); err != nil {
		response.ErrorCode = "operation_invalid"
		response.ErrorMessage = SanitizeMessage(err.Error(), 512)
		return response
	}
	switch operation.Type {
	case "probe_core":
		response = helper.probeCore(ctx, operation)
	case "restart_core":
		response = helper.restartCore(ctx, operation)
	case "tail_core_log":
		response = helper.tailCoreLog(ctx, operation)
	case "backup_config":
		response = helper.backupConfig(operation)
	}
	response.Sequence = operation.Sequence
	response.Output = SanitizeOutput(response.Output, MaxLogLines, MaxOutputSize)
	response.ErrorMessage = SanitizeMessage(response.ErrorMessage, 512)
	if response.CompletedAt.IsZero() {
		response.CompletedAt = helper.now()
	}
	if err := helper.saveResult(operation, response); err != nil {
		return HelperResponse{
			Sequence: operation.Sequence, Status: "failed",
			ErrorCode:    "operation_result_persist_failed",
			ErrorMessage: SanitizeMessage(err.Error(), 512), CompletedAt: helper.now(),
		}
	}
	return response
}

func validateHelperOperation(operation protocol.NodeOperation) error {
	if _, err := uuid.Parse(operation.ID); err != nil {
		return errors.New("operation ID is not a UUID")
	}
	if operation.Sequence < 1 || operation.Attempt < 1 {
		return errors.New("operation sequence or attempt is invalid")
	}
	switch operation.Type {
	case "probe_core", "restart_core", "backup_config":
		if operation.MaxLines != 0 {
			return errors.New("operation does not accept max_lines")
		}
	case "tail_core_log":
		if operation.MaxLines < 1 || operation.MaxLines > MaxLogLines {
			return errors.New("log line limit is invalid")
		}
	default:
		return errors.New("operation type is unsupported")
	}
	return nil
}

func (helper *Helper) probeCore(ctx context.Context, operation protocol.NodeOperation) HelperResponse {
	output, err := helper.command(ctx, "systemctl", "is-active", helper.ServiceUnit)
	response := HelperResponse{
		Sequence:    operation.Sequence,
		Output:      SanitizeOutput(string(output), operation.MaxLines, MaxOutputSize),
		CompletedAt: helper.now(),
	}
	if err != nil || strings.TrimSpace(string(output)) != "active" {
		response.Status = "failed"
		response.ErrorCode = "core_inactive"
		response.ErrorMessage = commandErrorMessage(err, "core service is not active")
		return response
	}
	response.Status = "succeeded"
	return response
}

func (helper *Helper) tailCoreLog(ctx context.Context, operation protocol.NodeOperation) HelperResponse {
	output, err := helper.command(
		ctx, "journalctl", "-u", helper.ServiceUnit, "-n",
		fmt.Sprintf("%d", operation.MaxLines), "--no-pager", "-o", "short-iso",
	)
	response := HelperResponse{
		Sequence:    operation.Sequence,
		Output:      SanitizeOutput(string(output), operation.MaxLines, MaxOutputSize),
		CompletedAt: helper.now(),
	}
	if err != nil {
		response.Status = "failed"
		response.ErrorCode = "core_log_failed"
		response.ErrorMessage = commandErrorMessage(err, "could not read core log")
		return response
	}
	response.Status = "succeeded"
	return response
}

func (helper *Helper) backupConfig(operation protocol.NodeOperation) HelperResponse {
	response := HelperResponse{Sequence: operation.Sequence, CompletedAt: helper.now()}
	if helper.CoreConfigPath == "" {
		response.Status = "failed"
		response.ErrorCode = "core_config_not_configured"
		response.ErrorMessage = "core_config_path is not configured for this adapter"
		return response
	}
	backup, err := helper.createBackup(operation.ID)
	if err != nil {
		response.Status = "failed"
		response.ErrorCode = "config_backup_failed"
		response.ErrorMessage = err.Error()
		return response
	}
	response.Status = "succeeded"
	response.Backup = backup
	response.Output = "configuration backup created"
	return response
}

func (helper *Helper) restartCore(ctx context.Context, operation protocol.NodeOperation) HelperResponse {
	response := HelperResponse{Sequence: operation.Sequence, CompletedAt: helper.now()}
	rollbackSource, _ := helper.latestBackup("")
	var preRestartBackup *protocol.Backup
	if helper.CoreConfigPath != "" {
		backup, err := helper.createBackup(operation.ID)
		if err != nil {
			response.Status = "failed"
			response.ErrorCode = "config_backup_failed"
			response.ErrorMessage = err.Error()
			return response
		}
		preRestartBackup = backup
		response.Backup = backup
	}
	restartOutput, restartErr := helper.command(ctx, "systemctl", "restart", helper.ServiceUnit)
	activeOutput, activeErr := helper.command(ctx, "systemctl", "is-active", helper.ServiceUnit)
	response.Output = string(restartOutput) + "\n" + string(activeOutput)
	if restartErr == nil && activeErr == nil && strings.TrimSpace(string(activeOutput)) == "active" {
		response.Status = "succeeded"
		response.CompletedAt = helper.now()
		return response
	}
	if rollbackSource == "" && preRestartBackup != nil {
		rollbackSource = preRestartBackup.LocalPath
	}
	if rollbackSource != "" && helper.CoreConfigPath != "" {
		if err := helper.restoreBackup(rollbackSource); err == nil {
			_, _ = helper.command(ctx, "systemctl", "restart", helper.ServiceUnit)
			rollbackActive, rollbackErr := helper.command(ctx, "systemctl", "is-active", helper.ServiceUnit)
			if rollbackErr == nil && strings.TrimSpace(string(rollbackActive)) == "active" {
				response.RolledBack = true
			}
		}
	}
	response.Status = "failed"
	response.ErrorCode = "core_restart_failed"
	response.ErrorMessage = commandErrorMessage(errors.Join(restartErr, activeErr), "core restart failed")
	response.CompletedAt = helper.now()
	return response
}

func (helper *Helper) createBackup(operationID string) (*protocol.Backup, error) {
	if existing, ok := helper.existingOperationBackup(operationID); ok {
		return helper.backupMetadata(existing)
	}
	info, err := os.Lstat(helper.CoreConfigPath)
	if err != nil {
		return nil, fmt.Errorf("inspect core configuration: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maxConfigBackupBytes {
		return nil, errors.New("core configuration must be a bounded regular file")
	}
	source, err := os.Open(helper.CoreConfigPath)
	if err != nil {
		return nil, fmt.Errorf("open core configuration: %w", err)
	}
	defer source.Close()
	openedInfo, err := source.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, errors.New("core configuration changed while opening")
	}
	if err := os.MkdirAll(helper.BackupDir, 0o700); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Chmod(helper.BackupDir, 0o700); err != nil {
		return nil, fmt.Errorf("secure backup directory: %w", err)
	}
	name := fmt.Sprintf(
		"%d-%s-%s.bak", helper.now().UnixMilli(), operationID,
		filepath.Base(helper.CoreConfigPath),
	)
	destinationPath := filepath.Join(helper.BackupDir, name)
	temporary, err := os.CreateTemp(helper.BackupDir, ".backup-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary backup: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return nil, fmt.Errorf("secure temporary backup: %w", err)
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(source, maxConfigBackupBytes+1))
	if err != nil || written > maxConfigBackupBytes {
		_ = temporary.Close()
		return nil, errors.New("copy core configuration failed or exceeded size limit")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return nil, fmt.Errorf("sync configuration backup: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return nil, fmt.Errorf("close configuration backup: %w", err)
	}
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return nil, fmt.Errorf("publish configuration backup: %w", err)
	}
	if err := os.Chmod(destinationPath, 0o600); err != nil {
		return nil, fmt.Errorf("secure configuration backup: %w", err)
	}
	return &protocol.Backup{
		LocalPath: destinationPath, SHA256: hex.EncodeToString(hash.Sum(nil)), SizeBytes: written,
	}, nil
}

func (helper *Helper) existingOperationBackup(operationID string) (string, bool) {
	entries, err := os.ReadDir(helper.BackupDir)
	if err != nil {
		return "", false
	}
	needle := "-" + operationID + "-"
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 && strings.Contains(entry.Name(), needle) &&
			strings.HasSuffix(entry.Name(), "-"+filepath.Base(helper.CoreConfigPath)+".bak") {
			return filepath.Join(helper.BackupDir, entry.Name()), true
		}
	}
	return "", false
}

func (helper *Helper) backupMetadata(backupPath string) (*protocol.Backup, error) {
	info, err := os.Lstat(backupPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Size() > maxConfigBackupBytes {
		return nil, errors.New("existing configuration backup is invalid")
	}
	file, err := os.Open(backupPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, maxConfigBackupBytes+1))
	if err != nil || written > maxConfigBackupBytes {
		return nil, errors.New("read existing configuration backup failed")
	}
	return &protocol.Backup{
		LocalPath: backupPath, SHA256: hex.EncodeToString(hash.Sum(nil)), SizeBytes: written,
	}, nil
}

func (helper *Helper) latestBackup(exclude string) (string, error) {
	entries, err := os.ReadDir(helper.BackupDir)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	type candidate struct {
		path     string
		modified time.Time
	}
	candidates := make([]candidate, 0)
	for _, entry := range entries {
		candidatePath := filepath.Join(helper.BackupDir, entry.Name())
		if candidatePath == exclude || entry.Type()&os.ModeSymlink != 0 ||
			!strings.HasSuffix(entry.Name(), "-"+filepath.Base(helper.CoreConfigPath)+".bak") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		candidates = append(candidates, candidate{path: candidatePath, modified: info.ModTime()})
	}
	sort.Slice(candidates, func(left, right int) bool {
		return candidates[left].modified.After(candidates[right].modified)
	})
	if len(candidates) == 0 {
		return "", nil
	}
	return candidates[0].path, nil
}

func (helper *Helper) restoreBackup(backupPath string) error {
	backupInfo, err := os.Lstat(backupPath)
	if err != nil || !backupInfo.Mode().IsRegular() || backupInfo.Mode()&os.ModeSymlink != 0 ||
		!strings.HasPrefix(filepath.Clean(backupPath), filepath.Clean(helper.BackupDir)+string(os.PathSeparator)) {
		return errors.New("rollback backup is invalid")
	}
	destinationInfo, err := os.Lstat(helper.CoreConfigPath)
	if err != nil || !destinationInfo.Mode().IsRegular() || destinationInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("rollback destination is invalid")
	}
	source, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer source.Close()
	temporary, err := os.CreateTemp(filepath.Dir(helper.CoreConfigPath), ".hyfleet-rollback-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(destinationInfo.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	written, err := io.Copy(temporary, io.LimitReader(source, maxConfigBackupBytes+1))
	if err != nil || written > maxConfigBackupBytes {
		_ = temporary.Close()
		return errors.New("rollback backup exceeds size limit or could not be copied")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceHelperFile(temporaryPath, helper.CoreConfigPath)
}

func (helper *Helper) loadResult(operation protocol.NodeOperation) (HelperResponse, bool) {
	if _, err := uuid.Parse(operation.ID); err != nil {
		return HelperResponse{}, false
	}
	path := filepath.Join(helper.LedgerDir, operation.ID+".json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 64*1024 {
		return HelperResponse{}, false
	}
	file, err := os.Open(path)
	if err != nil {
		return HelperResponse{}, false
	}
	defer file.Close()
	var response HelperResponse
	if err := json.NewDecoder(io.LimitReader(file, 64*1024)).Decode(&response); err != nil ||
		response.Sequence != operation.Sequence ||
		(response.Status != "succeeded" && response.Status != "failed") {
		return HelperResponse{}, false
	}
	return response, true
}

func (helper *Helper) saveResult(operation protocol.NodeOperation, response HelperResponse) error {
	if err := os.MkdirAll(helper.LedgerDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(helper.LedgerDir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(helper.LedgerDir, ".result-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := json.NewEncoder(temporary).Encode(response); err != nil {
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
	return os.Rename(temporaryPath, filepath.Join(helper.LedgerDir, operation.ID+".json"))
}

func (helper *Helper) command(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	runner := helper.RunCommand
	if runner == nil {
		runner = runBoundedCommand
	}
	return runner(ctx, name, arguments...)
}

func (helper *Helper) now() time.Time {
	if helper.Now == nil {
		return time.Now().UTC()
	}
	return helper.Now().UTC()
}

func runBoundedCommand(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, arguments...)
	command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C", "LC_ALL=C"}
	output, err := command.CombinedOutput()
	return []byte(SanitizeOutput(string(output), MaxLogLines, MaxOutputSize)), err
}

func commandErrorMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	return fallback + ": " + err.Error()
}
